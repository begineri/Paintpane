package ui

import (
	"image"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
)

const tuneTestImage = "../pkg/wallpaper/testdata/golden/ClevelandMuseum_135428.jpg"

type tuneHarness struct {
	t       *testing.T
	panel   fyne.CanvasObject
	chosen  []provider.TuningOptions
	mode    *segmented // this image's display mode (P26)
	canvasW fyne.Window
}

func newTuneHarness(t *testing.T, mutate func(*tuneRequest)) *tuneHarness {
	t.Helper()
	test.NewTempApp(t)
	h := &tuneHarness{t: t}
	req := tuneRequest{
		previewPath: tuneTestImage,
		global:      provider.DisplayFull, // the default display mode
		opts: provider.TuningOptions{
			FrameSize: 0.85,
			Matting:   provider.MattingOverrideOn,
			WallColor: provider.WallColorOverrideAlgorithmic,
		},
		onSelect: func(opts provider.TuningOptions, onDone func()) {
			h.chosen = append(h.chosen, opts)
			onDone()
		},
	}
	if mutate != nil {
		mutate(&req)
	}
	h.panel = newTunePanel(req)
	w := test.NewWindow(h.panel)
	t.Cleanup(w.Close)
	w.Resize(h.panel.MinSize())
	h.canvasW = w
	for _, o := range shownObjects(w.Canvas().Content()) {
		if s, ok := o.(*segmented); ok {
			h.mode = s
		}
	}
	if req.previewPath != "" {
		require.NotNil(t, h.mode, "Tune Image has the display mode control")
	}
	return h
}

// pick taps the display mode segment at i, as a click does.
func (h *tuneHarness) pick(i int) {
	h.t.Helper()
	w := h.mode.Size().Width / float32(len(h.mode.Options))
	test.TapAt(h.mode, fyne.NewPos(w*float32(i)+w/2, h.mode.Size().Height/2))
}

// last is the options the window sent last.
func (h *tuneHarness) last() provider.TuningOptions {
	h.t.Helper()
	require.NotEmpty(h.t, h.chosen)
	return h.chosen[len(h.chosen)-1]
}

func (h *tuneHarness) shows(text string) bool { return shownText(h.canvasW.Canvas().Content(), text) }
func (h *tuneHarness) resetShown() bool       { return shownButton(h.canvasW.Canvas().Content(), "Reset") }

// previewImage is the canvas image that shows the desktop result.
func (h *tuneHarness) previewImage() *canvas.Image {
	var found *canvas.Image
	for _, o := range shownObjects(h.canvasW.Canvas().Content()) {
		if img, ok := o.(*canvas.Image); ok && img.Image != nil && img.CornerRadius == tunePreviewRadius {
			found = img
		}
	}
	return found
}

// P26: the window offers the global mode's three choices for this image, in the same order, and
// starts on the one in effect. An image that follows the setting shows no Reset.
func TestTuneModeControlOffersTheThreeModes(t *testing.T) {
	for _, tc := range []struct {
		global provider.DisplayOverride
		index  int
	}{{provider.DisplayFull, 0}, {provider.DisplayFramed, 1}, {provider.DisplayCrop, 2}} {
		h := newTuneHarness(t, func(r *tuneRequest) { r.global = tc.global })
		assert.Equal(t, []string{"Full artwork", "Museum frame", "Smart crop"}, h.mode.Options)
		assert.Equal(t, tc.index, h.mode.Selected, "starts on the global mode %d", tc.global)
		assert.False(t, h.resetShown(), "follows the setting: nothing to reset")
		assert.Equal(t, tc.global == provider.DisplayFramed, h.shows("Frame size"), "frame rows only when framed")
		assert.False(t, h.shows("Add frame"), "the Add frame button is replaced")
	}
}

// P26, the owner's case: under Full artwork one image can be switched to Smart crop, dragged, and
// Reset brings it back to following the setting.
func TestTuneSmartCropUnderFullArtwork(t *testing.T) {
	h := newTuneHarness(t, func(r *tuneRequest) {
		r.previewPath = writeTestPicture(t, 300, 900)
		r.screenW, r.screenH = 1920, 1080
	})
	h.pick(2)
	require.Len(t, h.chosen, 1)
	assert.Equal(t, provider.DisplayCrop, h.last().Display, "this image is cropped")
	assert.Equal(t, provider.FrameOverrideInherit, h.last().FrameOverride)
	assert.True(t, h.last().IsAutoCrop(), "starting from the automatic crop")
	assert.Equal(t, 2, h.mode.Selected)
	assert.True(t, h.resetShown(), "differs from the setting")
	assert.False(t, h.shows("Frame size"))

	h.dragPreview(0, -30)
	require.Len(t, h.chosen, 2, "the cropped picture can be dragged")
	assert.Equal(t, provider.DisplayCrop, h.last().Display)
	assert.True(t, h.last().HasCropOffset)

	reset := findButton(h.canvasW.Canvas().Content(), "Reset")
	require.NotNil(t, reset)
	test.Tap(reset)
	require.Len(t, h.chosen, 3)
	assert.Equal(t, provider.DisplayInherit, h.last().Display, "follows the setting again")
	assert.True(t, h.last().IsAutoCrop())
	assert.Equal(t, 0, h.mode.Selected, "back on Full artwork")
	assert.False(t, h.resetShown())
}

