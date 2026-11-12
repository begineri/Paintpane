package wallpaper

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A2 phase 5: the three ways an image server can be "not reachable" (nothing listens, the
// connection is accepted and then never answered, the connection is dropped at once) each make a
// source pause after backoffFailures images, and a server that works is not held back.
func TestA2_BackoffForEachKindOfUnreachableServer(t *testing.T) {
	prevWait := downloadRetryWait
	downloadRetryWait = time.Millisecond
	defer func() { downloadRetryWait = prevWait }()

	// 1. nothing listens on the port.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	refusedURL := "http://" + l.Addr().String() + "/x.jpg"
	require.NoError(t, l.Close())

	// 2. the connection is accepted and never answered.
	var hung atomic.Int32
	release := make(chan struct{})
	hang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hung.Add(1)
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer hang.Close()
	defer close(release)

	// 3. the connection is closed without an answer.
	drop := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, _, err := w.(http.Hijacker).Hijack()
		if err == nil {
			_ = conn.Close()
		}
	}))
	defer drop.Close()

	cases := []struct {
		name   string
		url    string
		client *http.Client
	}{
		{"refused", refusedURL, &http.Client{Timeout: 2 * time.Second}},
		{"never answered", hang.URL + "/x.jpg", &http.Client{Timeout: 150 * time.Millisecond}},
		{"dropped", drop.URL + "/x.jpg", &http.Client{Timeout: 2 * time.Second}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wp := &Plugin{}
			src := &MockPacedProvider{id: "SMK"}
			dir := t.TempDir()
			start := time.Now()
			for i := 0; i < backoffFailures; i++ {
				_, err := wp.downloadMasterFile(context.Background(), c.client, c.url, filepath.Join(dir, "a.jpg"), src)
				require.Error(t, err)
				if i < backoffFailures-1 {
					assert.Zero(t, wp.downloadBackoff().pausedFor("SMK"), "image %d: not paused yet", i+1)
				}
			}
			assert.Equal(t, backoffFirstPause, wp.downloadBackoff().pausedFor("SMK").Round(time.Minute), "paused after %d images", backoffFailures)
			assert.Zero(t, wp.downloadBackoff().pausedFor("Met"), "other sources are not held")

			// While paused, an image that is already running stops after one try.
			begin := time.Now()
			_, err := wp.downloadMasterFile(context.Background(), c.client, c.url, filepath.Join(dir, "b.jpg"), src)
			require.Error(t, err)
			assert.Less(t, time.Since(begin), 1500*time.Millisecond, "a running download gives up at once while paused")
			t.Logf("%s: %d images in %v, then paused for %v", c.name, backoffFailures, time.Since(start).Round(time.Millisecond), wp.downloadBackoff().pausedFor("SMK").Round(time.Second))
		})
	}
}

// Four workers failing at the same moment count as four images, not twelve, and the pause is
// started once (the log shows one line).
func TestA2_BackoffWithFourWorkersFailingTogether(t *testing.T) {
	prevWait := downloadRetryWait
	downloadRetryWait = time.Millisecond
	defer func() { downloadRetryWait = prevWait }()
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer down.Close()

	wp := &Plugin{}
	src := &MockPacedProvider{id: "SMK"}
	dir := t.TempDir()
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = wp.downloadMasterFile(context.Background(), down.Client(), down.URL, filepath.Join(dir, "x"+string(rune('a'+i))+".jpg"), src)
		}(i)
	}
	wg.Wait()
	assert.Equal(t, backoffFirstPause, wp.downloadBackoff().pausedFor("SMK").Round(time.Minute), "the first pause is two minutes, not longer, after four images failed together")
}
