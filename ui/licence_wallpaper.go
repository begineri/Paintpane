package ui

import "github.com/dixieflatline76/Spice/v2/pkg/wallpaper"

// licenceAccepted tells the wallpaper plugin that the licence is accepted, so a display without a
// wallpaper from Paintpane (a fresh install) may get its first image (wallpaper.LicenceAccepted).
// Until then the desktop and the lock screen are left alone. Tests replace it.
var licenceAccepted = func() { wallpaper.GetInstance().LicenceAccepted() }
