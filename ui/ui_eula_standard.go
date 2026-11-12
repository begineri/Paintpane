//go:build !appstore

package ui

import (
	"fyne.io/fyne/v2"
	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/sysinfo"
	"github.com/dixieflatline76/Spice/v2/util"
	utilLog "github.com/dixieflatline76/Spice/v2/util/log"
)

// verifyEULA checks if the End User License Agreement has been accepted. If not, it will show the EULA and prompt the user to accept it.
// If the user declines, the application will quit.
// If the EULA has been accepted, the application will proceed to setup.
func (sa *SpiceApp) verifyEULA() {
	// Probe OpenGL availability before attempting window creation.
	// Fyne's GLFW crashes the process at the C level when OpenGL is unavailable
	// (e.g. RDP, broken GPU driver) — Go's recover() cannot catch this.
	if !sysinfo.CanCreateWindows() {
		utilLog.Println("OpenGL unavailable — skipping EULA and splash windows")
		// Don't create any windows — proceed directly to background mode.
		// The app will run headless with tray menu and hotkeys.
		sa.os.TransformToBackground()
		return
	}

	// Check if the EULA has been accepted
	if util.HasAcceptedEULA(sa.Preferences()) {
		sa.os.TransformToBackground() // EULA already accepted: run in the background
	} else {
		sa.displayEULAAcceptance() // Show the EULA if it hasn't been accepted
	}
}

// displayEULAAcceptance displays the End User License Agreement and prompts the user
// to accept it. If the user declines, the application will quit.
func (sa *SpiceApp) displayEULAAcceptance() {
	// The text follows the system language: at first run the app language is still System default.
	eulaText, err := eulaTextFor(sa.assetMgr, i18n.EffectiveLanguage())
	if err != nil {
		utilLog.Fatalf("Error loading EULA: %v", err)
	}

	// Create a new window for the EULA
	sa.os.TransformToForeground() // Ensure the app is in the foreground before showing the EULA
	eulaWindow := sa.NewWindow(i18n.T("License agreement"))
	eulaWindow.SetOnClosed(sa.os.TransformToBackground) // Set the close action to transform to background
	eulaWindow.Resize(fyne.NewSize(800, 600))
	eulaWindow.CenterOnScreen()
	eulaWindow.SetCloseIntercept(func() {
		// Prevent the window from being closed
	})

	eulaWindow.SetContent(newEULAContent(eulaText,
		func() {
			// Stop the service before quitting the application
			sa.Quit()
		},
		func() {
			sa.acceptEULA()
			eulaWindow.Close()
			sa.os.TransformToBackground() // Run in the background once the EULA is accepted
		}))
	eulaWindow.Show()
	styleTitleBar(eulaWindow)
}

// acceptEULA records the acceptance. On a first run (no acceptance of any version recorded before)
// it also registers the program to start with Windows, once (S3-6, registerAutostartOnFirstRun).
func (sa *SpiceApp) acceptEULA() {
	prefs := sa.Preferences()
	acceptedBefore := util.HasEverAcceptedEULA(prefs) // read before this acceptance is recorded
	util.MarkEULAAccepted(prefs)
	registerAutostartOnFirstRun(prefs, acceptedBefore)
}
