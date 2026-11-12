package ui

import (
	"image"
	"net/url"

	"fyne.io/fyne/v2"
	"github.com/dixieflatline76/Spice/v2/asset"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
)

// TunePreview tells Tune Image what the screen looks like and how to draw the wallpaper for it, so
// the preview shows the desktop result: the crop that fills the screen, the framed picture, or the
// whole image on the wall colour (ui_refresh.md section 12, J2 and J3).
type TunePreview struct {
	ScreenW, ScreenH int // the display's size in pixels; zero means 16:9
	// Global is the display mode set for all images (Full, Framed or Crop). The window offers the
	// same three for this image; choosing Global's makes the image follow the setting again.
	// Inherit means unknown and reads as Full.
	Global provider.DisplayOverride
	// Render draws src as the wallpaper would look at w x h pixels with the given options. It may
	// be nil, in which case the preview shows the image as it is.
	Render func(src image.Image, w, h int, opts provider.TuningOptions) image.Image
	// AutoPosition says where the automatic crop keeps src along the cropped axis, 0 to 100 percent,
	// so a drag starts from where the picture really is. It may be nil, or report false, in which
	// case a drag starts from the middle.
	AutoPosition func(src image.Image, w, h int) (percent float64, ok bool)
}

// PluginManager is the interface that must be implemented by all UI plugin managers.
type PluginManager interface {
	Register(Plugin)                                                      // Registers a plugin.
	Deregister(Plugin)                                                    // Deregisters a plugin.
	CreateMenuItem(string, func(), string) *fyne.MenuItem                 // Creates a menu item.
	CreateToggleMenuItem(string, func(bool), string, bool) *fyne.MenuItem // Creates a toggle menu item.
	OpenURL(*url.URL) error                                               // Opens a URL.
	OpenPreferences(tab string)                                           // OpenPreferences opens the preferences window, optionally navigating to a specific tab.
	GetPreferences() fyne.Preferences                                     // Returns the preferences.
	GetAssetManager() *asset.Manager                                      // Returns the asset manager.
	RefreshTrayMenu()                                                     // Refreshes the tray menu.
	RebuildTrayMenu()                                                     // Rebuilds the tray menu from scratch.

	// ShowTuneImagePopup displays the popup for tuning images (crop anchor, framing overrides).
	// The outer ring (UI layer) owns window lifecycle and OpenGL error recovery.
	// previewPath is the image shown in the window (the original when available).
	// onSelect receives the chosen tuning options and a done callback to invoke after processing.
	ShowTuneImagePopup(monitorID int, previewPath string, currentOpts provider.TuningOptions, effectiveOpts provider.TuningOptions, labels [9]string, values [9]provider.CropAnchor, preview TunePreview, lockAnchors bool, onSelect func(opts provider.TuningOptions, onDone func()), onClose func())
}

// App is the interface that must be implemented by all applications.
type App interface {
	Start()                    // Bam starts the Spice application.
	Lifecycle() fyne.Lifecycle // Lifecycle returns the application lifecycle.
}
