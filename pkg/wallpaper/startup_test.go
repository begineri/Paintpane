package wallpaper

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
)

// Start-up of a light background app (docs/lightweight-eval.md §4): a start that Windows made at
// logon waits 60–120 s before it fetches or processes; a full library is not fetched again at
// start; a start keeps last time's wallpaper unless the person asked for a change.

func TestHasAutostartFlag(t *testing.T) {
	assert.True(t, hasAutostartFlag([]string{"--autostart"}))
	assert.True(t, hasAutostartFlag([]string{"-x", "--autostart"}))
	assert.False(t, hasAutostartFlag(nil))
	assert.False(t, hasAutostartFlag([]string{"-probe-gl"}))
	assert.Equal(t, "--autostart", AutostartFlag, "the installer's Run entry passes this exact flag")
}

func TestAutostartDelayIsOneToTwoMinutes(t *testing.T) {
	seen := map[time.Duration]bool{}
	for i := 0; i < 500; i++ {
		d := autostartDelay()
		assert.GreaterOrEqual(t, d, 60*time.Second)
		assert.LessOrEqual(t, d, 120*time.Second)
		seen[d.Round(time.Second)] = true
	}
	assert.Greater(t, len(seen), 5, "the delay is spread out, not a constant")
}

func withAutostart(t *testing.T, autostart bool) {
	t.Helper()
	previous := detectAutostart
	detectAutostart = func() bool { return autostart }
	t.Cleanup(func() { detectAutostart = previous })
}

// addReadyImages puts n finished images in the store. Provider, query and an original on disk
// match the active query of the refresh test, so the store's reconcile at start keeps them.
func addReadyImages(t *testing.T, wp *Plugin, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("img%d", i)
		master, err := wp.fm.GetMasterPath(id, ".jpg")
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Dir(master), 0o755))
		require.NoError(t, os.WriteFile(master, []byte("x"), 0o600))
		wp.store.Add(provider.Image{ID: id, Provider: "A", SourceQueryID: "q1", DerivativePaths: map[string]string{"100x100": "d.jpg"}})
	}
}

func TestStartupDelayOnlyForAutostartWithALibrary(t *testing.T) {
	wp, _ := budgetTestPlugin(t, Cache100Images)

	withAutostart(t, true)
	assert.Zero(t, wp.startupDelay(), "an empty library never waits: its first images are what the person waits for")

	addReadyImages(t, wp, 3)
	d := wp.startupDelay()
	assert.GreaterOrEqual(t, d, 60*time.Second)
	assert.LessOrEqual(t, d, 120*time.Second)

	withAutostart(t, false)
	assert.Zero(t, wp.startupDelay(), "a start the person made does not wait")
}

// The library counts as full at 90 % of the cache: it settles a few images short of the limit
// (A2 §14.9 #3: 140–148 of 150), so the limit itself would almost never be reached.
func TestLibraryFull(t *testing.T) {
	wp, _ := budgetTestPlugin(t, Cache100Images)
	addReadyImages(t, wp, 89)
	assert.False(t, wp.libraryFull())
	addReadyImages(t, wp, 90)
	assert.True(t, wp.libraryFull(), "90 of 100")

	wp, _ = budgetTestPlugin(t, Cache150Images)
	addReadyImages(t, wp, 134)
	assert.False(t, wp.libraryFull(), "134 of 150")
	addReadyImages(t, wp, 135)
	assert.True(t, wp.libraryFull(), "135 of 150")
	addReadyImages(t, wp, 140)
	assert.True(t, wp.libraryFull(), "where A2's library settled")
}

func TestStartupFetchSkipsAFullLibrary(t *testing.T) {
	wp, _ := budgetTestPlugin(t, Cache100Images)
	syncer := &mockSyncerProvider{}
	syncer.On("ID").Return("TestProvider")
	wp.providers["TestProvider"] = syncer
	wp.ctx = context.Background()

	addReadyImages(t, wp, 100)
	wp.startupFetch()
	time.Sleep(200 * time.Millisecond)
	assert.Zero(t, syncer.syncCalled.Load(), "a full library does not ask the sources for anything")

	wp.store.Clear()
	addReadyImages(t, wp, 10)
	wp.startupFetch()
	assert.Eventually(t, func() bool { return syncer.syncCalled.Load() == 1 }, 2*time.Second, 10*time.Millisecond,
		"a library with room is topped up")
}

