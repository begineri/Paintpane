//go:build screenshots

package ui

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
)

// TestTrayScreenshots renders the app-drawn tray menu: open, and with the
// display mode submenu open to its left.
//
//	SCREENSHOT_DIR=<dir> go test -tags screenshots ./ui/ -run TestTrayScreenshots
func TestTrayScreenshots(t *testing.T) {
	out := os.Getenv("SCREENSHOT_DIR")
	if out == "" {
		t.Skip("SCREENSHOT_DIR not set")
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	a := test.NewTempApp(t)
	variants := map[string]themeMode{"light": themeModeLight, "dark": themeModeDark}
	for _, lang := range []string{"zh", "en"} {
		i18n.SetLanguage(lang)
		for vName, mode := range variants {
			a.Settings().SetTheme(newPaintpaneTheme(mode))
			prefix := filepath.Join(out, lang+"-"+vName+"-tray")
			items, footer := sampleTrayItems(nil)

			main := renderTrayView(newTrayMenuView(items, footer, trayMinWidth))
			saveTrayPNG(t, prefix+".png", compose(main, nil, 0))

			v := newTrayMenuView(items, footer, trayMinWidth)
			row := v.regions[v.regionByLabel(i18n.T("Display mode"))]
			v.active, v.openSub = v.regionByLabel(i18n.T("Display mode")), v.regionByLabel(i18n.T("Display mode"))
			sub := renderTrayView(newTrayMenuView(row.item.SubMenu.Items, nil, subMenuWidth(row.item.SubMenu.Items)))
			saveTrayPNG(t, prefix+"-submenu.png", compose(renderTrayView(v), sub, row.pos.Y-trayInset))

			// T15: the menu on Windows, with "Next lock screen picture".
			saveTrayPNG(t, prefix+"-lock.png", compose(renderTrayView(newTrayMenuView(withLockScreenRow(items, nil), footer, trayMinWidth)), nil, 0))

		}
	}
	i18n.SetLanguage("en")
}

func renderTrayView(v *trayMenuView) image.Image {
	v.iconFor = func(it schema.MenuItemSchema) fyne.Resource {
		if len(it.IconBytes) > 0 {
			return fyne.NewStaticResource(it.IconName, it.IconBytes)
		}
		return nil
	}
	v.Refresh()
	w := test.NewWindow(v)
	defer w.Close()
	w.SetPadded(false)
	w.Resize(v.MinSize())
	return w.Canvas().Capture()
}

// compose puts the menu on a grey desk, with a submenu to its left (1 px gap),
// its top at subTop in the menu's coordinates.
func compose(main, sub image.Image, subTop float32) image.Image {
	const margin = 16
	mb := main.Bounds()
	left := margin
	if sub != nil {
		left += sub.Bounds().Dx() + 1
	}
	h := mb.Dy()
	if sub != nil {
		h = max(h, int(subTop)+sub.Bounds().Dy())
	}
	out := image.NewRGBA(image.Rect(0, 0, left+mb.Dx()+margin, h+2*margin))
	draw.Draw(out, out.Bounds(), &image.Uniform{color.NRGBA{0x8A, 0x8F, 0x96, 0xFF}}, image.Point{}, draw.Src)
	draw.Draw(out, mb.Add(image.Pt(left, margin)), main, image.Point{}, draw.Over)
	if sub != nil {
		draw.Draw(out, sub.Bounds().Add(image.Pt(margin, margin+int(subTop))), sub, image.Point{}, draw.Over)
	}
	return out
}

func saveTrayPNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
