package ui

import (
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	fynecanvas "fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
)

// sampleTrayItems is the tray as the prototype draws it (docs/mockups/app-ui,
// tray page): Rijksmuseum's The Milkmaid, one display, Favorites active.
func sampleTrayItems(used *[]string) ([]schema.MenuItemSchema, []schema.MenuItemSchema) {
	act := func(name string) func() {
		return func() {
			if used != nil {
				*used = append(*used, name)
			}
		}
	}
	logo, _ := os.ReadFile("../pkg/wallpaper/providers/rijksmuseum/Rijksmuseum.png")
	modes := []schema.MenuItemSchema{
		{Label: i18n.T("Full artwork"), IsChecked: true, Action: act("full")},
		{Label: i18n.T("Museum frame"), Action: act("frame")},
		{Label: i18n.T("Smart crop"), Action: act("crop")},
	}
	items := []schema.MenuItemSchema{
		{Kind: schema.MenuItemNowShowing, Label: "The Milkmaid — Johannes Vermeer", Title: "The Milkmaid",
			Detail: "Johannes Vermeer", IconName: "Rijksmuseum", IconBytes: logo, Action: act("web")},
		{Kind: schema.MenuItemIconButton, Label: i18n.T("Previous wallpaper"), Glyph: "back", KeepOpen: true, Action: act("prev")},
		{Kind: schema.MenuItemIconButton, Label: i18n.T("Pause rotation"), Glyph: "pause", KeepOpen: true, Action: act("pause")},
		{Kind: schema.MenuItemIconButton, Label: i18n.T("Next wallpaper"), Glyph: "next", KeepOpen: true, Action: act("next")},
		{IsSeparator: true},
		{Kind: schema.MenuItemIconButton, Label: i18n.T("Remove from favorites"), Glyph: "heart", KeepOpen: true, Action: act("fav")},
		{IsSeparator: true},
		{Label: i18n.T("Tune image"), Glyph: "crop", Action: act("tune")},
		{Label: i18n.T("Don't show this image again"), Glyph: "block", Action: act("block")},
		{Label: i18n.T("Mark as NSFW"), Glyph: "eyeOff", Action: act("nsfw-mark")},
		{IsSeparator: true},
		{Label: i18n.T("Display mode"), Detail: i18n.T("Full artwork"), Glyph: "photo",
			SubMenu: &schema.MenuSchema{Label: i18n.T("Display mode"), Items: modes}},
		{Kind: schema.MenuItemSwitch, Label: i18n.T("Show NSFW images"), Action: act("nsfw")},
	}
	footer := []schema.MenuItemSchema{
		{Label: i18n.T("Settings…"), Glyph: "settings", Action: act("settings")},
		{Label: i18n.T("Quit"), Glyph: "quit", Action: act("quit")},
	}
	return items, footer
}

func newSampleTrayView(t *testing.T, used *[]string) (*trayMenuView, fyne.Window) {
	t.Helper()
	items, footer := sampleTrayItems(used)
	v := newTrayMenuView(items, footer, trayMinWidth)
	v.iconFor = func(it schema.MenuItemSchema) fyne.Resource {
		if len(it.IconBytes) > 0 {
			return fyne.NewStaticResource(it.IconName, it.IconBytes)
		}
		return nil
	}
	w := test.NewWindow(v)
	w.SetPadded(false)
	w.Resize(v.MinSize())
	t.Cleanup(w.Close)
	return v, w
}

func regionNamed(t *testing.T, v *trayMenuView, label string) trayRegion {
	t.Helper()
	i := v.regionByLabel(label)
	require.GreaterOrEqual(t, i, 0, "no region %q", label)
	return v.regions[i]
}

func near(t *testing.T, want, got float32, what string) {
	t.Helper()
	assert.InDelta(t, want, got, 0.6, what)
}

