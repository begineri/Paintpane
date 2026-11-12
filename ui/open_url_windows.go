//go:build windows

package ui

import "syscall"

var procAllowSetForegroundWindow = syscall.NewLazyDLL("user32.dll").NewProc("AllowSetForegroundWindow")

// asfwAny is ASFW_ANY ((DWORD)-1): every process may take the foreground.
const asfwAny = ^uint32(0)

// letBrowserComeForward lets the browser that opens a link bring its window to the front. Windows
// lets a process take the foreground only if the foreground process (or the one that got the last
// input) allows it. The tray menu runs its item after it has closed, so the app is no longer in
// front by then, and the browser it starts through the shell would open the page behind the
// other windows: the click on the tray's first row seemed to do nothing unless the browser
// already happened to be in front. The app still holds the last input (the click), so it may
// pass the right on. The right ends at the user's next key or click.
func letBrowserComeForward() bool {
	r, _, _ := procAllowSetForegroundWindow.Call(uintptr(asfwAny))
	return r != 0
}
