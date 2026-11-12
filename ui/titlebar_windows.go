package ui

import (
	"unsafe"

	"fyne.io/fyne/v2"
)

// dwmwaUseImmersiveDarkMode is DWMWA_USE_IMMERSIVE_DARK_MODE: Windows 10 20H1 and later, Windows 11.
const dwmwaUseImmersiveDarkMode = 20

// styleTitleBar makes the window's title bar follow the app's light or dark look. Windows draws the
// bar itself and follows the system theme, so without this a dark window keeps a white bar on a light
// system. Call it after the window is shown (the native window exists then); it is harmless on Windows
// versions that do not know the attribute.
func styleTitleBar(w fyne.Window) {
	if w == nil {
		return
	}
	h := trayHWND(w)
	if h == 0 {
		return
	}
	var dark int32
	if isDarkTheme() {
		dark = 1
	}
	procTrayDwmSetWindowAttr.Call(h, dwmwaUseImmersiveDarkMode, uintptr(unsafe.Pointer(&dark)), 4)
}

// styleOpenTitleBars restyles every open window's title bar after the theme changed.
func styleOpenTitleBars() {
	fyne.Do(func() {
		for _, w := range fyne.CurrentApp().Driver().AllWindows() {
			styleTitleBar(w)
		}
	})
}