// The menu's parts sit where the prototype puts them (T-S3 sizes, measured in
// the browser): a 216 × 273 menu with 28 px rows (v45: Tune image is a row).
func TestTrayMenuLayoutMatchesPrototype(t *testing.T) {
	i18n.SetLanguage("zh")
	defer i18n.SetLanguage("en")
	v, _ := newSampleTrayView(t, nil)

	size := v.MinSize()
	near(t, 216, size.Width, "menu width (the minimum)")
	near(t, 273, size.Height, "menu height")

	np := regionNamed(t, v, "The Milkmaid")
	near(t, 5, np.pos.X, "artwork block x")
	near(t, 5, np.pos.Y, "artwork block y")
	near(t, 206, np.size.Width, "artwork block width")
	near(t, 40, np.size.Height, "artwork block height")

	wantX := []float32{9, 58.25, 107.5, 179}
	for i, label := range []string{"Previous wallpaper", "Pause rotation", "Next wallpaper", "Remove from favorites"} {
		r := regionNamed(t, v, i18n.T(label))
		near(t, wantX[i], r.pos.X, label+" x")
		near(t, 47, r.pos.Y, label+" y")
		near(t, 28, r.size.Width, label+" width")
	}

	rows := map[string]float32{
		"Tune image": 86, "Don't show this image again": 114, "Mark as NSFW": 142, "Display mode": 177, "Show NSFW images": 205,
	}
	for label, y := range rows {
		r := regionNamed(t, v, i18n.T(label))
		near(t, y, r.pos.Y, label+" y")
		near(t, 28, r.size.Height, label+" height")
	}

	settings := regionNamed(t, v, "设置")
	quit := regionNamed(t, v, i18n.T("Quit"))
	near(t, 240, settings.pos.Y, "footer y")
	near(t, 7, settings.pos.X, "Settings x")
	near(t, 209, quit.pos.X+quit.size.Width, "Quit ends 2 px inside the row")
}

// T11: the switch, the › arrow, the last icon and Quit end on one vertical line.
func TestTrayMenuRightEdgesLineUp(t *testing.T) {
	for _, lang := range []string{"zh", "en"} {
		i18n.SetLanguage(lang)
		v, w := newSampleTrayView(t, nil)
		line := v.MinSize().Width - 15

		var arrow, track, quitText, lastIcon float32
		for _, o := range test.LaidOutObjects(w.Canvas().Content()) {
			switch obj := o.(type) {
			case *fynecanvas.Image:
				if obj.Resource == nil {
					continue
				}
				switch {
				case strings.Contains(obj.Resource.Name(), "chevronRight"):
					// The arrow's path ends at 16 of 24.
					arrow = obj.Position().X + obj.Size().Width*16/24
				case strings.Contains(obj.Resource.Name(), "heart") && obj.Size().Width == trayBtnIconSize:
					// The heart path ends at 22 of 24.
					lastIcon = obj.Position().X + obj.Size().Width*22/24
				}
			case *fynecanvas.Rectangle:
				if obj.Size().Width == traySwitchW {
					track = obj.Position().X + obj.Size().Width
				}
			case *fynecanvas.Text:
				if obj.Text == i18n.T("Quit") {
					quitText = obj.Position().X + fyne.MeasureText(obj.Text, obj.TextSize, obj.TextStyle).Width
				}
			}
		}
		for name, x := range map[string]float32{"arrow": arrow, "switch": track, "Quit": quitText, "last icon": lastIcon} {
			assert.InDelta(t, line, x, 1.5, "%s: %s ends at %.1f, the line is at %.1f", lang, name, x, line)
		}
	}
	i18n.SetLanguage("en")
}

