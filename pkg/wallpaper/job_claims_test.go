package wallpaper

import (
	"bytes"
	"context"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
)

// A2 phase 5 #4: one image is downloaded and processed by one job at a time.

func TestJobClaims(t *testing.T) {
	var c jobClaims
	release, ok := c.tryClaim("a")
	require.True(t, ok)
	_, ok = c.tryClaim("a")
	assert.False(t, ok, "a second job for the same image")
	other, ok := c.tryClaim("b")
	require.True(t, ok, "another image is not held up")
	other()

	got := make(chan func(), 1)
	go func() {
		r, err := c.claim(context.Background(), "a")
		if err == nil {
			got <- r
		}
	}()
	select {
	case <-got:
		t.Fatal("a waiting claim went ahead while the image was taken")
	case <-time.After(30 * time.Millisecond):
	}
	release()
	release() // giving back twice does no harm
	select {
	case r := <-got:
		r()
	case <-time.After(2 * time.Second):
		t.Fatal("the waiting claim did not get the image")
	}

	release, ok = c.tryClaim("a")
	require.True(t, ok)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := c.claim(ctx, "a")
	assert.ErrorIs(t, err, context.Canceled)
	release()
	assert.Empty(t, c.busy)
}

// While a slow download of an image is running, a second job for it is dropped at once: it
// neither reads the half-written master nor deletes the first job's files. The master only
// appears under its name when it is complete.
func TestProcessImageJob_SameImageTwiceAtOnce(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, quadrantImage(1600, 1000), nil))
	data := buf.Bytes()

	started := make(chan struct{})
	finish := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(data[:len(data)/2])
		w.(http.Flusher).Flush()
		close(started)
		<-finish
		_, _ = w.Write(data[len(data)/2:])
	}))
	defer srv.Close()

	env := newDisplayTestEnv(t)
	env.wp.httpClient = srv.Client()
	src := &MockPacedProvider{id: "SMK"}
	job := DownloadJob{Ctx: context.Background(), Provider: src,
		Image: provider.Image{ID: "SMK_KMS5989", Path: srv.URL + "/KMS5989.jpg", SourceQueryID: "q1"}}

	first := make(chan error, 1)
	go func() {
		_, err := env.wp.ProcessImageJob(context.Background(), job)
		first <- err
	}()
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("the first download did not start")
	}

	// The server has sent half the file; wait until the first job has written that half to its
	// temporary file. Nothing here depends on timing: the server holds the rest back until the
	// end of the checks.
	partPattern := filepath.Join(env.fm.GetDownloadDir(), "SMK_KMS5989.part-*.jpg")
	require.Eventually(t, func() bool {
		parts, _ := filepath.Glob(partPattern)
		if len(parts) != 1 {
			return false
		}
		fi, err := os.Stat(parts[0])
		return err == nil && fi.Size() >= int64(len(data)/2)
	}, 10*time.Second, 5*time.Millisecond, "the first half of the download goes to one temporary file")

	master := filepath.Join(env.fm.GetDownloadDir(), "SMK_KMS5989.jpg")
	_, err := os.Stat(master)
	assert.True(t, os.IsNotExist(err), "half a master is not under its name")

	_, err = env.wp.ProcessImageJob(context.Background(), job)
	assert.ErrorIs(t, err, ErrImageInProgress, "the second job is dropped")

	close(finish)
	select {
	case err := <-first:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("the first job did not finish")
	}
	f, err := os.Open(master)
	require.NoError(t, err)
	_, err = jpeg.Decode(f)
	f.Close()
	assert.NoError(t, err, "the master is complete")
	parts, _ := filepath.Glob(filepath.Join(env.fm.GetDownloadDir(), "SMK_KMS5989.part-*"))
	assert.Empty(t, parts, "no temporary file is left")

	// Once the first job is done the image can be processed again.
	_, err = env.wp.ProcessImageJob(context.Background(), job)
	assert.NoError(t, err)
}

// A failed download leaves neither a master nor a temporary file.
func TestDownloadMasterFile_AbortedDownloadLeavesNothing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100000")
		_, _ = w.Write([]byte("partial"))
	}))
	defer srv.Close()
	dir := t.TempDir()
	wp := &Plugin{}
	_, err := wp.downloadMasterFileOnce(context.Background(), srv.Client(), srv.URL, filepath.Join(dir, "x.jpg"), &MockPacedProvider{id: "Getty"})
	require.Error(t, err)
	entries, _ := os.ReadDir(dir)
	assert.Empty(t, entries)
}
