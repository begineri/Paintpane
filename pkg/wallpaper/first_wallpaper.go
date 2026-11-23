package wallpaper

import "github.com/dixieflatline76/Spice/v2/util/log"

// A display that has never had a wallpaper from Paintpane gets its first image as soon as one is
// ready (CmdFirstWallpaper), but not before the person has accepted the licence: until then the
// desktop and the lock screen are left alone. Downloading starts at once either way, so the first
// image is usually ready the moment the licence is accepted.

// LicenceAccepted tells the plugin that the licence is accepted: at start when this version was
// accepted before, when the person clicks Accept, or when the build accepts it itself. Displays
// waiting for their first wallpaper then get it. Calling it again does nothing.
func (wp *Plugin) LicenceAccepted() {
	wp.licenceMu.Lock()
	already := wp.licenceAccepted
	wp.licenceAccepted = true
	due := wp.firstWallpaperDue
	wp.licenceMu.Unlock()
	if !already && due {
		log.Printf("[Startup] Licence accepted; displays without a wallpaper get their first image.")
		wp.dispatch(-1, CmdFirstWallpaper)
	}
}

// firstWallpapersDue is the point in a start where displays without a wallpaper from Paintpane get
// their first image: now if the licence is accepted, otherwise when it is (LicenceAccepted).
func (wp *Plugin) firstWallpapersDue() {
	wp.licenceMu.Lock()
	wp.firstWallpaperDue = true
	accepted := wp.licenceAccepted
	wp.licenceMu.Unlock()
	if !accepted {
		log.Printf("[Startup] The licence is not accepted yet; the first wallpaper waits for it.")
		return
	}
	wp.dispatch(-1, CmdFirstWallpaper)
}
