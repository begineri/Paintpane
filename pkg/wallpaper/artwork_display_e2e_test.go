//go:build !linux

package wallpaper

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"github.com/disintegration/imaging"
	"github.com/dixieflatline76/Spice/v2/asset"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const e2eSettle = 60 * time.Second

// recordingOS stands in for the desktop. It records wallpaper calls instead of changing anything.
type recordingOS struct {
	mu       sync.Mutex
	monitors []Monitor
	set      map[int][]string
}

func (r *recordingOS) GetDesktopDimension() (int, int, error) {
	return r.monitors[0].Rect.Dx(), r.monitors[0].Rect.Dy(), nil
}

func (r *recordingOS) GetMonitors() ([]Monitor, error) { return r.monitors, nil }

func (r *recordingOS) Stat(path string) (os.FileInfo, error) { return os.Stat(path) }

func (r *recordingOS) SetWallpaper(path string, monitorID int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.set[monitorID] = append(r.set[monitorID], path)
	return nil
}

func (r *recordingOS) last(monitorID int) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	calls := r.set[monitorID]
	if len(calls) == 0 {
		return ""
	}
	return calls[len(calls)-1]
}

// e2eApp is the real plugin, pipeline, store, monitor actors and image processors over a temp
// directory. Only the desktop and the plugin manager are fakes, and no provider is registered,
// so nothing can reach the network.
type e2eApp struct {
	wp    *Plugin
	os    *recordingOS
	fm    *FileManager
	store *ImageStore
}

func newE2EApp(t *testing.T, root string, prefs fyne.Preferences, monitors ...Monitor) *e2eApp {
	t.Helper()
	ResetConfig()
	cfg := GetConfig(prefs)
	cfg.Queries = []ImageQuery{{ID: "q1", Active: true}}
	cfg.SetNightlyRefresh(false)
	cfg.SetWallpaperChangeFrequency(FrequencyNever)
	// Smart crop no longer frames what it cannot crop, so let it accept every shape the tests seed.
	cfg.Tuning.FlexibilityAbsoluteMaxDiff = 100
	cfg.Tuning.FlexibilityOrientationMaxDiff = 100

	fm := NewFileManager(root)
	require.NoError(t, fm.EnsureDirs())
	store := NewImageStore()
	store.SetAsyncSave(false)
	store.SetFileManager(fm, filepath.Join(root, "cache.json"))

	desktop := &recordingOS{monitors: monitors, set: map[int][]string{}}

	pm := new(MockPluginManager)
	pm.On("GetPreferences").Return(prefs).Maybe()
	pm.On("GetAssetManager").Return(&asset.Manager{}).Maybe()
	pm.On("RefreshTrayMenu").Return().Maybe()
	pm.On("RebuildTrayMenu").Return().Maybe()

	wp := &Plugin{
		os:                 desktop,
		cfg:                cfg,
		manager:            pm,
		store:              store,
		fm:                 fm,
		runOnUI:            func(f func()) { f() },
		interrupt:          util.NewSafeBool(),
		queryPages:         make(map[string]*util.SafeCounter),
		fitImageFlag:       util.NewSafeBool(),
		shuffleImageFlag:   util.NewSafeBool(),
		fetchingInProgress: util.NewSafeBool(),
		providers:          make(map[string]provider.ImageProvider),
		Monitors:           make(map[int]*MonitorController),
		httpClient:         &http.Client{Transport: failTransport{t: t}},
		imgProcessor:       NewVirtualFramer(NewSmartImageProcessor(desktop, cfg, nil), cfg),
		resetTimerCh:       make(chan struct{}, 1),
		stopNightlyRefresh: make(chan struct{}),
		queryContexts:      make(map[string]context.Context),
		queryCancelFuncs:   make(map[string]context.CancelFunc),
		downloadWaitGroup:  &sync.WaitGroup{},
		actionChan:         make(chan func(), 5),
	}
	return &e2eApp{wp: wp, os: desktop, fm: fm, store: store}
}

