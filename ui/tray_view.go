package ui

import (
	"fmt"
	"image/color"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
)

// The tray menu the app draws itself (docs/ui_refresh.md §12, T-S1 to T-S3,
// T7/T8 option B, T11, T-L). Sizes are the prototype's, measured in
// docs/mockups/app-ui (tray page), in Fyne units.
const (
	trayBorder    float32 = 1
	trayPad       float32 = 4
	trayInset             = trayBorder + trayPad // from the window edge to a row
	trayMinWidth  float32 = 216
	trayMaxWidth  float32 = 256
	trayRadius    float32 = 8
	trayRowH      float32 = 28
	trayRowPadX   float32 = 8
	trayRowGap    float32 = 8
	trayRowRadius float32 = 5
	trayIconSize  float32 = 16
	trayTextSize  float32 = 12
	trayNoteSize  float32 = 11 // current choice, artist, hover names
	trayLineH     float32 = 16
	traySepGap    float32 = 3
	traySepInset  float32 = 6

	// The artwork block.
	trayNPPadTop    float32 = 6
	trayNPPadBottom float32 = 2
	trayNPRadius    float32 = 6
	trayLogoSize    float32 = 24
	trayLogoRadius  float32 = 4
	trayTitleSize   float32 = 12.5

	// The row of icon buttons, spread across the menu.
	trayBtnSize      float32 = 28
	trayBtnIconSize  float32 = 18
	trayBtnPadTop    float32 = 2
	trayBtnPadX      float32 = 4
	trayBtnPadBottom float32 = 4
	trayDividerH     float32 = 18

	// Submenu arrow and switch. Their ends line up with the last icon button
	// and Quit (T11): the arrow box ends 3 px past the row padding, the switch
	// box 2 px.
	trayArrowOverhang  float32 = 3
	trayCurGap         float32 = 8 // margin on each side of the current choice, after the 8 px gap
	traySwitchBoxW     float32 = 36
	traySwitchOverhang float32 = 2
	traySwitchW        float32 = 27
	traySwitchH        float32 = 15
	traySwitchKnob     float32 = 12

	// Footer: Settings on the left, Quit on the right.
	trayFootPadX   float32 = 2
	trayFootBtnPad float32 = 8
	trayFootGap    float32 = 6
	trayFootSpace  float32 = 4

	// Submenus open to the right of their row (to the left when there is no
	// room): 1 px from the menu, the first item level with the row.
	traySubGap     float32 = 1
	traySubMinMode float32 = 150
)

// trayPalette holds the menu's colours: light or dark follows Appearance, the
// accent is always graphite (T-S1).
type trayPalette struct {
	bg, fg, ph, sep, hover, primary, switchOff, knobOn, knobOff color.Color
}

func currentTrayPalette() trayPalette {
	if isDarkTheme() {
		return trayPalette{
			bg: rgb(0x1F1F1D), fg: rgb(0xECEBE6), ph: rgb(0xA3A19A), sep: rgb(0x3A3936),
			hover: rgba(0xFFFFFF, 0x10), primary: rgb(0xE6E4DE), switchOff: rgb(0x4A4945),
			knobOn: rgb(0x1F1F1D), knobOff: rgb(0xFFFFFF),
		}
	}
	return trayPalette{
		bg: rgb(0xFFFFFF), fg: rgb(0x2B2A27), ph: rgb(0x75736C), sep: rgb(0xE6E4DE),
		hover: rgba(0x000000, 0x0C), primary: rgb(0x2F2E2B), switchOff: rgb(0xD9D7D0),
		knobOn: rgb(0xFFFFFF), knobOff: rgb(0xFFFFFF),
	}
}

