//go:build !linux

package wallpaper

import (
	"context"
	"encoding/json"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countingFramer counts real renders, so a test can prove a switch reused files.
type countingFramer struct {
	*VirtualFramer
	renders atomic.Int64
}

func (c *countingFramer) FitImage(ctx context.Context, img image.Image, w, h int, opts provider.TuningOptions) (image.Image, error) {
	c.renders.Add(1)
	return c.VirtualFramer.FitImage(ctx, img, w, h, opts)
}

func newCountingE2EApp(t *testing.T, monitors ...Monitor) (*e2eApp, *countingFramer) {
	t.Helper()
	app := newE2EApp(t, t.TempDir(), NewMockPreferences(), monitors...)
	counter := &countingFramer{VirtualFramer: app.wp.imgProcessor.(*VirtualFramer)}
	app.wp.imgProcessor = counter
	return app, counter
}

func filesUnder(t *testing.T, root string) map[string]time.Time {
	t.Helper()
	files := map[string]time.Time{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err == nil && !info.IsDir() {
			files[path] = info.ModTime()
		}
		return err
	})
	require.NoError(t, err)
	return files
}

func TestE2E_ReturningToAModeRendersNothing(t *testing.T) {
	defer ResetConfig()
	ids := []string{"portrait", "wide", "square"}
	app, counter := newCountingE2EApp(t, e2eMonitor(0, 800, 500))
	app.wp.cfg.SetSmartFitMode(SmartFitAggressive)
	app.seed(t, map[string][2]int{"portrait": {500, 1000}, "wide": {1600, 700}, "square": {900, 900}})
	app.wp.Activate()
	defer app.stop()

	fitted := filepath.Join(app.fm.GetDownloadDir(), FittedRootDir)
	fullDir := filepath.Join(FittedRootDir, FullArtDir)
	framedDir := filepath.Join(FittedRootDir, FramedArtDir)
	smartDir := filepath.Join(FittedRootDir, FlexibilityDir)

	app.wp.SetArtworkDisplayMode(ArtworkDisplayFull)
	app.waitUntilSettled(t, fullDir, ids)
	app.wp.SetArtworkDisplayMode(ArtworkDisplayFramed)
	app.waitUntilSettled(t, framedDir, ids)

	require.True(t, app.fm.WaitForDerivativeDeletes(context.Background()))
	before := filesUnder(t, fitted)
	assert.Len(t, before, len(ids)*3, "smart crop, full and framed files are all kept")
	rendered := counter.renders.Load()

	for _, step := range []struct {
		mode ArtworkDisplayMode
		dir  string
	}{
		{ArtworkDisplayFull, fullDir},
		{ArtworkDisplaySmartCrop, smartDir},
		{ArtworkDisplayFramed, framedDir},
		{ArtworkDisplayFull, fullDir},
	} {
		start := time.Now()
		app.wp.SetArtworkDisplayMode(step.mode)
		app.waitUntilSettled(t, step.dir, ids)
		t.Logf("switched to %v in %v", step.mode, time.Since(start).Round(time.Millisecond))
	}

	assert.Equal(t, rendered, counter.renders.Load(), "a mode already visited must render nothing")
	assert.Equal(t, before, filesUnder(t, fitted), "kept files must be neither removed nor rewritten")
	app.assertMastersIntact(t, ids)
}