// failTransport proves the whole flow is local: any HTTP request fails the test.
type failTransport struct{ t *testing.T }

func (f failTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	f.t.Errorf("unexpected network request: %s", r.URL)
	return nil, fmt.Errorf("network disabled in this test")
}

// seed writes masters and processes them, like a library downloaded in an earlier session.
func (a *e2eApp) seed(t *testing.T, images map[string][2]int) {
	t.Helper()
	for id, size := range images {
		path, err := a.fm.GetMasterPath(id, ".jpg")
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		f, err := os.Create(path)
		require.NoError(t, err)
		require.NoError(t, jpeg.Encode(f, quadrantImage(size[0], size[1]), &jpeg.Options{Quality: 95}))
		require.NoError(t, f.Close())

		img, err := a.wp.ProcessImageJob(context.Background(), DownloadJob{
			Ctx:   context.Background(),
			Image: provider.Image{ID: id, Path: "http://example.invalid/" + id + ".jpg", SourceQueryID: "q1"},
		})
		require.NoError(t, err)
		require.True(t, a.store.Add(img))
	}
}

// waitUntilSettled waits until every image and monitor shows the expected mode.
func (a *e2eApp) waitUntilSettled(t *testing.T, dirPart string, ids []string) {
	t.Helper()
	ok := assert.Eventually(t, func() bool {
		for _, id := range ids {
			img, exists := a.store.GetByID(id)
			if !exists {
				return false
			}
			for _, m := range a.os.monitors {
				path := img.DerivativePaths[fmt.Sprintf("%dx%d", m.Rect.Dx(), m.Rect.Dy())]
				if !strings.Contains(path, dirPart) {
					return false
				}
				if _, err := os.Stat(path); err != nil {
					return false
				}
			}
		}
		for _, m := range a.os.monitors {
			if !strings.Contains(a.os.last(m.ID), dirPart) {
				return false
			}
		}
		return true
	}, e2eSettle, 100*time.Millisecond, "library and wallpaper must settle in %s", dirPart)
	if !ok {
		for _, id := range ids {
			img, _ := a.store.GetByID(id)
			t.Logf("%s: %v", id, img.DerivativePaths)
		}
		for _, m := range a.os.monitors {
			t.Logf("monitor %d last wallpaper: %s", m.ID, a.os.last(m.ID))
		}
		t.FailNow()
	}
}

// settleFiles waits for background deletes, then lists derivative files on disk by directory.
func (a *e2eApp) derivativeFiles(t *testing.T) []string {
	t.Helper()
	require.True(t, a.fm.WaitTimeout(derivativeCleanupWait))
	var files []string
	err := filepath.Walk(filepath.Join(a.fm.GetDownloadDir(), FittedRootDir), func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			files = append(files, path)
		}
		return err
	})
	require.NoError(t, err)
	return files
}

func (a *e2eApp) assertMastersIntact(t *testing.T, ids []string) {
	t.Helper()
	for _, id := range ids {
		path, err := a.fm.GetMasterPath(id, ".jpg")
		require.NoError(t, err)
		assert.FileExists(t, path, "master must never be lost")
	}
}

func e2eMonitor(id, w, h int) Monitor {
	return Monitor{ID: id, Name: fmt.Sprintf("M%d", id), Rect: image.Rect(0, 0, w, h)}
}

func assertWholeOnScreen(t *testing.T, path string, srcW, srcH int) {
	t.Helper()
	got, err := imaging.Open(path)
	require.NoError(t, err)
	w, h := got.Bounds().Dx(), got.Bounds().Dy()
	scale := float64(w) / float64(srcW)
	if v := float64(h) / float64(srcH); v < scale {
		scale = v
	}
	artW, artH := int(float64(srcW)*scale), int(float64(srcH)*scale)
	x0, y0 := (w-artW)/2, (h-artH)/2
	const inset = 10
	assertNearColor(t, quadTL, got.At(x0+inset, y0+inset), "top-left")
	assertNearColor(t, quadTR, got.At(x0+artW-inset, y0+inset), "top-right")
	assertNearColor(t, quadBL, got.At(x0+inset, y0+artH-inset), "bottom-left")
	assertNearColor(t, quadBR, got.At(x0+artW-inset, y0+artH-inset), "bottom-right")
}

