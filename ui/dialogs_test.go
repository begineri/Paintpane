package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
)

// dialogButtons returns the dialog buttons of the overlay on top, left to right.
func dialogButtons(s *scenario) []*dialogButton {
	top := s.w.Canvas().Overlays().Top()
	if top == nil {
		return nil
	}
	var out []*dialogButton
	for _, o := range shownObjects(top) {
		if b, ok := o.(*dialogButton); ok {
			out = append(out, b)
		}
	}
	return out
}

func overlayShows(s *scenario, text string) bool {
	top := s.w.Canvas().Overlays().Top()
	if top == nil {
		return false
	}
	for _, o := range shownObjects(top) {
		if got, ok := labelText(o); ok && got == text {
			return true
		}
	}
	return false
}

// The confirm dialogs (A-1): buttons say what they do, in the app language, and the
// destructive one is red.
func TestScenarioConfirmDialogsNameTheirActions(t *testing.T) {
	inLanguages(t, func(t *testing.T, lang string) {
		want := map[string]struct{ page, cancel, clear, reset, title string }{
			"en": {"Advanced", "Cancel", "Clear cache", "Reset", "Clear cache?"},
			"zh": {"高级", "取消", "清除缓存", "重置", "清除缓存？"},
		}[lang]
		s := newScenarioWith(t, wallpaper.GetInstance())
		s.tapText(want.page)
		require.Equal(t, pageAdvanced, s.sa.prefsNav.Current())

		for name, confirmText := range map[string]string{"clearCache": want.clear, "resetAvoidSet": want.reset} {
			target := s.sa.prefsSM.allWidgets[name]
			require.NotNil(t, target, name)
			test.Tap(target.(fyne.Tappable))

			buttons := dialogButtons(s)
			require.Len(t, buttons, 2, "%s: a dialog with two buttons opens", name)
			assert.Equal(t, want.cancel, buttons[0].Text)
			assert.Equal(t, confirmText, buttons[1].Text, "%s: the button says what it does", name)
			assert.Equal(t, buttonNeutral, buttons[0].kind)
			assert.Equal(t, buttonDanger, buttons[1].kind, "%s: the destructive button is red", name)
			if name == "clearCache" {
				assert.True(t, overlayShows(s, want.title))
			}

			// Cancel closes the dialog without doing anything.
			test.Tap(buttons[0])
			assert.Nil(t, s.w.Canvas().Overlays().Top(), "%s: Cancel closes the dialog", name)
		}
	})
}
