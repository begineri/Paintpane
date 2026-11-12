package ui

import (
	"image/color"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
)

// The Shortcuts page, opened from General › View all shortcuts (S3-12, S3-S). It lists the keys
// in two groups, drawn as key caps: all displays, and one display. A group is greyed out when its
// switch is off.

// shortcutRow is one line of the page: what it does and the keys that do it.
type shortcutRow struct {
	label string
	keys  []string
}

// shortcutModifiers are the names of the modifier keys on this system: the pair that acts on all
// displays, and the one that acts on one display.
func shortcutModifiers() (all []string, one string) {
	if runtime.GOOS == "darwin" {
		return []string{"Cmd", "Ctrl"}, "Cmd"
	}
	return []string{"Ctrl", "Alt"}, "Alt"
}

func joinKeys(mods []string, key string) []string {
	keys := make([]string, 0, 2*len(mods)+1)
	for _, m := range mods {
		keys = append(keys, m, "+")
	}
	return append(keys, key)
}

// allDisplaysShortcuts are the keys that act on every display.
func allDisplaysShortcuts() []shortcutRow {
	mods, _ := shortcutModifiers()
	return []shortcutRow{
		{i18n.T("Next wallpaper"), joinKeys(mods, "→")},
		{i18n.T("Previous wallpaper"), joinKeys(mods, "←")},
		{i18n.T("Pause / resume rotation"), joinKeys(mods, "P")},
		{i18n.T("Shuffle"), joinKeys(mods, "R")},
		{i18n.T("View artwork on the web (main display)"), joinKeys(mods, "I")},
		{i18n.T("Detect displays again"), joinKeys(mods, "D")},
		{i18n.T("Open settings"), joinKeys(mods, "O")},
	}
}

// oneDisplayShortcuts are the keys that act on one display; the display number is held with the
// modifier, so only the last key is listed.
func oneDisplayShortcuts() []shortcutRow {
	return []shortcutRow{
		{i18n.T("Next wallpaper"), []string{"→"}},
		{i18n.T("Previous wallpaper"), []string{"←"}},
		{i18n.T("Add to favorites"), []string{"↑"}},
		{i18n.T("Don't show this image again"), []string{"↓"}},
		{i18n.T("Pause / resume rotation"), []string{"P"}},
		{i18n.T("Shuffle"), []string{"R"}},
		{i18n.T("View artwork on the web"), []string{"I"}},
		{i18n.T("Keep area: up, down, left, right (Smart crop)"), []string{"W", "S", "A", "D"}},
		{i18n.T("Keep area: center, auto (Smart crop)"), []string{"E", "Q"}},
	}
}

// newShortcutsPage builds the page. back returns to General.
func (sa *SpiceApp) newShortcutsPage(sm *SettingsManager, back func()) fyne.CanvasObject {
	wp := wallpaper.GetInstance()
	globalOn := !wp.GetShortcutsDisabled()
	if v, ok := sm.GetValue("enableShortcuts").(bool); ok {
		globalOn = v
	}
	oneOn := !wp.GetTargetedShortcutsDisabled()
	if v, ok := sm.GetValue("enableTargetedShortcuts").(bool); ok {
		oneOn = v
	}
	oneOn = oneOn && globalOn

	_, oneMod := shortcutModifiers()
	note := i18n.Tf("Hold {{.Key}} and the display number (1–9), then press the key below. For example, {{.Key}} + 2 + → changes the wallpaper on display 2.",
		map[string]string{"Key": oneMod})
	if !oneOn {
		note += " " + i18n.T("Off while Single-display shortcuts is off.")
	}

	all := dimmed(newCard(shortcutRows(allDisplaysShortcuts())...), !globalOn)
	one := dimmed(newCard(shortcutRows(oneDisplayShortcuts())...), !oneOn)
	body := pageColumn(
		vstack(labelToCard, noInset(newSectionLabel(i18n.T("All displays"))), all),
		vstack(labelToCard, noInset(newSectionLabel(i18n.T("One display"))), noInset(newSecondaryText(note)), one),
	)

	backBtn := newBackLink(i18n.T("General"), back)
	head := vstack(0,
		inset(hug(backBtn), backLinkTop, 0, 0, contentMarginX-backLinkPadX),
		inset(newPageTitle(i18n.T("Shortcuts")), 2, contentMarginX, pageTitleBottom, contentMarginX))
	return withHeader(head, container.NewVScroll(body))
}

