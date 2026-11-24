//go:build screenshots

// Renders the desktop wallpapers used in the Microsoft Store screenshots (docs/store-listing.md,
// section 4) with the app's own framer: Renoir's Two Sisters (On the Terrace), Art Institute of
// Chicago, public domain, shown whole and in a museum frame at 1920 x 1080. The painting is cut
// out of the intro page's desktop picture, so no download is needed. ui/store_screenshots_test.go
// puts the tray menu and the settings window on top. Run with `make store-screenshots`.
package wallpaper

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/disintegration/imaging"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
)

// storeShotW and storeShotH are a common 16:9 desktop; Partner Center asks for 1366 x 768 or larger.
const storeShotW, storeShotH = 1920, 1080

func TestStoreWallpapers(t *testing.T) {
	out := os.Getenv("SCREENSHOT_DIR")
	if out == "" {
		t.Skip("SCREENSHOT_DIR not set")
	}
	dir := filepath.Join(out, "store")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	desk, err := imaging.Open(filepath.Join("..", "..", "site", "assets", "img", "s1-desktop.jpg"))
	if err != nil {
		t.Fatal(err)
	}
	art := cutPainting(desk)
	if art.Bounds().Dx() < 600 || art.Bounds().Dy() < 900 {
		t.Fatalf("painting not found in the intro picture: %v", art.Bounds())
	}

	// A new user's defaults: algorithmic wall colour, frame at 85 %, no paper mat (config.go).
	framer := NewVirtualFramer(nil, &Config{VirtualWallColor: WallAlgorithmic, VirtualFrameSize: 0.85})
	full := framer.renderFullArtwork(art, storeShotW, storeShotH, provider.TuningOptions{})
	framed, err := framer.renderGalleryWall(art, storeShotW, storeShotH, provider.TuningOptions{})
	if err != nil {
		t.Fatal(err)
	}
	for name, img := range map[string]image.Image{"wall-full.png": full, "wall-frame.png": framed} {
		if err := imaging.Save(img, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
}

// cutPainting returns the part of a desktop picture that is not the plain wall colour around it.
func cutPainting(desk image.Image) image.Image {
	b := desk.Bounds()
	wall := desk.At(b.Min.X+2, b.Min.Y+2)
	differs := func(x, y int) bool { return storeColorDistance(desk.At(x, y), wall) > 40 }
	midX, midY := (b.Min.X+b.Max.X)/2, (b.Min.Y+b.Max.Y)/2
	left, right, top, bottom := b.Min.X, b.Max.X-1, b.Min.Y, b.Max.Y-1
	for left < midX && !differs(left, midY) {
		left++
	}
	for right > midX && !differs(right, midY) {
		right--
	}
	for top < midY && !differs(midX, top) {
		top++
	}
	for bottom > midY && !differs(midX, bottom) {
		bottom--
	}
	// One pixel in from each edge: the JPEG blends the painting's border into the wall.
	return imaging.Crop(desk, image.Rect(left+1, top+1, right, bottom))
}

func storeColorDistance(a, b color.Color) int {
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	d := func(x, y uint32) int {
		if x > y {
			return int(x-y) >> 8
		}
		return int(y-x) >> 8
	}
	return d(ar, br) + d(ag, bg) + d(ab, bb)
}
