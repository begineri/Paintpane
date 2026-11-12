package wallpaper

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type recordingSubmitter struct {
	mu   sync.Mutex
	jobs []DownloadJob
}

func (r *recordingSubmitter) Submit(_ context.Context, job DownloadJob) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.jobs = append(r.jobs, job)
	return true
}

func (r *recordingSubmitter) ids() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	ids := make([]string, 0, len(r.jobs))
	for _, job := range r.jobs {
		ids = append(ids, job.Image.ID)
	}
	return ids
}

func TestPruneToLimit_EvictsOldestAndKeepsDerivatives(t *testing.T) {
	tmpDir := t.TempDir()
	fm := NewFileManager(tmpDir)
	store := NewImageStore()
	store.SetAsyncSave(false)
	store.SetFileManager(fm, filepath.Join(tmpDir, "cache.json"))

	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("img%d", i)
		master, err := fm.GetMasterPath(id, ".jpg")
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Dir(master), 0755))
		require.NoError(t, os.WriteFile(master, []byte(id), 0644))
		// Flags that wouldn't satisfy Sync: pruning must not invalidate these images.
		store.Add(provider.Image{
			ID:              id,
			DerivativePaths: map[string]string{"100x100": id + "_d.jpg"},
			ProcessingFlags: map[string]bool{"SmartFit": true},
		})
	}

	store.PruneToLimit(5, nil)
	assert.Equal(t, 3, store.Count(), "under the limit: nothing to do")
	store.PruneToLimit(0, nil)
	assert.Equal(t, 3, store.Count(), "non-positive limit: nothing to do")

	store.PruneToLimit(2, nil)
	require.True(t, store.WaitTimeout(5*time.Second))

	assert.Equal(t, 2, store.Count())
	assert.False(t, store.Exists("img0"))
	kept, ok := store.GetByID("img2")
	require.True(t, ok)
	assert.NotEmpty(t, kept.DerivativePaths)
	assert.Equal(t, []string{"img1", "img2"}, store.GetIDsForResolution("100x100"))

	master, err := fm.GetMasterPath("img0", ".jpg")
	require.NoError(t, err)
	_, err = os.Stat(master)
	assert.True(t, os.IsNotExist(err), "evicted master should be deleted")
}

func TestReadyUnseenCount(t *testing.T) {
	store := NewImageStore()
	store.SetAsyncSave(false)
	ready := map[string]string{"100x100": "d.jpg"}

	store.Add(provider.Image{ID: "ready", DerivativePaths: ready})
	store.Add(provider.Image{ID: "seen", DerivativePaths: ready, Seen: true})
	store.Add(provider.Image{ID: "no-derivative"})

	assert.Equal(t, 1, store.ReadyUnseenCount())
}

func budgetTestPlugin(t *testing.T, cacheSize CacheSize) (*Plugin, *recordingSubmitter) {
	t.Helper()
	wp := newTestPlugin(t)
	wp.store.SetAsyncSave(false)

	previous := wp.cfg.GetCacheSize()
	wp.cfg.SetCacheSize(cacheSize)
	t.Cleanup(func() { wp.cfg.SetCacheSize(previous) })

	mockOS := &MockOS{}
	mockOS.On("GetMonitors").Return([]Monitor{{ID: 0, Rect: image.Rect(0, 0, 100, 100)}}, nil)
	wp.os = mockOS

	submitter := &recordingSubmitter{}
	wp.jobSubmitter = submitter
	return wp, submitter
}

func TestNewImageBudgetPerQuery(t *testing.T) {
	wp, _ := budgetTestPlugin(t, Cache100Images)
	queries := func(n int) []ImageQuery {
		qs := make([]ImageQuery, 0, n+1)
		for i := 0; i < n; i++ {
			qs = append(qs, ImageQuery{ID: fmt.Sprintf("q%d", i), Provider: "A", Active: true})
		}
		return append(qs, ImageQuery{ID: "inactive", Provider: "A", Active: false})
	}

	assert.Equal(t, 25, wp.newImageBudgetPerQuery(queries(4), ""))
	assert.Equal(t, 2, wp.newImageBudgetPerQuery(queries(50), ""), "rounds up")
	assert.Equal(t, 0, wp.newImageBudgetPerQuery(queries(4), "OtherProvider"))
	assert.Equal(t, 0, wp.newImageBudgetPerQuery(nil, ""))

	for i := 0; i < 90; i++ {
		wp.store.Add(provider.Image{ID: fmt.Sprintf("ready%d", i), DerivativePaths: map[string]string{"100x100": "d.jpg"}})
	}
	assert.Equal(t, 3, wp.newImageBudgetPerQuery(queries(4), ""), "only the remaining room is shared")

	for i := 0; i < 10; i++ {
		wp.store.Add(provider.Image{ID: fmt.Sprintf("more%d", i), DerivativePaths: map[string]string{"100x100": "d.jpg"}})
	}
	assert.Equal(t, 0, wp.newImageBudgetPerQuery(queries(4), ""), "full of unseen images")
}