func TestE2E_OtherSettingChangeStillRegenerates(t *testing.T) {
	defer ResetConfig()
	ids := []string{"portrait"}
	app, counter := newCountingE2EApp(t, e2eMonitor(0, 800, 500))
	app.wp.cfg.SetArtworkDisplayMode(ArtworkDisplayFull)
	app.seed(t, map[string][2]int{"portrait": {500, 1000}})
	app.wp.Activate()
	defer app.stop()

	fullDir := filepath.Join(FittedRootDir, FullArtDir)
	framedDir := filepath.Join(FittedRootDir, FramedArtDir)
	app.wp.SetArtworkDisplayMode(ArtworkDisplayFramed)
	app.waitUntilSettled(t, framedDir, ids)

	// Wall color is not part of the path, so both kept files are now wrong.
	rendered := counter.renders.Load()
	app.wp.cfg.VirtualWallColor = WallNeutral
	app.wp.RefreshImagesAndPulse()

	require.Eventually(t, func() bool {
		return counter.renders.Load() > rendered && len(app.derivativeFiles(t)) == 1
	}, e2eSettle, 100*time.Millisecond, "the full artwork file must be deleted and framed re-rendered")

	rendered = counter.renders.Load()
	app.wp.SetArtworkDisplayMode(ArtworkDisplayFull)
	app.waitUntilSettled(t, fullDir, ids)
	assert.Greater(t, counter.renders.Load(), rendered, "the deleted mode must render again")
}

func TestE2E_TuningRemovesOnlyStaleFilesOfOtherModes(t *testing.T) {
	defer ResetConfig()
	app, counter := newCountingE2EApp(t, e2eMonitor(0, 800, 500))
	app.wp.cfg.SetSmartFitMode(SmartFitAggressive)
	app.seed(t, map[string][2]int{"portrait": {500, 1000}, "other": {1200, 900}})
	app.wp.Activate()
	defer app.stop()

	ids := []string{"portrait", "other"}
	fullDir := filepath.Join(FittedRootDir, FullArtDir)
	framedDir := filepath.Join(FittedRootDir, FramedArtDir)
	app.wp.SetArtworkDisplayMode(ArtworkDisplayFramed)
	app.waitUntilSettled(t, framedDir, ids)
	app.wp.SetArtworkDisplayMode(ArtworkDisplayFull)
	app.waitUntilSettled(t, fullDir, ids)

	app.wp.monMu.RLock()
	mc := app.wp.Monitors[0]
	app.wp.monMu.RUnlock()
	var tuned string
	require.Eventually(t, func() bool {
		mc.mu.RLock()
		defer mc.mu.RUnlock()
		tuned = mc.State.CurrentImage.ID
		return tuned != ""
	}, e2eSettle, 20*time.Millisecond)
	var untouched string
	for _, id := range ids {
		if id != tuned {
			untouched = id
		}
	}

	rendered := counter.renders.Load()
	calls := func() int {
		app.os.mu.Lock()
		defer app.os.mu.Unlock()
		return len(app.os.set[0])
	}
	before := calls()
	app.wp.SetTuningOptions(0, provider.TuningOptions{FrameOverride: provider.FrameOverrideForceOn})
	require.Eventually(t, func() bool { return calls() > before }, e2eSettle, 20*time.Millisecond)

	root := filepath.Join(app.fm.GetDownloadDir(), FittedRootDir)
	assert.FileExists(t, filepath.Join(root, FullArtDir, StandardDir, "800x500", tuned+".jpg"), "the current file is kept")
	assert.NoFileExists(t, filepath.Join(root, FramedArtDir, StandardDir, "800x500", tuned+".jpg"), "the other mode's file is stale")
	assert.FileExists(t, filepath.Join(root, FramedArtDir, StandardDir, "800x500", untouched+".jpg"), "other images keep their files")

	// Back in the other mode, only the tuned image has to render.
	rendered = counter.renders.Load()
	app.wp.SetArtworkDisplayMode(ArtworkDisplayFramed)
	app.waitUntilSettled(t, framedDir, ids)
	assert.Equal(t, rendered+1, counter.renders.Load(), "exactly the tuned image renders again")
}

