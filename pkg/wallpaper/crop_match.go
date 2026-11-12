package wallpaper

import (
	"image"
	"math"

	"github.com/disintegration/imaging"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
)

// Matching the wallpaper on screen against the picture it was cropped from.
//
// Tune Image draws its preview from a small copy of the original. The automatic crop (faces,
// entropy) can choose a different part of a small copy than of the full original, so the preview
// must not run that analysis again: it asks where the wallpaper on screen really is instead.
const (
	// cropMatchSide is the length, in pixels, of the uncropped axis when the two pictures are
	// compared. Larger is more precise and slower; 256 takes a few milliseconds.
	cropMatchSide = 256
	// cropMatchMaxDiff is the largest mean difference (0-255 per pixel) still taken as a match.
	// A wallpaper cut from the picture differs by a few levels (JPEG, resampling); another
	// picture differs by tens.
	cropMatchMaxDiff = 24.0
)

// matchCropPercent finds where shown, a wallpaper cropped from src to fill a tgtW×tgtH screen,
// sits along the cropped axis of src: 0 to 100 percent, as in provider.TuningOptions.CropOffset.
// ok is false when src is not cropped for that screen, or when shown does not look like a part
// of src (a framed wallpaper, another picture).
func matchCropPercent(src, shown image.Image, tgtW, tgtH int) (percent float64, ok bool) {
	if src == nil || shown == nil || tgtW <= 0 || tgtH <= 0 {
		return 0, false
	}
	sb := src.Bounds()
	axis, _ := provider.CropTravel(sb.Dx(), sb.Dy(), tgtW, tgtH)
	if axis == provider.CropNone || shown.Bounds().Dx() <= 0 || shown.Bounds().Dy() <= 0 {
		return 0, false
	}

	// Scale both so the uncropped axis is cropMatchSide long; the cropped axis of src is then
	// longer than shown's by the travel.
	var smallSrc, smallShown *image.NRGBA
	var srcLen, shownLen int
	if axis == provider.CropVertical {
		srcLen = int(math.Round(float64(sb.Dy()) * cropMatchSide / float64(sb.Dx())))
		shownLen = int(math.Round(cropMatchSide * float64(tgtH) / float64(tgtW)))
		smallSrc = imaging.Resize(src, cropMatchSide, srcLen, imaging.Box)
		smallShown = imaging.Resize(shown, cropMatchSide, shownLen, imaging.Box)
	} else {
		srcLen = int(math.Round(float64(sb.Dx()) * cropMatchSide / float64(sb.Dy())))
		shownLen = int(math.Round(cropMatchSide * float64(tgtW) / float64(tgtH)))
		smallSrc = imaging.Resize(src, srcLen, cropMatchSide, imaging.Box)
		smallShown = imaging.Resize(shown, shownLen, cropMatchSide, imaging.Box)
	}
	steps := srcLen - shownLen
	if steps < 1 || shownLen < 1 {
		return 0, false
	}

	srcLum := luminance(smallSrc)
	shownLum := luminance(smallShown)
	srcW := smallSrc.Bounds().Dx()
	shownW := smallShown.Bounds().Dx()

	best, bestDiff := 0, math.MaxFloat64
	for s := 0; s <= steps; s++ {
		var sum float64
		if axis == provider.CropVertical {
			// Rows s .. s+shownLen of src against every row of shown.
			for y := 0; y < shownLen && sum < bestDiff; y++ {
				a := srcLum[(s+y)*srcW : (s+y)*srcW+cropMatchSide]
				b := shownLum[y*shownW : y*shownW+cropMatchSide]
				for i := range b {
					sum += math.Abs(float64(a[i]) - float64(b[i]))
				}
			}
		} else {
			for y := 0; y < cropMatchSide && sum < bestDiff; y++ {
				a := srcLum[y*srcW+s : y*srcW+s+shownLen]
				b := shownLum[y*shownW : y*shownW+shownLen]
				for i := range b {
					sum += math.Abs(float64(a[i]) - float64(b[i]))
				}
			}
		}
		if sum < bestDiff {
			best, bestDiff = s, sum
		}
	}
	if bestDiff/float64(shownLen*cropMatchSide) > cropMatchMaxDiff {
		return 0, false
	}
	return float64(best) / float64(steps) * 100, true
}

// luminance is the grey level of every pixel, row by row.
func luminance(img *image.NRGBA) []uint8 {
	b := img.Bounds()
	out := make([]uint8, 0, b.Dx()*b.Dy())
	for y := 0; y < b.Dy(); y++ {
		row := img.Pix[y*img.Stride : y*img.Stride+b.Dx()*4]
		for x := 0; x < len(row); x += 4 {
			out = append(out, uint8((299*int(row[x])+587*int(row[x+1])+114*int(row[x+2]))/1000))
		}
	}
	return out
}
