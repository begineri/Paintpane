//go:build !linux

package wallpaper

import (
	"context"
	"image"
	"image/color"
	"testing"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bands paints three stripes along one axis: red first, green in the middle, blue last.
func bands(w, h int, vertical bool) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := x
			size := w
			if vertical {
				p, size = y, h
			}
			c := color.RGBA{0, 255, 0, 255}
			if p < size/3 {
				c = color.RGBA{255, 0, 0, 255}
			} else if p >= size*2/3 {
				c = color.RGBA{0, 0, 255, 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func centreColor(img image.Image) color.RGBA {
	b := img.Bounds()
	r, g, bl, _ := img.At(b.Min.X+b.Dx()/2, b.Min.Y+b.Dy()/2).RGBA()
	return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8), 255}
}

func offsetProcessor() *SmartImageProcessor {
	ResetConfig()
	cfg := GetConfig(NewMockPreferences())
	cfg.SetSmartFitMode(SmartFitNormal)
	return &SmartImageProcessor{os: new(MockOS), config: cfg}
}

// A dragged position sets where the kept part sits along the cropped axis (v29).
func TestFitImageHonoursACropOffset(t *testing.T) {
	proc := offsetProcessor()
	red, green, blue := color.RGBA{255, 0, 0, 255}, color.RGBA{0, 255, 0, 255}, color.RGBA{0, 0, 255, 255}

	t.Run("a tall picture slides up and down", func(t *testing.T) {
		src := bands(300, 900, true) // 1:3, shown on 16:9, only a 300×169 slice is kept
		for _, tc := range []struct {
			offset float64
			want   color.RGBA
		}{{0, red}, {50, green}, {100, blue}} {
			opts := provider.TuningOptions{}
			opts.SetCropOffset(tc.offset)
			out, err := proc.FitImage(context.Background(), src, 320, 180, opts)
			require.NoError(t, err)
			assert.Equal(t, image.Rect(0, 0, 320, 180), out.Bounds())
			assert.Equal(t, tc.want, centreColor(out), "offset %v", tc.offset)
		}
	})

	t.Run("a wide picture slides left and right", func(t *testing.T) {
		src := bands(1800, 300, false) // 6:1
		for _, tc := range []struct {
			offset float64
			want   color.RGBA
		}{{0, red}, {50, green}, {100, blue}} {
			opts := provider.TuningOptions{}
			opts.SetCropOffset(tc.offset)
			out, err := proc.FitImage(context.Background(), src, 320, 180, opts)
			require.NoError(t, err)
			assert.Equal(t, tc.want, centreColor(out), "offset %v", tc.offset)
		}
	})

	t.Run("an offset past the ends stops at the ends", func(t *testing.T) {
		opts := provider.TuningOptions{HasCropOffset: true, CropOffset: 250}
		out, err := proc.FitImage(context.Background(), bands(300, 900, true), 320, 180, opts)
		require.NoError(t, err)
		assert.Equal(t, blue, centreColor(out))
	})
}

// Tune Image starts a drag from where the automatic crop really is.
func TestAutoCropPercent(t *testing.T) {
	proc := offsetProcessor()
	pct, ok := proc.AutoCropPercent(context.Background(), bands(300, 900, true), 320, 180)
	require.True(t, ok)
	assert.GreaterOrEqual(t, pct, 0.0)
	assert.LessOrEqual(t, pct, 100.0)

	_, ok = proc.AutoCropPercent(context.Background(), bands(320, 180, true), 320, 180)
	assert.False(t, ok, "nothing is cropped when the shapes match")

	// What the automatic crop picks, a drag from that position reproduces.
	auto, err := proc.FitImage(context.Background(), bands(300, 900, true), 320, 180, provider.TuningOptions{})
	require.NoError(t, err)
	opts := provider.TuningOptions{}
	opts.SetCropOffset(pct)
	same, err := proc.FitImage(context.Background(), bands(300, 900, true), 320, 180, opts)
	require.NoError(t, err)
	assert.Equal(t, centreColor(auto), centreColor(same))
}

// The keep-area shortcuts keep working: they mean 0, 50 and 100 along the cropped axis.
func TestFitImageAnchorsMeanZeroFiftyOneHundred(t *testing.T) {
	proc := offsetProcessor()
	red, green, blue := color.RGBA{255, 0, 0, 255}, color.RGBA{0, 255, 0, 255}, color.RGBA{0, 0, 255, 255}

	tall := bands(300, 900, true)
	for anchor, want := range map[provider.CropAnchor]color.RGBA{
		provider.AnchorTopLeft: red, provider.AnchorTopCenter: red, provider.AnchorTopRight: red,
		provider.AnchorMiddleLeft: green, provider.AnchorMiddleCenter: green, provider.AnchorMiddleRight: green,
		provider.AnchorBottomLeft: blue, provider.AnchorBottomCenter: blue, provider.AnchorBottomRight: blue,
	} {
		out, err := proc.FitImage(context.Background(), tall, 320, 180, provider.TuningOptions{Anchor: anchor})
		require.NoError(t, err)
		assert.Equal(t, want, centreColor(out), "tall picture, %v", anchor)
	}

	wide := bands(1800, 300, false)
	for anchor, want := range map[provider.CropAnchor]color.RGBA{
		provider.AnchorTopLeft: red, provider.AnchorMiddleLeft: red, provider.AnchorBottomLeft: red,
		provider.AnchorTopCenter: green, provider.AnchorMiddleCenter: green, provider.AnchorBottomCenter: green,
		provider.AnchorTopRight: blue, provider.AnchorMiddleRight: blue, provider.AnchorBottomRight: blue,
	} {
		out, err := proc.FitImage(context.Background(), wide, 320, 180, provider.TuningOptions{Anchor: anchor})
		require.NoError(t, err)
		assert.Equal(t, want, centreColor(out), "wide picture, %v", anchor)
	}
}