func TestE2E_UpgradeMovesLibraryToDefaultAtStartup(t *testing.T) {
	defer ResetConfig()
	useRealDisplayModeDefault(t)
	root := t.TempDir()
	prefs := NewMockPreferences()
	ids := []string{"portrait", "wide"}

	// A library from before this setting existed: Smart Crop files and a config without a mode.
	old := newE2EApp(t, root, prefs, e2eMonitor(0, 800, 500))
	old.wp.cfg.SetArtworkDisplayMode(ArtworkDisplaySmartCrop)
	old.wp.cfg.SetSmartFitMode(SmartFitAggressive)
	old.seed(t, map[string][2]int{"portrait": {500, 1000}, "wide": {1600, 700}})
	require.True(t, old.store.WaitTimeout(derivativeCleanupWait))
	smartFiles := filesUnder(t, filepath.Join(root, FittedRootDir, FlexibilityDir))
	require.Len(t, smartFiles, len(ids))

	var saved map[string]any
	require.NoError(t, json.Unmarshal([]byte(prefs.String(WallhavenConfigPrefKey)), &saved))
	delete(saved, "artwork_display_mode")
	data, err := json.Marshal(saved)
	require.NoError(t, err)
	prefs.SetString(WallhavenConfigPrefKey, string(data))

	app := newE2EApp(t, root, prefs, e2eMonitor(0, 800, 500))
	require.Equal(t, ArtworkDisplayFull, app.wp.cfg.GetArtworkDisplayMode())
	app.wp.Activate()
	defer app.stop()

	dir := filepath.Join(FittedRootDir, FullArtDir)
	assert.Eventually(t, func() bool {
		for _, id := range ids {
			img, ok := app.store.GetByID(id)
			if !ok || !strings.Contains(img.DerivativePaths["800x500"], dir) {
				return false
			}
		}
		return true
	}, e2eSettle, 50*time.Millisecond, "the library must move to Full Artwork at startup")
	app.assertMastersIntact(t, ids)
	assert.Equal(t, smartFiles, filesUnder(t, filepath.Join(root, FittedRootDir, FlexibilityDir)), "Smart Crop files are kept")
}

func TestReusedDerivativeReportsFramedFlag(t *testing.T) {
	env := newDisplayTestEnv(t)
	env.wp.cfg.SetSmartFitMode(SmartFitAggressive)
	// Smart crop no longer frames what it cannot crop, so let it accept this portrait.
	env.wp.cfg.Tuning.FlexibilityAbsoluteMaxDiff = 100
	env.wp.cfg.Tuning.FlexibilityOrientationMaxDiff = 100
	env.addLibraryImage(t, "portrait", 500, 1000)
	flag := "VirtualFramed:800x500"
	framed := func() bool {
		img, ok := env.store.GetByID("portrait")
		require.True(t, ok)
		return img.ProcessingFlags[flag]
	}
	smartCropRendered := framed()

	env.wp.cfg.SetArtworkDisplayMode(ArtworkDisplayFull)
	env.apply(t)
	require.True(t, env.fm.WaitForDerivativeDeletes(context.Background()))
	assert.False(t, framed(), "Full Artwork draws no frame")

	env.wp.cfg.SetArtworkDisplayMode(ArtworkDisplayFramed)
	env.apply(t)
	assert.True(t, framed(), "Museum Frame frames")

	// Every step from here reuses a file, so the flag can only come from the decision.
	env.wp.cfg.SetArtworkDisplayMode(ArtworkDisplayFull)
	env.apply(t)
	assert.False(t, framed(), "reused Full Artwork file")

	env.wp.cfg.SetArtworkDisplayMode(ArtworkDisplayFramed)
	env.apply(t)
	assert.True(t, framed(), "reused Museum Frame file")

	env.wp.cfg.SetArtworkDisplayMode(ArtworkDisplaySmartCrop)
	env.apply(t)
	assert.Equal(t, smartCropRendered, framed(), "reused Smart Crop file reports what rendering reported")
}

