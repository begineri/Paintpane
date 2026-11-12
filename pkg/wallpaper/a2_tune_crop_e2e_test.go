//go:build !linux

package wallpaper

import (
	"image/color"
	"path/filepath"
	"testing"
	"time"

	"github.com/disintegration/imaging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
)

// A2: dragging the preview in Tune Image (docs/ui_refresh.md J3, v29) reaches the desktop.
// A 700x600 picture on an 800x500 screen in Smart crop is cropped along the vertical axis
// (it is scaled to 800x686 and 500 rows are kept). The seeded picture has four coloured
// quadrants, so the dragged position (0, 50 or 100 %) can be read back from where the
// colour changes in the picture the desktop shows: at row 343 minus the rows cropped off the top.
func TestA2_TuneDragChoosesTheKeptPartOnTheDesktop(t *testing.T) {
	defer ResetConfig()
	app := newE2EApp(t, t.TempDir(), NewMockPreferences(), e2eMonitor(0, 800, 500))
	app.seed(t, map[string][2]int{"pic": {700, 600}})
	app.wp.cfg.SetSmartFitMode(SmartFitAggressive)
	app.wp.Activate()
	defer app.stop()
	app.wp.SetArtworkDisplayMode(ArtworkDisplayFull)
	app.waitUntilSettled(t, filepath.Join(FittedRootDir, FullArtDir), []string{"pic"})
	app.wp.SetArtworkDisplayMode(ArtworkDisplaySmartCrop)
	app.waitUntilSettled(t, filepath.Join(FittedRootDir, FlexibilityDir), []string{"pic"})

	// boundary is the first row, going down the left edge, where the top-left quadrant ends.
	boundary := func(path string) int {
		img, err := imaging.Open(path)
		require.NoError(t, err)
		require.Equal(t, 800, img.Bounds().Dx())
		require.Equal(t, 500, img.Bounds().Dy())
		for y := 0; y < img.Bounds().Dy(); y++ {
			if !isNear(quadTL, img.At(10, y)) {
				return y
			}
		}
		return -1
	}
	setOffset := func(pct float64) string {
		app.os.mu.Lock()
		calls := len(app.os.set[0])
		app.os.mu.Unlock()
		app.wp.SetTuningOptions(0, provider.TuningOptions{CropOffset: pct, HasCropOffset: true})
		require.Eventually(t, func() bool {
			app.os.mu.Lock()
			defer app.os.mu.Unlock()
			return len(app.os.set[0]) > calls
		}, 10*time.Second, 50*time.Millisecond, "the desktop is set again after a drag to %v %%", pct)
		return app.os.last(0)
	}

	const scaled, kept = 686, 500
	for _, pct := range []float64{0, 100, 50, 25} {
		want := scaled/2 - int(pct/100*float64(scaled-kept))
		got := boundary(setOffset(pct))
		assert.InDelta(t, want, got, 12, "dragged to %v %%: the colour changes at row %d (want about %d)", pct, got, want)
	}
}

func isNear(want color.RGBA, got color.Color) bool {
	r, g, b, _ := got.RGBA()
	near := func(a uint8, c uint32) bool {
		d := int(a) - int(c>>8)
		return d > -40 && d < 40
	}
	return near(want.R, r) && near(want.G, g) && near(want.B, b)
}
