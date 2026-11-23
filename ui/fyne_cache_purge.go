package ui

import (
	"time"
	_ "unsafe" // go:linkname
)

// Fyne (2.7.4) keeps a renderer for every widget it has measured or drawn, and a canvas entry for
// every object it has drawn, in package-level caches of fyne.io/fyne/v2/internal/cache. Entries
// not used for a minute (cache.ValidDuration) are dropped, but only from the GLFW paint loop and
// only after some window has redrawn. Closing a window drops the entries of what it showed, not
// those of pages it built and hid; with no window left drawing, as is usual for a tray app, those
// stay until the next window opens, and with them the closed window's widgets, images and
// callbacks. purgeFyneRenderCaches does the same purge without waiting for a redraw. The functions
// are unexported, so they are linked by name; a Fyne update that renames them fails the build.

//go:linkname fyneDestroyExpiredRenderers fyne.io/fyne/v2/internal/cache.destroyExpiredRenderers
func fyneDestroyExpiredRenderers(now time.Time)

//go:linkname fyneDestroyExpiredCanvases fyne.io/fyne/v2/internal/cache.destroyExpiredCanvases
func fyneDestroyExpiredCanvases(now time.Time)

// fyneCacheValid is fyne.io/fyne/v2/internal/cache.ValidDuration (unless FYNE_CACHE sets another).
const fyneCacheValid = time.Minute

// purgeFyneRenderCaches drops the renderers and canvas entries that have not been used since
// unusedSince, as Fyne's own purge does for those unused for a minute. Open windows mark what
// they show as used whenever they draw, and hidden windows at least every 50 seconds, so their
// entries stay. Runs on the UI thread.
func purgeFyneRenderCaches(unusedSince time.Time) {
	expiredBy := unusedSince.Add(fyneCacheValid)
	fyneDestroyExpiredRenderers(expiredBy)
	fyneDestroyExpiredCanvases(expiredBy)
}
