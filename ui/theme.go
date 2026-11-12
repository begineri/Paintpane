package ui

import (
	_ "embed"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
)

// themeMode is the saved theme choice. The stored value is always one of these
// English identifiers; translated labels are only used for display.
type themeMode string

const (
	themeModeSystem themeMode = "System"
	themeModeLight  themeMode = "Light"
	themeModeDark   themeMode = "Dark"
)

// themeModes is the display order of the theme selector.
var themeModes = []themeMode{themeModeSystem, themeModeLight, themeModeDark}

// parseThemeMode maps a saved value to a theme mode. Older builds saved the
// translated label (for example "深色"); those values never took effect, so they
// are read back as System, which is what those users were seeing.
func parseThemeMode(saved string) themeMode {
	switch themeMode(saved) {
	case themeModeLight:
		return themeModeLight
	case themeModeDark:
		return themeModeDark
	default:
		return themeModeSystem
	}
}

// label returns the translated name. The literal i18n.T calls let gen_i18n find the keys.
func (m themeMode) label() string {
	switch m {
	case themeModeLight:
		return i18n.T("Light")
	case themeModeDark:
		return i18n.T("Dark")
	default:
		return i18n.T("System")
	}
}

func themeModeIndex(m themeMode) int {
	for i, mode := range themeModes {
		if mode == m {
			return i
		}
	}
	return 0
}

// Colour names Paintpane adds on top of Fyne's.
const (
	colorNameSurface      fyne.ThemeColorName = "paintpane.surface"
	colorNameSelectedTint fyne.ThemeColorName = "paintpane.selectedTint"
	colorNameSelectedText fyne.ThemeColorName = "paintpane.selectedText"
	colorNameSwitchOff    fyne.ThemeColorName = "paintpane.switchOff"
)

func rgb(hex uint32) color.Color {
	return color.NRGBA{R: uint8(hex >> 16), G: uint8(hex >> 8), B: uint8(hex), A: 0xFF}
}

func rgba(hex uint32, alpha uint8) color.Color {
	return color.NRGBA{R: uint8(hex >> 16), G: uint8(hex >> 8), B: uint8(hex), A: alpha}
}

var lightPalette = map[fyne.ThemeColorName]color.Color{
	theme.ColorNameBackground:        rgb(0xFFFFFF),
	theme.ColorNameForeground:        rgb(0x2B2A27),
	theme.ColorNamePlaceHolder:       rgb(0x75736C),
	theme.ColorNameButton:            rgb(0xF1F0EC),
	theme.ColorNameDisabledButton:    rgb(0xF4F3F0),
	theme.ColorNameDisabled:          rgb(0xB0AEA6),
	theme.ColorNameInputBackground:   rgb(0xF6F5F2),
	theme.ColorNameInputBorder:       rgb(0xE0DED7),
	theme.ColorNameSeparator:         rgb(0xE6E4DE),
	theme.ColorNameHeaderBackground:  rgb(0xF6F5F2),
	theme.ColorNameMenuBackground:    rgb(0xFFFFFF),
	theme.ColorNameOverlayBackground: rgb(0xFFFFFF),
	theme.ColorNameHover:             rgba(0x000000, 0x0C),
	theme.ColorNamePressed:           rgba(0x000000, 0x18),
	theme.ColorNameError:             rgb(0xC93A33),
	theme.ColorNameSuccess:           rgb(0x2E7D32),
	theme.ColorNameWarning:           rgb(0xB35F00),
	theme.ColorNameScrollBar:         rgba(0x000000, 0x40),
	theme.ColorNameShadow:            rgba(0x000000, 0x22),
	colorNameSurface:                 rgb(0xF6F5F2),
	colorNameSwitchOff:               rgb(0xD9D7D0),
}

var darkPalette = map[fyne.ThemeColorName]color.Color{
	theme.ColorNameBackground:        rgb(0x1F1F1D),
	theme.ColorNameForeground:        rgb(0xECEBE6),
	theme.ColorNamePlaceHolder:       rgb(0xA3A19A),
	theme.ColorNameButton:            rgb(0x2F2F2C),
	theme.ColorNameDisabledButton:    rgb(0x2A2A28),
	theme.ColorNameDisabled:          rgb(0x6A6964),
	theme.ColorNameInputBackground:   rgb(0x2A2A28),
	theme.ColorNameInputBorder:       rgb(0x3F3E3B),
	theme.ColorNameSeparator:         rgb(0x3A3936),
	theme.ColorNameHeaderBackground:  rgb(0x272725),
	theme.ColorNameMenuBackground:    rgb(0x262624),
	theme.ColorNameOverlayBackground: rgb(0x262624),
	theme.ColorNameHover:             rgba(0xFFFFFF, 0x10),
	theme.ColorNamePressed:           rgba(0xFFFFFF, 0x1E),
	theme.ColorNameError:             rgb(0xF0605A),
	theme.ColorNameSuccess:           rgb(0x66BB6A),
	theme.ColorNameWarning:           rgb(0xFFB547),
	theme.ColorNameScrollBar:         rgba(0xFFFFFF, 0x40),
	theme.ColorNameShadow:            rgba(0x000000, 0x66),
	colorNameSurface:                 rgb(0x272725),
	colorNameSwitchOff:               rgb(0x4A4945),
}