func fetchPage(t *testing.T, wp *Plugin, q ImageQuery, ids []string, isFav bool, budget int) {
	t.Helper()
	p := &MockImageProvider{}
	p.On("ID").Return("A")
	p.On("Type").Return(provider.TypeCommunity)
	images := make([]provider.Image, 0, len(ids))
	for _, id := range ids {
		images = append(images, provider.Image{ID: id})
	}
	p.On("FetchImages", mock.Anything, mock.Anything, mock.Anything).Return(images, nil)

	var sourcesMu sync.Mutex
	wp.fetchFromProvider(context.Background(), q, p, isFav, budget, &sourcesMu, map[string]bool{}, util.NewSafeInt())
}

func TestFetchFromProvider_StopsAtBudgetWithoutAdvancingPage(t *testing.T) {
	wp, submitter := budgetTestPlugin(t, Cache100Images)
	q := ImageQuery{ID: "q", Provider: "A", Active: true}

	fetchPage(t, wp, q, []string{"1", "2", "3"}, false, 2)

	assert.Equal(t, []string{"A_1", "A_2"}, submitter.ids())
	assert.Equal(t, 1, wp.queryPages["q"].Value(), "image 3 is still on this page")
}

func TestFetchFromProvider_AdvancesPageWhenBudgetSuffices(t *testing.T) {
	wp, submitter := budgetTestPlugin(t, Cache100Images)
	q := ImageQuery{ID: "q", Provider: "A", Active: true}

	fetchPage(t, wp, q, []string{"1", "2"}, false, 2)

	assert.Len(t, submitter.ids(), 2)
	assert.Equal(t, 2, wp.queryPages["q"].Value())
}

func TestFetchFromProvider_HealingIgnoresBudget(t *testing.T) {
	wp, submitter := budgetTestPlugin(t, Cache100Images)
	q := ImageQuery{ID: "q", Provider: "A", Active: true}
	wp.store.Add(provider.Image{ID: "A_needs-healing"}) // In the store, no derivatives yet.

	fetchPage(t, wp, q, []string{"new", "needs-healing"}, false, 0)

	assert.Equal(t, []string{"A_needs-healing"}, submitter.ids())
}

func TestFetchFromProvider_FavoritesIgnoreBudget(t *testing.T) {
	wp, submitter := budgetTestPlugin(t, Cache100Images)

	fetchPage(t, wp, ImageQuery{ID: FavoritesQueryID, Provider: "A", Active: true}, []string{"1", "2"}, false, 0)
	assert.Len(t, submitter.ids(), 2, "the Favorites query is exempt during a normal cycle")

	fetchPage(t, wp, ImageQuery{ID: "q", Provider: "A", Active: true}, []string{"3"}, true, 0)
	assert.Len(t, submitter.ids(), 3, "a targeted Favorites fetch is exempt")
}

func TestPipelineWorkerCount(t *testing.T) {
	prefs := NewMockPreferences()
	wp := &Plugin{cfg: &Config{Preferences: prefs}}
	assert.Equal(t, min(runtime.NumCPU(), DefaultMaxConcurrentProcessors), wp.pipelineWorkerCount(), "default")

	wp.cfg.SetMaxConcurrentProcessors(2)
	assert.Equal(t, 2, wp.pipelineWorkerCount(), "saved setting is honored")

	wp.cfg.MaxConcurrentProcessors = 1
	assert.Equal(t, 1, wp.pipelineWorkerCount(), "explicit override wins")
}

func TestStampProcessingFlags(t *testing.T) {
	img := provider.Image{ProcessingFlags: map[string]bool{
		"FrameSize_0.50":        true,
		"VirtualFramed:100x100": true,
		"incompatible:200x200":  true,
	}}

	stampProcessingFlags(&img, map[string]bool{"SmartFit": true, "FrameSize_0.85": true, "Matting": false})

	assert.Equal(t, map[string]bool{
		"SmartFit":              true,
		"FrameSize_0.85":        true,
		"Matting":               false,
		"VirtualFramed:100x100": true,
		"incompatible:200x200":  true,
	}, img.ProcessingFlags, "stale settings go, outcomes stay")

	var empty provider.Image
	stampProcessingFlags(&empty, map[string]bool{"SmartFit": true})
	assert.True(t, empty.ProcessingFlags["SmartFit"])
}

