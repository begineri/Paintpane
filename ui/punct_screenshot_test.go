//go:build screenshots

package ui

import (
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	fynecanvas "fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
)

// TestPunctuationScreenshots renders the screens where · and curly quotes show, for reviewing
// font_narrow.go: the Wikimedia Commons source page (collection names with ·), the confirmation
// for deleting "Wikimedia Featured Astronomy" (quotes and ·) and the tray menu with an artwork
// title in quotes, in English, German and Chinese.
//
//	SCREENSHOT_DIR=<dir> go test -tags screenshots ./ui/ -run TestPunctuationScreenshots
func TestPunctuationScreenshots(t *testing.T) {
	out := os.Getenv("SCREENSHOT_DIR")
	if out == "" {
		t.Skip("SCREENSHOT_DIR not set")
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { i18n.SetLanguage("en") })

	for _, lang := range []string{"en", "de", "zh"} {
		i18n.SetLanguage(lang)
		s := newScenarioWith(t, wallpaper.GetInstance())
		s.sa.App.Settings().SetTheme(newPaintpaneTheme(themeModeLight))

		// The Sources page: group labels such as "Museums · 10".
		s.sa.prefsNav.Select(pageSources)
		savePunctCanvas(t, s, filepath.Join(out, lang+"-sources.png"))

		// wallhaven: collection names such as "General · Random".
		openPunctSource(t, s, "wallhaven")
		savePunctCanvas(t, s, filepath.Join(out, lang+"-wallhaven-source.png"))

		openPunctSource(t, s, "Wikimedia", "维基")
		savePunctCanvas(t, s, filepath.Join(out, lang+"-wikimedia-source.png"))

		// The confirmation for the Astronomy collection.
		found := false
		for _, o := range shownObjects(s.w.Canvas().Content()) {
			b, ok := o.(*dangerButton)
			if !ok || b.Disabled() || found {
				continue
			}
			test.Tap(b)
			flushUI()
			if top := s.w.Canvas().Overlays().Top(); top != nil {
				if punctTextShown(top, "Astronom") || punctTextShown(top, "天文") {
					savePunctCanvas(t, s, filepath.Join(out, lang+"-delete-dialog.png"))
					found = true
				}
				s.w.Canvas().Overlays().Remove(top)
			}
		}
		if !found {
			t.Errorf("%s: no delete confirmation for the Astronomy collection", lang)
		}

		// The tray menu, its first line an artwork title with quotes (· in Chinese).
		items, footer := sampleTrayItems(nil)
		items[0].Title, items[0].Detail = "Young Italian Woman, with ‘Puck’ the Dog", "Thérèse Schwartze"
		if lang == "zh" {
			items[0].Title, items[0].Detail = "安布鲁瓦兹·沃拉尔肖像", "保罗·塞尚"
		}
		items[0].Label = items[0].Title + " — " + items[0].Detail
		saveTrayPNG(t, filepath.Join(out, lang+"-tray.png"), compose(renderTrayView(newTrayMenuView(items, footer, trayMinWidth)), nil, 0))
	}
}

// openPunctSource opens the first source whose title contains one of names.
func openPunctSource(t *testing.T, s *scenario, names ...string) {
	t.Helper()
	s.sa.prefsNav.Select(pageSources)
	s.searchEntry().SetText("")
	for _, st := range sourceTitles(t, s) {
		for _, n := range names {
			if strings.Contains(st, n) {
				s.searchEntry().SetText(st)
				s.tapText(st)
				return
			}
		}
	}
	t.Fatalf("no source named %v", names)
}

func punctTextShown(o fyne.CanvasObject, sub string) bool {
	for _, x := range test.LaidOutObjects(o) {
		if txt, ok := x.(*fynecanvas.Text); ok && strings.Contains(txt.Text, sub) {
			return true
		}
	}
	return false
}

func savePunctCanvas(t *testing.T, s *scenario, path string) {
	t.Helper()
	flushUI()
	// The software painter cannot draw rectangles with a nil fill; the GL painter skips them.
	for _, o := range test.LaidOutObjects(s.w.Canvas().Content()) {
		if r, ok := o.(*fynecanvas.Rectangle); ok && r.FillColor == nil {
			r.FillColor = color.Transparent
		}
	}
	for _, ov := range s.w.Canvas().Overlays().List() {
		for _, o := range test.LaidOutObjects(ov) {
			if r, ok := o.(*fynecanvas.Rectangle); ok && r.FillColor == nil {
				r.FillColor = color.Transparent
			}
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, s.w.Canvas().Capture()); err != nil {
		t.Fatal(err)
	}
}