var sizeOverrides = map[fyne.ThemeSizeName]float32{
	theme.SizeNameInputRadius:        7,
	theme.SizeNameSelectionRadius:    6,
	theme.SizeNameInputBorder:        1,
	theme.SizeNameSeparatorThickness: 1,
	theme.SizeNameScrollBarRadius:    4,
	theme.SizeNameCaptionText:        12,
	theme.SizeNameText:               13, // G8
}

// paintpaneTheme is the application theme: Fyne's default theme with Paintpane
// colours and sizes. When forced is set, the variant Fyne passes in is ignored.
type paintpaneTheme struct {
	base    fyne.Theme
	mode    themeMode
	forced  bool
	variant fyne.ThemeVariant
	accent  accentDef
}

// newPaintpaneTheme returns the theme for a mode with the default accent; use
// withAccent to choose another.
func newPaintpaneTheme(mode themeMode) *paintpaneTheme {
	t := &paintpaneTheme{base: theme.DefaultTheme(), mode: mode, accent: parseAccent(accentDefault)}
	switch mode {
	case themeModeLight:
		t.forced, t.variant = true, theme.VariantLight
	case themeModeDark:
		t.forced, t.variant = true, theme.VariantDark
	}
	return t
}

// withAccent sets the accent by its saved key and returns the theme.
func (t *paintpaneTheme) withAccent(key string) *paintpaneTheme {
	t.accent = parseAccent(key)
	return t
}

func (t *paintpaneTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if t.forced {
		variant = t.variant
	}
	if c, ok := accentColor(t.accent, name, variant == theme.VariantDark); ok {
		return c
	}
	palette := lightPalette
	if variant == theme.VariantDark {
		palette = darkPalette
	}
	if c, ok := palette[name]; ok {
		return c
	}
	return t.base.Color(name, variant)
}

// One font family with Chinese glyphs, Regular and Bold, for all text (G1):
// a subset of Noto Sans SC built by assets-src/fonts/subset.py (SIL OFL 1.1,
// see NOTICE.md). Chinese and Latin share one design and Chinese gets a real
// bold. Characters outside the subset fall back to Fyne's font, then to a
// system font.
var (
	//go:embed fonts/NotoSansSC-Regular.subset.otf
	notoSansSCRegular []byte
	//go:embed fonts/NotoSansSC-Bold.subset.otf
	notoSansSCBold []byte

	fontRegular = fyne.NewStaticResource("NotoSansSC-Regular.subset.otf", notoSansSCRegular)
	fontBold    = fyne.NewStaticResource("NotoSansSC-Bold.subset.otf", notoSansSCBold)
)

func (t *paintpaneTheme) Font(style fyne.TextStyle) fyne.Resource {
	switch {
	case style.Monospace, style.Symbol, style.Italic:
		return t.base.Font(style) // Noto Sans SC has no monospace or italic
	case style.Bold:
		return fontBold
	default:
		return fontRegular
	}
}

func (t *paintpaneTheme) Icon(name fyne.ThemeIconName) fyne.Resource { return t.base.Icon(name) }

func (t *paintpaneTheme) Size(name fyne.ThemeSizeName) float32 {
	if s, ok := sizeOverrides[name]; ok {
		return s
	}
	return t.base.Size(name)
}

// applyTheme installs the theme for the given mode and accent key on the running app. Installing
// a theme makes Fyne drop its font and text caches and redraw every window, so a call that would
// not change the look does nothing: the settings page applies a choice when it is made and again
// when its pending changes are saved.
func applyTheme(app fyne.App, mode themeMode, accent string) {
	if cur, ok := app.Settings().Theme().(*paintpaneTheme); ok && cur.mode == mode && cur.accent.key == parseAccent(accent).key {
		return
	}
	app.Settings().SetTheme(newPaintpaneTheme(mode).withAccent(accent))
	styleOpenTitleBars()
}

// isDarkTheme reports whether the current theme renders dark, honouring a forced mode.
func isDarkTheme() bool {
	settings := fyne.CurrentApp().Settings()
	if t, ok := settings.Theme().(*paintpaneTheme); ok && t.forced {
		return t.variant == theme.VariantDark
	}
	return settings.ThemeVariant() == theme.VariantDark
}

// paintpaneColor looks up a colour, including Paintpane's own names, even when
// another theme is installed (tests use Fyne's test theme, which does not know them).
func paintpaneColor(name fyne.ThemeColorName) color.Color {
	if _, ok := fyne.CurrentApp().Settings().Theme().(*paintpaneTheme); ok {
		return theme.Color(name)
	}
	palette := lightPalette
	if isDarkTheme() {
		palette = darkPalette
	}
	if c, ok := palette[name]; ok {
		return c
	}
	if c, ok := accentColor(parseAccent(accentDefault), name, isDarkTheme()); ok {
		return c
	}
	return theme.Color(name)
}
