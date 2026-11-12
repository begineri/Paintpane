//go:build !linux

package wallpaper

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/disintegration/imaging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/pkg/ui"
)

// tuneCapture is the plugin manager with Tune Image replaced by a recorder of what the window gets.
type tuneCapture struct {
	*MockPluginManager
	previewPath string
	eff         provider.TuningOptions
	preview     ui.TunePreview
	onSelect    func(provider.TuningOptions, func())
	onClose     func()
}

func (c *tuneCapture) ShowTuneImagePopup(_ int, previewPath string, _ provider.TuningOptions, eff provider.TuningOptions, _ [9]string, _ [9]provider.CropAnchor, preview ui.TunePreview, _ bool, onSelect func(provider.TuningOptions, func()), onClose func()) {
	c.previewPath, c.eff, c.preview, c.onSelect, c.onClose = previewPath, eff, preview, onSelect, onClose
}

// tuneSession drives Tune Image the way ui/tune_window.go does, against the real plugin.
type tuneSession struct {
	t   *testing.T
	app *e2eApp
	cap *tuneCapture
	src image.Image // the small copy the window draws its preview from
	w   int
	h   int
}

func openTuneSession(t *testing.T, app *e2eApp, cap *tuneCapture) *tuneSession {
	t.Helper()
	app.wp.showTuneImagePopup(0)
	require.NotNil(t, cap.onSelect, "Tune Image opened")
	full, err := imaging.Open(cap.previewPath)
	require.NoError(t, err)
	r := app.os.monitors[0].Rect
	// As loadTuneSource: at most 1000 pixels on the longer side.
	return &tuneSession{t: t, app: app, cap: cap, src: imaging.Fit(full, 1000, 1000, imaging.Lanczos), w: r.Dx(), h: r.Dy()}
}

// preview is what the window shows for opts, drawn at half the screen size.
func (s *tuneSession) preview(opts provider.TuningOptions) image.Image {
	out := s.cap.preview.Render(s.src, s.w/2, s.h/2, opts)
	require.NotNil(s.t, out)
	return out
}

func (s *tuneSession) desktop() image.Image {
	img, err := imaging.Open(s.app.os.last(0))
	require.NoError(s.t, err)
	return img
}

// apply sends opts as the window does and waits until the window would take its overlay off.
func (s *tuneSession) apply(opts provider.TuningOptions) time.Duration {
	s.t.Helper()
	done := make(chan struct{})
	start := time.Now()
	s.cap.onSelect(opts, func() { close(done) })
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		s.t.Fatal("Tune Image never finished applying")
	}
	return time.Since(start)
}

// meanDiff is the mean difference per channel (0-255) between two pictures of the same shape.
func meanDiff(a, b image.Image) float64 {
	const w, h = 160, 100
	sa, sb := imaging.Resize(a, w, h, imaging.Box), imaging.Resize(b, w, h, imaging.Box)
	var sum float64
	for i := range sa.Pix {
		if i%4 == 3 {
			continue
		}
		sum += math.Abs(float64(sa.Pix[i]) - float64(sb.Pix[i]))
	}
	return sum / float64(w*h*3)
}

func seedTextured(t *testing.T, app *e2eApp, id string, w, h int) string {
	t.Helper()
	path, err := app.fm.GetMasterPath(id, ".jpg")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, texturedImage(w, h), &jpeg.Options{Quality: 92}))
	require.NoError(t, os.WriteFile(path, buf.Bytes(), 0644))
	img, err := app.wp.ProcessImageJob(context.Background(), DownloadJob{
		Ctx:   context.Background(),
		Image: provider.Image{ID: id, Path: "http://example.invalid/" + id + ".jpg", SourceQueryID: "q1"},
	})
	require.NoError(t, err)
	require.True(t, app.store.Add(img))
	return path
}

func newTuneE2E(t *testing.T) (*e2eApp, *tuneCapture, string) {
	t.Helper()
	app := newE2EApp(t, t.TempDir(), NewMockPreferences(), e2eMonitor(0, 1280, 800))
	app.wp.cfg.SetSmartFitMode(SmartFitAggressive)
	cap := &tuneCapture{MockPluginManager: app.wp.manager.(*MockPluginManager)}
	app.wp.manager = cap
	master := seedTextured(t, app, "pic", 1600, 2400)
	app.wp.Activate()
	t.Cleanup(app.stop)
	app.wp.SetArtworkDisplayMode(ArtworkDisplayFull)
	app.waitUntilSettled(t, filepath.Join(FittedRootDir, FullArtDir), []string{"pic"})
	return app, cap, master
}