func (a *e2eApp) stop() {
	a.wp.Deactivate()
	a.fm.WaitTimeout(derivativeCleanupWait)
}

func TestNext_ReplacesStaleDeckOfTheSameSize(t *testing.T) {
	tmp := t.TempDir()
	fm := NewFileManager(tmp)
	store := NewImageStore()
	store.SetAsyncSave(false)
	store.SetFileManager(fm, filepath.Join(tmp, "cache.json"))

	rebuilt := filepath.Join(tmp, "rebuilt.jpg")
	require.NoError(t, os.WriteFile(rebuilt, []byte("x"), 0600))
	require.True(t, store.Add(provider.Image{ID: "not_rebuilt_yet", FilePath: filepath.Join(tmp, "deleted.jpg")}))
	require.True(t, store.Add(provider.Image{ID: "rebuilt", FilePath: rebuilt, DerivativePaths: map[string]string{"800x500": rebuilt}}))

	desktop := &recordingOS{monitors: []Monitor{e2eMonitor(0, 800, 500)}, set: map[int][]string{}}
	mc := NewMonitorController(0, desktop.monitors[0], store, fm, desktop, nil, nil)
	mc.State.ShuffleIDs = []string{"not_rebuilt_yet"} // Left over from before the switch

	mc.next(true)

	assert.Equal(t, rebuilt, desktop.last(0))
	assert.Equal(t, []string{"rebuilt"}, mc.State.ShuffleIDs)
}

func TestE2E_SwitchingThroughAllModes(t *testing.T) {
	defer ResetConfig()
	library := map[string][2]int{"portrait": {500, 1000}, "wide": {1600, 700}, "square": {900, 900}, "match": {1600, 1000}}
	ids := []string{"portrait", "wide", "square", "match"}

	app := newE2EApp(t, t.TempDir(), NewMockPreferences(), e2eMonitor(0, 800, 500), e2eMonitor(1, 500, 800))
	app.wp.cfg.SetSmartFitMode(SmartFitAggressive)
	app.seed(t, library)
	app.wp.Activate()
	defer app.stop()

	steps := []struct {
		mode ArtworkDisplayMode
		dir  string
	}{
		{ArtworkDisplayFull, filepath.Join(FittedRootDir, FullArtDir)},
		{ArtworkDisplayFramed, filepath.Join(FittedRootDir, FramedArtDir)},
		{ArtworkDisplayFull, filepath.Join(FittedRootDir, FullArtDir)},
		{ArtworkDisplaySmartCrop, filepath.Join(FittedRootDir, FlexibilityDir)},
	}
	for _, step := range steps {
		app.wp.SetArtworkDisplayMode(step.mode)
		app.waitUntilSettled(t, step.dir, ids)

		// The current mode must be complete. Files of other modes are kept for the way back.
		inMode := 0
		for _, f := range app.derivativeFiles(t) {
			if strings.Contains(f, step.dir) {
				inMode++
			}
		}
		assert.Equal(t, len(ids)*2, inMode, "one derivative per image per monitor after %v", step.mode)

		if step.mode == ArtworkDisplayFull {
			for _, m := range app.os.monitors {
				res := fmt.Sprintf("%dx%d", m.Rect.Dx(), m.Rect.Dy())
				for id, size := range library {
					img, _ := app.store.GetByID(id)
					assertWholeOnScreen(t, img.DerivativePaths[res], size[0], size[1])
				}
			}
		}
	}
	app.assertMastersIntact(t, ids)
}

