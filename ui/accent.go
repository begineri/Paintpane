package ui

import (
	"image/color"
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
)

// Accent colour setting (docs/ui_refresh.md §12, C2). The accent changes the
// switches, selected nav item and segment, links, primary buttons, focus and
// selection, and the Tune Image anchor highlight. Backgrounds and text stay.
// Graphite is the default, also for people upgrading from a build whose accent
// was orange.

// accentColors are the colours one accent supplies for one theme variant.
type accentColors struct {
	primary   uint32 // switches, primary buttons, selected tab
	onPrimary uint32 // text and check marks on the primary colour
	link      uint32 // hyperlinks
	selText   uint32 // text of the selected nav item and segment
	selTint   uint32 // background of the selected nav item and segment
	highlight uint32 // Tune Image: the chosen part of the picture
}

type accentDef struct {
	key         string
	light, dark accentColors
}

const accentDefault = "graphite"

// accents is the order the swatches are shown in.
var accents = []accentDef{
	{"graphite",
		accentColors{0x2F2E2B, 0xFFFFFF, 0x2F2E2B, 0x2B2A27, 0xE2E0DA, 0xFFFFFF},
		accentColors{0xE6E4DE, 0x1F1F1D, 0xECEBE6, 0xFFFFFF, 0x3D3C38, 0xFFFFFF}},
	{"dusty",
		accentColors{0x4D6C8C, 0xFFFFFF, 0x3F5E7D, 0x3A5774, 0xEAF0F5, 0xA8C2DB},
		accentColors{0x86A6C6, 0x0F1C29, 0xA8C2DB, 0xB0C8DF, 0x22303D, 0xA8C2DB}},
	{"violet",
		accentColors{0x6A56C2, 0xFFFFFF, 0x5946AD, 0x5341A5, 0xF0EDFA, 0xB9A9FF},
		accentColors{0x9D8CF0, 0x1A1240, 0xBCB0F8, 0xC3B8FA, 0x2D2748, 0xB9A9FF}},
	{"sage",
		accentColors{0x4C705D, 0xFFFFFF, 0x41624F, 0x3D5D4A, 0xEAF2ED, 0xA9CDB7},
		accentColors{0x8DB89E, 0x10241A, 0xA9CDB7, 0xB0D2BC, 0x22322A, 0xA9CDB7}},
	{"celadon",
		accentColors{0x2A7774, 0xFFFFFF, 0x22625F, 0x1F5D5A, 0xE3F1F0, 0x8ED3CD},
		accentColors{0x5FB8B2, 0x082624, 0x8ED3CD, 0x96D7D1, 0x1A3432, 0x8ED3CD}},
	{"brass",
		accentColors{0x86681F, 0xFFFFFF, 0x765A15, 0x70550F, 0xF6F0E1, 0xE2C37E},
		accentColors{0xD4AE5A, 0x2A1E05, 0xE2C37E, 0xE6C985, 0x3A3120, 0xE2C37E}},
	{"blue",
		accentColors{0x4B5FD6, 0xFFFFFF, 0x3A4BB8, 0x3546B5, 0xECEEFC, 0x9AA8FF},
		accentColors{0x7C8CF0, 0x10143A, 0xA9B4F8, 0xB3BDFA, 0x272B48, 0x9AA8FF}},
	{"green",
		accentColors{0x2A7F58, 0xFFFFFF, 0x22704F, 0x1F6A4A, 0xE4F2EA, 0x8EDDB6},
		accentColors{0x55BF8E, 0x0A2A1C, 0x86D8B0, 0x8EDDB6, 0x1C3428, 0x8EDDB6}},
	{"red",
		accentColors{0xC4523A, 0xFFFFFF, 0xA6412B, 0x9E3D29, 0xFBEBE6, 0xF5AE9C},
		accentColors{0xE27D66, 0x2E0F08, 0xF2A592, 0xF5AE9C, 0x3D231C, 0xF5AE9C}},
	{"orange",
		accentColors{0xFF9800, 0x2B1A00, 0xB35F00, 0xB35F00, 0xFFF1DC, 0xFAC775},
		accentColors{0xFF9800, 0x2B1A00, 0xFFB547, 0xFFB547, 0x3A2C14, 0xFAC775}},
}

// accentLabel returns the translated name. The literal i18n.T calls let gen_i18n find the keys.
func accentLabel(key string) string {
	switch key {
	case "dusty":
		return i18n.T("Dusty blue")
	case "violet":
		return i18n.T("Violet")
	case "sage":
		return i18n.T("Sage")
	case "celadon":
		return i18n.T("Celadon")
	case "brass":
		return i18n.T("Brass")
	case "blue":
		return i18n.T("Bedroom blue")
	case "green":
		return i18n.T("Window green")
	case "red":
		return i18n.T("Ochre red")
	case "orange":
		return i18n.T("Classic orange")
	default:
		return i18n.T("Graphite")
	}
}