func TestWouldFrame_MatchesFitImage(t *testing.T) {
	sizes := [][2]int{{500, 1000}, {1000, 800}, {1600, 1000}, {3000, 700}, {900, 900}}
	overrides := []provider.FrameOverrideMode{provider.FrameOverrideInherit, provider.FrameOverrideForceOn, provider.FrameOverrideForceOff}
	modes := []ArtworkDisplayMode{ArtworkDisplaySmartCrop, ArtworkDisplayFull, ArtworkDisplayFramed}

	env := newDisplayTestEnv(t)
	cfg := env.wp.cfg
	cfg.SetSmartFitMode(SmartFitNormal)
	framer := env.wp.imgProcessor.(*VirtualFramer)

	for _, mode := range modes {
		cfg.SetArtworkDisplayMode(mode)
		for _, size := range sizes {
			for _, override := range overrides {
				opts := provider.TuningOptions{FrameOverride: override}
				rendered := false
				ctx := context.WithValue(context.Background(), provider.VirtualFramedKey, &rendered)
				_, err := framer.FitImage(ctx, quadrantImage(size[0], size[1]), displayTestW, displayTestH, opts)
				require.NoError(t, err)
				assert.Equal(t, rendered, framer.WouldFrame("", size[0], size[1], displayTestW, displayTestH, opts),
					"mode %v size %v override %v", mode, size, override)
			}
		}
	}

	assert.False(t, framer.WouldFrame("", 0, 0, displayTestW, displayTestH, provider.TuningOptions{}), "unknown size")
}

func TestSync_KeepsFilesOnlyForDisplayModeChanges(t *testing.T) {
	base := map[string]bool{"SmartFit": true, "Matting": true, "FrameSize_0.85": true, flagDisplayFull: true, flagDisplayFramed: false}
	tests := []struct {
		name   string
		change func(target map[string]bool)
		want   ImageSyncAction
	}{
		{"same settings", func(map[string]bool) {}, ImageActionKeep},
		{"full to framed", func(m map[string]bool) { m[flagDisplayFull] = false; m[flagDisplayFramed] = true }, ImageActionRepoint},
		{"full to smart crop", func(m map[string]bool) { m[flagDisplayFull] = false }, ImageActionRepoint},
		{"matting changed", func(m map[string]bool) { m["Matting"] = false }, ImageActionInvalidate},
		{"frame size changed", func(m map[string]bool) { delete(m, "FrameSize_0.85"); m["FrameSize_0.90"] = true }, ImageActionInvalidate},
		{"mode and matting changed", func(m map[string]bool) { m[flagDisplayFull] = false; m["Matting"] = false }, ImageActionInvalidate},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			fm := NewFileManager(tmp)
			store := NewImageStore()
			store.SetAsyncSave(false)
			store.SetFileManager(fm, filepath.Join(tmp, "cache.json"))
			createMasterFile(t, fm, "img")

			flags := map[string]bool{"VirtualFramed:800x500": true, "incompatible:1x1": true}
			target := map[string]bool{}
			for k, v := range base {
				flags[k] = v
				target[k] = v
			}
			tt.change(target)

			img := provider.Image{ID: "img", ProcessingFlags: flags}
			assert.Equal(t, tt.want, store.determineSyncAction(img, nil, target))
		})
	}
}

func TestSync_RepointKeepsFilesAndForgetsPaths(t *testing.T) {
	tmp := t.TempDir()
	fm := NewFileManager(tmp)
	store := NewImageStore()
	store.SetAsyncSave(false)
	store.SetFileManager(fm, filepath.Join(tmp, "cache.json"))
	createMasterFile(t, fm, "img")

	path, err := fm.GetDerivativePath("img", ".jpg", filepath.Join(FittedRootDir, FullArtDir, StandardDir, "800x500"))
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte("x"), 0600))
	require.True(t, store.Add(provider.Image{
		ID:              "img",
		ProcessingFlags: map[string]bool{flagDisplayFull: true},
		DerivativePaths: map[string]string{"800x500": path},
	}))

	store.Sync(100, map[string]bool{flagDisplayFramed: true}, nil)
	require.True(t, fm.WaitForDerivativeDeletes(context.Background()))
	require.True(t, fm.WaitTimeout(derivativeCleanupWait))

	got, ok := store.GetByID("img")
	require.True(t, ok)
	assert.Empty(t, got.DerivativePaths, "the image must not keep pointing at the old mode")
	assert.True(t, got.ProcessingFlags[flagDisplayFramed])
	assert.False(t, got.ProcessingFlags[flagDisplayFull])
	assert.FileExists(t, path, "the file is kept for the return to this mode")
}

