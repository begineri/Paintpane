package wallpaper

import (
	"context"
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/disintegration/imaging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
)

// texturedImage is a w×h picture whose every part looks different, like a painting: smooth
// gradients with a few blobs, so a crop of it can be found again.
func texturedImage(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			fx, fy := float64(x)/float64(w), float64(y)/float64(h)
			r := 128 + 100*math.Sin(fx*7+fy*3)
			g := 128 + 100*math.Cos(fy*11-fx*2)
			b := 128 + 100*math.Sin((fx+fy)*5)
			if (x/(w/7+1)+y/(h/9+1))%3 == 0 {
				r, b = b, r
			}
			img.SetNRGBA(x, y, color.NRGBA{uint8(r), uint8(g), uint8(b), 255})
		}
	}
	return img
}

// The wallpaper is cut from the full original; the match is made against a small copy, as in
// Tune Image, and finds where the cut was made.
func TestMatchCropPercentFindsTheCropOnScreen(t *testing.T) {
	defer ResetConfig()
	proc := offsetProcessor()
	cases := []struct {
		name       string
		w, h       int
		scrW, scrH int
	}{
		{"tall picture, wide screen", 2400, 3000, 1280, 800},
		{"wide picture", 4000, 1200, 1280, 800},
		{"close to the screen shape", 1700, 1000, 1280, 800},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			full := texturedImage(c.w, c.h)
			small := imaging.Fit(full, 1000, 1000, imaging.Lanczos)
			for _, want := range []float64{0, 23, 50, 81, 100} {
				opts := provider.TuningOptions{}
				opts.SetCropOffset(want)
				shown, err := proc.FitImage(context.Background(), full, c.scrW, c.scrH, opts)
				require.NoError(t, err)
				got, ok := matchCropPercent(small, shown, c.scrW, c.scrH)
				require.True(t, ok, "the wallpaper at %v %% is found", want)
				assert.InDelta(t, want, got, 3, "cut at %v %%, found at %v %%", want, got)
			}
		})
	}
}

func TestMatchCropPercentRefusesWhatIsNotACrop(t *testing.T) {
	src := texturedImage(1200, 1800)
	// Not cropped for this screen.
	_, ok := matchCropPercent(src, src, 1200, 1800)
	assert.False(t, ok)
	// Another picture.
	other := imaging.Rotate180(texturedImage(1280, 800))
	_, ok = matchCropPercent(src, other, 1280, 800)
	assert.False(t, ok, "a picture that is not part of src is not a match")
	// A framed wallpaper: the picture small in the middle of a wall.
	wall := imaging.New(1280, 800, color.NRGBA{90, 90, 90, 255})
	framed := imaging.Overlay(wall, imaging.Resize(src, 0, 600, imaging.Lanczos), image.Pt(440, 100), 1)
	_, ok = matchCropPercent(src, framed, 1280, 800)
	assert.False(t, ok, "a framed picture is not a crop")
}
