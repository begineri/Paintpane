package ui

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
)

// writeTestPicture saves a w×h picture and returns its path.
func writeTestPicture(t *testing.T, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 90, 255})
		}
	}
	path := filepath.Join(t.TempDir(), "picture.png")
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	require.NoError(t, png.Encode(f, img))
	return path
}

func shownButton(root fyne.CanvasObject, text string) bool {
	for _, o := range shownObjects(root) {
		if b, ok := o.(*widget.Button); ok && b.Text == text {
			return true
		}
	}
	return false
}

func findButton(root fyne.CanvasObject, text string) *widget.Button {
	for _, o := range shownObjects(root) {
		if b, ok := o.(*widget.Button); ok && b.Text == text {
			return b
		}
	}
	return nil
}

// dragPreview drags the preview by (dx, dy) logical pixels.
func (h *tuneHarness) dragPreview(dx, dy float32) {
	h.t.Helper()
	var surface *dragSurface
	for _, o := range shownObjects(h.canvasW.Canvas().Content()) {
		if s, ok := o.(*dragSurface); ok {
			surface = s
		}
	}
	require.NotNil(h.t, surface, "the preview can be dragged")
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(surface).Add(fyne.NewPos(20, 20))
	test.Drag(h.canvasW.Canvas(), pos, dx, dy)
}

func smartCropHarness(t *testing.T, w, h int, mutate func(*tuneRequest)) *tuneHarness {
	return newTuneHarness(t, func(r *tuneRequest) {
		r.global = provider.DisplayCrop
		r.previewPath = writeTestPicture(t, w, h)
		r.screenW, r.screenH = 1920, 1080
		if mutate != nil {
			mutate(r)
		}
	})
}

// Dragging a tall picture slides the kept part up and down; the picture follows the pointer (J3).
func TestTuneDragMovesTheKeptPart(t *testing.T) {
	h := smartCropHarness(t, 300, 900, func(r *tuneRequest) {
		r.autoPosition = func(image.Image, int, int) (float64, bool) { return 20, true }
	})
	require.Empty(t, h.chosen)

	h.dragPreview(0, -30) // the picture goes up, so the kept part moves down
	require.Len(t, h.chosen, 1, "letting go applies the choice")
	got := h.chosen[0]
	assert.True(t, got.HasCropOffset)
	assert.Greater(t, got.CropOffset, 20.0, "starts from where the automatic crop was, and moves down")
	assert.Equal(t, provider.AnchorAuto, got.Anchor)
	assert.True(t, shownButton(h.canvasW.Canvas().Content(), "Reset"), "Reset appears after a drag")

	// Sideways does nothing to a tall picture.
	h.dragPreview(40, 0)
	assert.Len(t, h.chosen, 1)

	// It stays where it is let go, at any position, and stops at the ends.
	h.dragPreview(0, 5000)
	assert.Equal(t, 0.0, h.chosen[len(h.chosen)-1].CropOffset)
	h.dragPreview(0, -5000)
	assert.Equal(t, 100.0, h.chosen[len(h.chosen)-1].CropOffset)
}

func TestTuneDragWideAndNearlyScreenShaped(t *testing.T) {
	// A wide picture slides left and right.
	h := smartCropHarness(t, 1800, 300, nil)
	h.dragPreview(-30, 0)
	require.Len(t, h.chosen, 1)
	assert.Greater(t, h.chosen[0].CropOffset, 50.0)
	h.dragPreview(0, 30)
	assert.Len(t, h.chosen, 1, "up and down does nothing to a wide picture")

	// Close to the screen shape: still draggable, with a small range (v29).
	near := smartCropHarness(t, 2000, 1080, nil)
	near.dragPreview(-30, 0)
	require.Len(t, near.chosen, 1)
	assert.True(t, near.chosen[0].HasCropOffset)

	// Exactly the screen shape: nothing to drag.
	same := smartCropHarness(t, 1920, 1080, nil)
	same.dragPreview(-30, 0)
	assert.Empty(t, same.chosen)
}

// The preview follows the pointer while dragging: the renderer is asked for the new offset.
func TestTuneDragRedrawsThePreview(t *testing.T) {
	var offsets []float64
	h := smartCropHarness(t, 300, 900, func(r *tuneRequest) {
		r.render = func(src image.Image, w, hh int, opts provider.TuningOptions) image.Image {
			offsets = append(offsets, opts.CropOffset)
			return image.NewRGBA(image.Rect(0, 0, w, hh))
		}
	})
	before := len(offsets)
	h.dragPreview(0, -20)
	require.Greater(t, len(offsets), before)
	assert.NotEqual(t, 50.0, offsets[len(offsets)-1])
}

// Reset returns to the automatic crop, and goes away again.
func TestTuneReset(t *testing.T) {
	h := smartCropHarness(t, 300, 900, nil)
	h.dragPreview(0, -30)
	require.Len(t, h.chosen, 1)

	reset := findButton(h.canvasW.Canvas().Content(), "Reset")
	require.NotNil(t, reset)
	test.Tap(reset)
	require.Len(t, h.chosen, 2)
	assert.True(t, h.chosen[1].IsAutoCrop())
	assert.False(t, h.chosen[1].HasCropOffset)
	assert.False(t, shownButton(h.canvasW.Canvas().Content(), "Reset"))
}

// A picture whose crop a shortcut set (an anchor) starts from that position, with Reset offered.
func TestTuneStartsFromAnAnchor(t *testing.T) {
	h := smartCropHarness(t, 300, 900, func(r *tuneRequest) {
		r.opts.Anchor = provider.AnchorBottomCenter
	})
	assert.True(t, shownButton(h.canvasW.Canvas().Content(), "Reset"))
	h.dragPreview(0, 30) // the kept part moves up from the bottom (100 %)
	require.Len(t, h.chosen, 1)
	assert.Less(t, h.chosen[0].CropOffset, 100.0)
	assert.Greater(t, h.chosen[0].CropOffset, 80.0)
}

// Nothing can be dragged when the display mode never crops.
func TestTuneNoDragWhenTheDisplayModeNeverCrops(t *testing.T) {
	h := newTuneHarness(t, func(r *tuneRequest) {
		r.global = provider.DisplayFull
		r.previewPath = writeTestPicture(t, 300, 900)
	})
	for _, o := range shownObjects(h.canvasW.Canvas().Content()) {
		if s, ok := o.(*dragSurface); ok {
			pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(s).Add(fyne.NewPos(20, 20))
			test.Drag(h.canvasW.Canvas(), pos, 0, -30)
		}
	}
	assert.Empty(t, h.chosen)
}