func TestSync_KeepsDerivativesOfImagesProcessedWithCurrentSettings(t *testing.T) {
	tmpDir := t.TempDir()
	fm := NewFileManager(tmpDir)
	store := NewImageStore()
	store.SetAsyncSave(false)
	store.SetFileManager(fm, filepath.Join(tmpDir, "cache.json"))

	target := map[string]bool{"SmartFit": true, "FrameFallback": true, "FrameSize_0.85": true, "Matting": false}
	for _, id := range []string{"plain", "framed"} {
		master, err := fm.GetMasterPath(id, ".jpg")
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Dir(master), 0755))
		require.NoError(t, os.WriteFile(master, []byte(id), 0644))

		img := provider.Image{ID: id, DerivativePaths: map[string]string{"100x100": id + "_d.jpg"}}
		stampProcessingFlags(&img, target)
		if id == "framed" {
			img.ProcessingFlags["VirtualFramed:100x100"] = true
		}
		store.Add(img)
	}

	store.Sync(100, target, nil)
	for _, id := range []string{"plain", "framed"} {
		img, ok := store.GetByID(id)
		require.True(t, ok)
		assert.NotEmpty(t, img.DerivativePaths, "%s should survive a restart with unchanged settings", id)
	}

	changed := map[string]bool{"SmartFit": true, "FrameFallback": false, "FrameSize_0.85": true, "Matting": false}
	store.Sync(100, changed, nil)
	require.True(t, store.WaitTimeout(5*time.Second))
	for _, id := range []string{"plain", "framed"} {
		img, ok := store.GetByID(id)
		require.True(t, ok)
		assert.Empty(t, img.DerivativePaths, "%s should be invalidated when a setting changes", id)
	}
}

func TestProcessImageJob_StampsEverySettingSyncChecks(t *testing.T) {
	wp, _ := budgetTestPlugin(t, Cache100Images)
	previous := wp.cfg.GetSmartFitMode()
	wp.cfg.SetSmartFitMode(SmartFitOff) // Derivative is the master, so no image decoding is needed.
	t.Cleanup(func() { wp.cfg.SetSmartFitMode(previous) })

	ip := &MockImageProcessor{}
	ip.On("CheckCompatibility", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	wp.imgProcessor = ip

	master, err := wp.fm.GetMasterPath("img", ".jpg")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(master), 0755))
	require.NoError(t, os.WriteFile(master, []byte("master"), 0644))

	result, err := wp.ProcessImageJob(context.Background(), DownloadJob{
		Image: provider.Image{ID: "img", Path: "http://example.invalid/img.jpg", Width: 100, Height: 100},
	})
	require.NoError(t, err)

	store := NewImageStore()
	assert.True(t, store.flagsMatch(result, wp.targetProcessingFlags()), "got flags: %v", result.ProcessingFlags)
}

// The wallpaper on screen is often among the oldest images: pruning must not take it, or its
// master and derivatives, away from Tune Image, Favourite and Block.
func TestPruneToLimit_KeepsImagesOnShow(t *testing.T) {
	tmpDir := t.TempDir()
	fm := NewFileManager(tmpDir)
	store := NewImageStore()
	store.SetAsyncSave(false)
	store.SetFileManager(fm, filepath.Join(tmpDir, "cache.json"))

	for i := 0; i < 4; i++ {
		id := fmt.Sprintf("img%d", i)
		master, err := fm.GetMasterPath(id, ".jpg")
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Dir(master), 0755))
		require.NoError(t, os.WriteFile(master, []byte(id), 0644))
		store.Add(provider.Image{ID: id, DerivativePaths: map[string]string{"100x100": id + "_d.jpg"}})
	}

	store.PruneToLimit(2, map[string]bool{"img0": true})
	require.True(t, store.WaitTimeout(5*time.Second))

	assert.True(t, store.Exists("img0"), "the image on screen stays")
	assert.False(t, store.Exists("img1"))
	assert.False(t, store.Exists("img2"))
	assert.True(t, store.Exists("img3"))
	master, err := fm.GetMasterPath("img0", ".jpg")
	require.NoError(t, err)
	_, err = os.Stat(master)
	assert.NoError(t, err, "the master of the image on screen is kept")

	// Only images on show: nothing is evicted, the store stays over the limit.
	store.PruneToLimit(1, map[string]bool{"img0": true, "img3": true})
	assert.Equal(t, 2, store.Count())
}

func TestImagesOnShow(t *testing.T) {
	wp := &Plugin{Monitors: map[int]*MonitorController{}}
	a := &MonitorController{ID: 0, State: &MonitorState{}}
	a.State.CurrentImage = provider.Image{ID: "desk0"}
	a.State.CurrentID = "desk0"
	a.lockImageID = "lock"
	b := &MonitorController{ID: 1, State: &MonitorState{}}
	b.State.CurrentImage = provider.Image{ID: "desk1"}
	empty := &MonitorController{ID: 2, State: &MonitorState{}}
	wp.Monitors[0], wp.Monitors[1], wp.Monitors[2] = a, b, empty

	assert.Equal(t, map[string]bool{"desk0": true, "desk1": true, "lock": true}, wp.imagesOnShow())
}
