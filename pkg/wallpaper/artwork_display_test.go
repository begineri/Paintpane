//go:build !linux

package wallpaper

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/disintegration/imaging"
	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	displayTestW = 800
	displayTestH = 500
)

var (
	quadTL = color.RGBA{255, 0, 0, 255}
	quadTR = color.RGBA{0, 255, 0, 255}
	quadBL = color.RGBA{0, 0, 255, 255}
	quadBR = color.RGBA{255, 255, 0, 255}
)

// quadrantImage has a different color in each corner, so any crop or misplacement is detectable.
func quadrantImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			switch {
			case x < w/2 && y < h/2:
				img.SetRGBA(x, y, quadTL)
			case y < h/2:
				img.SetRGBA(x, y, quadTR)
			case x < w/2:
				img.SetRGBA(x, y, quadBL)
			default:
				img.SetRGBA(x, y, quadBR)
			}
		}
	}
	return img
}

func assertNearColor(t *testing.T, want color.RGBA, got color.Color, msg string) {
	t.Helper()
	r, g, b, _ := got.RGBA()
	near := func(a uint8, b uint32) bool {
		d := int(a) - int(b>>8)
		return d > -40 && d < 40
	}
	assert.True(t, near(want.R, r) && near(want.G, g) && near(want.B, b), "%s: want %v, got %v %v %v", msg, want, r>>8, g>>8, b>>8)
}

func TestFitImage_DecisionTable(t *testing.T) {
	const (
		crop  = "crop"
		full  = "full"
		frame = "frame"
	)
	tests := []struct {
		name     string
		mode     ArtworkDisplayMode
		override provider.FrameOverrideMode
		want     string
	}{
		{"smart crop force on", ArtworkDisplaySmartCrop, provider.FrameOverrideForceOn, frame},
		{"smart crop force off", ArtworkDisplaySmartCrop, provider.FrameOverrideForceOff, crop},
		{"full inherit", ArtworkDisplayFull, provider.FrameOverrideInherit, full},
		{"full force on", ArtworkDisplayFull, provider.FrameOverrideForceOn, frame},
		{"full force off", ArtworkDisplayFull, provider.FrameOverrideForceOff, full},
		{"framed inherit", ArtworkDisplayFramed, provider.FrameOverrideInherit, frame},
		{"framed force on", ArtworkDisplayFramed, provider.FrameOverrideForceOn, frame},
		{"framed force off", ArtworkDisplayFramed, provider.FrameOverrideForceOff, full},
		{"unknown mode behaves as smart crop", ArtworkDisplayMode(99), provider.FrameOverrideForceOff, crop},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := new(MockImageProcessor)
			cfg := &Config{ArtworkDisplayMode: tt.mode, VirtualFrameSize: 0.85}
			framer := NewVirtualFramer(next, cfg)
			src := quadrantImage(200, 400)
			opts := provider.TuningOptions{FrameOverride: tt.override}

			if tt.want == crop {
				next.On("FitImage", mock.Anything, src, displayTestW, displayTestH, opts).
					Return(createColoredTestImage(displayTestW, displayTestH, color.Black), nil)
			}

			framed := false
			ctx := context.WithValue(context.Background(), provider.VirtualFramedKey, &framed)
			out, err := framer.FitImage(ctx, src, displayTestW, displayTestH, opts)

			require.NoError(t, err)
			next.AssertExpectations(t)
			if tt.want != crop {
				next.AssertNotCalled(t, "FitImage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			}
			assert.Equal(t, tt.want == frame, framed, "framed flag")
			assert.Equal(t, displayTestW, out.Bounds().Dx())
			assert.Equal(t, displayTestH, out.Bounds().Dy())
		})
	}
}