// parseAccent maps a saved key to an accent; an unknown or empty key is graphite.
func parseAccent(key string) accentDef {
	for _, a := range accents {
		if a.key == key {
			return a
		}
	}
	return accents[0]
}

func accentIndex(key string) int {
	for i, a := range accents {
		if a.key == key {
			return i
		}
	}
	return 0
}

func (a accentDef) colors(dark bool) accentColors {
	if dark {
		return a.dark
	}
	return a.light
}

// Names of the colours the accent changes.
const (
	colorNameHighlight  fyne.ThemeColorName = "paintpane.highlight"  // Tune Image: chosen part of the picture
	colorNameSwitchKnob fyne.ThemeColorName = "paintpane.switchKnob" // knob of a switch
)

// accentColor returns the colour an accent supplies for a theme colour name.
func accentColor(a accentDef, name fyne.ThemeColorName, dark bool) (color.Color, bool) {
	c := a.colors(dark)
	switch name {
	case theme.ColorNamePrimary:
		return rgb(c.primary), true
	case theme.ColorNameForegroundOnPrimary:
		return rgb(c.onPrimary), true
	case theme.ColorNameHyperlink:
		return rgb(c.link), true
	case theme.ColorNameFocus:
		if dark {
			return rgba(c.primary, 0x80), true
		}
		return rgba(c.primary, 0x66), true
	case theme.ColorNameSelection:
		if dark {
			return rgba(c.primary, 0x50), true
		}
		return rgba(c.primary, 0x40), true
	case colorNameSelectedTint:
		return rgb(c.selTint), true
	case colorNameSelectedText:
		return rgb(c.selText), true
	case colorNameHighlight:
		return rgb(c.highlight), true
	case colorNameSwitchKnob:
		// A white knob on the pale graphite track of the dark theme would vanish.
		if dark && a.key == "graphite" {
			return rgb(0x1F1F1D), true
		}
		return rgb(0xFFFFFF), true
	}
	return nil, false
}

// graphiteAccent reports whether the current theme uses the graphite accent.
func graphiteAccent() bool {
	t, ok := fyne.CurrentApp().Settings().Theme().(*paintpaneTheme)
	return ok && t.accent.key == accentDefault
}

// accentLink is a link that is underlined when the accent is graphite: its
// colour is then the text colour, so the underline is what marks it as a link.
// It looks at the accent whenever it is drawn or refreshed, and installing a
// theme refreshes every widget in every window, so switching to or from
// graphite restyles the links in place without rebuilding the page.
type accentLink struct {
	widget.Hyperlink
}

func newHyperlink(text string, u *url.URL) *accentLink {
	l := &accentLink{}
	l.Text = text
	l.URL = u
	l.ExtendBaseWidget(l)
	l.syncUnderline()
	return l
}

func (l *accentLink) syncUnderline() {
	l.TextStyle.Underline = graphiteAccent()
}

// CreateRenderer styles the link for the accent shown when it is first drawn.
func (l *accentLink) CreateRenderer() fyne.WidgetRenderer {
	l.syncUnderline()
	return l.Hyperlink.CreateRenderer()
}

// Refresh restyles the link for the current accent, then redraws it.
func (l *accentLink) Refresh() {
	l.syncUnderline()
	l.Hyperlink.Refresh()
}

// swatchColors returns the swatch fill of every accent for a theme variant.
func swatchColors(dark bool) []color.Color {
	out := make([]color.Color, len(accents))
	for i, a := range accents {
		out[i] = rgb(a.colors(dark).primary)
	}
	return out
}

// ---------------------------------------------------------------------------
// Swatch row
// ---------------------------------------------------------------------------

const (
	swatchSize float32 = 20 // G8
	swatchGap  float32 = 6
)

// swatchPicker is a row of round colour swatches; the selected one has a ring
// and a check mark (S3-3A). names are the swatches' accessible names.
type swatchPicker struct {
	widget.DisableableWidget
	names     []string
	fills     func() []color.Color // read on every refresh, so the swatches follow the theme
	checks    func() []color.Color // colour of the check on each swatch
	Selected  int
	OnChanged func(int)
}

var (
	_ fyne.Tappable      = (*swatchPicker)(nil)
	_ desktop.Cursorable = (*swatchPicker)(nil)
)