func TestTrayMenuLongTitleIsCut(t *testing.T) {
	items, footer := sampleTrayItems(nil)
	items[0].Title = "A View of the Grand Canal with the Church of Santa Maria della Salute at Dusk"
	v := newTrayMenuView(items, footer, trayMinWidth)
	w := test.NewWindow(v)
	defer w.Close()
	w.Resize(v.MinSize())
	assert.Equal(t, trayMaxWidth, v.MinSize().Width, "a long title widens the menu up to 256")
	found := false
	for _, o := range test.LaidOutObjects(w.Canvas().Content()) {
		if txt, ok := o.(*fynecanvas.Text); ok && strings.HasPrefix(txt.Text, "A View of the Grand Canal") {
			found = true
			assert.True(t, strings.HasSuffix(txt.Text, "…"), txt.Text)
			assert.LessOrEqual(t, txt.Position().X+fyne.MeasureText(txt.Text, txt.TextSize, txt.TextStyle).Width, trayMaxWidth-trayInset-trayRowPadX+0.5)
		}
	}
	assert.True(t, found)
}

func TestTrayMenuStructure(t *testing.T) {
	v, _ := newSampleTrayView(t, nil)
	assert.Equal(t, []string{
		"np:The Milkmaid|Johannes Vermeer",
		"btn:Previous wallpaper", "btn:Pause rotation", "btn:Next wallpaper", "|", "btn:Remove from favorites",
		"---",
		"row:Tune image", "row:Don't show this image again", "row:Mark as NSFW",
		"---",
		"row:Display mode|Full artwork ›", "switch:Show NSFW images",
		"---",
		"foot:Settings", "foot:Quit",
	}, v.labels())
}

// Playback and favourite keep the menu open, so the artwork block updates in
// place; other items close it. The switch flips at once.
func TestTrayMenuUse(t *testing.T) {
	var used []string
	v, w := newSampleTrayView(t, &used)
	var kept []bool
	v.onUse = func(it schema.MenuItemSchema, keepOpen bool) {
		kept = append(kept, keepOpen)
		it.Action()
	}
	for _, label := range []string{"Next wallpaper", "Tune image", "Show NSFW images", "Quit"} {
		r := regionNamed(t, v, label)
		test.TapCanvas(w.Canvas(), r.pos.Add(fyne.NewPos(r.size.Width/2, r.size.Height/2)))
	}
	assert.Equal(t, []string{"next", "tune", "nsfw", "quit"}, used)
	assert.Equal(t, []bool{true, false, true, false}, kept)
	assert.Contains(t, v.labels(), "switch:Show NSFW images [on]", "the switch shows its new state before the menu is rebuilt")
}

// The keyboard moves line by line, across a line with Left and Right; Left
// opens a submenu (as Right does), Enter uses the item.
func TestTrayMenuKeyboard(t *testing.T) {
	var used []string
	v, _ := newSampleTrayView(t, &used)
	v.onUse = func(it schema.MenuItemSchema, _ bool) { it.Action() }
	var opened []string
	v.onSubmenu = func(it *schema.MenuItemSchema, top float32, byKey bool) {
		if it != nil {
			opened = append(opened, it.Label)
			assert.True(t, byKey)
			near(t, 177, top, "submenu row top")
		}
	}
	active := func() string {
		if v.active < 0 {
			return ""
		}
		return v.regions[v.active].item.Label
	}
	v.typedKey(fyne.KeyDown)
	assert.Equal(t, "The Milkmaid — Johannes Vermeer", active())
	v.typedKey(fyne.KeyDown)
	assert.Equal(t, "Previous wallpaper", active())
	assert.Equal(t, v.active, v.tip, "an icon button reached by keyboard shows its name")
	v.typedKey(fyne.KeyRight)
	v.typedKey(fyne.KeyRight)
	assert.Equal(t, "Next wallpaper", active())
	v.typedKey(fyne.KeyDown)
	assert.Equal(t, "Tune image", active())
	v.typedKey(fyne.KeyDown)
	assert.Equal(t, "Don't show this image again", active())
	v.typedKey(fyne.KeyDown)
	v.typedKey(fyne.KeyDown)
	assert.Equal(t, "Display mode", active())
	v.typedKey(fyne.KeyLeft)
	assert.Equal(t, []string{"Display mode"}, opened)
	v.typedKey(fyne.KeyRight)
	assert.Equal(t, []string{"Display mode", "Display mode"}, opened, "the arrow points right, so Right opens it too")
	v.typedKey(fyne.KeyDown)
	v.typedKey(fyne.KeyDown)
	assert.Equal(t, "Settings…", active())
	v.typedKey(fyne.KeyDown)
	assert.Equal(t, "The Milkmaid — Johannes Vermeer", active(), "Down wraps to the top")
	v.typedKey(fyne.KeyUp)
	assert.Equal(t, "Settings…", active(), "Up wraps to the footer's first item")
	v.typedKey(fyne.KeyReturn)
	assert.Equal(t, []string{"settings"}, used)
}

