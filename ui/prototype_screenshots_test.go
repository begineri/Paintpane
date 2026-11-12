//go:build screenshots

// Full-height captures of every preferences page and source, used as the
// reference images of the UI prototype in docs/mockups/app-ui. Run with:
//
//	SCREENSHOT_DIR=<dir> go test -tags screenshots ./ui/ -run TestPrototypeScreenshots
package ui

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	fynecanvas "fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"

	"github.com/dixieflatline76/Spice/v2/asset"
	"github.com/dixieflatline76/Spice/v2/config"
	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	spiceui "github.com/dixieflatline76/Spice/v2/pkg/ui"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
)

func TestPrototypeScreenshots(t *testing.T) {
	out := os.Getenv("SCREENSHOT_DIR")
	if out == "" {
		t.Skip("SCREENSHOT_DIR not set")
	}
	_ = os.MkdirAll(out, 0o755)
	a := test.NewTempApp(t)
	wp := wallpaper.GetInstance()
	wp.Init(&screenshotManager{prefs: a.Preferences(), am: asset.NewManager()})

	sources := []string{
		"Art Institute of Chicago", "Cleveland Museum of Art", "National Palace Museum", "Rijksmuseum",
		"Statens Museum for Kunst", "The Getty", "The Metropolitan Museum of Art",
		"Favorites", "Local folders", "Pexels", "wallhaven", "Wikimedia",
	}
	for _, lang := range []string{"zh", "en"} {
		i18n.SetLanguage(lang)
		for vName, mode := range map[string]themeMode{"light": themeModeLight, "dark": themeModeDark} {
			// Save the mode too: the General page's theme selector applies the saved
			// mode, which otherwise switches later captures back to System (light).
			config.NewAppConfig(a.Preferences()).SetTheme(string(mode))
			a.Settings().SetTheme(newPaintpaneTheme(mode))
			protoMode = mode
			pre := filepath.Join(out, lang+"-"+vName+"-")
			protoCapture(t, a, wp, pageSources, nil, pre+"sources.png")
			for _, s := range sources {
				name := strings.ToLower(strings.ReplaceAll(s, " ", "-"))
				src := s
				protoCapture(t, a, wp, pageSources, func(w fyne.Window) {
					expandCommunity(w)
					if !tapSourceRowAny(w.Canvas().Content(), src) {
						t.Logf("source %q not found", src)
					}
				}, pre+"src-"+name+".png")
			}
			for i, m := range []string{"smart", "full", "frame"} {
				idx := i
				protoCapture(t, a, wp, pageWallpaper, func(w fyne.Window) {
					for _, o := range test.LaidOutObjects(w.Canvas().Content()) {
						if s, ok := o.(*segmented); ok && len(s.Options) == 3 && s.Options[0] != i18n.T("Off") {
							s.SetSelectedIndex(idx)
						}
					}
				}, pre+"wallpaper-"+m+".png")
			}
			protoCapture(t, a, wp, pageGeneral, nil, pre+"general.png")
			protoCapture(t, a, wp, pageAdvanced, nil, pre+"advanced.png")
		}
	}
	i18n.SetLanguage("en")
}

func expandCommunity(w fyne.Window) {
	for _, o := range test.LaidOutObjects(w.Canvas().Content()) {
		area, ok := o.(*tapArea)
		if !ok {
			continue
		}
		for _, inner := range test.LaidOutObjects(area.content) {
			if r, ok := inner.(*styledText); ok && strings.HasPrefix(r.Text, i18n.T("Community")+" ·") {
				test.Tap(area)
				return
			}
		}
	}
}

func tapSourceRowAny(root fyne.CanvasObject, key string) bool {
	if tapSourceRow(root, i18n.T(key)) {
		return true
	}
	return tapSourceRow(root, key)
}

// protoMode is the theme of the current capture. Building the General page
// re-applies the theme, and the software painter sometimes captures before that
// change has reached every object, so protoCapture checks the background.
var protoMode themeMode

func protoCapture(t *testing.T, a fyne.App, wp *wallpaper.Plugin, page string, act func(fyne.Window), path string) {
	t.Helper()
	w := test.NewWindow(nil)
	defer w.Close()
	w.SetPadded(false)
	const tall = 2400
	w.Resize(fyne.NewSize(defaultPrefsWindowWidth, tall))
	sa := &SpiceApp{App: a, appConfig: config.NewAppConfig(a.Preferences()), plugins: []spiceui.Plugin{wp}, prefsWindow: w}
	sa.RebuildPreferencesContent("")
	// Apply changes now, so the auto-apply timer cannot rebuild widgets while a
	// later capture runs (the test driver runs fyne.Do on the timer goroutine).
	defer sa.prefsSM.FlushPending()
	sa.prefsNav.Select(page)
	w.Resize(fyne.NewSize(defaultPrefsWindowWidth, tall))
	if act != nil {
		act(w)
		sa.prefsSM.refreshWidgetStates()
		w.Content().Refresh()
	}
	for _, o := range test.LaidOutObjects(w.Canvas().Content()) {
		if r, ok := o.(*fynecanvas.Rectangle); ok && r.FillColor == nil {
			r.FillColor = color.Transparent
		}
	}
	// The software painter sometimes captures before a theme change has reached
	// every object; such a capture has the wrong background and is not cropped.
	want := lightPalette[theme.ColorNameBackground]
	if protoMode == themeModeDark {
		want = darkPalette[theme.ColorNameBackground]
	}
	var img image.Image
	for try := 0; try < 10; try++ {
		w.Content().Refresh()
		time.Sleep(150 * time.Millisecond)
		raw := w.Canvas().Capture()
		if !sameColor(raw.At(raw.Bounds().Max.X-10, 10), want) {
			a.Settings().SetTheme(newPaintpaneTheme(protoMode))
			continue
		}
		img = cropToContent(raw, want)
		if img.Bounds().Dy() < tall {
			break
		}
	}
	if img == nil {
		t.Errorf("%s: background never matched the %s theme", filepath.Base(path), protoMode)
		return
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_ = png.Encode(f, img)
}

// cropToContent cuts trailing rows whose content area is plain background.
func cropToContent(img image.Image, bg color.Color) image.Image {
	b := img.Bounds()
	br, bgc, bb, _ := bg.RGBA()
	last := 0
	for y := b.Max.Y - 1; y >= 0; y-- {
		plain := true
		for x := 190; x < b.Max.X-20; x += 2 {
			r, g, bl, _ := img.At(x, y).RGBA()
			if absDiff(r, br) > 0x0800 || absDiff(g, bgc) > 0x0800 || absDiff(bl, bb) > 0x0800 {
				plain = false
				break
			}
		}
		if !plain {
			last = y
			break
		}
	}
	h := last + 24
	if h < 640 {
		h = 640
	}
	if h > b.Max.Y {
		h = b.Max.Y
	}
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx(), h))
	draw.Draw(dst, dst.Bounds(), img, b.Min, draw.Src)
	return dst
}

func sameColor(c, want color.Color) bool {
	r, g, b, _ := c.RGBA()
	wr, wg, wb, _ := want.RGBA()
	return absDiff(r, wr) <= 0x0800 && absDiff(g, wg) <= 0x0800 && absDiff(b, wb) <= 0x0800
}

func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}
