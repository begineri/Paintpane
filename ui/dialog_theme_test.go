package ui

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
)

// A2 §15.3b #8: after switching to dark, the clear-cache dialog's title could not be seen in A2's
// headless capture. These tests follow the real app: it starts in the Paintpane theme (ui.go), and
// the Appearance control applies 0.8 s after the tap (or when the window closes).

// overlayTextColors returns the colour of each piece of text drawn in the window's dialogs.
func overlayTextColors(s *scenario) map[string]color.Color {
	out := map[string]color.Color{}
	for _, ov := range s.w.Canvas().Overlays().List() {
		for _, o := range shownObjects(ov) {
			if tx, ok := o.(*canvas.Text); ok {
				out[tx.Text] = tx.Color
			}
		}
	}
	return out
}

func dialogPanelFill(t *testing.T, s *scenario) color.Color {
	t.Helper()
	for _, ov := range s.w.Canvas().Overlays().List() {
		for _, o := range shownObjects(ov) {
			if b, ok := o.(*dialogBackground); ok {
				return panelRect(b).FillColor
			}
		}
	}
	t.Fatal("no dialog panel shown")
	return nil
}

func panelRect(b *dialogBackground) *canvas.Rectangle {
	for _, o := range shownObjects(b) {
		if r, ok := o.(*canvas.Rectangle); ok {
			return r
		}
	}
	return nil
}

func startInPaintpaneTheme(t *testing.T) *scenario {
	t.Helper()
	s := newScenarioWith(t, wallpaper.GetInstance())
	applyTheme(s.sa.App, themeModeLight, accentDefault)
	t.Cleanup(func() { fyne.CurrentApp().Settings().SetTheme(theme.DefaultTheme()) })
	s.sa.RebuildPreferencesContent("")
	return s
}

func TestClearCacheDialogAfterSwitchingToDark(t *testing.T) {
	s := startInPaintpaneTheme(t)
	s.tapText(i18n.T("General"))
	s.tapText(i18n.T("Dark"))
	s.sa.prefsSM.FlushPending() // the 0.8 s apply
	require.True(t, isDarkTheme())

	s.tapText(i18n.T("Advanced"))
	s.tapText(i18n.T("Clear cache"))
	texts := overlayTextColors(s)
	assert.Equal(t, darkPalette[theme.ColorNameForeground], texts[i18n.T("Clear cache?")], "the title is drawn in the dark theme's text colour")
	assert.Equal(t, darkPalette[theme.ColorNameOverlayBackground], dialogPanelFill(t, s), "on the dark panel")
}

// A dialog that is open while the theme changes (the Appearance change applies 0.8 s after the tap,
// so the person may have opened a dialog by then) is redrawn in the new theme.
func TestOpenDialogFollowsAThemeSwitch(t *testing.T) {
	s := startInPaintpaneTheme(t)
	s.tapText(i18n.T("Advanced"))
	s.tapText(i18n.T("Clear cache"))
	assert.Equal(t, lightPalette[theme.ColorNameForeground], overlayTextColors(s)[i18n.T("Clear cache?")])

	applyTheme(s.sa.App, themeModeDark, accentDefault)
	texts := overlayTextColors(s)
	assert.Equal(t, darkPalette[theme.ColorNameForeground], texts[i18n.T("Clear cache?")], "the title follows the switch")
	assert.Equal(t, darkPalette[theme.ColorNameOverlayBackground], dialogPanelFill(t, s), "and so does the panel")
}

// A2 §15.3b #8: only the rounded panel shows. The pop-up's own square panel is transparent and no
// larger than the content, so no square corners or band show around the rounded one, in either theme.
func TestDialogHasNoSquarePanel(t *testing.T) {
	for _, mode := range []themeMode{themeModeLight, themeModeDark} {
		s := startInPaintpaneTheme(t)
		applyTheme(s.sa.App, mode, accentDefault)
		s.tapText(i18n.T("Advanced"))
		s.tapText(i18n.T("Clear cache"))

		overlays := s.w.Canvas().Overlays().List()
		require.Len(t, overlays, 1)
		popup, ok := overlays[0].(*widget.PopUp)
		require.True(t, ok)
		canvasSize := s.w.Canvas().Size()
		var squares int
		for _, o := range test.WidgetRenderer(popup).Objects() {
			r, ok := o.(*canvas.Rectangle)
			if !ok || r.Size() == canvasSize { // the dimmed underlay covers the window
				continue
			}
			squares++
			_, _, _, alpha := r.FillColor.RGBA()
			assert.Zero(t, alpha, "%s: the pop-up's square panel is transparent", mode)
			assert.Equal(t, popup.Content.Size(), r.Size(), "%s: and no larger than the rounded panel", mode)
			assert.Equal(t, popup.Content.Position(), r.Position(), mode)
		}
		assert.Equal(t, 1, squares)
		assert.Equal(t, paintpaneColor(theme.ColorNameOverlayBackground), dialogPanelFill(t, s), "%s: the rounded panel keeps its colour", mode)
		popup.Hide()
	}
}