// Material icon paths (24×24), as the prototype draws them.
var trayGlyphPaths = map[string]string{
	"back":         "M20 11H7.83l5.59-5.59L12 4l-8 8 8 8 1.41-1.41L7.83 13H20v-2z",
	"next":         "M12 4l-1.41 1.41L16.17 11H4v2h12.17l-5.58 5.59L12 20l8-8z",
	"pause":        "M6 19h4V5H6v14zm8-14v14h4V5h-4z",
	"play":         "M8 5v14l11-7z",
	"heart":        "M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z",
	"heartOutline": "M16.5 3c-1.74 0-3.41.81-4.5 2.09C10.91 3.81 9.24 3 7.5 3 4.42 3 2 5.42 2 8.5c0 3.78 3.4 6.86 8.55 11.54L12 21.35l1.45-1.32C18.6 15.36 22 12.28 22 8.5 22 5.42 19.58 3 16.5 3zm-4.4 15.55-.1.1-.1-.1C7.14 14.24 4 11.39 4 8.5 4 6.5 5.5 5 7.5 5c1.54 0 3.04.99 3.57 2.36h1.87C13.46 5.99 14.96 5 16.5 5c2 0 3.5 1.5 3.5 3.5 0 2.89-3.14 5.74-7.9 10.05z",
	"crop":         "M17 15h2V7c0-1.1-.9-2-2-2H9v2h8v8zM7 17V1H5v4H1v2h4v10c0 1.1.9 2 2 2h10v4h2v-4h4v-2H7z",
	"block":        "M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zM4 12c0-4.42 3.58-8 8-8 1.85 0 3.55.63 4.9 1.69L5.69 16.9A7.902 7.902 0 0 1 4 12zm8 8c-1.85 0-3.55-.63-4.9-1.69L18.31 7.1A7.902 7.902 0 0 1 20 12c0 4.42-3.58 8-8 8z",
	"eyeOff":       "M12 7c2.76 0 5 2.24 5 5 0 .65-.13 1.26-.36 1.83l2.92 2.92c1.51-1.26 2.7-2.89 3.43-4.75-1.73-4.39-6-7.5-11-7.5-1.4 0-2.74.25-3.98.7l2.16 2.16C10.74 7.13 11.35 7 12 7zM2 4.27l2.28 2.28.46.46A11.8 11.8 0 0 0 1 12c1.73 4.39 6 7.5 11 7.5 1.55 0 3.03-.3 4.38-.84l.42.42L19.73 22 21 20.73 3.27 3 2 4.27zM7.53 9.8l1.55 1.55c-.05.21-.08.43-.08.65 0 1.66 1.34 3 3 3 .22 0 .44-.03.65-.08l1.55 1.55c-.67.33-1.41.53-2.2.53-2.76 0-5-2.24-5-5 0-.79.2-1.53.53-2.2zm4.31-.78 3.15 3.15.02-.16c0-1.66-1.34-3-3-3l-.17.01z",
	"computer":     "M20 18c1.1 0 1.99-.9 1.99-2L22 6c0-1.1-.9-2-2-2H4c-1.1 0-2 .9-2 2v10c0 1.1.9 2 2 2H0v2h24v-2h-4zM4 6h16v10H4V6z",
	"photo":        "M9 2 7.17 4H4c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V6c0-1.1-.9-2-2-2h-3.17L15 2H9zm3 15c-2.76 0-5-2.24-5-5s2.24-5 5-5 5 2.24 5 5-2.24 5-5 5zm0-8.2a3.2 3.2 0 1 0 0 6.4 3.2 3.2 0 0 0 0-6.4z",
	"settings":     "M19.14 12.94c.04-.3.06-.61.06-.94 0-.32-.02-.64-.07-.94l2.03-1.58a.49.49 0 0 0 .12-.61l-1.92-3.32a.49.49 0 0 0-.59-.22l-2.39.96c-.5-.38-1.03-.7-1.62-.94l-.36-2.54a.48.48 0 0 0-.48-.41h-3.84c-.24 0-.43.17-.47.41l-.36 2.54c-.59.24-1.13.57-1.62.94l-2.39-.96a.48.48 0 0 0-.59.22L2.74 8.87c-.12.21-.08.47.12.61l2.03 1.58c-.05.3-.09.63-.09.94s.02.64.07.94l-2.03 1.58a.49.49 0 0 0-.12.61l1.92 3.32c.12.22.37.29.59.22l2.39-.96c.5.38 1.03.7 1.62.94l.36 2.54c.05.24.24.41.48.41h3.84c.24 0 .44-.17.47-.41l.36-2.54c.59-.24 1.13-.56 1.62-.94l2.39.96c.22.08.47 0 .59-.22l1.92-3.32a.49.49 0 0 0-.12-.61l-2.01-1.58zM12 15.6A3.6 3.6 0 1 1 12 8.4a3.6 3.6 0 0 1 0 7.2z",
	"quit":         "M10.09 15.59 11.5 17l5-5-5-5-1.41 1.41L12.67 11H3v2h9.67l-2.58 2.59zM19 3H5a2 2 0 0 0-2 2v4h2V5h14v14H5v-4H3v4a2 2 0 0 0 2 2h14c1.1 0 2-.9 2-2V5c0-1.1-.9-2-2-2z",
	"check":        "M9 16.17 4.83 12l-1.42 1.41L9 19 21 7l-1.41-1.41z",
	"chevronRight": "M10 6 8.59 7.41 13.17 12l-4.58 4.59L10 18l6-6z",
	"download":     "M19 9h-4V3H9v6H5l7 7 7-7zM5 18v2h14v-2H5z",
	"lock":         "M18 8h-1V6c0-2.76-2.24-5-5-5S7 3.24 7 6v2H6c-1.1 0-2 .9-2 2v10c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V10c0-1.1-.9-2-2-2zm-6 9c-1.1 0-2-.9-2-2s.9-2 2-2 2 .9 2 2-.9 2-2 2zm3.1-9H8.9V6c0-1.71 1.39-3.1 3.1-3.1 1.71 0 3.1 1.39 3.1 3.1v2z",
}

