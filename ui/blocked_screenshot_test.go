//go:build screenshots

package ui

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	fynecanvas "fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"github.com/disintegration/imaging"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
)

// TestBlockedScreenshots renders the blocked-images row and page (S4-5, S4-B)
// for comparison with the prototype: the Advanced page, the thumbnails, a
// tooltip, and the empty page.
//
//	SCREENSHOT_DIR=<dir> go test -tags screenshots ./ui/ -run TestBlockedScreenshots
func TestBlockedScreenshots(t *testing.T) {
	out := os.Getenv("SCREENSHOT_DIR")
	if out == "" {
		t.Skip("SCREENSHOT_DIR not set")
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	thumbs := prototypeThumbs(t)
	variants := map[string]themeMode{"light": themeModeLight, "dark": themeModeDark}
	for _, lang := range []string{"zh", "en"} {
		i18n.SetLanguage(lang)
		for vName, mode := range variants {
			prefix := filepath.Join(out, lang+"-"+vName+"-blocked")
			items := sampleBlocked(t)
			for i := range thumbs {
				items[i].ThumbPath = thumbs[i]
			}

			s := newScenarioWith(t, &blockedPlugin{items: items})
			s.sa.App.Settings().SetTheme(newPaintpaneTheme(mode))
			s.sa.RebuildPreferencesContent("")
			s.w.Resize(fyne.NewSize(defaultPrefsWindowWidth, defaultPrefsWindowHeight))
			s.sa.prefsNav.Select(pageAdvanced)
			saveCapture(t, s.w, prefix+"-advanced.png")

			s.tapText(i18n.T("View"))
			saveCapture(t, s.w, prefix+".png")
			g := s.blockedGrid()
			g.showTip(1, false)
			saveCapture(t, s.w, prefix+"-tip.png")
			g.showTip(0, true)
			saveCapture(t, s.w, prefix+"-tip-x.png")

			s2 := newScenarioWith(t, &blockedPlugin{})
			s2.sa.App.Settings().SetTheme(newPaintpaneTheme(mode))
			s2.sa.RebuildPreferencesContent("")
			s2.w.Resize(fyne.NewSize(defaultPrefsWindowWidth, defaultPrefsWindowHeight))
			s2.sa.prefsNav.Select(pageAdvanced)
			s2.tapText(i18n.T("View"))
			saveCapture(t, s2.w, prefix+"-empty.png")
		}
	}
	i18n.SetLanguage("en")
}

// thumbSources are the paintings the thumbnails are cut from: the prototype's
// sample artwork, which its build script makes and which is not checked in, and
// otherwise museum paintings the repository keeps as test data (A2 §12.9 #6).
var thumbSources = [][]string{
	{"../docs/mockups/app-ui/assets/artwork.jpg"},
	{"../pkg/wallpaper/testdata/golden/ClevelandMuseum_135428.jpg", "../pkg/wallpaper/testdata/golden/NationalPalaceMuseum_223.jpg"},
}

// prototypeThumbs crops sample artwork the way the prototype's tiles do
// (docs/mockups/app-ui, S4-B): four parts of a painting.
func prototypeThumbs(t *testing.T) []string {
	t.Helper()
	var srcs []image.Image
	for _, set := range thumbSources {
		srcs = srcs[:0]
		for _, path := range set {
			img, err := imaging.Open(path)
			if err != nil {
				break
			}
			srcs = append(srcs, img)
		}
		if len(srcs) == len(set) {
			break
		}
	}
	if len(srcs) == 0 {
		t.Skip("no sample artwork found")
	}
	origins := [][2]float64{{0.15, 0.20}, {0.90, 0.85}, {0.80, 0.10}, {0.20, 0.90}}
	var paths []string
	for i, o := range origins {
		src := srcs[i%len(srcs)]
		b := src.Bounds()
		w, h := b.Dx()*10/26, b.Dy()*10/26
		x := int(o[0] * float64(b.Dx()-w))
		y := int(o[1] * float64(b.Dy()-h))
		crop := imaging.Crop(src, image.Rect(x, y, x+w, y+h))
		path := filepath.Join(t.TempDir(), string(rune('a'+i))+".jpg")
		if err := imaging.Save(imaging.Fit(crop, 360, 360, imaging.Lanczos), path); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	return paths
}

func saveCapture(t *testing.T, w fyne.Window, path string) {
	t.Helper()
	// The software painter cannot draw rectangles with a nil fill; the GL painter skips them.
	for _, o := range test.LaidOutObjects(w.Canvas().Content()) {
		if r, ok := o.(*fynecanvas.Rectangle); ok && r.FillColor == nil {
			r.FillColor = color.Transparent
		}
	}
	saveTrayPNG(t, path, w.Canvas().Capture())
}