// blockingSubmitter never accepts a job until released, like a pipeline busy with slow work.
type blockingSubmitter struct {
	release chan struct{}
	jobs    chan DownloadJob
}

func (b *blockingSubmitter) Submit(_ context.Context, job DownloadJob) bool {
	b.jobs <- job
	<-b.release
	return true
}

func TestRebuild_ReusedImagesDoNotWaitForTheQueue(t *testing.T) {
	env := newDisplayTestEnv(t)
	for _, id := range []string{"a_needs_render", "b_kept", "c_kept", "d_kept"} {
		env.addLibraryImage(t, id, 1000, 800)
	}
	env.wp.cfg.SetArtworkDisplayMode(ArtworkDisplayFull)
	env.apply(t) // Renders all four in Full Artwork
	env.wp.cfg.SetArtworkDisplayMode(ArtworkDisplaySmartCrop)
	env.apply(t)
	require.True(t, env.fm.WaitForDerivativeDeletes(context.Background()))

	// Only one image has no Full Artwork file, and it comes first in the library.
	fullPath := func(id string) string {
		return filepath.Join(env.fm.GetDownloadDir(), FittedRootDir, FullArtDir, StandardDir, "800x500", id+".jpg")
	}
	require.NoError(t, os.Remove(fullPath("a_needs_render")))
	env.wp.cfg.SetArtworkDisplayMode(ArtworkDisplayFull)
	env.wp.syncStoreWithConfig()

	sub := &blockingSubmitter{release: make(chan struct{}), jobs: make(chan DownloadJob, 4)}
	env.wp.jobSubmitter = sub
	done := make(chan struct{})
	go func() {
		env.wp.rebuildMissingDerivatives()
		close(done)
	}()

	job := <-sub.jobs // The queue is now stuck on the image that needs rendering.
	assert.Equal(t, "a_needs_render", job.Image.ID)
	for _, id := range []string{"b_kept", "c_kept", "d_kept"} {
		assert.Equal(t, fullPath(id), env.derivativePath(t, id), "%s must be back while the queue is stuck", id)
		img, _ := env.store.GetByID(id)
		assert.True(t, img.ProcessingFlags[flagDisplayFull], "%s must carry the current settings", id)
	}
	close(sub.release)
	<-done
}

func TestReattachDerivatives_NeedsEveryResolution(t *testing.T) {
	env := newDisplayTestEnv(t)
	env.wp.cfg.SetArtworkDisplayMode(ArtworkDisplayFull)
	env.addLibraryImage(t, "img", 1000, 800)
	img, _ := env.store.GetByID("img")
	img.DerivativePaths = nil

	two := []Resolution{{Width: 800, Height: 500}, {Width: 500, Height: 800}}
	_, ok := env.wp.reattachDerivatives(img, two)
	assert.False(t, ok, "the second monitor has no file yet")

	attached, ok := env.wp.reattachDerivatives(img, two[:1])
	require.True(t, ok)
	assert.NotEmpty(t, attached.DerivativePaths["800x500"])
	assert.Equal(t, attached.DerivativePaths["800x500"], attached.FilePath)
	assert.Nil(t, img.DerivativePaths, "the store's copy must not be modified")
}

func TestIsStaleDisplayMode(t *testing.T) {
	tmp := t.TempDir()
	store := NewImageStore()
	store.SetAsyncSave(false)
	store.SetFileManager(NewFileManager(tmp), filepath.Join(tmp, "cache.json"))
	full := provider.Image{ProcessingFlags: map[string]bool{flagDisplayFull: true}}
	smart := provider.Image{ProcessingFlags: map[string]bool{"SmartFit": true}}

	assert.False(t, store.isStaleDisplayMode(full), "nothing to compare with before the first Sync")

	store.Sync(100, map[string]bool{flagDisplayFull: true, flagDisplayFramed: false}, nil)
	assert.False(t, store.isStaleDisplayMode(full))
	assert.True(t, store.isStaleDisplayMode(smart))

	store.Sync(100, map[string]bool{flagDisplayFull: false, flagDisplayFramed: false, "SmartFit": true}, nil)
	assert.True(t, store.isStaleDisplayMode(full))
	assert.False(t, store.isStaleDisplayMode(smart))
}

