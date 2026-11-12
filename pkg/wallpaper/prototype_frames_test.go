//go:build screenshots

// Renders the framed results shown in the UI prototype's Tune Image preview
// (docs/mockups/app-ui) with the app's own VirtualFramer, so the prototype shows
// the real frame, mat and wall colours. Run with:
//
//	SCREENSHOT_DIR=<dir> go test -tags screenshots ./pkg/wallpaper/ -run TestPrototypeFrames
package wallpaper

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/disintegration/imaging"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
)

func TestPrototypeFrames(t *testing.T) {
	out := os.Getenv("SCREENSHOT_DIR")
	if out == "" {
		t.Skip("SCREENSHOT_DIR not set")
	}
	dir := filepath.Join(out, "frames")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	img, err := imaging.Open("testdata/golden/ClevelandMuseum_135428.jpg", imaging.AutoOrientation(true))
	if err != nil {
		t.Fatal(err)
	}
	framer := NewVirtualFramer(nil, &Config{VirtualWallColor: WallAlgorithmic})
	walls := map[string]provider.WallColorOverrideMode{"art": provider.WallColorOverrideAlgorithmic, "neutral": provider.WallColorOverrideNeutral}
	mats := map[string]provider.MattingOverrideMode{"mat": provider.MattingOverrideOn, "nomat": provider.MattingOverrideOff}
	for size := 50; size <= 100; size += 5 {
		for wName, wall := range walls {
			for mName, mat := range mats {
				opts := provider.TuningOptions{FrameSize: float64(size) / 100, WallColor: wall, Matting: mat}
				res, err := framer.renderGalleryWall(img, 1920, 1080, opts)
				if err != nil {
					t.Fatal(err)
				}
				small := imaging.Resize(res, 720, 405, imaging.Lanczos)
				name := filepath.Join(dir, fmt.Sprintf("frame-%d-%s-%s.jpg", size, mName, wName))
				if err := imaging.Save(small, name, imaging.JPEGQuality(82)); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}
