package ui

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
)

// v43 (owner chose B): every destructive button is grey with red text, in the settings rows and in
// dialogs, light and dark.

func dangerButtonColors(b *dangerButton) (bg, fg color.Color) {
	r := test.WidgetRenderer(b).(*dangerButtonRenderer)
	return r.bg.FillColor, r.text.Color
}

func dialogButtonColors(b *dialogButton) (bg, fg color.Color) {
	r := test.WidgetRenderer(b).(*dialogButtonRenderer)
	return r.bg.FillColor, r.text.Color
}

func shownDangerButton(t *testing.T, s *scenario, text string) *dangerButton {
	t.Helper()
	for _, o := range shownObjects(s.w.Canvas().Content()) {
		if b, ok := o.(*dangerButton); ok && b.Text == text {
			return b
		}
	}
	t.Fatalf("no red-text button reads %q", text)
	return nil
}

func TestDestructiveButtonsAreGreyWithRedText(t *testing.T) {
	for _, mode := range []themeMode{themeModeLight, themeModeDark} {
		for _, lang := range []string{"en", "zh"} {
			i18n.SetLanguage(lang)
			s := startInPaintpaneTheme(t)
			applyTheme(s.sa.App, mode, accentDefault)
			grey, red := paintpaneColor(theme.ColorNameButton), paintpaneColor(theme.ColorNameError)
			s.tapText(i18n.T("Advanced"))

			for _, text := range []string{i18n.T("Clear cache"), i18n.T("Reset")} {
				b := shownDangerButton(t, s, text)
				b.Enable() // Reset is greyed out while nothing is blocked
				b.Refresh()
				bg, fg := dangerButtonColors(b)
				assert.Equal(t, grey, bg, "%s/%s: %q in the row has the button fill", mode, lang, text)
				assert.Equal(t, red, fg, "%s/%s: %q in the row has red text", mode, lang, text)
			}

			s.tapText(i18n.T("Clear cache"))
			buttons := dialogButtons(s)
			require.Len(t, buttons, 2)
			require.Equal(t, buttonDanger, buttons[1].kind)
			bg, fg := dialogButtonColors(buttons[1])
			assert.Equal(t, grey, bg, "%s/%s: the dialog's Clear cache has the button fill, not a red one", mode, lang)
			assert.Equal(t, red, fg, "%s/%s: and red text, like the row's", mode, lang)
			cbg, cfg := dialogButtonColors(buttons[0])
			assert.Equal(t, grey, cbg, "Cancel keeps the button fill")
			assert.NotEqual(t, red, cfg, "and its usual text colour")
			test.Tap(buttons[0])
		}
	}
	i18n.SetLanguage("en")
}
