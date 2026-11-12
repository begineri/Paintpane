package smk

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testClient(hedgeAfter, headerTimeout, stall time.Duration, tries int) *http.Client {
	return &http.Client{Transport: &hedgeTransport{
		next:       &http.Transport{ResponseHeaderTimeout: headerTimeout},
		hedgeAfter: hedgeAfter,
		tries:      tries,
		stall:      stall,
	}}
}

// stuckFirst answers every request except the first n, which never get headers (the stalls seen on
// iip.smk.dk). It counts the requests it saw and how many are still open.
type stuckFirst struct {
	n       int32
	seen    atomic.Int32
	open    atomic.Int32
	release chan struct{}
}

func (s *stuckFirst) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.open.Add(1)
	defer s.open.Add(-1)
	if s.seen.Add(1) <= s.n {
		select {
		case <-s.release:
		case <-r.Context().Done():
		}
		return
	}
	_, _ = w.Write([]byte("image"))
}

// A request without headers gets a second one next to it, and the second answer is used; the
// stuck request is cancelled.
func TestHedgeTransportSendsAgainWhenStalled(t *testing.T) {
	h := &stuckFirst{n: 1, release: make(chan struct{})}
	srv := httptest.NewServer(h)
	defer srv.Close()
	defer close(h.release)

	start := time.Now()
	resp, err := testClient(50*time.Millisecond, 5*time.Second, time.Second, 3).Get(srv.URL)
	require.NoError(t, err)
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, "image", string(b))
	assert.Less(t, time.Since(start), 2*time.Second)
	assert.Equal(t, int32(2), h.seen.Load())
	assert.Eventually(t, func() bool { return h.open.Load() == 0 }, 2*time.Second, 10*time.Millisecond, "the stuck request is cancelled")
}

// A fast answer sends no extra request.
func TestHedgeTransportFastAnswerIsNotRepeated(t *testing.T) {
	h := &stuckFirst{release: make(chan struct{})}
	srv := httptest.NewServer(h)
	defer srv.Close()
	defer close(h.release)

	resp, err := testClient(200*time.Millisecond, 5*time.Second, time.Second, 3).Get(srv.URL)
	require.NoError(t, err)
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	time.Sleep(300 * time.Millisecond)
	assert.Equal(t, int32(1), h.seen.Load())
}

// When every request is stuck, no more than tries are sent and the download fails with a timeout
// once the last one gives up, so the downloader can retry or back off.
func TestHedgeTransportGivesUpAfterTries(t *testing.T) {
	h := &stuckFirst{n: 100, release: make(chan struct{})}
	srv := httptest.NewServer(h)
	defer srv.Close()
	defer close(h.release)

	start := time.Now()
	_, err := testClient(50*time.Millisecond, 300*time.Millisecond, time.Second, 3).Get(srv.URL)
	require.Error(t, err)
	var ne net.Error
	require.True(t, errors.As(err, &ne), "got %T: %v", err, err)
	assert.True(t, ne.Timeout())
	assert.Equal(t, int32(3), h.seen.Load())
	elapsed := time.Since(start)
	assert.GreaterOrEqual(t, elapsed, 400*time.Millisecond, "the last request waits its full header timeout")
	assert.Less(t, elapsed, 2*time.Second)
}

// An error that comes back at once (nothing listening) is returned, not hedged; the downloader
// waits before it tries again.
func TestHedgeTransportReturnsImmediateErrors(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	url := "http://" + l.Addr().String() + "/x.jpg"
	require.NoError(t, l.Close())

	start := time.Now()
	_, err = testClient(time.Second, time.Second, time.Second, 3).Get(url)
	require.Error(t, err)
	assert.Less(t, time.Since(start), time.Second)
}

// A body that keeps arriving, however slowly, is not cut off; the app's shared client gave up on
// any download after 60 s in total.
func TestStallBodyKeepsSlowBodies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for i := 0; i < 6; i++ {
			_, _ = w.Write([]byte("0123456789"))
			w.(http.Flusher).Flush()
			time.Sleep(60 * time.Millisecond)
		}
	}))
	defer srv.Close()

	resp, err := testClient(time.Second, time.Second, 200*time.Millisecond, 3).Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Len(t, b, 60)
}

// A body that stops moving ends with a timeout error, which the downloader retries.
func TestStallBodyCutsStalledBodies(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("first bytes"))
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	start := time.Now()
	resp, err := testClient(time.Second, time.Second, 150*time.Millisecond, 3).Get(srv.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	_, err = io.ReadAll(resp.Body)
	require.Error(t, err)
	assert.Less(t, time.Since(start), 2*time.Second)
	var ne net.Error
	require.True(t, errors.As(err, &ne), "got %T: %v", err, err)
	assert.True(t, ne.Timeout())
}

// Cancelling the download cancels every request it sent.
func TestHedgeTransportCancel(t *testing.T) {
	h := &stuckFirst{n: 100, release: make(chan struct{})}
	srv := httptest.NewServer(h)
	defer srv.Close()
	defer close(h.release)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	_, err = testClient(50*time.Millisecond, 5*time.Second, time.Second, 3).Do(req)
	require.Error(t, err)
	assert.Eventually(t, func() bool { return h.open.Load() == 0 }, 2*time.Second, 10*time.Millisecond)
}

func TestProviderUsesOwnImageClient(t *testing.T) {
	shared := &http.Client{}
	p := NewProvider(nil, shared)
	assert.NotSame(t, shared, p.GetClient(), "images use SMK's own client")
	tr, ok := p.GetClient().Transport.(*hedgeTransport)
	require.True(t, ok)
	assert.Equal(t, imageTries, tr.tries)
	assert.Equal(t, imageHeaderTimeout, tr.next.(*http.Transport).ResponseHeaderTimeout)
	assert.Equal(t, imageDownloadAttempts, p.DownloadAttempts())
}
