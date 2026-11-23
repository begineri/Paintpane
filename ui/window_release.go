package ui

import (
	"image/color"
	"runtime/debug"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
)

// windowPurgeAfter is how long after a Tune Image or tray menu window closes Paintpane purges
// Fyne's render caches and returns the freed memory to Windows, as for the settings window
// (prefsReleaseAfter).
var windowPurgeAfter = time.Minute

// windowPurgeGen cancels a pending purge when another window closes before it runs; the later
// purge covers both. windowPurges counts the purges done. Used on the UI thread only.
var windowPurgeGen, windowPurges int

// releaseClosingWindow lets Fyne forget a window that is closing, the way prefsClosed does for the
// settings window (prefs_release.go): it replaces the content with an empty rectangle, so Fyne
// rescales the window and deletes its text textures, which otherwise keep the window, with every
// widget, image and callback it showed, reachable after it is gone; and a minute later it drops
// what Fyne's render caches still hold of it. Without this each Tune Image window that was opened
// and closed left about 4 MB on the Go heap. Runs on the UI thread while the window still exists.
func releaseClosingWindow(w fyne.Window) {
	w.SetContent(canvas.NewRectangle(color.Transparent))
	purgeClosedWindowsLater()
}

// purgeClosedWindowsLater purges Fyne's render caches of entries unused since now and returns the
// freed memory to Windows windowPurgeAfter from now. Runs on the UI thread.
func purgeClosedWindowsLater() {
	windowPurgeGen++
	gen := windowPurgeGen
	// A second after closing: the closed window drew its last frame before that, and every window
	// still open or hidden has used its cache entries again by the time this runs.
	closed := time.Now().Add(time.Second)
	time.AfterFunc(windowPurgeAfter, func() {
		onUIThread(func() {
			if windowPurgeGen != gen {
				return
			}
			purgeFyneRenderCaches(closed)
			windowPurges++
			go debug.FreeOSMemory()
		})
	})
}
