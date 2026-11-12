package ui

import "fyne.io/fyne/v2"

// Preferences window size, in Fyne logical units.
const (
	defaultPrefsWindowWidth  float32 = 880
	defaultPrefsWindowHeight float32 = 640
	minPrefsWindowWidth      float32 = 560
	minPrefsWindowHeight     float32 = 400

	prefsWindowWidthKey  = "prefs_window_width"
	prefsWindowHeightKey = "prefs_window_height"
)

// savedPrefsWindowSize returns the last saved window size, or the default when
// none is saved or the saved one is implausibly small.
func (sa *SpiceApp) savedPrefsWindowSize() (float32, float32) {
	prefs := sa.Preferences()
	w := float32(prefs.Float(prefsWindowWidthKey))
	h := float32(prefs.Float(prefsWindowHeightKey))
	if w < minPrefsWindowWidth || h < minPrefsWindowHeight {
		return defaultPrefsWindowWidth, defaultPrefsWindowHeight
	}
	return w, h
}

func (sa *SpiceApp) savePrefsWindowSize(size fyne.Size) {
	if size.Width < minPrefsWindowWidth || size.Height < minPrefsWindowHeight {
		return
	}
	prefs := sa.Preferences()
	prefs.SetFloat(prefsWindowWidthKey, float64(size.Width))
	prefs.SetFloat(prefsWindowHeightKey, float64(size.Height))
}
