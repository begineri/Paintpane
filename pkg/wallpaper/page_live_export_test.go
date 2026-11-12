//go:build pagelive

package wallpaper

import (
	"context"
	"image"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/config"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/util"
)

// LiveQuery is one query of a live page-reset run.
type LiveQuery struct{ ID, Provider, URL string }

// LiveRound is what one query did in one start: the page it fetched and how many images new to the
// library it queued (images the library has are skipped before they are queued).
type LiveRound struct {
	Page, New int
}

// LiveStart is one start of the app against real sources: the pages, cursors and library saved by
// the previous start are read from the working directory, one fetch round runs, and every queued
// image is put in the library as if it had been downloaded (no image is downloaded).
func LiveStart(t *testing.T, sources []provider.ImageProvider, queries []LiveQuery) map[string]LiveRound {
	t.Helper()
	dir := filepath.Join(config.GetWorkingDir(), "wallpaper_downloads")
	store := NewImageStore()
	wp := &Plugin{
		cfg:                &Config{Preferences: NewMockPreferences()},
		fetchingInProgress: util.NewSafeBool(),
		queryPages:         make(map[string]*util.SafeCounter),
		providers:          make(map[string]provider.ImageProvider),
		store:              store,
		fm:                 NewFileManager(dir),
		ctx:                context.Background(),
		queryContexts:      make(map[string]context.Context),
		queryCancelFuncs:   make(map[string]context.CancelFunc),
	}
	require.NoError(t, wp.fm.EnsureDirs())
	wp.store.SetAsyncSave(false)
	wp.store.SetFileManager(wp.fm, filepath.Join(dir, "live_library.json"))
	require.NoError(t, wp.store.LoadCache())
	wp.cfg.SetCacheSize(Cache150Images)
	for _, q := range queries {
		wp.cfg.Queries = append(wp.cfg.Queries, ImageQuery{ID: q.ID, Provider: q.Provider, URL: q.URL, Active: true})
	}
	for _, p := range sources {
		wp.providers[p.ID()] = p
	}
	mockOS := &MockOS{}
	mockOS.On("GetMonitors").Return([]Monitor{{ID: 0, Rect: image.Rect(0, 0, 1920, 1080)}}, nil)
	wp.os = mockOS
	sub := &recordingSubmitter{}
	wp.jobSubmitter = sub

	if os.Getenv("PAGELIVE_OLD") == "" {
		wp.loadQueryPages()
		wp.restorePageCursors()
	} // else: as before the fix, every start begins at page 1
	before := map[string]int{}
	for _, q := range queries {
		before[q.ID] = 1
		if pg, ok := wp.queryPages[q.ID]; ok {
			before[q.ID] = pg.Value()
		}
	}
	wp.FetchNewImages(false)
	require.Eventually(t, func() bool { return !wp.fetchingInProgress.Value() }, 5*time.Minute, 100*time.Millisecond)

	out := map[string]LiveRound{}
	for id, page := range before {
		out[id] = LiveRound{Page: page}
	}
	sub.mu.Lock()
	jobs := append([]DownloadJob(nil), sub.jobs...)
	sub.mu.Unlock()
	for _, j := range jobs {
		r := out[j.Image.SourceQueryID]
		if _, known := wp.store.GetByID(j.Image.ID); !known {
			r.New++
			img := j.Image
			img.DerivativePaths = map[string]string{"1920x1080": "downloaded.jpg"}
			wp.store.Add(img)
		}
		out[j.Image.SourceQueryID] = r
	}
	store.SaveCache()
	return out
}