// Smart crop (docs/test-report-v1.0.md §15.3 item 6): the preview shows the crop on the desktop,
// follows a drag, and Reset brings the automatic crop back at once without decoding the original.
func TestTuneImagePreviewMatchesTheDesktopAndResetIsQuick(t *testing.T) {
	defer ResetConfig()
	app, cap, master := newTuneE2E(t)
	app.wp.SetArtworkDisplayMode(ArtworkDisplaySmartCrop)
	app.waitUntilSettled(t, filepath.Join(FittedRootDir, FlexibilityDir), []string{"pic"})

	s := openTuneSession(t, app, cap)
	defer cap.onClose()
	autoFile, err := os.ReadFile(app.os.last(0))
	require.NoError(t, err)

	// Opening: the window starts at the crop on screen and draws it.
	auto, ok := cap.preview.AutoPosition(s.src, s.w, s.h)
	require.True(t, ok, "the position of the wallpaper on screen is known")
	at := func(p float64) provider.TuningOptions {
		o := cap.eff
		o.SetCropOffset(p)
		return o
	}
	assert.Less(t, meanDiff(s.preview(at(auto)), s.desktop()), 6.0, "the preview is the desktop (automatic crop at %.1f %%)", auto)

	// Dragging: what the preview shows when let go is what the desktop gets.
	for _, p := range []float64{0, 100, 37} {
		s.apply(at(p))
		assert.Less(t, meanDiff(s.preview(at(p)), s.desktop()), 6.0, "dragged to %v %%", p)
		assert.Greater(t, meanDiff(s.preview(at(auto)), s.desktop()), 6.0, "and the desktop did move away from the automatic crop")
	}

	// Reset: the automatic crop comes back as it was, without the original (it is gone now, so
	// rendering it again would fail).
	require.NoError(t, os.Remove(master))
	reset := cap.eff
	reset.ResetCrop()
	took := s.apply(reset)
	t.Logf("Reset took %v", took)
	assert.Less(t, took, time.Second, "Reset is quick")
	got, err := os.ReadFile(app.os.last(0))
	require.NoError(t, err)
	assert.True(t, bytes.Equal(autoFile, got), "the desktop shows the automatic crop it showed before the drags")

	again, ok := cap.preview.AutoPosition(s.src, s.w, s.h)
	require.True(t, ok)
	assert.InDelta(t, auto, again, 1, "after Reset the window finds the automatic crop on screen again")
	img, _ := app.store.GetByID("pic")
	assert.True(t, img.GetTuning("1280x800").IsAutoCrop(), "the picture is back on the automatic crop")
}

// Full artwork and Museum frame: the preview is the desktop too.
func TestTuneImagePreviewMatchesTheDesktopInWholeImageModes(t *testing.T) {
	defer ResetConfig()
	app, cap, _ := newTuneE2E(t)

	app.wp.SetArtworkDisplayMode(ArtworkDisplayFramed)
	app.waitUntilSettled(t, filepath.Join(FittedRootDir, FramedArtDir), []string{"pic"})
	s := openTuneSession(t, app, cap)
	assert.Equal(t, provider.DisplayFramed, cap.eff.Display, "Museum frame starts framed")
	assert.Equal(t, provider.FrameOverrideInherit, cap.eff.FrameOverride, "the older frame switch is folded into Display")
	assert.Less(t, meanDiff(s.preview(cap.eff), s.desktop()), 6.0, "Museum frame")
	cap.onClose()

	app.wp.SetArtworkDisplayMode(ArtworkDisplayFull)
	app.waitUntilSettled(t, filepath.Join(FittedRootDir, FullArtDir), []string{"pic"})
	s = openTuneSession(t, app, cap)
	defer cap.onClose()
	assert.Equal(t, provider.DisplayFull, cap.eff.Display)
	assert.Less(t, meanDiff(s.preview(cap.eff), s.desktop()), 6.0, "Full artwork")

	framed := cap.eff
	framed.Display = provider.DisplayFramed
	s.apply(framed)
	assert.Less(t, meanDiff(s.preview(framed), s.desktop()), 6.0, "Full artwork, this image in Museum frame")

	// P26: this image in Smart crop under Full artwork; the window asks where the automatic crop
	// sits once it is on screen, and the preview drawn there is the desktop.
	cropped := cap.eff
	cropped.Display = provider.DisplayCrop
	s.apply(cropped)
	auto, ok := cap.preview.AutoPosition(s.src, s.w, s.h)
	require.True(t, ok, "the cropped picture has an automatic position")
	at := cropped
	at.SetCropOffset(auto)
	assert.Less(t, meanDiff(s.preview(at), s.desktop()), 6.0, "Full artwork, this image in Smart crop")
	at.SetCropOffset(100)
	s.apply(at)
	assert.Less(t, meanDiff(s.preview(at), s.desktop()), 6.0, "and dragged to the end")

	following := cap.eff
	following.Display = provider.DisplayInherit
	took := s.apply(following)
	assert.Less(t, took, time.Second, "following the setting again reuses the render from before")
	assert.Less(t, meanDiff(s.preview(cap.eff), s.desktop()), 6.0, "Full artwork again")
}
