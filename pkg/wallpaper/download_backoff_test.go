package wallpaper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/util"
)

// A2 §12.9 #4: a source whose image server cannot be reached is paused after a
// few failed images, for longer each time, until an image downloads again.

func newTestBackoff() (*downloadBackoff, *time.Time) {
	now := time.Date(2026, 11, 8, 3, 0, 0, 0, time.UTC)
	b := newDownloadBackoff()
	b.now = func() time.Time { return now }
	return b, &now
}

func TestDownloadBackoff_PausesAfterFailuresInARow(t *testing.T) {
	b, now := newTestBackoff()

	assert.Zero(t, b.failed("SMK"))
	assert.Zero(t, b.failed("SMK"))
	assert.Zero(t, b.pausedFor("SMK"), "two failed images do not pause yet")
	assert.Equal(t, 2*time.Minute, b.failed("SMK"), "the third pauses for two minutes")
	assert.Equal(t, 2*time.Minute, b.pausedFor("SMK"))
	assert.Zero(t, b.pausedFor("Met"), "other sources go on")

	assert.Zero(t, b.failed("SMK"), "downloads that were already running do not count while paused")

	*now = now.Add(2 * time.Minute)
	assert.Zero(t, b.pausedFor("SMK"))
	assert.Equal(t, 4*time.Minute, b.failed("SMK"), "after a pause one failure pauses again, twice as long")
	*now = now.Add(4 * time.Minute)
	assert.Equal(t, 8*time.Minute, b.failed("SMK"))
	*now = now.Add(8 * time.Minute)
	assert.Equal(t, 16*time.Minute, b.failed("SMK"))
	*now = now.Add(16 * time.Minute)
	assert.Equal(t, 30*time.Minute, b.failed("SMK"), "never longer than 30 minutes")
	*now = now.Add(30 * time.Minute)
	assert.Equal(t, 30*time.Minute, b.failed("SMK"))

	*now = now.Add(30 * time.Minute)
	b.succeeded("SMK")
	assert.Zero(t, b.failed("SMK"), "an image that downloads ends the backoff")
	assert.Zero(t, b.failed("SMK"))
	assert.Equal(t, 2*time.Minute, b.failed("SMK"))
}

func TestDownloadBackoff_SuccessResetsTheCount(t *testing.T) {
	b, _ := newTestBackoff()
	b.failed("SMK")
	b.failed("SMK")
	b.succeeded("SMK")
	assert.Zero(t, b.failed("SMK"))
	assert.Zero(t, b.failed("SMK"), "failures must be in a row")
}

// The held source's jobs wait in its own queue; other sources' jobs go through.
func TestDispatcher_HeldProviderWaitsOthersFlow(t *testing.T) {
	prev := maxHoldCheck
	maxHoldCheck = 10 * time.Millisecond
	defer func() { maxHoldCheck = prev }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	out := make(chan DownloadJob, 10)
	var wg sync.WaitGroup
	d := NewDispatcher(ctx, out, nil, nil, &wg)
	var held atomic.Bool
	held.Store(true)
	d.hold = func(id string) time.Duration {
		if id == "SMK" && held.Load() {
			return time.Hour
		}
		return 0
	}

	smk := &MockPacedProvider{id: "SMK"}
	met := &MockPacedProvider{id: "Met"}
	require.True(t, d.Submit(DownloadJob{Ctx: ctx, Provider: smk}))
	require.True(t, d.Submit(DownloadJob{Ctx: ctx, Provider: met}))

	select {
	case job := <-out:
		assert.Equal(t, "Met", job.Provider.ID(), "the other source is not held up")
	case <-time.After(2 * time.Second):
		t.Fatal("the other source's job did not go through")
	}
	select {
	case job := <-out:
		t.Fatalf("a held source's job went through: %s", job.Provider.ID())
	case <-time.After(50 * time.Millisecond):
	}

	held.Store(false)
	select {
	case job := <-out:
		assert.Equal(t, "SMK", job.Provider.ID(), "the job goes on when the pause ends")
	case <-time.After(2 * time.Second):
		t.Fatal("the held job did not go on after the pause")
	}
}