func TestE2E_FullArtworkWorksWithSmartFitDisabled(t *testing.T) {
	defer ResetConfig()
	app := newE2EApp(t, t.TempDir(), NewMockPreferences(), e2eMonitor(0, 800, 500))
	app.wp.cfg.SetSmartFitMode(SmartFitOff)
	app.wp.cfg.SetSmartFit(false)
	app.seed(t, map[string][2]int{"portrait": {500, 1000}})
	app.wp.Activate()
	defer app.stop()

	app.wp.SetArtworkDisplayMode(ArtworkDisplayFull)
	app.waitUntilSettled(t, filepath.Join(FittedRootDir, FullArtDir), []string{"portrait"})
	assertWholeOnScreen(t, app.os.last(0), 500, 1000)
}

func TestE2E_RapidSwitchingEndsOnLastChoice(t *testing.T) {
	defer ResetConfig()
	ids := []string{"a", "b", "c", "d", "e", "f"}
	library := map[string][2]int{}
	for _, id := range ids {
		library[id] = [2]int{1200, 900}
	}
	app := newE2EApp(t, t.TempDir(), NewMockPreferences(), e2eMonitor(0, 800, 500))
	app.wp.cfg.SetSmartFitMode(SmartFitAggressive)
	app.seed(t, library)
	app.wp.Activate()
	defer app.stop()

	for i := 0; i < 3; i++ {
		app.wp.SetArtworkDisplayMode(ArtworkDisplayFull)
		time.Sleep(30 * time.Millisecond)
		app.wp.SetArtworkDisplayMode(ArtworkDisplayFramed)
		time.Sleep(30 * time.Millisecond)
	}
	app.wp.SetArtworkDisplayMode(ArtworkDisplayFull)

	dir := filepath.Join(FittedRootDir, FullArtDir)
	app.waitUntilSettled(t, dir, ids)
	app.assertMastersIntact(t, ids)

	// A little later the result must still hold, with no late job pointing images elsewhere.
	time.Sleep(500 * time.Millisecond)
	app.waitUntilSettled(t, dir, ids)
}

func TestE2E_RestartKeepsModeAndDoesNotRebuild(t *testing.T) {
	defer ResetConfig()
	root := t.TempDir()
	prefs := NewMockPreferences()
	ids := []string{"portrait", "wide"}

	app := newE2EApp(t, root, prefs, e2eMonitor(0, 800, 500))
	app.seed(t, map[string][2]int{"portrait": {500, 1000}, "wide": {1600, 700}})
	app.wp.Activate()
	app.wp.SetArtworkDisplayMode(ArtworkDisplayFramed)
	dir := filepath.Join(FittedRootDir, FramedArtDir)
	app.waitUntilSettled(t, dir, ids)
	require.True(t, app.store.WaitTimeout(derivativeCleanupWait))
	files := app.derivativeFiles(t)
	stamps := map[string]time.Time{}
	for _, f := range files {
		info, err := os.Stat(f)
		require.NoError(t, err)
		stamps[f] = info.ModTime()
	}
	app.stop()

	restarted := newE2EApp(t, root, prefs, e2eMonitor(0, 800, 500))
	assert.Equal(t, ArtworkDisplayFramed, restarted.wp.cfg.GetArtworkDisplayMode())
	restarted.wp.Activate()
	defer restarted.stop()

	restarted.wp.RefreshImagesAndPulse()
	assert.Eventually(t, func() bool { return strings.Contains(restarted.os.last(0), dir) }, e2eSettle, 100*time.Millisecond)
	require.True(t, restarted.store.WaitTimeout(derivativeCleanupWait))
	time.Sleep(500 * time.Millisecond)

	for f, stamp := range stamps {
		info, err := os.Stat(f)
		require.NoError(t, err, "derivative must survive a restart")
		assert.Equal(t, stamp, info.ModTime(), "framed derivative must not be rebuilt on restart: %s", f)
	}
}

