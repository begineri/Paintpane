package ui

import (
	"fmt"
	"os"
	"os/exec"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"

	"github.com/dixieflatline76/Spice/v2/config"
	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/storeimport"
	"github.com/dixieflatline76/Spice/v2/util"
	utilLog "github.com/dixieflatline76/Spice/v2/util/log"
)

// First start of the Microsoft Store edition (docs/ui_refresh.md §12 v51, X–X6): right after the
// licence is accepted, if the portable exe's settings are on this PC, one small window asks whether
// to bring over its settings, favorites and blocked images. Asked once; closing the window is
// "Don't import". Importing copies the files, then the app restarts so the settings are read
// fresh (pkg/storeimport).

const storeImportWidth float32 = 420

// Tests replace these.
var (
	storeImportEnabled  = config.IsStoreDistribution
	storeImportWebPrefs = storeimport.WebPrefsPath
	storeImportRun      = (*SpiceApp).importFromWebVersion
)

// offerStoreImport shows the import window on the Store edition's first start, once.
func (sa *SpiceApp) offerStoreImport(acceptedBefore bool) {
	prefs := sa.Preferences()
	if !storeImportEnabled() || acceptedBefore || prefs.Bool(storeimport.AskedKey) {
		return
	}
	if !storeimport.Found(storeImportWebPrefs()) {
		prefs.SetBool(storeimport.AskedKey, true) // nothing to import; do not look again
		return
	}
	utilLog.Printf("[Store] Found the portable exe's settings; asking whether to import them")

	w := sa.NewWindow(i18n.T("Paintpane"))
	answered, closed := false, false
	closeWindow := func() {
		if !closed {
			closed = true
			w.Close()
		}
	}
	answer := func(doImport bool) {
		if answered {
			return
		}
		answered = true
		prefs.SetBool(storeimport.AskedKey, true)
		if !doImport {
			utilLog.Printf("[Store] Not importing the portable exe's data")
			closeWindow()
			return
		}
		if err := storeImportRun(sa); err != nil {
			utilLog.Printf("[Store] Import failed: %v", err)
			answered = false // the window stays, so the person can try again or close it
			showErrorDialog(w, err)
			return
		}
		closeWindow()
	}
	// The title bar's close button is "Don't import".
	w.SetOnClosed(func() { closed = true; answer(false) })
	w.SetContent(newStoreImportContent(func() { answer(false) }, func() { answer(true) }))
	w.SetFixedSize(true)
	w.Resize(fyne.NewSize(storeImportWidth, w.Content().MinSize().Height))
	w.CenterOnScreen()
	w.Show()
	styleTitleBar(w)
}

// newStoreImportContent is the window's content: the question, one sentence, the three things
// that come over and the two buttons.
func newStoreImportContent(onSkip, onImport func()) fyne.CanvasObject {
	title := newDialogTitle(i18n.T("Import from the downloaded version?"), storeImportWidth)
	text := noInset(newDialogText(i18n.T("You can bring over the settings and favorites from the downloaded version."), storeImportWidth))

	// The tray's glyphs (Material icons) in the placeholder colour, like the blocked-images page.
	item := func(glyph, label string) fyne.CanvasObject {
		img := canvas.NewImageFromResource(trayGlyph(glyph, theme.Color(theme.ColorNamePlaceHolder)))
		img.SetMinSize(fyne.NewSize(20, 20))
		img.FillMode = canvas.ImageFillContain
		text := newStyledText(label, theme.TextSize(), true, theme.ColorNameForeground)
		return container.New(layout.NewCustomPaddedHBoxLayout(10), container.NewCenter(img), container.NewCenter(text))
	}
	list := newCard(
		item("settings", i18n.T("Settings")),
		item("heart", i18n.T("Favorites")),
		item("block", i18n.T("Blocked images")),
	)

	skip := newDialogButton(i18n.T("Don't import"), buttonNeutral, onSkip)
	imp := newDialogButton(i18n.T("Import"), buttonPrimary, onImport)
	buttons := container.New(&rightRowLayout{gap: 8}, skip, imp)

	body := vstack(8, title, text, inset(list, 6, 0, 0, 0), inset(buttons, 14, 0, 0, 0))
	return container.NewStack(newDialogBackground(), inset(body, dialogPadding, dialogPadding, dialogPadding, dialogPadding))
}

// importFromWebVersion copies the portable exe's favorites and blocked-image thumbnails, leaves its
// settings for the next start (keeping this edition's licence acceptance) and restarts the app.
func (sa *SpiceApp) importFromWebVersion() error {
	n, err := storeimport.CopyData(storeimport.WebDataDir(), config.GetWorkingDir())
	if err != nil {
		return err
	}
	prefs := sa.Preferences()
	keep := map[string]any{
		util.EULAPreferenceKey: prefs.String(util.EULAPreferenceKey),
		storeimport.AskedKey:   true,
	}
	if err := storeimport.Prepare(storeImportWebPrefs(), storeimport.PendingPath(), storeimport.NotImported, keep); err != nil {
		return err
	}
	utilLog.Printf("[Store] Imported %d files from the portable exe; restarting to read its settings", n)

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("restart: %w", err)
	}
	// #nosec G204 -- this program itself, with a fixed flag
	if err := exec.Command(exe, storeimport.RestartFlag).Start(); err != nil {
		_ = os.Remove(storeimport.PendingPath()) // nothing half-done for a later start
		return fmt.Errorf("restart: %w", err)
	}
	go sa.quit()
	return nil
}
