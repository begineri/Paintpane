package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAccentDefaultsToGraphite(t *testing.T) {
	assert.Equal(t, "graphite", parseAccent("").key)
	assert.Equal(t, "graphite", parseAccent("no-such-colour").key)
	assert.Equal(t, "sage", parseAccent("sage").key)
	assert.Equal(t, 10, len(accents), "all ten colours stay")
}

func TestAccentColoursFollowTheTheme(t *testing.T) {
	th := newPaintpaneTheme(themeModeLight).withAccent("celadon")
	assert.Equal(t, rgb(0x2A7774), th.Color(theme.ColorNamePrimary, theme.VariantLight))
	assert.Equal(t, rgb(0x2A7774), th.Color(theme.ColorNamePrimary, theme.VariantDark),
		"a forced light theme ignores the variant Fyne passes")
	system := newPaintpaneTheme(themeModeSystem).withAccent("celadon")
	assert.Equal(t, rgb(0x5FB8B2), system.Color(theme.ColorNamePrimary, theme.VariantDark))

	graphite := newPaintpaneTheme(themeModeDark).withAccent("graphite")
	assert.Equal(t, rgb(0x1F1F1D), graphite.Color(colorNameSwitchKnob, theme.VariantDark), "dark knob on the pale track")
	assert.Equal(t, rgb(0xFFFFFF), newPaintpaneTheme(themeModeDark).withAccent("sage").Color(colorNameSwitchKnob, theme.VariantDark))
}

func TestLinksAreUnderlinedOnlyInGraphite(t *testing.T) {
	a := test.NewTempApp(t)
	a.Settings().SetTheme(newPaintpaneTheme(themeModeLight))
	assert.True(t, newHyperlink("link", nil).TextStyle.Underline, "graphite: the link colour is the text colour")
	a.Settings().SetTheme(newPaintpaneTheme(themeModeLight).withAccent("sage"))
	assert.False(t, newHyperlink("link", nil).TextStyle.Underline, "other accents mark links by colour")
}

func TestScenarioAccentColorSetting(t *testing.T) {
	s := newScenario(t)
	s.tapText("General")
	assert.True(t, s.shows("Accent color"))
	assert.Equal(t, "graphite", s.sa.appConfig.GetAccent(), "graphite until the user chooses")

	picker, ok := s.sa.prefsSM.allWidgets["accent"].(*swatchPicker)
	require.True(t, ok, "the accent row is a swatch picker")
	assert.Equal(t, 0, picker.Selected)
	require.Len(t, picker.names, 10)

	picker.SetSelectedIndex(3)
	s.sa.prefsSM.FlushPending()
	assert.Equal(t, "sage", s.sa.appConfig.GetAccent(), "a click saves the colour")
	th, ok := fyne.CurrentApp().Settings().Theme().(*paintpaneTheme)
	require.True(t, ok, "and applies it at once")
	assert.Equal(t, "sage", th.accent.key)
}