func newSwatchPicker(names []string, selected int, onChanged func(int)) *swatchPicker {
	s := &swatchPicker{names: names, Selected: selected, OnChanged: onChanged}
	s.fills = func() []color.Color { return swatchColors(isDarkTheme()) }
	s.checks = func() []color.Color {
		dark := isDarkTheme()
		out := make([]color.Color, len(accents))
		for i, a := range accents {
			out[i] = rgb(a.colors(dark).onPrimary)
		}
		return out
	}
	s.ExtendBaseWidget(s)
	return s
}

// SetSelectedIndex selects a swatch and fires OnChanged when it changes.
func (s *swatchPicker) SetSelectedIndex(i int) {
	if i < 0 || i >= len(s.names) || i == s.Selected {
		return
	}
	s.Selected = i
	if s.OnChanged != nil {
		s.OnChanged(i)
	}
	s.Refresh()
}

func (s *swatchPicker) cellAt(p fyne.Position) int {
	// Taps in a gap go to the nearer swatch.
	i := int((p.X + swatchGap/2) / (swatchSize + swatchGap))
	if i < 0 || i >= len(s.names) {
		return -1
	}
	return i
}

func (s *swatchPicker) Tapped(ev *fyne.PointEvent) {
	if s.Disabled() {
		return
	}
	if i := s.cellAt(ev.Position); i >= 0 {
		s.SetSelectedIndex(i)
	}
}

func (s *swatchPicker) Cursor() desktop.Cursor { return desktop.PointerCursor }

func (s *swatchPicker) CreateRenderer() fyne.WidgetRenderer {
	r := &swatchRenderer{sp: s}
	for range s.names {
		dot := canvas.NewCircle(color.Transparent)
		dot.StrokeWidth = 1
		ring := canvas.NewCircle(color.Transparent)
		ring.StrokeWidth = 2
		r.dots = append(r.dots, dot)
		r.rings = append(r.rings, ring)
		r.ticks = append(r.ticks, [2]*canvas.Line{canvas.NewLine(color.Transparent), canvas.NewLine(color.Transparent)})
	}
	r.Refresh()
	return r
}

type swatchRenderer struct {
	sp    *swatchPicker
	dots  []*canvas.Circle
	rings []*canvas.Circle
	ticks [][2]*canvas.Line // the two strokes of each check mark
}

func (r *swatchRenderer) MinSize() fyne.Size {
	n := float32(len(r.dots))
	return fyne.NewSize(n*swatchSize+(n-1)*swatchGap, swatchSize+6)
}

func (r *swatchRenderer) Layout(size fyne.Size) {
	y := (size.Height - swatchSize) / 2
	for i := range r.dots {
		x := float32(i) * (swatchSize + swatchGap)
		r.dots[i].Move(fyne.NewPos(x, y))
		r.dots[i].Resize(fyne.NewSize(swatchSize, swatchSize))
		r.rings[i].Move(fyne.NewPos(x-3, y-3))
		r.rings[i].Resize(fyne.NewSize(swatchSize+6, swatchSize+6))
		// A check mark: a short stroke down to the bottom point, a long one up from it.
		t := r.ticks[i]
		t[0].Position1, t[0].Position2 = fyne.NewPos(x+5.5, y+10.5), fyne.NewPos(x+8.5, y+13.5)
		t[1].Position1, t[1].Position2 = fyne.NewPos(x+8.5, y+13.5), fyne.NewPos(x+14.5, y+6.5)
	}
}

func (r *swatchRenderer) Refresh() {
	fills := r.sp.fills()
	onColors := r.sp.checks()
	for i := range r.dots {
		if i < len(fills) {
			r.dots[i].FillColor = fills[i]
		}
		r.dots[i].StrokeColor = rgba(0x000000, 0x1F)
		r.rings[i].StrokeColor = color.Transparent
		for _, l := range r.ticks[i] {
			l.StrokeWidth = 2
			l.StrokeColor = color.Transparent
		}
		if i == r.sp.Selected {
			r.rings[i].StrokeColor = paintpaneColor(theme.ColorNameForeground)
			if i < len(onColors) {
				for _, l := range r.ticks[i] {
					l.StrokeColor = onColors[i]
				}
			}
		}
		r.dots[i].Refresh()
		r.rings[i].Refresh()
		for _, l := range r.ticks[i] {
			l.Refresh()
		}
	}
	r.Layout(r.sp.Size())
}

func (r *swatchRenderer) Objects() []fyne.CanvasObject {
	var objs []fyne.CanvasObject
	for i := range r.dots {
		objs = append(objs, r.rings[i], r.dots[i], r.ticks[i][0], r.ticks[i][1])
	}
	return objs
}

func (r *swatchRenderer) Destroy() {}