var (
	trayGlyphMu    sync.Mutex
	trayGlyphCache = map[string]fyne.Resource{}
)

// trayGlyph returns the named icon filled with c, or nil for an unknown name.
func trayGlyph(name string, c color.Color) fyne.Resource {
	path, ok := trayGlyphPaths[name]
	if !ok {
		return nil
	}
	r, g, b, a := c.RGBA()
	hex := fmt.Sprintf("#%02X%02X%02X", r>>8, g>>8, b>>8)
	key := name + hex
	trayGlyphMu.Lock()
	defer trayGlyphMu.Unlock()
	if res, ok := trayGlyphCache[key]; ok {
		return res
	}
	svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="%s" fill-opacity="%.3f" d="%s"/></svg>`,
		hex, float64(a)/0xFFFF, path)
	res := fyne.NewStaticResource("tray-"+name+".svg", []byte(svg))
	trayGlyphCache[key] = res
	return res
}

// trayRegionKind is what a part of the menu does when used.
type trayRegionKind int

const (
	trayRegionNowShowing trayRegionKind = iota
	trayRegionButton                    // icon button in the icon row
	trayRegionRow                       // text row (with or without submenu)
	trayRegionSwitch                    // row with a switch
	trayRegionFooter                    // Settings / Quit
)

// trayRegion is one usable part of the menu, with its place.
type trayRegion struct {
	kind trayRegionKind
	item schema.MenuItemSchema
	pos  fyne.Position
	size fyne.Size
	line int // regions on one line share it; arrow keys move across lines
}

func (r trayRegion) contains(p fyne.Position) bool {
	return p.X >= r.pos.X && p.X < r.pos.X+r.size.Width && p.Y >= r.pos.Y && p.Y < r.pos.Y+r.size.Height
}

// trayBlock is one vertical piece of the menu.
type trayBlock struct {
	kind  string // "np", "buttons", "sep", "row", "switch", "footer"
	items []schema.MenuItemSchema
}

// trayMenuView draws a tray menu (or one of its submenus) and handles the mouse
// and keyboard. The window around it belongs to the platform code.
type trayMenuView struct {
	widget.BaseWidget

	items    []schema.MenuItemSchema
	footer   []schema.MenuItemSchema
	minWidth float32
	// drawBorder draws the 1 px outline; off when the system draws it (Windows 11).
	drawBorder bool
	pal        trayPalette
	iconFor    func(schema.MenuItemSchema) fyne.Resource

	// onUse runs an item. keepOpen is set for items that change something
	// shown in the menu itself (playback, favourite, switch).
	onUse func(it schema.MenuItemSchema, keepOpen bool)
	// onSubmenu opens (item non-nil) or closes (nil) the submenu of a row;
	// top is the row's top in this view, byKey whether the keyboard opened it.
	onSubmenu func(it *schema.MenuItemSchema, top float32, byKey bool)

	mu       sync.Mutex
	regions  []trayRegion
	blocks   []trayBlock
	size     fyne.Size
	active   int // highlighted region, -1 for none
	openSub  int // region whose submenu is open, -1 for none
	tip      int // icon button whose name is shown, -1 for none
	mouse    fyne.Position
	hasMouse bool
	switched map[string]bool // switches flipped before the menu is rebuilt
	hoverGen int
}

func newTrayMenuView(items, footer []schema.MenuItemSchema, minWidth float32) *trayMenuView {
	v := &trayMenuView{minWidth: minWidth, drawBorder: true, active: -1, openSub: -1, tip: -1}
	v.ExtendBaseWidget(v)
	v.setItems(items, footer)
	return v
}

// setItems replaces the menu's content; the highlight follows the mouse.
func (v *trayMenuView) setItems(items, footer []schema.MenuItemSchema) {
	v.mu.Lock()
	v.items, v.footer = items, footer
	v.switched = nil
	v.pal = currentTrayPalette()
	v.layoutLocked()
	v.active, v.tip = -1, -1
	if v.openSub >= 0 {
		v.openSub = -1
	}
	if v.hasMouse {
		v.active = v.regionAtLocked(v.mouse)
	}
	v.mu.Unlock()
	v.Refresh()
}

// refreshPalette re-reads light or dark.
func (v *trayMenuView) refreshPalette() {
	v.mu.Lock()
	v.pal = currentTrayPalette()
	v.mu.Unlock()
	v.Refresh()
}

func (v *trayMenuView) blocksFor() []trayBlock {
	var blocks []trayBlock
	add := func(kind string, it schema.MenuItemSchema) {
		if kind == "buttons" && len(blocks) > 0 && blocks[len(blocks)-1].kind == "buttons" {
			blocks[len(blocks)-1].items = append(blocks[len(blocks)-1].items, it)
			return
		}
		blocks = append(blocks, trayBlock{kind: kind, items: []schema.MenuItemSchema{it}})
	}
	for i, it := range v.items {
		switch {
		case it.IsSeparator:
			// A separator between two icon buttons is the row's divider.
			if len(blocks) > 0 && blocks[len(blocks)-1].kind == "buttons" &&
				i+1 < len(v.items) && v.items[i+1].Kind == schema.MenuItemIconButton {
				add("buttons", it)
				continue
			}
			if len(blocks) == 0 || blocks[len(blocks)-1].kind == "sep" {
				continue
			}
			add("sep", it)
		case it.Kind == schema.MenuItemNowShowing:
			add("np", it)
		case it.Kind == schema.MenuItemIconButton:
			add("buttons", it)
		case it.Kind == schema.MenuItemSwitch:
			add("switch", it)
		default:
			add("row", it)
		}
	}
	if len(v.footer) > 0 {
		if len(blocks) > 0 && blocks[len(blocks)-1].kind != "sep" {
			blocks = append(blocks, trayBlock{kind: "sep"})
		}
		blocks = append(blocks, trayBlock{kind: "footer", items: v.footer})
	}
	for len(blocks) > 0 && blocks[len(blocks)-1].kind == "sep" {
		blocks = blocks[:len(blocks)-1]
	}
	return blocks
}

func trayTextW(s string, size float32, bold bool) float32 {
	if s == "" {
		return 0
	}
	return fyne.MeasureText(s, size, fyne.TextStyle{Bold: bold}).Width
}

// footerLabel is the footer text: "Settings…" loses its ellipsis there.
func footerLabel(it schema.MenuItemSchema) string {
	return strings.TrimSuffix(it.Label, "…")
}

func (v *trayMenuView) rowRightW(it schema.MenuItemSchema, kind string) float32 {
	switch {
	case kind == "switch":
		return trayRowGap + traySwitchBoxW - traySwitchOverhang
	case it.SubMenu != nil:
		w := trayRowGap + trayCurGap + trayIconSize - trayArrowOverhang
		if it.Detail != "" {
			w += trayRowGap + trayCurGap + trayTextW(it.Detail, trayNoteSize, false)
		}
		return w
	}
	return 0
}

// layoutLocked places every region and works out the menu's size.
func (v *trayMenuView) layoutLocked() {
	v.blocks = v.blocksFor()
	// Width: the widest content, between the menu's minimum and maximum.
	need := v.minWidth - 2*trayInset
	for _, b := range v.blocks {
		var w float32
		switch b.kind {
		case "np":
			it := b.items[0]
			w = trayRowPadX + trayLogoSize + trayRowGap +
				fyne.Max(trayTextW(it.Title, trayTitleSize, true), trayTextW(it.Detail, trayNoteSize, false)) + trayRowPadX
		case "row", "switch":
			it := b.items[0]
			w = trayRowPadX + trayIconSize + trayRowGap + trayTextW(it.Label, trayTextSize, it.Emphasis) + v.rowRightW(it, b.kind) + trayRowPadX
		case "buttons":
			for _, it := range b.items {
				if it.IsSeparator {
					w += 1 + 2*trayFootSpace
				} else {
					w += trayBtnSize
				}
			}
			w += 2 * trayBtnPadX
		case "footer":
			w = 2 * trayFootPadX
			for i, it := range b.items {
				if i > 0 {
					w += trayFootSpace
				}
				w += 2*trayFootBtnPad + trayIconSize + trayFootGap + trayTextW(footerLabel(it), trayTextSize, false)
			}
		}
		need = fyne.Max(need, w)
	}
	width := fyne.Min(fyne.Max(need+2*trayInset, v.minWidth), fyne.Max(trayMaxWidth, v.minWidth))
	inner := width - 2*trayInset

	v.regions = v.regions[:0]
	y := trayInset
	line := 0
	for _, b := range v.blocks {
		switch b.kind {
		case "np":
			it := b.items[0]
			lines := float32(1)
			if it.Detail != "" {
				lines = 2
			}
			h := trayNPPadTop + fyne.Max(trayLogoSize, lines*trayLineH) + trayNPPadBottom
			v.regions = append(v.regions, trayRegion{kind: trayRegionNowShowing, item: it,
				pos: fyne.NewPos(trayInset, y), size: fyne.NewSize(inner, h), line: line})
			y += h
		case "buttons":
			h := trayBtnPadTop + trayBtnSize + trayBtnPadBottom
			var fixed float32
			n := len(b.items)
			for _, it := range b.items {
				if it.IsSeparator {
					fixed++
				} else {
					fixed += trayBtnSize
				}
			}
			gap := float32(0)
			if n > 1 {
				gap = (inner - 2*trayBtnPadX - fixed) / float32(n-1)
			}
			x := trayInset + trayBtnPadX
			for _, it := range b.items {
				if it.IsSeparator {
					v.regions = append(v.regions, trayRegion{kind: -1, item: it,
						pos: fyne.NewPos(x, y+trayBtnPadTop+(trayBtnSize-trayDividerH)/2), size: fyne.NewSize(1, trayDividerH), line: line})
					x += 1 + gap
					continue
				}
				v.regions = append(v.regions, trayRegion{kind: trayRegionButton, item: it,
					pos: fyne.NewPos(x, y+trayBtnPadTop), size: fyne.NewSize(trayBtnSize, trayBtnSize), line: line})
				x += trayBtnSize + gap
			}
			y += h
		case "sep":
			v.regions = append(v.regions, trayRegion{kind: -1, pos: fyne.NewPos(trayInset+traySepInset, y+traySepGap),
				size: fyne.NewSize(inner-2*traySepInset, 1), line: -1})
			y += 2*traySepGap + 1
			continue
		case "row", "switch":
			kind := trayRegionRow
			if b.kind == "switch" {
				kind = trayRegionSwitch
			}
			v.regions = append(v.regions, trayRegion{kind: kind, item: b.items[0],
				pos: fyne.NewPos(trayInset, y), size: fyne.NewSize(inner, trayRowH), line: line})
			y += trayRowH
		case "footer":
			x := trayInset + trayFootPadX
			right := trayInset + inner - trayFootPadX
			for i, it := range b.items {
				w := 2*trayFootBtnPad + trayIconSize + trayFootGap + trayTextW(footerLabel(it), trayTextSize, false)
				px := x
				if i == len(b.items)-1 && i > 0 {
					px = right - w // the last one (Quit) sits on the right
				}
				v.regions = append(v.regions, trayRegion{kind: trayRegionFooter, item: it,
					pos: fyne.NewPos(px, y), size: fyne.NewSize(w, trayRowH), line: line})
				x += w + trayFootSpace
			}
			y += trayRowH
		}
		line++
	}
	v.size = fyne.NewSize(width, y+trayInset)
}

func (v *trayMenuView) regionAtLocked(p fyne.Position) int {
	for i, r := range v.regions {
		if r.kind >= 0 && r.contains(p) {
			return i
		}
	}
	return -1
}

// usable reports whether a region reacts to the mouse and keyboard.
func (r trayRegion) usable() bool {
	if r.kind < 0 {
		return false
	}
	return r.item.Action != nil || r.item.SubMenu != nil
}

// --- mouse ---

func (v *trayMenuView) MouseIn(e *desktop.MouseEvent) { v.MouseMoved(e) }

func (v *trayMenuView) MouseMoved(e *desktop.MouseEvent) {
	v.mu.Lock()
	entered := !v.hasMouse
	v.mouse, v.hasMouse = e.Position, true
	i := v.regionAtLocked(e.Position)
	if i >= 0 && !v.regions[i].usable() {
		i = -1
	}
	// A row the keyboard highlighted still opens its submenu when the mouse
	// comes onto it.
	changed := i != v.active || entered
	if changed {
		v.active = i
		v.tip = -1
		v.hoverGen++
	}
	gen := v.hoverGen
	var r trayRegion
	if i >= 0 {
		r = v.regions[i]
	}
	open := v.openSub
	v.mu.Unlock()
	if !changed {
		return
	}
	v.Refresh()
	// Hovering a row with a submenu opens it after a short pause; hovering
	// another row closes an open one. Hovering an icon button shows its name.
	switch {
	case i >= 0 && r.kind == trayRegionButton:
		v.after(500*time.Millisecond, gen, func() { v.showTip(i) })
	case i >= 0 && r.item.SubMenu != nil && i != open:
		v.after(250*time.Millisecond, gen, func() { v.openSubmenu(i, false) })
	case i >= 0 && open >= 0 && i != open:
		v.after(250*time.Millisecond, gen, func() { v.closeSubmenu() })
	}
}

func (v *trayMenuView) MouseOut() {
	v.mu.Lock()
	v.hasMouse = false
	// Leave a row with an open submenu highlighted, like Windows does.
	if v.active != v.openSub {
		v.active = v.openSub
	}
	v.tip = -1
	v.hoverGen++
	v.mu.Unlock()
	v.Refresh()
}

// after runs fn on the UI thread after d, unless the mouse has moved on since. The timer fires on
// its own goroutine; showing a tip or a submenu redraws the menu, which must happen on the UI
// thread (onUIThread; tests queue it for the test goroutine).
func (v *trayMenuView) after(d time.Duration, gen int, fn func()) {
	time.AfterFunc(d, func() {
		onUIThread(func() {
			v.mu.Lock()
			same := v.hoverGen == gen
			v.mu.Unlock()
			if same {
				fn()
			}
		})
	})
}

func (v *trayMenuView) showTip(i int) {
	v.mu.Lock()
	v.tip = i
	v.mu.Unlock()
	v.Refresh()
}

func (v *trayMenuView) Tapped(e *fyne.PointEvent) {
	v.mu.Lock()
	i := v.regionAtLocked(e.Position)
	v.mu.Unlock()
	if i >= 0 {
		v.use(i, false)
	}
}

// use runs region i: opens its submenu, flips its switch or runs its action.
func (v *trayMenuView) use(i int, byKey bool) {
	v.mu.Lock()
	if i < 0 || i >= len(v.regions) || !v.regions[i].usable() {
		v.mu.Unlock()
		return
	}
	r := v.regions[i]
	v.mu.Unlock()
	switch {
	case r.item.SubMenu != nil:
		v.openSubmenu(i, byKey)
		return
	case r.kind == trayRegionSwitch:
		v.mu.Lock()
		if v.switched == nil {
			v.switched = map[string]bool{}
		}
		v.switched[r.item.Label] = !v.switched[r.item.Label]
		v.mu.Unlock()
		v.Refresh()
		if v.onUse != nil {
			v.onUse(r.item, true)
		}
		return
	}
	if v.onUse != nil {
		v.onUse(r.item, r.item.KeepOpen)
	}
}

func (v *trayMenuView) openSubmenu(i int, byKey bool) {
	v.mu.Lock()
	if i < 0 || i >= len(v.regions) || v.regions[i].item.SubMenu == nil {
		v.mu.Unlock()
		return
	}
	v.openSub, v.active = i, i
	r := v.regions[i]
	v.mu.Unlock()
	v.Refresh()
	if v.onSubmenu != nil {
		it := r.item
		v.onSubmenu(&it, r.pos.Y, byKey)
	}
}

func (v *trayMenuView) closeSubmenu() {
	v.mu.Lock()
	was := v.openSub
	v.openSub = -1
	v.mu.Unlock()
	if was < 0 {
		return
	}
	v.Refresh()
	if v.onSubmenu != nil {
		v.onSubmenu(nil, 0, false)
	}
}

// --- keyboard ---

// typedKey moves the highlight or uses the highlighted item; it reports
// whether it handled the key. The submenu opens to the right, or to the left
// when there is no room: either Left or Right opens it (the caller closes a
// submenu on Left, Right or Escape).
func (v *trayMenuView) typedKey(name fyne.KeyName) bool {
	v.mu.Lock()
	cur := v.active
	var usable []int
	for i, r := range v.regions {
		if r.usable() {
			usable = append(usable, i)
		}
	}
	v.mu.Unlock()
	if len(usable) == 0 {
		return false
	}
	pos := -1
	for k, i := range usable {
		if i == cur {
			pos = k
		}
	}
	set := func(i int) {
		v.mu.Lock()
		v.active = i
		v.tip = -1
		if i >= 0 && v.regions[i].kind == trayRegionButton {
			v.tip = i
		}
		v.hoverGen++
		v.mu.Unlock()
		v.Refresh()
	}
	lineOf := func(i int) int { return v.regions[i].line }
	switch name {
	case fyne.KeyDown, fyne.KeyTab:
		if pos < 0 {
			set(usable[0])
			return true
		}
		for k := 1; k <= len(usable); k++ {
			n := usable[(pos+k)%len(usable)]
			if lineOf(n) != lineOf(cur) {
				// Enter a line at its first item.
				for _, m := range usable {
					if lineOf(m) == lineOf(n) {
						set(m)
						return true
					}
				}
			}
		}
		return true
	case fyne.KeyUp:
		if pos < 0 {
			set(usable[len(usable)-1])
			return true
		}
		for k := 1; k <= len(usable); k++ {
			n := usable[(pos-k+2*len(usable))%len(usable)]
			if lineOf(n) != lineOf(cur) {
				for _, m := range usable {
					if lineOf(m) == lineOf(n) {
						set(m)
						return true
					}
				}
			}
		}
		return true
	case fyne.KeyLeft, fyne.KeyRight:
		if pos < 0 {
			return false
		}
		step := 1
		if name == fyne.KeyLeft {
			step = -1
		}
		n := pos + step
		if n >= 0 && n < len(usable) && lineOf(usable[n]) == lineOf(cur) {
			set(usable[n])
			return true
		}
		if v.regions[cur].item.SubMenu != nil {
			v.openSubmenu(cur, true)
			return true
		}
		return false
	case fyne.KeyReturn, fyne.KeyEnter, fyne.KeySpace:
		if pos >= 0 {
			v.use(cur, true)
			return true
		}
	}
	return false
}

// highlightFirst highlights the first usable item (a submenu opened by key).
func (v *trayMenuView) highlightFirst() {
	v.mu.Lock()
	for i, r := range v.regions {
		if r.usable() {
			v.active = i
			break
		}
	}
	v.mu.Unlock()
	v.Refresh()
}

// --- drawing ---

func (v *trayMenuView) MinSize() fyne.Size {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.size
}

func (v *trayMenuView) CreateRenderer() fyne.WidgetRenderer {
	r := &trayMenuRenderer{v: v}
	r.build()
	return r
}

type trayMenuRenderer struct {
	v       *trayMenuView
	objects []fyne.CanvasObject
}

func (r *trayMenuRenderer) Layout(fyne.Size)             {}
func (r *trayMenuRenderer) MinSize() fyne.Size           { return r.v.MinSize() }
func (r *trayMenuRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *trayMenuRenderer) Destroy()                     {}
func (r *trayMenuRenderer) Refresh()                     { r.build(); canvas.Refresh(r.v) }
func place(o fyne.CanvasObject, x, y, w, h float32) fyne.CanvasObject {
	o.Move(fyne.NewPos(x, y))
	o.Resize(fyne.NewSize(w, h))
	return o
}

// fitText cuts s with "…" so it fits in w.
func fitText(s string, size float32, bold bool, w float32) string {
	if s == "" || trayTextW(s, size, bold) <= w {
		return s
	}
	runes := []rune(s)
	lo, hi := 0, len(runes)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if trayTextW(strings.TrimRight(string(runes[:mid]), " ")+"…", size, bold) <= w {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return strings.TrimRight(string(runes[:lo]), " ") + "…"
}

// text places a line of text vertically centred on cy.
func (r *trayMenuRenderer) text(s string, size float32, bold bool, c color.Color, x, cy, w float32, align fyne.TextAlign) fyne.CanvasObject {
	s = fitText(s, size, bold, w)
	t := canvas.NewText(s, c)
	t.TextSize = size
	t.TextStyle = fyne.TextStyle{Bold: bold}
	t.Alignment = align
	h := t.MinSize().Height
	return place(t, x, cy-h/2, w, h)
}

func (r *trayMenuRenderer) glyph(name string, c color.Color, x, y, size float32) fyne.CanvasObject {
	res := trayGlyph(name, c)
	if res == nil {
		return nil
	}
	img := canvas.NewImageFromResource(res)
	img.FillMode = canvas.ImageFillContain
	return place(img, x, y, size, size)
}

func (r *trayMenuRenderer) build() {
	v := r.v
	v.mu.Lock()
	defer v.mu.Unlock()
	p := v.pal
	size := v.size
	objs := []fyne.CanvasObject{}
	add := func(o fyne.CanvasObject) {
		if o != nil {
			objs = append(objs, o)
		}
	}

	bg := canvas.NewRectangle(p.bg)
	bg.CornerRadius = trayRadius
	if v.drawBorder {
		bg.StrokeColor = p.sep
		bg.StrokeWidth = trayBorder
	}
	add(place(bg, 0, 0, size.Width, size.Height))

	right := size.Width - trayInset // inner right edge of the rows
	for i, rg := range v.regions {
		x, y, w, h := rg.pos.X, rg.pos.Y, rg.size.Width, rg.size.Height
		if rg.kind < 0 {
			sep := canvas.NewRectangle(p.sep)
			add(place(sep, x, y, w, h))
			continue
		}
		if i == v.active || i == v.openSub {
			hl := canvas.NewRectangle(p.hover)
			hl.CornerRadius = trayRowRadius
			if rg.kind == trayRegionNowShowing {
				hl.CornerRadius = trayNPRadius
			}
			add(place(hl, x, y, w, h))
		}
		it := rg.item
		switch rg.kind {
		case trayRegionNowShowing:
			lx := x + trayRowPadX
			ly := y + trayNPPadTop
			if v.iconFor != nil {
				if res := v.iconFor(it); res != nil {
					img := canvas.NewImageFromResource(res)
					img.FillMode = canvas.ImageFillContain
					img.CornerRadius = trayLogoRadius
					add(place(img, lx, ly, trayLogoSize, trayLogoSize))
				}
			}
			tx := lx + trayLogoSize + trayRowGap
			tw := right - trayRowPadX - tx
			add(r.text(it.Title, trayTitleSize, true, p.fg, tx, ly+trayLineH/2, tw, fyne.TextAlignLeading))
			if it.Detail != "" {
				add(r.text(it.Detail, trayNoteSize, false, p.ph, tx, ly+trayLineH*1.5, tw, fyne.TextAlignLeading))
			}
		case trayRegionButton:
			add(r.glyph(it.Glyph, p.fg, x+(w-trayBtnIconSize)/2, y+(h-trayBtnIconSize)/2, trayBtnIconSize))
		case trayRegionRow, trayRegionSwitch:
			cy := y + h/2
			ix := x + trayRowPadX
			iconColor := p.ph
			if it.Emphasis {
				iconColor = p.primary
			}
			glyph := it.Glyph
			if it.IsChecked && rg.kind == trayRegionRow {
				glyph, iconColor = "check", p.primary
			}
			if glyph != "" {
				add(r.glyph(glyph, iconColor, ix, cy-trayIconSize/2, trayIconSize))
			}
			tx := ix + trayIconSize + trayRowGap
			end := right - trayRowPadX
			if rg.kind == trayRegionSwitch {
				boxR := right - trayRowPadX + traySwitchOverhang
				boxL := boxR - traySwitchBoxW
				on := it.IsChecked != v.switched[it.Label]
				sx := boxL + (traySwitchBoxW-traySwitchW)/2
				track := canvas.NewRectangle(p.switchOff)
				knobC := p.knobOff
				if on {
					track.FillColor, knobC = p.primary, p.knobOn
				}
				track.CornerRadius = traySwitchH / 2
				add(place(track, sx, cy-traySwitchH/2, traySwitchW, traySwitchH))
				knob := canvas.NewCircle(knobC)
				inset := (traySwitchH - traySwitchKnob) / 2
				kx := sx + inset
				if on {
					kx = sx + traySwitchW - inset - traySwitchKnob
				}
				add(place(knob, kx, cy-traySwitchKnob/2, traySwitchKnob, traySwitchKnob))
				end = boxL - trayRowGap
			} else if it.SubMenu != nil {
				ax := right - trayRowPadX + trayArrowOverhang - trayIconSize
				add(r.glyph("chevronRight", p.ph, ax, cy-trayIconSize/2, trayIconSize))
				end = ax - trayRowGap - trayCurGap
				if it.Detail != "" {
					// The label is cut before the choice, unless the choice
					// would take more than half the room.
					cw := trayTextW(it.Detail, trayNoteSize, false)
					cx := end - cw
					if mid := tx + (end-tx)/2; cx < mid {
						cx, cw = mid, end-mid
					}
					add(r.text(it.Detail, trayNoteSize, false, p.ph, cx, cy, cw, fyne.TextAlignLeading))
					end = cx - trayRowGap - trayCurGap
				}
			}
			add(r.text(it.Label, trayTextSize, it.Emphasis, p.fg, tx, cy, end-tx, fyne.TextAlignLeading))
		case trayRegionFooter:
			ix := x + trayFootBtnPad
			add(r.glyph(it.Glyph, p.ph, ix, y+(h-trayIconSize)/2, trayIconSize))
			tx := ix + trayIconSize + trayFootGap
			add(r.text(footerLabel(it), trayTextSize, false, p.fg, tx, y+h/2, x+w-trayFootBtnPad-tx, fyne.TextAlignLeading))
		}
	}

	// The name of the hovered icon button, below it.
	if v.tip >= 0 && v.tip < len(v.regions) {
		rg := v.regions[v.tip]
		label := rg.item.Label
		tw := trayTextW(label, trayNoteSize, false) + 16
		th := float32(22)
		tx := rg.pos.X + rg.size.Width/2 - tw/2
		tx = fyne.Max(trayInset, fyne.Min(tx, size.Width-trayInset-tw))
		ty := rg.pos.Y + rg.size.Height + 4
		box := canvas.NewRectangle(p.bg)
		box.CornerRadius = 4
		box.StrokeColor = p.sep
		box.StrokeWidth = 1
		add(place(box, tx, ty, tw, th))
		add(r.text(label, trayNoteSize, false, p.fg, tx, ty+th/2, tw, fyne.TextAlignCenter))
	}
	r.objects = objs
}

// trayMenuLabels lists the menu's visible items, for tests and the debug API.
func (v *trayMenuView) labels() []string {
	v.mu.Lock()
	defer v.mu.Unlock()
	var out []string
	for _, r := range v.regions {
		switch r.kind {
		case trayRegionNowShowing:
			out = append(out, "np:"+r.item.Title+"|"+r.item.Detail)
		case trayRegionButton:
			out = append(out, "btn:"+r.item.Label)
		case trayRegionRow:
			s := "row:" + r.item.Label
			if r.item.Detail != "" {
				s += "|" + r.item.Detail
			}
			if r.item.SubMenu != nil {
				s += " ›"
			}
			if r.item.IsChecked {
				s += " [x]"
			}
			out = append(out, s)
		case trayRegionSwitch:
			s := "switch:" + r.item.Label
			if r.item.IsChecked != v.switched[r.item.Label] {
				s += " [on]"
			}
			out = append(out, s)
		case trayRegionFooter:
			out = append(out, "foot:"+footerLabel(r.item))
		default:
			if r.line < 0 {
				out = append(out, "---")
			} else {
				out = append(out, "|")
			}
		}
	}
	return out
}

// regionByLabel finds a usable region by its label (debug API, tests).
func (v *trayMenuView) regionByLabel(label string) int {
	v.mu.Lock()
	defer v.mu.Unlock()
	for i, r := range v.regions {
		if r.kind >= 0 && (r.item.Label == label || footerLabel(r.item) == label || r.item.Title == label) {
			return i
		}
	}
	return -1
}