func TestTrayMenuHover(t *testing.T) {
	v, _ := newSampleTrayView(t, nil)
	r := regionNamed(t, v, "Mark as NSFW")
	v.MouseMoved(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: r.pos.Add(fyne.NewPos(20, 10))}})
	assert.Equal(t, "Mark as NSFW", v.regions[v.active].item.Label)
	v.MouseOut()
	assert.Equal(t, -1, v.active)
}

// T-L (option 2): the tray is not locked while Tune Image is open; an item
// that changes the wallpaper closes Tune Image first, then runs.
func TestTrayClosesTuneImageFirst(t *testing.T) {
	test.NewTempApp(t)
	var events []string
	open := func(sa *SpiceApp) {
		w := test.NewWindow(nil)
		w.SetOnClosed(func() { events = append(events, "close tune") })
		sa.tuneWindow = w
	}
	act := func(name string) func() { return func() { events = append(events, name) } }
	sa := &SpiceApp{}
	items := sa.closeTuneBefore([]schema.MenuItemSchema{
		{Label: "Next", Action: act("next"), ChangesWallpaper: true},
		{Label: "Pause", Action: act("pause")},
		{Label: "Display mode", SubMenu: &schema.MenuSchema{Items: []schema.MenuItemSchema{
			{Label: "Museum frame", Action: act("frame"), ChangesWallpaper: true},
		}}},
	})

	open(sa)
	items[1].Action()
	assert.Equal(t, []string{"pause"}, events, "pause leaves Tune Image open")
	require.NotNil(t, sa.tuneWindow)

	items[0].Action()
	assert.Equal(t, []string{"pause", "close tune", "next"}, events, "next closes Tune Image first")
	assert.Nil(t, sa.tuneWindow)

	items[0].Action()
	assert.Equal(t, []string{"pause", "close tune", "next", "next"}, events, "nothing to close")

	events = nil
	open(sa)
	items[2].SubMenu.Items[0].Action()
	assert.Equal(t, []string{"close tune", "frame"}, events, "submenu items too")
}

func TestSubMenuWidth(t *testing.T) {
	items, _ := sampleTrayItems(nil)
	assert.Equal(t, traySubMinMode, subMenuWidth(items[11].SubMenu.Items), "a list of choices is 150 wide")
	assert.Equal(t, trayMinWidth, subMenuWidth(items[:10]), "a display's submenu is as wide as the menu")
	v := newTrayMenuView(items[11].SubMenu.Items, nil, traySubMinMode)
	near(t, 150, v.MinSize().Width, "mode submenu width")
	near(t, 94, v.MinSize().Height, "mode submenu height")
	assert.Equal(t, "row:Full artwork [x]", v.labels()[0], "the current choice has a check")
}