// Downloads from an unreachable server count towards the pause; once paused, a
// running download stops retrying, and a source that works is not affected.
func TestDownloadMasterFile_UnreachableServerPausesItsSource(t *testing.T) {
	prevWait := downloadRetryWait
	downloadRetryWait = time.Millisecond
	defer func() { downloadRetryWait = prevWait }()

	var smkRequests atomic.Int32
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		smkRequests.Add(1)
		w.WriteHeader(http.StatusGatewayTimeout)
	}))
	defer down.Close()
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("jpeg"))
	}))
	defer up.Close()

	wp := &Plugin{}
	smk := &MockPacedProvider{id: "SMK"}
	met := &MockPacedProvider{id: "Met"}
	dir := t.TempDir()

	for i := 0; i < backoffFailures; i++ {
		_, err := wp.downloadMasterFile(context.Background(), down.Client(), down.URL, filepath.Join(dir, "smk.jpg"), smk)
		require.Error(t, err)
	}
	assert.Equal(t, int32(backoffFailures*downloadAttempts), smkRequests.Load(), "each image was tried in full")
	assert.Equal(t, backoffFirstPause, wp.downloadBackoff().pausedFor("SMK").Round(time.Minute))

	smkRequests.Store(0)
	_, err := wp.downloadMasterFile(context.Background(), down.Client(), down.URL, filepath.Join(dir, "smk.jpg"), smk)
	require.Error(t, err)
	assert.Equal(t, int32(1), smkRequests.Load(), "while paused a download does not retry")

	path, err := wp.downloadMasterFile(context.Background(), up.Client(), up.URL, filepath.Join(dir, "met.jpg"), met)
	require.NoError(t, err)
	assert.FileExists(t, path)
	assert.Zero(t, wp.downloadBackoff().pausedFor("Met"))
}

// A paused source is not fetched: no new jobs pile up behind the pause and its
// pages do not move on. (An unexpected FetchImages call fails the mock.)
func TestFetchFromProvider_SkipsAPausedSource(t *testing.T) {
	wp := &Plugin{queryPages: map[string]*util.SafeCounter{}}
	for i := 0; i < backoffFailures; i++ {
		wp.downloadBackoff().failed("SMK")
	}
	p := &MockProvider{IDVal: "SMK"}
	var mu sync.Mutex
	queued := util.NewSafeInt()
	wp.fetchFromProvider(context.Background(), ImageQuery{ID: "q1", Provider: "SMK"}, p, false, 10, &mu, map[string]bool{}, queued)
	p.AssertNotCalled(t, "FetchImages", mock.Anything, mock.Anything, mock.Anything)
	assert.Zero(t, queued.Value())
	assert.NotContains(t, wp.queryPages, "q1", "the page is not touched")
}

// A 404 is an answer, not an unreachable server: it does not count.
func TestDownloadMasterFile_NotFoundDoesNotPause(t *testing.T) {
	missing := httptest.NewServer(http.NotFoundHandler())
	defer missing.Close()
	wp := &Plugin{}
	src := &MockPacedProvider{id: "Getty"}
	for i := 0; i < backoffFailures+1; i++ {
		_, err := wp.downloadMasterFile(context.Background(), missing.Client(), missing.URL, filepath.Join(t.TempDir(), "x.jpg"), src)
		require.Error(t, err)
	}
	assert.Zero(t, wp.downloadBackoff().pausedFor("Getty"))
}

type fewAttemptsProvider struct {
	MockPacedProvider
	attempts int
}

func (p *fewAttemptsProvider) DownloadAttempts() int { return p.attempts }

