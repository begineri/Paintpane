package ui

import (
	"image/color"
	"runtime/debug"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"

	utilLog "github.com/dixieflatline76/Spice/v2/util/log"
)

// prefsReleaseAfter is how long after the settings window closes Paintpane hands the memory it
// used back to Windows, the same delay as the tray menu's windows (trayReleaseAfter).
var prefsReleaseAfter = time.Minute

// What a closed settings window leaves behind (Fyne 2.7.4):
//
//   - Fyne caches a GL texture for every run of text a window draws, keyed by the window's canvas,
//     and removes a window's entries only when that window rescales. Closing a window does not, so
//     each closed settings window stayed reachable through its canvas, with every widget, image and
//     closure on its pages: the Go heap grew with every open and close.
//   - The driver remembers the last focused window and does not forget it when that window is
//     closed, so the last settings window stayed reachable until another window took the focus.
//   - Fyne keeps every font it parsed to measure text (about 30 MB, mostly the system font Chinese
//     text falls back to) and its theme caches for the life of the app.
//
//   - Fyne's renderer cache keeps the widgets of pages the window built and hid until some window
//     redraws (fyne_cache_purge.go).
//
// While the window closes, Paintpane therefore replaces its content with an empty rectangle (Fyne
// then rescales the window, deleting its text textures in its own GL context, and whatever still
// holds the window no longer holds the pages) and sets the same theme again (Fyne then empties its
// font and theme caches; the next window parses the fonts again, with no measurable difference in
// opening time). A minute later it purges the renderer cache and returns the memory to Windows.

// prefsClosed is the settings window's close handler: it applies a change made just before
// closing, remembers the window size, releases what Fyne keeps of the window and drops it. It runs
// on the UI thread while the window still exists, for the title bar's close button and for Close.
func (sa *SpiceApp) prefsClosed(w fyne.Window) {
	if sa.prefsSM != nil {
		sa.prefsSM.FlushPending()
		sa.prefsSM = nil
	}
	sa.savePrefsWindowSize(w.Canvas().Size())
	sa.os.TransformToBackground()
	w.SetContent(canvas.NewRectangle(color.Transparent))
	if t := sa.Settings().Theme(); t != nil {
		sa.Settings().SetTheme(t)
	}
	sa.prefsWindow = nil
	sa.prefsNav = nil
	sa.releasePrefsLater()
}

// releasePrefsLater drops what Fyne's render caches still hold of the closed window
// (fyne_cache_purge.go) and returns the freed memory to Windows prefsReleaseAfter from now, unless
// the settings window has been opened again since. Runs on the UI thread.
func (sa *SpiceApp) releasePrefsLater() {
	sa.prefsReleaseGen++
	gen := sa.prefsReleaseGen
	// A second after closing: the closed window drew its last frame before that, and every
	// window still open or hidden has used its cache entries again by the time this runs.
	closed := time.Now().Add(time.Second)
	time.AfterFunc(prefsReleaseAfter, func() {
		onUIThread(func() {
			if sa.prefsReleaseGen != gen || sa.prefsWindow != nil {
				return
			}
			purgeFyneRenderCaches(closed)
			sa.prefsReleased++
			utilLog.Debugf("Settings window closed a minute ago: dropped its widgets from Fyne's caches, returning freed memory to Windows")
			// Without this the Go runtime hands the pages back only gradually.
			go debug.FreeOSMemory()
		})
	})
}

// cancelPrefsRelease keeps the memory for a settings window opened again. Runs on the UI thread.
func (sa *SpiceApp) cancelPrefsRelease() {
	sa.prefsReleaseGen++
}