func TestFailedLocalRebuildKeepsFilesOfOtherModes(t *testing.T) {
	env := newDisplayTestEnv(t)
	env.wp.cfg.SetArtworkDisplayMode(ArtworkDisplayFull)
	env.addLibraryImage(t, "kept", 1000, 800)
	fullPath := env.derivativePath(t, "kept")
	require.FileExists(t, fullPath)

	img, _ := env.store.GetByID("kept")
	env.wp.cfg.SetArtworkDisplayMode(ArtworkDisplayFramed)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := env.wp.ProcessImageJob(cancelled, DownloadJob{Ctx: cancelled, Image: imageForRebuild(img)})
	require.Error(t, err)
	require.True(t, env.fm.WaitTimeout(derivativeCleanupWait))

	assert.FileExists(t, fullPath, "a cancelled rebuild must not wipe another mode's files")
}

func TestSaveDerivative_LeavesNoTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "img.jpg")

	require.NoError(t, saveDerivative(quadrantImage(40, 20), target))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "img.jpg", entries[0].Name())

	// Failing to encode must leave nothing behind, least of all at the final path.
	bad := filepath.Join(dir, "img.unsupported")
	assert.Error(t, saveDerivative(quadrantImage(40, 20), bad))
	assert.NoFileExists(t, bad)
	entries, err = os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestDeleteOtherDerivativesAtResolution(t *testing.T) {
	fm := NewFileManager(t.TempDir())
	root := filepath.Join(fm.GetDownloadDir(), FittedRootDir)
	write := func(parts ...string) string {
		path := filepath.Join(append([]string{root}, parts...)...)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
		require.NoError(t, os.WriteFile(path, []byte("x"), 0600))
		return path
	}
	keep := write(FullArtDir, StandardDir, "800x500", "img.jpg")
	stale := write(FramedArtDir, StandardDir, "800x500", "img.jpg")
	otherRes := write(FramedArtDir, StandardDir, "500x800", "img.jpg")
	otherImage := write(FramedArtDir, StandardDir, "800x500", "other.jpg")

	fm.DeleteOtherDerivativesAtResolution("img", "800x500", keep)

	assert.FileExists(t, keep)
	assert.NoFileExists(t, stale)
	assert.FileExists(t, otherRes, "tuning is per resolution")
	assert.FileExists(t, otherImage)

	fm.DeleteOtherDerivativesAtResolution("img", "500x800", filepath.Join(root, "missing.jpg"))
	assert.FileExists(t, otherRes, "with nothing to keep, nothing is deleted")
}

func TestDeleteOtherDerivatives_RecognisesFileThroughJunction(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("uses a Windows junction")
	}
	tmp := t.TempDir()
	target := filepath.Join(tmp, "moved")
	link := filepath.Join(tmp, "wallpaper_downloads")
	require.NoError(t, os.Mkdir(target, 0755))
	out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput()
	require.NoError(t, err, string(out))

	fm := NewFileManager(link) // Resolves to the target
	rel := filepath.Join(FittedRootDir, FullArtDir, StandardDir, "800x500", "img.jpg")
	real := filepath.Join(target, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(real), 0755))
	require.NoError(t, os.WriteFile(real, []byte("x"), 0600))

	// A path stored before the root was resolved still goes through the junction.
	stored := filepath.Join(link, rel)
	require.False(t, strings.EqualFold(stored, real))

	fm.DeleteOtherDerivativesAtResolution("img", "800x500", stored)
	assert.FileExists(t, real, "the current file must be recognised through its old path")
}