// A provider whose client retries on its own (SMK) sets how often the downloader tries an image.
func TestDownloadMasterFile_ProviderSetsAttempts(t *testing.T) {
	prevWait := downloadRetryWait
	downloadRetryWait = time.Millisecond
	defer func() { downloadRetryWait = prevWait }()

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	wp := &Plugin{}
	p := &fewAttemptsProvider{MockPacedProvider: MockPacedProvider{id: "SMK"}, attempts: 2}
	_, err := wp.downloadMasterFile(context.Background(), srv.Client(), srv.URL+"/a.jpg", filepath.Join(t.TempDir(), "a.jpg"), p)
	require.Error(t, err)
	assert.Equal(t, int32(2), hits.Load())

	hits.Store(0)
	_, err = wp.downloadMasterFile(context.Background(), srv.Client(), srv.URL+"/b.jpg", filepath.Join(t.TempDir(), "b.jpg"), &MockPacedProvider{id: "Other"})
	require.Error(t, err)
	assert.Equal(t, int32(downloadAttempts), hits.Load(), "other sources keep the default")
}

type outageProvider struct {
	MockPacedProvider
	attempts int
	left     time.Duration
}

func (p *outageProvider) DownloadAttempts() int                   { return p.attempts }
func (p *outageProvider) OutagePause(now time.Time) time.Duration { return p.left }

// A source that reports a known outage (SMK during the Danish night) is paused until it ends after
// one failed attempt; outside the outage the usual retries and backoff apply.
func TestDownloadMasterFile_KnownOutagePausesAfterOneFailure(t *testing.T) {
	prevWait := downloadRetryWait
	downloadRetryWait = time.Millisecond
	defer func() { downloadRetryWait = prevWait }()

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusGatewayTimeout)
	}))
	defer srv.Close()
	dir := t.TempDir()

	wp := &Plugin{}
	night := &outageProvider{MockPacedProvider: MockPacedProvider{id: "SMK"}, attempts: 2, left: 7 * time.Hour}
	_, err := wp.downloadMasterFile(context.Background(), srv.Client(), srv.URL, filepath.Join(dir, "a.jpg"), night)
	require.Error(t, err)
	assert.Equal(t, int32(1), hits.Load(), "no second attempt during the outage")
	assert.Equal(t, 7*time.Hour, wp.downloadBackoff().pausedFor("SMK").Round(time.Minute))
	assert.Zero(t, wp.downloadBackoff().pausedFor("Met"), "other sources go on")

	wp.downloadBackoff().pauseForOutage("SMK", time.Hour)
	assert.Equal(t, 7*time.Hour, wp.downloadBackoff().pausedFor("SMK").Round(time.Minute), "a shorter pause does not cut the longer one")

	hits.Store(0)
	day := &outageProvider{MockPacedProvider: MockPacedProvider{id: "SMK"}, attempts: 2}
	wp2 := &Plugin{}
	_, err = wp2.downloadMasterFile(context.Background(), srv.Client(), srv.URL, filepath.Join(dir, "b.jpg"), day)
	require.Error(t, err)
	assert.Equal(t, int32(2), hits.Load(), "by day both attempts are made")
	assert.Zero(t, wp2.downloadBackoff().pausedFor("SMK"), "and one failed image does not pause")
}

// When the outage pause ends, the next failure starts the usual short pause, and a download that
// works ends it.
func TestDownloadBackoff_AfterOutagePause(t *testing.T) {
	b, now := newTestBackoff()
	b.pauseForOutage("SMK", 4*time.Hour)
	assert.Equal(t, 4*time.Hour, b.pausedFor("SMK"))
	assert.Zero(t, b.failed("SMK"), "downloads that were already running do not count")

	*now = now.Add(4 * time.Hour)
	assert.Zero(t, b.pausedFor("SMK"))
	assert.Equal(t, backoffFirstPause, b.failed("SMK"), "the first failure in the morning pauses briefly")

	b.pauseForOutage("SMK", time.Hour)
	b.succeeded("SMK")
	assert.Zero(t, b.pausedFor("SMK"), "an image that downloads ends the pause")
}
