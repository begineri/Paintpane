package ui

import (
	"image"
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
)

// Scenarios from the A2 real-mouse test (docs/test-report-v1.0.md §15.2 items 4a-4c).

// recordRenders makes the harness's renderer note the options of every preview it draws.
func recordRenders(into *[]provider.TuningOptions) func(*tuneRequest) {
	return func(r *tuneRequest) {
		r.render = func(_ image.Image, w, h int, opts provider.TuningOptions) image.Image {
			*into = append(*into, opts)
			return image.NewRGBA(image.Rect(0, 0, w, h))
		}
	}
}

// 4c: in Smart crop the preview opens on the crop the desktop shows (where autoPosition finds the
// wallpaper), drawn at that position rather than by running the automatic crop again on the
// small copy, and follows the picture while it is dragged.
func TestTunePreviewOpensOnTheCropOnScreenAndFollowsTheDrag(t *testing.T) {
	var renders []provider.TuningOptions
	h := smartCropHarness(t, 300, 900, func(r *tuneRequest) {
		recordRenders(&renders)(r)
		r.autoPosition = func(image.Image, int, int) (float64, bool) { return 80, true }
	})
	require.NotEmpty(t, renders)
	first := renders[len(renders)-1]
	assert.True(t, first.HasCropOffset, "the preview is drawn at a position, not by the automatic analysis")
	assert.Equal(t, 80.0, first.CropOffset, "at the position of the wallpaper on screen")

	h.dragPreview(0, -30)
	require.Len(t, h.chosen, 1)
	last := renders[len(renders)-1]
	assert.Equal(t, h.chosen[0].CropOffset, last.CropOffset, "what the preview shows when let go is what the desktop gets")
	assert.Greater(t, last.CropOffset, 80.0)
}

// 4c: after Reset has been applied, the window asks again where the picture on screen is and draws
// that, so it also shows the truth when it opened on a picture dragged in an earlier session.
func TestTuneResetShowsTheCropNowOnScreen(t *testing.T) {
	var renders []provider.TuningOptions
	asked := 0
	h := smartCropHarness(t, 300, 900, func(r *tuneRequest) {
		recordRenders(&renders)(r)
		r.opts.SetCropOffset(10) // dragged earlier
		r.autoPosition = func(image.Image, int, int) (float64, bool) {
			asked++
			if asked == 1 {
				return 50, true // a guess: the screen still shows the dragged crop
			}
			return 64, true // read from the wallpaper Reset put on screen
		}
	})
	assert.Equal(t, 10.0, renders[len(renders)-1].CropOffset, "opens on the dragged position")
	reset := findButton(h.canvasW.Canvas().Content(), "Reset")
	require.NotNil(t, reset)
	test.Tap(reset)
	require.Len(t, h.chosen, 1)
	assert.True(t, h.chosen[0].IsAutoCrop())
	assert.Equal(t, 2, asked, "asked again once Reset was applied")
	assert.Equal(t, 64.0, renders[len(renders)-1].CropOffset, "the preview shows the crop now on the desktop")

	// A drag now starts from there.
	h.dragPreview(0, 1)
	require.Len(t, h.chosen, 2)
	assert.InDelta(t, 64.0, h.chosen[1].CropOffset, 2)
}

// 4a: with no current wallpaper, "Preview unavailable" is one line across the screen-shaped box,
// and nothing else is offered (there is nothing to frame or move).
func TestTuneEmptyStateIsOneLine(t *testing.T) {
	for _, smart := range []bool{false, true} {
		h := newTuneHarness(t, func(r *tuneRequest) {
			r.previewPath = ""
			if smart {
				r.global = provider.DisplayCrop
			}
		})
		root := h.canvasW.Canvas().Content()
		var note *widget.RichText
		for _, o := range shownObjects(root) {
			if rt, ok := o.(*widget.RichText); ok && rt.String() == "Preview unavailable" {
				note = rt
			}
		}
		require.NotNil(t, note, "the empty state says why there is no preview")
		size := note.Size()
		lineHeight := note.MinSize().Height
		assert.Greater(t, size.Width, 3*size.Height, "the text runs across, on one line (got %v)", size)
		assert.LessOrEqual(t, size.Height, lineHeight, "one line high")
		for _, o := range shownObjects(root) {
			_, isMode := o.(*segmented)
			assert.False(t, isMode, "nothing to show differently: no display mode control")
		}
		assert.False(t, shownButton(root, "Reset"))
	}
}