func shortcutRows(list []shortcutRow) []fyne.CanvasObject {
	rows := make([]fyne.CanvasObject, 0, len(list))
	for _, r := range list {
		rows = append(rows, newSettingRow(newRowLabel(r.label), nil, keyCaps(r.keys), false))
	}
	return rows
}

// dimmed greys out obj when off, by laying the page background over it; nothing in it is
// interactive.
func dimmed(obj fyne.CanvasObject, off bool) fyne.CanvasObject {
	if !off {
		return obj
	}
	veil := canvas.NewRectangle(color.NRGBA{})
	veil.FillColor = withAlpha(theme.Color(theme.ColorNameBackground), 0xB0)
	return container.NewStack(obj, veil)
}

// keyCaps draws keys as small rounded caps; a "+" between two keys is plain text.
func keyCaps(keys []string) fyne.CanvasObject {
	items := make([]fyne.CanvasObject, 0, len(keys))
	for _, k := range keys {
		if k == "+" {
			items = append(items, container.NewCenter(newStyledText("+", 12, false, theme.ColorNamePlaceHolder)))
			continue
		}
		items = append(items, container.NewCenter(newKeyCap(k)))
	}
	return container.New(&hstackLayout{gap: 4}, items...)
}

// keyCap is one key: a rounded, bordered cap with a thicker bottom edge.
type keyCap struct {
	widget.BaseWidget
	text string
}

func newKeyCap(text string) *keyCap {
	k := &keyCap{text: text}
	k.ExtendBaseWidget(k)
	return k
}

const (
	keyCapHeight float32 = 22
	keyCapMinW   float32 = 22
	keyCapPadX   float32 = 6
)

func (k *keyCap) CreateRenderer() fyne.WidgetRenderer {
	edge := canvas.NewRectangle(color.Transparent)
	edge.CornerRadius = 5
	cap := canvas.NewRectangle(color.Transparent)
	cap.CornerRadius = 5
	cap.StrokeWidth = 1
	label := newStyledText(k.text, 12, false, theme.ColorNameForeground)
	r := &keyCapRenderer{k: k, edge: edge, cap: cap, label: label}
	r.Refresh()
	return r
}

type keyCapRenderer struct {
	k     *keyCap
	edge  *canvas.Rectangle
	cap   *canvas.Rectangle
	label *styledText
}

func (r *keyCapRenderer) MinSize() fyne.Size {
	return fyne.NewSize(fyne.Max(keyCapMinW, r.label.MinSize().Width+2*keyCapPadX), keyCapHeight)
}

func (r *keyCapRenderer) Layout(size fyne.Size) {
	// The edge shows 1 px below the cap, as the thicker bottom border of a key.
	r.edge.Move(fyne.NewPos(0, 1))
	r.edge.Resize(size.Subtract(fyne.NewSize(0, 1)))
	r.cap.Move(fyne.NewPos(0, 0))
	r.cap.Resize(size.Subtract(fyne.NewSize(0, 1)))
	m := r.label.MinSize()
	r.label.Move(fyne.NewPos((size.Width-m.Width)/2, (size.Height-1-m.Height)/2))
	r.label.Resize(m)
}

func (r *keyCapRenderer) Refresh() {
	r.edge.FillColor = theme.Color(theme.ColorNameSeparator)
	r.cap.FillColor = paintpaneColor(colorNameSurface)
	r.cap.StrokeColor = theme.Color(theme.ColorNameSeparator)
	r.label.SetText(r.k.text)
	r.Layout(r.k.Size())
	canvas.Refresh(r.k)
}

func (r *keyCapRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.edge, r.cap, r.label}
}

func (r *keyCapRenderer) Destroy() {}
