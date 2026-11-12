package wallpaper

import (
	"context"
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"

	"github.com/dixieflatline76/Spice/v2/util/log"
)

// Originals are capped at about 4K unless High-resolution originals is on (S4-3H; the owner,
// 2026-11-08: "限制约4K … 默认关 … 5k，8k屏觉模糊，可以自己打开，代价是磁盘占用增多").
//
// The cap is "just enough for a 4K screen in every display mode": an original is shrunk, keeping
// its proportions, until its long edge is 3840 or its short edge 2160, whichever comes first, so
// it still covers a 3840x2160 screen turned the same way (Full artwork and Museum frame need only
// the long edge; Smart crop needs the short edge to cover the screen too, which matters for wide
// pictures). Then at most capMaxPixels (two 4K screens), so an extreme panorama or scroll stays
// bounded. That is 8.3 MP for 16:9, 11 MP for 4:3, 14.7 MP for a square, 16.6 MP for 3.6:1 and
// wider. Anything up to 3840x2160, including the 2560x1600 and 1920x1080 screens most people
// have, gets no upscaling; 5K and 8K screens do, which is what the switch is for.
const (
	capLongEdge  = 3840
	capShortEdge = 2160
	capMaxPixels = 2 * 3840 * 2160
	// capShrinkSlack: a downloaded original up to this much larger (in pixels) than its capped size
	// is kept as it is; shrinking it would re-encode the file for little saving.
	capShrinkSlack = 1.2
	// capJPEGQuality is the quality a shrunk JPEG original is saved at.
	capJPEGQuality = 92
)

// cappedSize returns the size an original of w x h is kept at while originals are capped. It
// returns w, h unchanged when the original is no larger than the cap.
func cappedSize(w, h int) (int, int) {
	if w <= 0 || h <= 0 {
		return w, h
	}
	long, short := float64(max(w, h)), float64(min(w, h))
	s := math.Max(capLongEdge/long, capShortEdge/short)
	if area := float64(w) * float64(h); area*s*s > capMaxPixels {
		s = math.Sqrt(capMaxPixels / area)
	}
	if s >= 1 {
		return w, h
	}
	cw := max(1, int(math.Round(float64(w)*s)))
	ch := max(1, int(math.Round(float64(h)*s)))
	return cw, ch
}

// SMK chooses the size it lists images at by asking the config for this method by name
// (providers/smk highResSetting, docs/smk-download.md §7); renaming it would silently turn SMK's
// high-resolution originals off, so the build breaks instead.
var _ interface{ GetHighResOriginals() bool } = (*Config)(nil)

// capOriginals reports whether originals are capped (High-resolution originals is off).
func (wp *Plugin) capOriginals() bool {
	return wp.cfg != nil && !wp.cfg.GetHighResOriginals()
}

// expectedMasterSize is the size an original reported as w x h will have on disk.
func (wp *Plugin) expectedMasterSize(w, h int) (int, int) {
	if !wp.capOriginals() {
		return w, h
	}
	return cappedSize(w, h)
}

// capMaster shrinks the original at path to its capped size when it is more than capShrinkSlack
// larger, writing the smaller file in its place (under a temporary name first, so nothing reads
// half a file). It returns the size of the original on disk afterwards, or 0, 0 when the file is
// not an image it can read (making the derivatives then reports the error). Callers check
// capOriginals first.
func capMaster(ctx context.Context, path string) (int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	cfg, _, err := image.DecodeConfig(f)
	f.Close()
	if err != nil {
		return 0, 0, nil
	}
	w, h := cfg.Width, cfg.Height
	cw, ch := cappedSize(w, h)
	if float64(w)*float64(h) <= capShrinkSlack*float64(cw)*float64(ch) {
		return w, h, nil
	}
	ext := strings.ToLower(filepath.Ext(path))
	format, err := imaging.FormatFromExtension(ext)
	if err != nil || (format != imaging.JPEG && format != imaging.PNG) {
		return w, h, nil // kept as it is; only JPEG and PNG originals are rewritten
	}

	// Resizing keeps an intermediate of the new width by the old height and the result, both 4
	// bytes per pixel, next to the decoded original (measured: a 42 MP original took 198 MB to
	// shrink, 125 MB to fit for the screen).
	extraMB := (int64(cw)*int64(h) + int64(cw)*int64(ch)) * 4 >> 20
	release, err := acquireMasterDecodePlus(ctx, path, extraMB)
	if err != nil {
		return w, h, err
	}
	defer release()
	src, err := openMaster(path)
	if err != nil {
		return w, h, fmt.Errorf("decoding original to shrink it: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return w, h, err
	}
	small := imaging.Resize(src, cw, ch, imaging.Lanczos)
	src = nil

	tmp := strings.TrimSuffix(path, filepath.Ext(path)) + ".cap.tmp" + filepath.Ext(path)
	if err := imaging.Save(small, tmp, imaging.JPEGQuality(capJPEGQuality)); err != nil {
		_ = os.Remove(tmp)
		return w, h, fmt.Errorf("saving shrunk original: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return w, h, fmt.Errorf("replacing original: %w", err)
	}
	log.Debugf("Original %s shrunk from %dx%d to %dx%d (High-resolution originals is off).", filepath.Base(path), w, h, cw, ch)
	return cw, ch, nil
}