// Museum frame shows the frame rows; picking the global mode's segment follows the setting again.
func TestTuneMuseumFrameAndBack(t *testing.T) {
	h := newTuneHarness(t, nil)
	h.pick(1)
	assert.Equal(t, provider.DisplayFramed, h.last().Display)
	assert.True(t, h.shows("Frame size"))
	assert.True(t, h.resetShown())

	h.pick(0)
	assert.Equal(t, provider.DisplayInherit, h.last().Display, "Full artwork is the setting: follow it")
	assert.False(t, h.shows("Frame size"))
	assert.False(t, h.resetShown())

	n := len(h.chosen)
	h.pick(0)
	assert.Len(t, h.chosen, n, "picking the mode already shown does nothing")
}

// P26: under Smart crop the control is there too (replacing v22's rule of no Add frame button
// there), so one image can be shown whole; it then has nothing to drag.
func TestTuneFullArtworkUnderSmartCrop(t *testing.T) {
	h := newTuneHarness(t, func(r *tuneRequest) {
		r.global = provider.DisplayCrop
		r.previewPath = writeTestPicture(t, 300, 900)
	})
	assert.Equal(t, 2, h.mode.Selected)
	h.pick(0)
	assert.Equal(t, provider.DisplayFull, h.last().Display)
	for _, o := range shownObjects(h.canvasW.Canvas().Content()) {
		if s, ok := o.(*dragSurface); ok {
			assert.False(t, s.allowed(), "a picture shown whole has nothing to drag")
		}
	}
}

// Tuning saved with the older frame switch opens on the mode it gives, and the window no longer
// sends the switch, so the image is not pinned to it.
func TestTuneOlderFrameSwitchIsFoldedIn(t *testing.T) {
	h := newTuneHarness(t, func(r *tuneRequest) {
		r.global = provider.DisplayCrop
		r.opts.FrameOverride = provider.FrameOverrideForceOn
	})
	assert.Equal(t, 1, h.mode.Selected, "framed")
	assert.True(t, h.resetShown(), "framed by its own choice, not the setting")
	h.pick(2)
	assert.Equal(t, provider.DisplayInherit, h.last().Display)
	assert.Equal(t, provider.FrameOverrideInherit, h.last().FrameOverride)

	h = newTuneHarness(t, func(r *tuneRequest) {
		r.global = provider.DisplayFramed
		r.opts.FrameOverride = provider.FrameOverrideForceOff
	})
	assert.Equal(t, 0, h.mode.Selected, "unframed under Museum frame is shown whole")
}

// While a choice is being applied, another tap is ignored and the control stays on the mode shown.
func TestTuneModeIgnoredWhileApplying(t *testing.T) {
	var pending []func()
	h := newTuneHarness(t, func(r *tuneRequest) {
		r.onSelect = func(_ provider.TuningOptions, onDone func()) { pending = append(pending, onDone) }
	})
	h.pick(1)
	h.pick(2)
	require.Len(t, pending, 1, "the second tap is dropped")
	assert.Equal(t, 1, h.mode.Selected)
}

// The preview is always the shape of the screen (J3), so changing the mode does not change its size.
func TestTunePreviewKeepsItsSizeWhenTheModeChanges(t *testing.T) {
	h := newTuneHarness(t, nil)
	before := h.previewImage()
	require.NotNil(t, before)
	assert.Equal(t, image.Rect(0, 0, 720, 406), before.Image.Bounds(), "16:9, twice the 360×203 it shows at")
	size := before.Size()

	for _, i := range []int{1, 2, 0} {
		h.pick(i)
		after := h.previewImage()
		require.NotNil(t, after)
		assert.Equal(t, image.Rect(0, 0, 720, 406), after.Image.Bounds())
		assert.Equal(t, size, after.Size(), "the picture does not jump")
	}
}

func TestTunePreviewFollowsTheShapeOfTheScreen(t *testing.T) {
	assert.Equal(t, fyne.NewSize(360, 203), tunePreviewSize(0, 0))
	assert.Equal(t, fyne.NewSize(360, 203), tunePreviewSize(1920, 1080))
	assert.Equal(t, fyne.NewSize(360, 151), tunePreviewSize(3440, 1440))
	assert.Equal(t, fyne.NewSize(307, 230), tunePreviewSize(1024, 768))

	h := newTuneHarness(t, func(r *tuneRequest) { r.screenW, r.screenH = 3440, 1440 })
	assert.Equal(t, image.Rect(0, 0, 720, 302), h.previewImage().Image.Bounds())
}

// What the preview shows comes from the renderer the plugin provides, told the mode in effect
// (it does not know the global mode the window was given).
func TestTunePreviewIsDrawnByTheProvidedRenderer(t *testing.T) {
	var got []provider.TuningOptions
	h := newTuneHarness(t, func(r *tuneRequest) {
		r.render = func(src image.Image, w, hh int, opts provider.TuningOptions) image.Image {
			got = append(got, opts)
			return image.NewRGBA(image.Rect(0, 0, w, hh))
		}
	})
	require.NotEmpty(t, got, "the preview is drawn when the window opens")
	assert.Equal(t, provider.DisplayFull, got[len(got)-1].Display)

	h.pick(1)
	assert.Equal(t, provider.DisplayFramed, got[len(got)-1].Display, "and again framed")
	h.pick(2)
	assert.Equal(t, provider.DisplayCrop, got[len(got)-1].Display, "and cropped")
}

func shownText(root fyne.CanvasObject, text string) bool {
	for _, o := range shownObjects(root) {
		if got, ok := labelText(o); ok && got == text {
			return true
		}
	}
	return false
}

func TestTuneNoNoteWhenTheDisplayModeNeverCrops(t *testing.T) {
	h := newTuneHarness(t, func(r *tuneRequest) { r.global = provider.DisplayFull })
	for _, o := range shownObjects(h.canvasW.Canvas().Content()) {
		if text, ok := labelText(o); ok {
			assert.NotContains(t, text, "nothing is cropped", "J6 is dropped")
		}
	}
}