func TestAfterStartupDelay(t *testing.T) {
	wp, _ := budgetTestPlugin(t, Cache100Images)

	// No delay: runs at once, on the caller's goroutine.
	ran := false
	wp.afterStartupDelay(0, func() { ran = true })
	assert.True(t, ran)

	// A delay: not before it is over.
	wp.ctx = context.Background()
	var late atomic.Bool
	wp.afterStartupDelay(150*time.Millisecond, func() { late.Store(true) })
	time.Sleep(50 * time.Millisecond)
	assert.False(t, late.Load(), "it waits")
	assert.Eventually(t, late.Load, 2*time.Second, 10*time.Millisecond, "and then runs")

	// Deactivated meanwhile: never.
	ctx, cancel := context.WithCancel(context.Background())
	wp.ctx = ctx
	var cancelled atomic.Bool
	wp.afterStartupDelay(150*time.Millisecond, func() { cancelled.Store(true) })
	cancel()
	time.Sleep(300 * time.Millisecond)
	assert.False(t, cancelled.Load(), "deactivating the plugin cancels the waiting work")
}

// Defaults of a person who never chose (D2 follow-ups, owner's choice 2026-11-08).
func TestStartDefaults(t *testing.T) {
	cfg := GetConfig(NewMockPreferences())
	assert.Equal(t, Cache150Images, cfg.GetCacheSize())
	assert.Equal(t, 150, cfg.GetCacheSize().Size())

	// What was saved is kept.
	cfg.SetCacheSize(Cache300Images)
	assert.Equal(t, 300, cfg.GetCacheSize().Size())
	cfg.SetCacheSize(Cache100Images)
	assert.Equal(t, 100, cfg.GetCacheSize().Size())
}

func TestCacheSizeDropDownMatchesTheSavedSize(t *testing.T) {
	var counts []string
	for _, s := range GetCacheSizes() {
		counts = append(counts, s.String())
	}
	assert.Equal(t, []string{"100", "150", "300", "500", "1000", "5000", "10000"}, counts)

	for i, s := range GetCacheSizes() {
		assert.Equal(t, i, cacheSizeIndex(s.(CacheSize)), "the position of %s", s)
		assert.Equal(t, s.(CacheSize), cacheSizeAt(i), "the size at position %d", i)
	}
	assert.Equal(t, 1, cacheSizeIndex(DefaultCacheSize), "the default is shown as 150")
	assert.Equal(t, DefaultCacheSize, cacheSizeAt(-1))
	assert.Equal(t, DefaultCacheSize, cacheSizeAt(99))

	// A size that was saved before 150 existed still reads the same.
	cfg := GetConfig(NewMockPreferences())
	cfg.SetInt(CacheSizePrefKey, int(Cache500Images))
	assert.Equal(t, 500, cfg.GetCacheSize().Size())
	cfg.SetInt(CacheSizePrefKey, 2) // the retired 200
	assert.Equal(t, 300, cfg.GetCacheSize().Size(), "retired values still become 300")
}

// A start keeps the wallpaper from last time; the next scheduled change replaces it. The old
// "change wallpaper on start" setting is gone, and a value saved for it is not read.
func TestE2E_StartKeepsLastWallpaper(t *testing.T) {
	defer ResetConfig()
	prefs := NewMockPreferences()
	prefs.SetBool("wallpaper_chg_img_on_start_key", true) // saved by an earlier version

	app := newE2EApp(t, t.TempDir(), prefs, e2eMonitor(0, 800, 500))
	app.seed(t, map[string][2]int{"a": {800, 500}, "b": {500, 800}, "c": {900, 900}})
	app.wp.Activate()
	defer app.stop()

	time.Sleep(4 * time.Second) // the old setting changed the wallpaper 3 seconds after start
	assert.Empty(t, app.wallpaperCalls(0), "the start did not change the wallpaper")

	app.wp.SetNextWallpaper(0, true) // the scheduled change still works
	app.waitForWallpaperCalls(t, 0, 1)
}
