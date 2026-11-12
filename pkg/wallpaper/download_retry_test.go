package wallpaper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/stretchr/testify/assert"
)

// shortTimeoutClient fails a request that gets no response headers within 100 ms, like the app's
// client does after 15 s, so a hung server shows up as a timeout quickly.
func shortTimeoutClient() *http.Client {
	return &http.Client{Transport: &http.Transport{ResponseHeaderTimeout: 100 * time.Millisecond}}
}

func fastRetries(t *testing.T) {
	old := downloadRetryWait
	downloadRetryWait = 5 * time.Millisecond
	t.Cleanup(func() { downloadRetryWait = old })
}

// hang makes the handler send nothing until the client gives up.
func hang(w http.ResponseWriter, r *http.Request) {
	select {
	case <-r.Context().Done():
	case <-time.After(3 * time.Second):
	}
}

func TestDownloadMasterFile_RetriesAfterTimeouts(t *testing.T) {
	fastRetries(t)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= 3 { // the first three requests hang, the fourth works
			hang(w, r)
			return
		}
		_, _ = w.Write([]byte("image-bytes"))
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "img.jpg")
	got, err := (&Plugin{}).downloadMasterFile(context.Background(), shortTimeoutClient(), srv.URL, dest, nil)

	assert.NoError(t, err)
	assert.Equal(t, dest, got)
	assert.EqualValues(t, 4, calls.Load())
	data, _ := os.ReadFile(dest)
	assert.Equal(t, "image-bytes", string(data))
}

func TestDownloadMasterFile_GivesUpAfterAllAttempts(t *testing.T) {
	fastRetries(t)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		hang(w, r)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "img.jpg")
	_, err := (&Plugin{}).downloadMasterFile(context.Background(), shortTimeoutClient(), srv.URL, dest, nil)

	assert.Error(t, err)
	assert.EqualValues(t, downloadAttempts, calls.Load())
	_, statErr := os.Stat(dest)
	assert.True(t, os.IsNotExist(statErr), "no partial file is left")
}

func TestDownloadMasterFile_DoesNotRetryClientErrors(t *testing.T) {
	fastRetries(t)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.NotFound(w, r)
	}))
	defer srv.Close()

	_, err := (&Plugin{}).downloadMasterFile(context.Background(), srv.Client(), srv.URL, filepath.Join(t.TempDir(), "img.jpg"), nil)

	assert.Error(t, err)
	assert.EqualValues(t, 1, calls.Load(), "a 404 will not change, so it is tried once")
}

func TestDownloadMasterFile_RetriesServerUnavailable(t *testing.T) {
	fastRetries(t)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	_, err := (&Plugin{}).downloadMasterFile(context.Background(), srv.Client(), srv.URL, filepath.Join(t.TempDir(), "img.jpg"), nil)

	assert.NoError(t, err)
	assert.EqualValues(t, 2, calls.Load())
}

func TestDownloadMasterFile_StopsWhenCancelled(t *testing.T) {
	fastRetries(t)
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		hang(w, r)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := (&Plugin{}).downloadMasterFile(ctx, srv.Client(), srv.URL, filepath.Join(t.TempDir(), "img.jpg"), nil)

	assert.Error(t, err)
	assert.EqualValues(t, 1, calls.Load(), "no more tries once the job is cancelled")
}

func TestCheckMasterSize(t *testing.T) {
	assert.NoError(t, checkMasterSize(provider.Image{}), "unknown size passes; it is measured after download")
	assert.NoError(t, checkMasterSize(provider.Image{Width: 6000, Height: 4000}))
	assert.NoError(t, checkMasterSize(provider.Image{Width: 10000, Height: 10000}), "exactly the limit passes")
	assert.Error(t, checkMasterSize(provider.Image{Width: 10001, Height: 10000}))
	assert.Error(t, checkMasterSize(provider.Image{Width: 42208, Height: 9870}), "the 212 MB Wikimedia mosaic")
}
