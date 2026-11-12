//go:build !windows

package ui

// installTrayPopup does nothing outside Windows: the native tray menu shows.
func (sa *SpiceApp) installTrayPopup() {}
