//go:build !linux

package wallpaper

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/disintegration/imaging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A10 acceptance (docs/prd.md): a portrait monitor, and a portrait + landscape pair, must each show
// the whole artwork at its own resolution and orientation. The desktop is simulated with
// recordingOS, so no real monitors are needed.

// shownID is the image behind a wallpaper file the plugin handed to the desktop.
func shownID(path string) string {
	return strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
}

// waitForWallpaperCalls waits until the desktop has been told to set at least n wallpapers on a monitor.
func (a *e2eApp) waitForWallpaperCalls(t *testing.T, monitorID, n int) {
	t.Helper()
	require.Eventually(t, func() bool {
		a.os.mu.Lock()
		defer a.os.mu.Unlock()
		return len(a.os.set[monitorID]) >= n
	}, e2eSettle, 50*time.Millisecond, "monitor %d should receive wallpaper call #%d", monitorID, n)
}

// showFirstWallpapers asks every monitor for its first wallpaper (the harness does not change
// wallpapers on start) and waits until the desktop has received them.
func (a *e2eApp) showFirstWallpapers(t *testing.T) {
	t.Helper()
	for _, m := range a.os.monitors {
		a.wp.SetNextWallpaper(m.ID, true)
	}
	for _, m := range a.os.monitors {
		a.waitForWallpaperCalls(t, m.ID, 1)
	}
}

func (a *e2eApp) wallpaperCalls(monitorID int) []string {
	a.os.mu.Lock()
	defer a.os.mu.Unlock()
	return append([]string(nil), a.os.set[monitorID]...)
}

func TestA10_PortraitAndLandscapeMonitorsEachShowTheWholeArtwork(t *testing.T) {
	useRealDisplayModeDefault(t)
	defer ResetConfig()
	library := map[string][2]int{
		"portrait": {2000, 3200}, "tall": {900, 2400}, "wide": {3200, 1400},
		"panorama": {4800, 1200}, "square": {2000, 2000}, "match": {3840, 2160},
	}
	ids := []string{"portrait", "tall", "wide", "panorama", "square", "match"}

	landscape := e2eMonitor(0, 1920, 1080)
	portrait := e2eMonitor(1, 1080, 1920)
	app := newE2EApp(t, t.TempDir(), NewMockPreferences(), landscape, portrait)
	app.seed(t, library)
	app.wp.Activate()
	defer app.stop()

	assert.Equal(t, ArtworkDisplayFull, app.wp.cfg.GetArtworkDisplayMode(), "full artwork is the default mode")
	app.showFirstWallpapers(t)
	app.waitUntilSettled(t, filepath.Join(FittedRootDir, FullArtDir), ids)

	// Every image has one derivative per monitor, at that monitor's own size and orientation,
	// with the whole artwork inside it.
	for _, m := range []Monitor{landscape, portrait} {
		res := fmt.Sprintf("%dx%d", m.Rect.Dx(), m.Rect.Dy())
		for id, size := range library {
			img, ok := app.store.GetByID(id)
			require.True(t, ok)
			path := img.DerivativePaths[res]
			require.NotEmpty(t, path, "%s has no derivative for %s", id, res)

			got, err := imaging.Open(path)
			require.NoError(t, err)
			assert.Equal(t, m.Rect.Dx(), got.Bounds().Dx(), "%s on %s: width", id, res)
			assert.Equal(t, m.Rect.Dy(), got.Bounds().Dy(), "%s on %s: height", id, res)
			assertWholeOnScreen(t, path, size[0], size[1])
		}
	}

	// The two monitors never share a file: each has its own resolution folder.
	lp, _ := app.store.GetByID("portrait")
	assert.NotEqual(t, lp.DerivativePaths["1920x1080"], lp.DerivativePaths["1080x1920"])

	// The wallpapers actually handed to the desktop belong to the right monitor.
	assert.Contains(t, app.os.last(0), "1920x1080")
	assert.Contains(t, app.os.last(1), "1080x1920")
	app.assertMastersIntact(t, ids)
}

func TestA10_RotatingKeepsEachMonitorWholeAndOnItsOwnResolution(t *testing.T) {
	useRealDisplayModeDefault(t)
	defer ResetConfig()
	library := map[string][2]int{
		"portrait": {2000, 3200}, "tall": {900, 2400}, "wide": {3200, 1400},
		"panorama": {4800, 1200}, "square": {2000, 2000}, "match": {3840, 2160},
	}
	ids := []string{"portrait", "tall", "wide", "panorama", "square", "match"}

	app := newE2EApp(t, t.TempDir(), NewMockPreferences(), e2eMonitor(0, 1920, 1080), e2eMonitor(1, 1080, 1920))
	app.seed(t, library)
	app.wp.Activate()
	defer app.stop()
	app.showFirstWallpapers(t)
	app.waitUntilSettled(t, filepath.Join(FittedRootDir, FullArtDir), ids)

	const steps = 12
	for step := 1; step <= steps; step++ {
		for _, id := range []int{0, 1} {
			before := len(app.wallpaperCalls(id))
			app.wp.SetNextWallpaper(id, true)
			app.waitForWallpaperCalls(t, id, before+1)
		}
	}

	for _, m := range []Monitor{e2eMonitor(0, 1920, 1080), e2eMonitor(1, 1080, 1920)} {
		res := fmt.Sprintf("%dx%d", m.Rect.Dx(), m.Rect.Dy())
		calls := app.wallpaperCalls(m.ID)
		require.GreaterOrEqual(t, len(calls), steps)
		for _, path := range calls {
			assert.Contains(t, path, res, "monitor %d must only get files made for %s", m.ID, res)
			if _, err := os.Stat(path); err != nil {
				continue // an older file may be removed once rebuilt; the current one is checked below
			}
			size := library[shownID(path)]
			assertWholeOnScreen(t, path, size[0], size[1])
		}
	}
}

// Both monitors rotate on their own shuffled decks, so nothing stops them from showing the same
// artwork at the same time. A10 says "互不重复"; whether that means "never the same artwork on two
// monitors" is a product question for the owner, so this test only measures and logs how often it
// happens instead of failing.
func TestA10_MeasureSameArtworkOnBothMonitors(t *testing.T) {
	useRealDisplayModeDefault(t)
	defer ResetConfig()
	library := map[string][2]int{}
	var ids []string
	for i := 0; i < 8; i++ {
		id := fmt.Sprintf("art%d", i)
		ids = append(ids, id)
		library[id] = [2]int{1600 + 100*i, 1000 + 150*(i%3)}
	}

	app := newE2EApp(t, t.TempDir(), NewMockPreferences(), e2eMonitor(0, 1920, 1080), e2eMonitor(1, 1080, 1920))
	app.seed(t, library)
	app.wp.Activate()
	defer app.stop()
	app.showFirstWallpapers(t)
	app.waitUntilSettled(t, filepath.Join(FittedRootDir, FullArtDir), ids)

	same, rounds := 0, 12
	for r := 0; r < rounds; r++ {
		for _, id := range []int{0, 1} {
			before := len(app.wallpaperCalls(id))
			app.wp.SetNextWallpaper(id, true)
			app.waitForWallpaperCalls(t, id, before+1)
		}
		if shownID(app.os.last(0)) == shownID(app.os.last(1)) {
			same++
		}
	}
	t.Logf("A10 measurement: the same artwork was on both monitors in %d of %d rounds (library of %d)", same, rounds, len(ids))
}