func TestTrayGlyphsParse(t *testing.T) {
	for name := range trayGlyphPaths {
		res := trayGlyph(name, rgb(0x2B2A27))
		require.NotNil(t, res, name)
		img := fynecanvas.NewImageFromResource(res)
		w := test.NewWindow(img)
		w.Resize(fyne.NewSize(24, 24))
		pix := w.Canvas().Capture()
		w.Close()
		dark := 0
		b := pix.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				r, _, _, _ := pix.At(x, y).RGBA()
				if r < 0x8000 {
					dark++
				}
			}
		}
		assert.Greater(t, dark, 0, "%s draws something", name)
	}
	assert.Nil(t, trayGlyph("nope", rgb(0)))
}

// Resting on a row with a submenu opens it; resting on an icon button shows its name.
func TestTrayMenuHoverOpensSubmenu(t *testing.T) {
	v, _ := newSampleTrayView(t, nil)
	opened := make(chan string, 1)
	v.onSubmenu = func(it *schema.MenuItemSchema, _ float32, byKey bool) {
		if it != nil {
			assert.False(t, byKey)
			opened <- it.Label
		}
	}
	r := regionNamed(t, v, "Display mode")
	v.MouseMoved(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: r.pos.Add(fyne.NewPos(30, 10))}})
	// The hover timers hand their work to the UI thread; here that is this goroutine (flushUI).
	var got string
	require.Eventually(t, func() bool {
		flushUI()
		select {
		case got = <-opened:
			return true
		default:
			return false
		}
	}, 2*time.Second, 20*time.Millisecond, "the submenu did not open")
	assert.Equal(t, "Display mode", got)

	b := regionNamed(t, v, "Next wallpaper")
	v.MouseMoved(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: b.pos.Add(fyne.NewPos(10, 10))}})
	assert.Eventually(t, func() bool {
		flushUI()
		return v.tip == v.active && v.tip >= 0
	}, 2*time.Second, 20*time.Millisecond)
}

// The hover timers never touch the menu from their own goroutine: the tip waits for the UI
// thread (here the test goroutine) and the menu changes only then.
func TestTrayMenuHoverTipRunsOnUIThread(t *testing.T) {
	v, _ := newSampleTrayView(t, nil)
	b := regionNamed(t, v, "Next wallpaper")
	v.MouseMoved(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: b.pos.Add(fyne.NewPos(10, 10))}})
	time.Sleep(700 * time.Millisecond) // past the 500 ms delay
	v.mu.Lock()
	tip := v.tip
	v.mu.Unlock()
	assert.Equal(t, -1, tip, "the timer only queued the tip")
	flushUI()
	assert.Equal(t, v.active, v.tip)
	assert.GreaterOrEqual(t, v.tip, 0)
}

// T14 (prototype v36): a newer version shows as its own group just above the Settings / Quit
// footer, in bold with the download icon, as "Update to v1.0.1" in the UI language. Without
// one there is no item.
func TestTrayUpdateItem(t *testing.T) {
	assert.Empty(t, (&SpiceApp{}).updateMenuItems())

	inLanguages(t, func(t *testing.T, lang string) {
		release, _ := url.Parse("https://example.com/releases/v1.0.1")
		sa := &SpiceApp{update: &trayUpdate{version: "v1.0.1", url: release}}
		want := map[string]string{"en": "Update to v1.0.1", "zh": "更新到 v1.0.1"}[lang]

		items, footer := sampleTrayItems(nil)
		items = append(items, sa.updateMenuItems()...)
		v := newTrayMenuView(items, footer, trayMinWidth)
		w := test.NewWindow(v)
		defer w.Close()
		w.Resize(v.MinSize())

		labels := v.labels()
		n := len(labels)
		require.GreaterOrEqual(t, n, 5)
		assert.Equal(t, []string{"---", "row:" + want, "---"}, labels[n-5:n-2], "its own group above the footer")
		assert.True(t, strings.HasPrefix(labels[n-2], "foot:"))
		assert.Equal(t, "download", items[len(items)-1].Glyph)

		bold := false
		for _, o := range test.LaidOutObjects(w.Canvas().Content()) {
			if txt, ok := o.(*fynecanvas.Text); ok && txt.Text == want {
				bold = txt.TextStyle.Bold
			}
		}
		assert.True(t, bold, "the label is bold")
	})
}