// P26: an image's own display mode from Tune Image wins over the global mode, in all nine pairs,
// so Smart crop is reachable from Full artwork and Museum frame too.
func TestFitImage_ImageDisplayModeWins(t *testing.T) {
	globals := []ArtworkDisplayMode{ArtworkDisplayFull, ArtworkDisplayFramed, ArtworkDisplaySmartCrop}
	for _, global := range globals {
		for _, own := range []provider.DisplayOverride{provider.DisplayFull, provider.DisplayFramed, provider.DisplayCrop} {
			next := new(MockImageProcessor)
			cfg := &Config{ArtworkDisplayMode: global, VirtualFrameSize: 0.85}
			framer := NewVirtualFramer(next, cfg)
			src := quadrantImage(200, 400)
			opts := provider.TuningOptions{Display: own}
			if own == provider.DisplayCrop {
				next.On("FitImage", mock.Anything, src, displayTestW, displayTestH, opts).
					Return(createColoredTestImage(displayTestW, displayTestH, color.Black), nil)
			}
			framed := false
			ctx := context.WithValue(context.Background(), provider.VirtualFramedKey, &framed)
			_, err := framer.FitImage(ctx, src, displayTestW, displayTestH, opts)
			require.NoError(t, err)
			next.AssertExpectations(t)
			if own != provider.DisplayCrop {
				next.AssertNotCalled(t, "FitImage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			}
			assert.Equal(t, own == provider.DisplayFramed, framed, "global %d, image %d: framed flag", global, own)
			assert.Equal(t, own == provider.DisplayFramed, framer.WouldFrame("", 200, 400, displayTestW, displayTestH, opts), "WouldFrame agrees")
		}
	}
}

// The older frame switch reads as before when the image has no display mode of its own.
func TestDisplayModeFor_OlderFrameSwitch(t *testing.T) {
	on := provider.TuningOptions{FrameOverride: provider.FrameOverrideForceOn}
	off := provider.TuningOptions{FrameOverride: provider.FrameOverrideForceOff}
	assert.Equal(t, ArtworkDisplayFramed, displayModeFor(ArtworkDisplayFull, on))
	assert.Equal(t, ArtworkDisplayFull, displayModeFor(ArtworkDisplayFramed, off))
	assert.Equal(t, ArtworkDisplaySmartCrop, displayModeFor(ArtworkDisplaySmartCrop, off))
	assert.Equal(t, ArtworkDisplayFull, displayModeFor(ArtworkDisplayFull, off))
	assert.Equal(t, ArtworkDisplaySmartCrop, displayModeFor(ArtworkDisplayFull, provider.TuningOptions{Display: provider.DisplayCrop, FrameOverride: provider.FrameOverrideForceOn}), "its own mode wins over the old switch")
	assert.Equal(t, ArtworkDisplayFramed, displayModeFor(ArtworkDisplayFramed, provider.TuningOptions{}), "nothing set: the global mode")
}

// Tune Image starts from the mode in effect, with the old switch folded into it.
func TestEffectiveTuning_ResolvesTheDisplayMode(t *testing.T) {
	cfg := &Config{ArtworkDisplayMode: ArtworkDisplayFull, VirtualFrameSize: 0.85}
	img := provider.Image{}
	eff := effectiveTuning(provider.TuningOptions{}, img, "800x500", cfg)
	assert.Equal(t, provider.DisplayFull, eff.Display)
	assert.Equal(t, provider.FrameOverrideInherit, eff.FrameOverride)
	eff = effectiveTuning(provider.TuningOptions{FrameOverride: provider.FrameOverrideForceOn}, img, "800x500", cfg)
	assert.Equal(t, provider.DisplayFramed, eff.Display)
	assert.Equal(t, provider.FrameOverrideInherit, eff.FrameOverride)
	eff = effectiveTuning(provider.TuningOptions{Display: provider.DisplayCrop}, img, "800x500", cfg)
	assert.Equal(t, provider.DisplayCrop, eff.Display)
}

func TestFitImage_FullArtworkIsNeverCropped(t *testing.T) {
	tests := []struct {
		name              string
		srcW, srcH        int
		artX, artY        int // Top-left of the art on the canvas
		artW, artH        int
		wallX, wallY      int // A point on the bars, if there are any
		expectWallVisible bool
	}{
		{"portrait gets side bars", 200, 400, 275, 0, 250, 500, 50, 250, true},
		{"wide gets top and bottom bars", 1000, 250, 0, 150, 800, 200, 400, 50, true},
		{"matching aspect fills the screen", 1600, 1000, 0, 0, 800, 500, 0, 0, false},
		{"small image is enlarged", 40, 25, 0, 0, 800, 500, 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{ArtworkDisplayMode: ArtworkDisplayFull, VirtualWallColor: WallNeutral}
			framer := NewVirtualFramer(new(MockImageProcessor), cfg)

			out, err := framer.FitImage(context.Background(), quadrantImage(tt.srcW, tt.srcH), displayTestW, displayTestH, provider.TuningOptions{})
			require.NoError(t, err)
			require.Equal(t, image.Rect(0, 0, displayTestW, displayTestH), out.Bounds())

			const inset = 8
			left, right := tt.artX+inset, tt.artX+tt.artW-1-inset
			top, bottom := tt.artY+inset, tt.artY+tt.artH-1-inset
			assertNearColor(t, quadTL, out.At(left, top), "top-left corner")
			assertNearColor(t, quadTR, out.At(right, top), "top-right corner")
			assertNearColor(t, quadBL, out.At(left, bottom), "bottom-left corner")
			assertNearColor(t, quadBR, out.At(right, bottom), "bottom-right corner")

			if tt.expectWallVisible {
				wall := framer.calculateWallColor(quadrantImage(tt.srcW, tt.srcH), provider.TuningOptions{})
				wr, wg, wb, _ := wall.RGBA()
				assertNearColor(t, color.RGBA{uint8(wr >> 8), uint8(wg >> 8), uint8(wb >> 8), 255}, out.At(tt.wallX, tt.wallY), "bars use the wall color")
			}
		})
	}
}

func TestFitImage_FullArtworkHonorsCancelledContext(t *testing.T) {
	cfg := &Config{ArtworkDisplayMode: ArtworkDisplayFull}
	framer := NewVirtualFramer(new(MockImageProcessor), cfg)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := framer.FitImage(ctx, quadrantImage(200, 400), displayTestW, displayTestH, provider.TuningOptions{})
	assert.Error(t, err)
}

func TestCheckCompatibility_DisplayModes(t *testing.T) {
	aspectErr := errors.New("aspect ratio diff too large")

	t.Run("whole-image modes accept any aspect ratio", func(t *testing.T) {
		for _, mode := range []ArtworkDisplayMode{ArtworkDisplayFull, ArtworkDisplayFramed} {
			next := new(MockImageProcessor)
			next.On("CheckCompatibility", 100, 400, displayTestW, displayTestH).Return(aspectErr)
			framer := NewVirtualFramer(next, &Config{ArtworkDisplayMode: mode})
			assert.NoError(t, framer.CheckCompatibility(100, 400, displayTestW, displayTestH))
		}
	})

	t.Run("size errors still pass through", func(t *testing.T) {
		sizeErr := errors.New("insufficient size")
		next := new(MockImageProcessor)
		next.On("CheckCompatibility", 10, 10, displayTestW, displayTestH).Return(sizeErr)
		framer := NewVirtualFramer(next, &Config{ArtworkDisplayMode: ArtworkDisplayFull})
		assert.ErrorIs(t, framer.CheckCompatibility(10, 10, displayTestW, displayTestH), sizeErr)
	})

	t.Run("smart crop keeps the existing result", func(t *testing.T) {
		next := new(MockImageProcessor)
		next.On("CheckCompatibility", 100, 400, displayTestW, displayTestH).Return(aspectErr)
		framer := NewVirtualFramer(next, &Config{})
		assert.ErrorIs(t, framer.CheckCompatibility(100, 400, displayTestW, displayTestH), aspectErr)
	})
}

func TestArtworkDisplayMode_Persistence(t *testing.T) {
	useRealDisplayModeDefault(t)
	ResetConfig()
	defer ResetConfig()
	prefs := NewMockPreferences()

	cfg := GetConfig(prefs)
	assert.Equal(t, ArtworkDisplayFull, cfg.GetArtworkDisplayMode(), "nothing saved means the default")

	cfg.SetArtworkDisplayMode(ArtworkDisplayFramed)

	ResetConfig()
	assert.Equal(t, ArtworkDisplayFramed, GetConfig(prefs).GetArtworkDisplayMode(), "mode must survive a save and load")
}

func TestArtworkDisplayMode_DefaultKeepsSavedChoice(t *testing.T) {
	useRealDisplayModeDefault(t)
	defer ResetConfig()
	savedConfig := func(t *testing.T, mutate func(saved map[string]any)) ArtworkDisplayMode {
		t.Helper()
		ResetConfig()
		prefs := NewMockPreferences()
		cfg := GetConfig(prefs)
		cfg.SetArtworkDisplayMode(ArtworkDisplaySmartCrop)

		var saved map[string]any
		require.NoError(t, json.Unmarshal([]byte(prefs.String(WallhavenConfigPrefKey)), &saved))
		require.Contains(t, saved, "artwork_display_mode")
		mutate(saved)
		data, err := json.Marshal(saved)
		require.NoError(t, err)
		prefs.SetString(WallhavenConfigPrefKey, string(data))

		ResetConfig()
		return GetConfig(prefs).GetArtworkDisplayMode()
	}

	assert.Equal(t, ArtworkDisplayFull, savedConfig(t, func(saved map[string]any) {
		delete(saved, "artwork_display_mode")
	}), "a config from before this setting existed gets the default")

	assert.Equal(t, ArtworkDisplaySmartCrop, savedConfig(t, func(map[string]any) {}), "a saved Smart Crop is kept")

	assert.Equal(t, ArtworkDisplayFramed, savedConfig(t, func(saved map[string]any) {
		saved["artwork_display_mode"] = int(ArtworkDisplayFramed)
	}), "a saved Museum Frame is kept")
}

func TestGetArtworkDisplayMode_UnknownValueIsSmartCrop(t *testing.T) {
	assert.Equal(t, ArtworkDisplaySmartCrop, (&Config{ArtworkDisplayMode: ArtworkDisplayMode(42)}).GetArtworkDisplayMode())
}

// displayTestEnv is a real store, file manager and processing chain with no network.
type displayTestEnv struct {
	wp    *Plugin
	store *ImageStore
	fm    *FileManager
}

// inlineSubmitter runs jobs immediately, the same way the pipeline's state manager applies results.
type inlineSubmitter struct {
	env  *displayTestEnv
	jobs []DownloadJob
}

func (s *inlineSubmitter) Submit(ctx context.Context, job DownloadJob) bool {
	s.jobs = append(s.jobs, job)
	img, err := s.env.wp.ProcessImageJob(ctx, job)
	if err != nil {
		return true
	}
	if !s.env.store.Add(img) {
		s.env.store.replace(img)
	}
	return true
}

func newDisplayTestEnv(t *testing.T) *displayTestEnv {
	t.Helper()
	ResetConfig()
	t.Cleanup(ResetConfig)
	cfg := GetConfig(NewMockPreferences())
	cfg.Queries = append(cfg.Queries, ImageQuery{ID: "q1", Active: true})

	tmp := t.TempDir()
	fm := NewFileManager(tmp)
	require.NoError(t, fm.EnsureDirs())
	store := NewImageStore()
	store.SetAsyncSave(false)
	store.SetFileManager(fm, filepath.Join(tmp, "cache.json"))

	mockOS := new(MockOS)
	mockOS.On("GetMonitors").Return([]Monitor{{ID: 0, Rect: image.Rect(0, 0, displayTestW, displayTestH)}}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	wp := &Plugin{
		os:           mockOS,
		cfg:          cfg,
		store:        store,
		fm:           fm,
		ctx:          ctx,
		imgProcessor: NewVirtualFramer(NewSmartImageProcessor(mockOS, cfg, nil), cfg),
	}
	return &displayTestEnv{wp: wp, store: store, fm: fm}
}

// addLibraryImage writes a master and processes it, like an image that was downloaded earlier.
func (e *displayTestEnv) addLibraryImage(t *testing.T, id string, w, h int) {
	t.Helper()
	path, err := e.fm.GetMasterPath(id, ".jpg")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	f, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, jpeg.Encode(f, quadrantImage(w, h), &jpeg.Options{Quality: 95}))
	require.NoError(t, f.Close())

	img, err := e.wp.ProcessImageJob(context.Background(), DownloadJob{
		Ctx:   context.Background(),
		Image: provider.Image{ID: id, Path: "http://example.invalid/" + id + ".jpg", SourceQueryID: "q1"},
	})
	require.NoError(t, err)
	require.True(t, e.store.Add(img))
}

// apply follows what RefreshImagesAndPulse does to the store after a settings change.
func (e *displayTestEnv) apply(t *testing.T) *inlineSubmitter {
	t.Helper()
	e.wp.syncStoreWithConfig()
	sub := &inlineSubmitter{env: e}
	e.wp.jobSubmitter = sub
	e.wp.rebuildMissingDerivatives()
	return sub
}

func (e *displayTestEnv) derivativePath(t *testing.T, id string) string {
	t.Helper()
	img, ok := e.store.GetByID(id)
	require.True(t, ok, "image must stay in the store")
	return img.DerivativePaths["800x500"]
}

func TestGetBaseDerivativeDir_DisplayModes(t *testing.T) {
	env := newDisplayTestEnv(t)
	cfg := env.wp.cfg

	for _, smartFit := range []SmartFitMode{SmartFitOff, SmartFitNormal, SmartFitAggressive} {
		cfg.SetSmartFitMode(smartFit)

		cfg.SetArtworkDisplayMode(ArtworkDisplayFull)
		assert.Equal(t, filepath.Join("fitted", "full", "standard"), env.wp.getBaseDerivativeDir())

		cfg.SetArtworkDisplayMode(ArtworkDisplayFramed)
		assert.Equal(t, filepath.Join("fitted", "framed", "standard"), env.wp.getBaseDerivativeDir())
	}

	cfg.SetArtworkDisplayMode(ArtworkDisplaySmartCrop)
	cfg.SetSmartFitMode(SmartFitOff)
	assert.Equal(t, "", env.wp.getBaseDerivativeDir(), "Smart Crop keeps the existing behaviour")
}

func TestSwitchingModes_RebuildsLibraryFromMasters(t *testing.T) {
	env := newDisplayTestEnv(t)
	env.wp.cfg.SetSmartFitMode(SmartFitAggressive)
	env.addLibraryImage(t, "lib_1", 1000, 800)
	env.addLibraryImage(t, "lib_2", 1000, 700)
	before := env.derivativePath(t, "lib_1")
	require.Contains(t, before, filepath.Join("fitted", "flexibility"))

	for _, step := range []struct {
		mode ArtworkDisplayMode
		dir  string
	}{
		{ArtworkDisplayFull, filepath.Join("fitted", "full", "standard")},
		{ArtworkDisplayFramed, filepath.Join("fitted", "framed", "standard")},
		{ArtworkDisplaySmartCrop, filepath.Join("fitted", "flexibility")},
	} {
		env.wp.cfg.SetArtworkDisplayMode(step.mode)
		sub := env.apply(t)
		require.True(t, env.fm.WaitTimeout(derivativeCleanupWait))

		if step.mode == ArtworkDisplaySmartCrop {
			assert.Empty(t, sub.jobs, "the images were rendered in this mode before, so their files are reused")
		} else {
			assert.Len(t, sub.jobs, 2, "every library image must be rebuilt")
		}
		for _, job := range sub.jobs {
			assert.Nil(t, job.Provider, "a rebuild must not use the network")
		}
		for _, id := range []string{"lib_1", "lib_2"} {
			path := env.derivativePath(t, id)
			assert.Contains(t, path, step.dir)
			got, err := imaging.Open(path)
			require.NoError(t, err)
			assert.Equal(t, image.Rect(0, 0, displayTestW, displayTestH), got.Bounds())
		}
	}
}

func TestSyncKeepsProcessedImagesValid(t *testing.T) {
	for _, mode := range []ArtworkDisplayMode{ArtworkDisplaySmartCrop, ArtworkDisplayFull, ArtworkDisplayFramed} {
		t.Run(mode.String(), func(t *testing.T) {
			env := newDisplayTestEnv(t)
			env.wp.cfg.SetArtworkDisplayMode(mode)

			// A fresh download must already match what Sync compares against.
			env.addLibraryImage(t, "fresh", 1000, 800)
			want := env.derivativePath(t, "fresh")
			require.NotEmpty(t, want)

			for i := 0; i < 3; i++ {
				sub := env.apply(t)
				assert.Empty(t, sub.jobs, "nothing should need rebuilding (Sync %d)", i)
				assert.Equal(t, want, env.derivativePath(t, "fresh"), "derivative must survive Sync %d", i)
			}
		})
	}
}

func TestReplace_WakesWaitingMonitors(t *testing.T) {
	store := NewImageStore()
	store.SetAsyncSave(false)
	require.True(t, store.Add(provider.Image{ID: "img"}))
	waiting := store.GetUpdateChannel()

	require.True(t, store.replace(provider.Image{ID: "img", DerivativePaths: map[string]string{"800x500": "x.jpg"}}))

	select {
	case <-waiting:
	default:
		t.Fatal("a rebuilt image must wake monitors waiting for images")
	}
}

func TestFlagsMatch_FramedTagIsMetadata(t *testing.T) {
	flags := map[string]bool{"SmartFit": true}
	store := NewImageStore()
	img := provider.Image{ProcessingFlags: map[string]bool{"SmartFit": true, "VirtualFramed:800x500": true}}
	assert.True(t, store.flagsMatch(img, flags))
}

func TestProcessingFlags_DefaultModeDoesNotInvalidateExistingImages(t *testing.T) {
	env := newDisplayTestEnv(t)

	// As stamped before this feature existed: no display flags at all.
	old := env.wp.targetProcessingFlags()
	delete(old, "DisplayFull")
	delete(old, "DisplayFramed")

	assert.True(t, env.store.flagsMatch(provider.Image{ProcessingFlags: old}, env.wp.targetProcessingFlags()))
}

func TestImageForRebuild_DoesNotShareMaps(t *testing.T) {
	img := provider.Image{
		ID:              "img",
		Width:           100,
		ProcessingFlags: map[string]bool{"incompatible:1x1": true},
		Tuning:          map[string]provider.TuningOptions{"800x500": {FrameOverride: provider.FrameOverrideForceOn}},
		DerivativePaths: map[string]string{"800x500": "x.jpg"},
	}

	clone := imageForRebuild(img)
	clone.ProcessingFlags["changed"] = true
	clone.Tuning["changed"] = provider.TuningOptions{}

	assert.NotContains(t, img.ProcessingFlags, "changed")
	assert.NotContains(t, img.Tuning, "changed")
	assert.Equal(t, 100, clone.Width)
	assert.True(t, clone.ProcessingFlags["incompatible:1x1"], "metadata carries over")
	assert.Equal(t, provider.FrameOverrideForceOn, clone.Tuning["800x500"].FrameOverride, "per-image tuning carries over")
	assert.Empty(t, clone.DerivativePaths, "derivatives are about to be rebuilt")
}

func TestWaitForDerivativeDeletes(t *testing.T) {
	fm := NewFileManager(t.TempDir())
	require.NoError(t, fm.EnsureDirs())

	assert.True(t, fm.WaitForDerivativeDeletes(context.Background()), "nothing started yet")

	dir := filepath.Join(fm.rootDir, FittedRootDir, FullArtDir, StandardDir, "800x500")
	require.NoError(t, os.MkdirAll(dir, 0755))
	derivatives := []string{filepath.Join(dir, "a.jpg"), filepath.Join(dir, "b.jpg")}
	for _, path := range derivatives {
		require.NoError(t, os.WriteFile(path, []byte("x"), 0600))
	}

	fm.AsyncDeleteDerivativesBatch([]string{"a"})
	fm.AsyncDeleteDerivativesBatch([]string{"b"})
	require.True(t, fm.WaitForDerivativeDeletes(context.Background()))
	for _, path := range derivatives {
		assert.NoFileExists(t, path, "waiting must cover every batch")
	}

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	blocked := make(chan struct{})
	defer close(blocked)
	fm.derivativeDeleteMu.Lock()
	fm.lastDerivativeDelete = blocked
	fm.derivativeDeleteMu.Unlock()
	assert.False(t, fm.WaitForDerivativeDeletes(cancelled), "must stop when the context ends")
}

func TestNeedsDerivativeRebuild(t *testing.T) {
	env := newDisplayTestEnv(t)
	env.wp.cfg.SetArtworkDisplayMode(ArtworkDisplayFull)
	env.addLibraryImage(t, "has_master", 1000, 800)
	complete, _ := env.store.GetByID("has_master")

	assert.False(t, env.wp.needsDerivativeRebuild(complete), "complete image")

	missing := complete
	missing.DerivativePaths = map[string]string{}
	assert.True(t, env.wp.needsDerivativeRebuild(missing), "derivative missing")

	incompatible := missing
	incompatible.DerivativePaths = map[string]string{"other": "x.jpg"}
	incompatible.ProcessingFlags = map[string]bool{"incompatible:800x500": true}
	assert.False(t, env.wp.needsDerivativeRebuild(incompatible), "tagged incompatible")

	noMaster := missing
	noMaster.ID = "no_master"
	assert.False(t, env.wp.needsDerivativeRebuild(noMaster), "master missing")
}

func TestLocalRebuildFailure_KeepsMaster(t *testing.T) {
	env := newDisplayTestEnv(t)
	env.wp.cfg.SetArtworkDisplayMode(ArtworkDisplayFull)
	env.addLibraryImage(t, "precious", 1000, 800)
	img, _ := env.store.GetByID("precious")
	img.DerivativePaths = map[string]string{}
	require.NoError(t, env.fm.DeleteDerivatives("precious"))

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := env.wp.ProcessImageJob(cancelled, DownloadJob{Ctx: cancelled, Image: img})
	require.Error(t, err)

	master, err := env.wp.masterPathFor(img)
	require.NoError(t, err)
	assert.FileExists(t, master, "a failed or superseded rebuild must not delete the only copy")
}

func TestSetArtworkDisplayMode_SameModeDoesNothing(t *testing.T) {
	env := newDisplayTestEnv(t)
	env.wp.cfg.SetArtworkDisplayMode(ArtworkDisplayFull)
	env.wp.jobSubmitter = nil

	// The refresh path would panic here with no fetch machinery set up, so reaching the end
	// shows that selecting the current mode returned early.
	env.wp.SetArtworkDisplayMode(ArtworkDisplayFull)
	assert.Equal(t, ArtworkDisplayFull, env.wp.cfg.GetArtworkDisplayMode())
}

func findMenuItem(items []schema.MenuItemSchema, label string) (schema.MenuItemSchema, bool) {
	for _, item := range items {
		if item.Label == label {
			return item, true
		}
	}
	return schema.MenuItemSchema{}, false
}

func TestArtworkDisplayMenuItem(t *testing.T) {
	env := newDisplayTestEnv(t)
	env.wp.cfg.SetArtworkDisplayMode(ArtworkDisplayFramed)

	item := env.wp.artworkDisplayMenuItem()
	require.NotNil(t, item.SubMenu)
	require.Len(t, item.SubMenu.Items, 3)

	checked := []string{}
	for _, sub := range item.SubMenu.Items {
		assert.NotNil(t, sub.Action)
		if sub.IsChecked {
			checked = append(checked, sub.Label)
		}
	}
	assert.Equal(t, []string{trayDisplayModeLabel(ArtworkDisplayFramed)}, checked)
	assert.Equal(t, trayDisplayModeLabel(ArtworkDisplayFramed), item.Detail, "the row shows the current choice")
}

func TestCreateTrayMenu_IncludesDisplayModeAndTuneImage(t *testing.T) {
	env := newDisplayTestEnv(t)
	env.wp.cfg.SetSmartFitMode(SmartFitOff)
	mc := NewMonitorController(0, Monitor{ID: 0, Rect: image.Rect(0, 0, displayTestW, displayTestH)}, env.store, env.fm, env.wp.os, env.wp.cfg, env.wp.imgProcessor)
	env.wp.Monitors = map[int]*MonitorController{0: mc}

	tune := i18n.T("Tune image")
	hasTune := func() bool {
		_, ok := findMenuItem(env.wp.CreateTrayMenu().Items, tune)
		return ok
	}

	assert.False(t, hasTune(), "Smart Crop with Smart Fit off has nothing to tune")
	_, ok := findMenuItem(env.wp.CreateTrayMenu().Items, i18n.T("Display mode"))
	assert.True(t, ok, "the switch must always be available")

	env.wp.cfg.SetArtworkDisplayMode(ArtworkDisplayFull)
	assert.True(t, hasTune(), "frames can be toggled per image in whole-image modes")
}

// Smart crop fills the screen and never frames, so the frame settings are not shown there; Museum
// frame shows all three, Full artwork only the wall colour (ui_refresh.md section 12, v19).
func TestPrefs_FrameSettingsFollowDisplayMode(t *testing.T) {
	ResetConfig()
	defer ResetConfig()
	cfg := GetConfig(NewMockPreferences())
	cfg.SetSmartFitMode(SmartFitNormal)
	cfg.SetArtworkDisplayMode(ArtworkDisplaySmartCrop)

	wp := &Plugin{cfg: cfg}
	sm := NewMockSettingsManager()
	sm.RenderTabs(*wp.CreatePrefsSchema(sm))
	sm.refreshWidgetStates()
	frame := []string{"virtualPaperMatting"}
	require.NotContains(t, sm.allWidgets, "virtualFramingFallback", "the switch is gone")

	for _, name := range append([]string{"virtualWallColor"}, frame...) {
		assert.False(t, sm.allWidgets[name].Visible(), "%s hidden in Smart Crop", name)
	}

	sm.SetValue("artworkDisplayMode", artworkDisplayModeIndex(ArtworkDisplayFramed))
	for _, name := range append([]string{"virtualWallColor"}, frame...) {
		assert.True(t, sm.allWidgets[name].Visible(), "%s shown in Museum Frame", name)
	}

	sm.SetValue("artworkDisplayMode", artworkDisplayModeIndex(ArtworkDisplayFull))
	assert.True(t, sm.allWidgets["virtualWallColor"].Visible(), "wall colour shown in Full Artwork")
	for _, name := range frame {
		assert.False(t, sm.allWidgets[name].Visible(), "%s hidden in Full Artwork", name)
	}
}

func TestFullArtworkDerivativeLayout(t *testing.T) {
	env := newDisplayTestEnv(t)
	env.wp.cfg.SetArtworkDisplayMode(ArtworkDisplayFull)
	env.addLibraryImage(t, "portrait", 500, 1000)

	path := env.derivativePath(t, "portrait")
	assert.True(t, strings.Contains(path, filepath.Join("fitted", "full", "standard", "800x500")), path)

	img, _ := env.store.GetByID("portrait")
	assert.False(t, img.ProcessingFlags["VirtualFramed:800x500"], "Full Artwork draws no frame")
}
