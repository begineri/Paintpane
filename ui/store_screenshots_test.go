//go:build screenshots

// Puts the app's own tray menu and settings window, rendered with Fyne's software painter, on the
// desktop pictures from pkg/wallpaper/store_screenshots_test.go, for the Microsoft Store
// screenshots (docs/store-listing.md, section 4). Nothing opens on the screen. Run with
// `make store-screenshots`, which renders the wallpapers first.
package ui

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/disintegration/imaging"

	"github.com/dixieflatline76/Spice/v2/asset"
	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
)

func TestStoreScreenshots(t *testing.T) {
	out := os.Getenv("SCREENSHOT_DIR")
	if out == "" {
		t.Skip("SCREENSHOT_DIR not set")
	}
	dir := filepath.Join(out, "store")
	full, err := imaging.Open(filepath.Join(dir, "wall-full.png"))
	if err != nil {
		t.Skipf("render the wallpapers first (make store-screenshots): %v", err)
	}
	framed, err := imaging.Open(filepath.Join(dir, "wall-frame.png"))
	if err != nil {
		t.Fatal(err)
	}

	a := test.NewTempApp(t)
	a.Settings().SetTheme(newPaintpaneTheme(themeModeLight))
	wp := wallpaper.GetInstance()
	wp.Init(&screenshotManager{prefs: a.Preferences(), am: asset.NewManager()})

	for _, lang := range []string{"zh", "en"} {
		i18n.SetLanguage(lang)
		save := func(name string, img image.Image) {
			t.Helper()
			if err := imaging.Save(img, filepath.Join(dir, lang+"-"+name+".png")); err != nil {
				t.Fatal(err)
			}
		}
		save("1-full-artwork", full)
		save("2-museum-frame", framed)

		// The tray menu as it opens from the notification area, bottom right.
		items, footer := storeTrayItems()
		menu := renderTrayView(newTrayMenuView(withLockScreenRow(items, nil), footer, trayMinWidth))
		mb := menu.Bounds()
		save("3-tray-menu", withWindow(full, menu, image.Pt(storeW-mb.Dx()-24, storeH-mb.Dy()-24), 8))

		// Settings > Sources, beside the painting.
		page := filepath.Join(t.TempDir(), "sources.png")
		capturePreferences(t, a, wp, screenshotScreen{name: "sources", page: pageSources}, page)
		prefs, err := imaging.Open(page)
		if err != nil {
			t.Fatal(err)
		}
		pb := prefs.Bounds()
		save("4-sources", withWindow(full, prefs, image.Pt(storeW-pb.Dx()-96, (storeH-pb.Dy())/2), 8))
	}
	i18n.SetLanguage("en")
}

const storeW, storeH = 1920, 1080

// storeTrayItems is the sample tray menu showing the painting on the desktop.
func storeTrayItems() ([]schema.MenuItemSchema, []schema.MenuItemSchema) {
	items, footer := sampleTrayItems(nil)
	logo, _ := os.ReadFile("../pkg/wallpaper/providers/artic/artic.png")
	items[0] = schema.MenuItemSchema{Kind: schema.MenuItemNowShowing,
		Label:     "Two Sisters (On the Terrace) — Pierre-Auguste Renoir",
		Title:     "Two Sisters (On the Terrace)",
		Detail:    "Pierre-Auguste Renoir",
		IconName:  "Art Institute of Chicago",
		IconBytes: logo,
		Action:    func() {}}
	return items, footer
}

// withWindow draws win on desk at pos with rounded corners and a soft shadow, the way Windows 11
// shows a window or menu.
func withWindow(desk, win image.Image, pos image.Point, radius int) image.Image {
	out := imaging.Clone(desk)
	b := win.Bounds()
	const blur, spread, drop = 16.0, 40, 8
	sh := image.NewGray(image.Rect(0, 0, b.Dx()+2*spread, b.Dy()+2*spread))
	fillRounded(sh, image.Rect(spread, spread, spread+b.Dx(), spread+b.Dy()), radius, color.Gray{255})
	soft := imaging.Blur(sh, blur) // grey levels: how dark the shadow is at each pixel
	origin := pos.Sub(image.Pt(spread, spread-drop))
	for y := 0; y < soft.Bounds().Dy(); y++ {
		for x := 0; x < soft.Bounds().Dx(); x++ {
			px, py := origin.X+x, origin.Y+y
			if !image.Pt(px, py).In(out.Bounds()) {
				continue
			}
			k := float64(soft.NRGBAAt(x, y).R) / 255 * 0.45
			c := out.NRGBAAt(px, py)
			c.R, c.G, c.B = uint8(float64(c.R)*(1-k)), uint8(float64(c.G)*(1-k)), uint8(float64(c.B)*(1-k))
			out.SetNRGBA(px, py, c)
		}
	}

	mask := image.NewAlpha(image.Rect(0, 0, b.Dx(), b.Dy()))
	fillRounded(mask, mask.Bounds(), radius, color.Alpha{255})
	rounded := image.NewNRGBA(mask.Bounds())
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c := color.NRGBAModel.Convert(win.At(b.Min.X+x, b.Min.Y+y)).(color.NRGBA)
			c.A = uint8(uint16(c.A) * uint16(mask.AlphaAt(x, y).A) / 255)
			rounded.SetNRGBA(x, y, c)
		}
	}
	return imaging.Overlay(out, rounded, pos, 1)
}

// fillRounded sets every pixel of r inside a rectangle with corners of the given radius to c.
func fillRounded(img interface{ Set(int, int, color.Color) }, r image.Rectangle, radius int, c color.Color) {
	inside := func(x, y int) bool {
		cx, cy := x, y
		switch {
		case x < r.Min.X+radius:
			cx = r.Min.X + radius
		case x >= r.Max.X-radius:
			cx = r.Max.X - radius - 1
		}
		switch {
		case y < r.Min.Y+radius:
			cy = r.Min.Y + radius
		case y >= r.Max.Y-radius:
			cy = r.Max.Y - radius - 1
		}
		dx, dy := x-cx, y-cy
		return dx*dx+dy*dy <= radius*radius
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if inside(x, y) {
				img.Set(x, y, c)
			}
		}
	}
}
