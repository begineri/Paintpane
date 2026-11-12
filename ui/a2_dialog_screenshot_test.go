//go:build screenshots

package ui

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
)

// TestA2ClearCacheDialog renders the Advanced page with the "Clear cache?" dialog open, in light and dark.
//
//	SCREENSHOT_DIR=<dir> go test -tags screenshots ./ui/ -run TestA2ClearCacheDialog -count=1
func TestA2ClearCacheDialog(t *testing.T) {
	out := os.Getenv("SCREENSHOT_DIR")
	if out == "" {
		t.Skip("SCREENSHOT_DIR not set")
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, lang := range []string{"zh", "en"} {
		i18n.SetLanguage(lang)
		for name, mode := range map[string]themeMode{"light": themeModeLight, "dark": themeModeDark} {
			s := newScenarioWith(t, wallpaper.GetInstance())
			s.sa.App.Settings().SetTheme(newPaintpaneTheme(mode))
			s.sa.RebuildPreferencesContent("")
			s.w.Resize(fyne.NewSize(defaultPrefsWindowWidth, defaultPrefsWindowHeight))
			s.sa.prefsNav.Select(pageAdvanced)
			s.tapText(i18n.T("Clear cache"))
			saveCapture(t, s.w, filepath.Join(out, lang+"-"+name+"-clear-cache-dialog.png"))
		}
	}
	i18n.SetLanguage("en")
}

// TestA2ClearCacheDialogAfterLiveSwitch switches to dark with the 外观 control (as a person does), then opens the dialog.
//
// Like the app, it starts in the Paintpane theme (ui.go applies it at start), and the switch is
// applied before the dialog opens: the control applies 0.8 s after the tap. Without these two
// steps the window starts in Fyne's test theme and the dialog is built before the switch, so its
// title is drawn dark on dark and the side bar stays light (A2 §15.3b #8, screenshot 22).
func TestA2ClearCacheDialogAfterLiveSwitch(t *testing.T) {
	out := os.Getenv("SCREENSHOT_DIR")
	if out == "" {
		t.Skip("SCREENSHOT_DIR not set")
	}
	i18n.SetLanguage("zh")
	defer i18n.SetLanguage("en")
	s := startInPaintpaneTheme(t)
	s.w.Resize(fyne.NewSize(defaultPrefsWindowWidth, defaultPrefsWindowHeight))
	s.tapText(i18n.T("General"))
	s.tapText(i18n.T("Dark"))
	s.sa.prefsSM.FlushPending()
	s.tapText(i18n.T("Advanced"))
	s.tapText(i18n.T("Clear cache"))
	saveCapture(t, s.w, filepath.Join(out, "zh-live-dark-clear-cache-dialog.png"))
}
