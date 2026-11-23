//go:build screenshots && !appstore

package ui

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
)

// TestStoreImportScreenshots renders the Store edition's import window (v51, X–X6) in Chinese and
// English, light and dark. The Store edition's General page comes from TestPreferencesScreenshots
// built with -tags "screenshots msstore".
//
//	SCREENSHOT_DIR=<dir> go test -tags screenshots ./ui/ -run TestStoreImportScreenshots -count=1
func TestStoreImportScreenshots(t *testing.T) {
	out := os.Getenv("SCREENSHOT_DIR")
	if out == "" {
		t.Skip("SCREENSHOT_DIR not set")
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	defer i18n.SetLanguage("en")
	for _, lang := range []string{"zh", "en"} {
		for name, mode := range map[string]themeMode{"light": themeModeLight, "dark": themeModeDark} {
			i18n.SetLanguage(lang)
			useStoreImport(t, true)
			a := test.NewTempApp(t)
			a.Settings().SetTheme(newPaintpaneTheme(mode))
			sa := newEULARun(t, a).sa
			sa.offerStoreImport(false)
			s := importWindow(t, a)
			if s == nil {
				t.Fatal("the import window did not open")
			}
			s.w.Resize(fyne.NewSize(storeImportWidth, s.w.Content().MinSize().Height))
			saveCapture(t, s.w, filepath.Join(out, lang+"-"+name+"-store-import.png"))
		}
	}
}