// T15 (prototype v44): "Next lock screen picture" is a row at the top of the group above Display
// mode, with a lock icon. Using it keeps the menu open (the desktop does not change); the label is
// translated (zh 锁屏换一张, zh-Hant 鎖定畫面換一張).
func TestTrayMenuNextLockScreenRow(t *testing.T) {
	for lang, want := range map[string]string{"en": "Next lock screen picture", "zh": "锁屏换一张", "zh-Hant": "鎖定畫面換一張"} {
		i18n.SetLanguage(lang)
		assert.Equal(t, want, i18n.T("Next lock screen picture"), lang)
	}
	i18n.SetLanguage("en")

	var used []string
	items, footer := sampleTrayItems(&used)
	items = withLockScreenRow(items, func() { used = append(used, "lock") })
	v := newTrayMenuView(items, footer, trayMinWidth)
	w := test.NewWindow(v)
	w.SetPadded(false)
	w.Resize(v.MinSize())
	t.Cleanup(w.Close)

	labels := v.labels()
	assert.Equal(t, []string{"---", "row:Next lock screen picture", "row:Display mode|Full artwork ›"}, labels[10:13])
	require.NotNil(t, trayGlyph("lock", rgb(0x2B2A27)))

	var kept []bool
	v.onUse = func(it schema.MenuItemSchema, keepOpen bool) {
		kept = append(kept, keepOpen)
		it.Action()
	}
	r := regionNamed(t, v, "Next lock screen picture")
	test.TapCanvas(w.Canvas(), r.pos.Add(fyne.NewPos(r.size.Width/2, r.size.Height/2)))
	assert.Equal(t, []string{"lock"}, used)
	assert.Equal(t, []bool{true}, kept)
}

// withLockScreenRow adds T15 where the wallpaper plugin puts it on Windows: first in the group
// above Display mode.
func withLockScreenRow(items []schema.MenuItemSchema, action func()) []schema.MenuItemSchema {
	at := len(items) - 2 // before Display mode and Show NSFW images
	lock := schema.MenuItemSchema{Label: i18n.T("Next lock screen picture"), Glyph: "lock", KeepOpen: true, Action: action}
	out := append([]schema.MenuItemSchema{}, items[:at]...)
	out = append(out, lock)
	return append(out, items[at:]...)
}

// The whole first row opens the artwork's page: the logo, the title line and the artist line
// (2026-11-12). It closes the menu (keepOpen false); SpiceApp.OpenURL lets the browser come to
// the front although the menu is gone by then.
func TestTrayMenuFirstRowOpensThePageFromEitherLine(t *testing.T) {
	var used []string
	v, w := newSampleTrayView(t, &used)
	var kept []bool
	v.onUse = func(it schema.MenuItemSchema, keepOpen bool) {
		kept = append(kept, keepOpen)
		it.Action()
	}
	r := regionNamed(t, v, "The Milkmaid — Johannes Vermeer")
	require.Equal(t, trayRegionNowShowing, r.kind)
	for _, at := range []fyne.Position{
		r.pos.Add(fyne.NewPos(trayInset+trayLogoSize/2, r.size.Height/2)),  // logo
		r.pos.Add(fyne.NewPos(r.size.Width/2, trayNPPadTop+trayLineH/2)),   // title
		r.pos.Add(fyne.NewPos(r.size.Width/2, trayNPPadTop+trayLineH*1.5)), // artist
		r.pos.Add(fyne.NewPos(r.size.Width-4, r.size.Height-2)),            // right edge, bottom
	} {
		test.TapCanvas(w.Canvas(), at)
	}
	assert.Equal(t, []string{"web", "web", "web", "web"}, used)
	assert.Equal(t, []bool{false, false, false, false}, kept)
}