func TestE2E_TuneImageOverridesFollowTheDecisionTable(t *testing.T) {
	defer ResetConfig()
	app := newE2EApp(t, t.TempDir(), NewMockPreferences(), e2eMonitor(0, 800, 500))
	app.seed(t, map[string][2]int{"portrait": {500, 1000}})
	app.wp.Activate()
	defer app.stop()

	const res = "800x500"
	framedNow := func() bool {
		app.wp.monMu.RLock()
		mc := app.wp.Monitors[0]
		app.wp.monMu.RUnlock()
		mc.mu.RLock()
		defer mc.mu.RUnlock()
		return mc.State.CurrentImage.ProcessingFlags["VirtualFramed:"+res]
	}

	app.wp.SetArtworkDisplayMode(ArtworkDisplayFramed)
	app.waitUntilSettled(t, filepath.Join(FittedRootDir, FramedArtDir), []string{"portrait"})
	require.Eventually(t, framedNow, e2eSettle, 50*time.Millisecond, "Museum Frame frames by default")

	app.os.mu.Lock()
	calls := len(app.os.set[0])
	app.os.mu.Unlock()
	app.wp.SetTuningOptions(0, provider.TuningOptions{FrameOverride: provider.FrameOverrideForceOff})
	require.Eventually(t, func() bool {
		app.os.mu.Lock()
		defer app.os.mu.Unlock()
		return len(app.os.set[0]) > calls
	}, e2eSettle, 50*time.Millisecond)
	assert.False(t, framedNow(), "unframing in Museum Frame draws no frame")
	assertWholeOnScreen(t, app.os.last(0), 500, 1000)

	app.wp.cfg.SetArtworkDisplayMode(ArtworkDisplayFull)
	app.os.mu.Lock()
	calls = len(app.os.set[0])
	app.os.mu.Unlock()
	app.wp.SetTuningOptions(0, provider.TuningOptions{FrameOverride: provider.FrameOverrideForceOn})
	require.Eventually(t, func() bool {
		app.os.mu.Lock()
		defer app.os.mu.Unlock()
		return len(app.os.set[0]) > calls
	}, e2eSettle, 50*time.Millisecond)
	assert.True(t, framedNow(), "framing one image in Full Artwork still works")
}

// P26: under Full artwork one image can be set to Smart crop in Tune Image; the desktop then shows
// it filling the screen, and following the setting again shows it whole.
func TestE2E_TuneImageSmartCropUnderFullArtwork(t *testing.T) {
	defer ResetConfig()
	app := newE2EApp(t, t.TempDir(), NewMockPreferences(), e2eMonitor(0, 800, 500))
	app.seed(t, map[string][2]int{"portrait": {500, 1000}})
	app.wp.Activate()
	defer app.stop()

	app.wp.SetArtworkDisplayMode(ArtworkDisplayFull)
	app.waitUntilSettled(t, filepath.Join(FittedRootDir, FullArtDir), []string{"portrait"})

	apply := func(opts provider.TuningOptions) string {
		t.Helper()
		app.os.mu.Lock()
		calls := len(app.os.set[0])
		app.os.mu.Unlock()
		app.wp.SetTuningOptions(0, opts)
		require.Eventually(t, func() bool {
			app.os.mu.Lock()
			defer app.os.mu.Unlock()
			return len(app.os.set[0]) > calls
		}, e2eSettle, 50*time.Millisecond)
		return app.os.last(0)
	}

	shown, err := imaging.Open(apply(provider.TuningOptions{Display: provider.DisplayCrop}))
	require.NoError(t, err)
	b := shown.Bounds()
	// Filling the screen, the picture reaches both side edges: no wall on the left or right.
	left, right := shown.At(b.Min.X+3, b.Dy()/2), shown.At(b.Max.X-4, b.Dy()/2)
	assert.True(t, nearAny(left, quadTL, quadBL), "the left edge is the picture's left half")
	assert.True(t, nearAny(right, quadTR, quadBR), "the right edge is the picture's right half")

	assertWholeOnScreen(t, apply(provider.TuningOptions{}), 500, 1000)
}

func nearAny(got color.Color, want ...color.RGBA) bool {
	r, g, b, _ := got.RGBA()
	for _, w := range want {
		d := func(a uint8, b uint32) bool { x := int(a) - int(b>>8); return x > -40 && x < 40 }
		if d(w.R, r) && d(w.G, g) && d(w.B, b) {
			return true
		}
	}
	return false
}
