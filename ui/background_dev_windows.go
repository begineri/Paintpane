//go:build !release && windows

package ui

import (
	"os"
	"syscall"
	"unsafe"

	"github.com/go-gl/glfw/v3.3/glfw"
	"golang.org/x/sys/windows"

	utilLog "github.com/dixieflatline76/Spice/v2/util/log"
)

// Background mode, for dev builds that a script or another Claude session starts to
// measure or check something while the owner works at the same PC. With
// PAINTPANE_DEV_BACKGROUND=1 in the environment, the windows the app opens (settings,
// Tune Image, licence, About splash) open behind the owner's windows and do not take the
// focus: they are shown, drawn and captured as usual (GET /debug/prefs.png), but the
// owner's window stays in front and keeps the keyboard. Release builds do not have this
// file, so their windows always come to the front.
//
//   - GLFW's FOCUS_ON_SHOW hint is off, so showing a window does not call
//     BringWindowToTop and SetForegroundWindow. (Refusing the activation later is too
//     late: Windows has already taken the focus from the owner's window by then.)
//   - CreatePreferencesWindow does not call RequestFocus (see devBackground).
//   - Each top-level window made on the UI thread is created at the bottom of the z-order,
//     and when it is shown or moved without being activated it stays at the bottom: GLFW
//     shows windows with SW_SHOWNA, which would otherwise put them above every other.
//     A click on the window still brings it forward, and topmost windows are left alone.
//
// The app-drawn tray menu (POST /debug/traymenu/open) is a topmost window that takes the
// focus and closes when it loses it, as the tray's own menu does; it is not changed.
const devBackgroundEnv = "PAINTPANE_DEV_BACKGROUND"

var (
	bgUser32           = windows.NewLazySystemDLL("user32.dll")
	procSetWindowsHook = bgUser32.NewProc("SetWindowsHookExW")
	procCallNextHook   = bgUser32.NewProc("CallNextHookEx")
	procSetWindowLong  = bgUser32.NewProc("SetWindowLongPtrW")
	procGetWindowLong  = bgUser32.NewProc("GetWindowLongPtrW")
	procCallWindowProc = bgUser32.NewProc("CallWindowProcW")

	devBackgroundHook uintptr
	devBackgroundProc = map[uintptr]uintptr{} // window → its own window procedure (UI thread only)
	devBackgroundSub  = syscall.NewCallback(devBackgroundWndProc)
)

const (
	whCBT               = 5
	hcbtCreateWnd       = 3
	hwndBottom          = 1
	hwndTopmost         = ^uintptr(0)     // HWND_TOPMOST (-1)
	hwndNoTopmost       = ^uintptr(0) - 1 // HWND_NOTOPMOST (-2)
	wsChild             = 0x40000000
	wsExTopmost         = 0x8
	wmWindowPosChanging = 0x0046
	wmNCDestroy         = 0x0082
	swpNoZOrder         = 0x0004
	swpNoActivateBg     = 0x0010
	swpShowWindow       = 0x0040
)

var (
	gwlpWndProc = negIndex(4)  // GWLP_WNDPROC (-4)
	gwlExStyle  = negIndex(20) // GWL_EXSTYLE (-20)
)

func negIndex(n int) uintptr { i := -n; return uintptr(i) }

// cbtCreateWnd is CBT_CREATEWND; createStruct is the start of CREATESTRUCTW, up to style.
type cbtCreateWnd struct {
	cs          *createStruct
	insertAfter uintptr
}

type createStruct struct {
	createParams uintptr
	instance     uintptr
	menu         uintptr
	parent       uintptr
	cy, cx, y, x int32
	style        int32
}

// windowPos is WINDOWPOS.
type windowPos struct {
	hwnd, insertAfter uintptr
	x, y, cx, cy      int32
	flags             uint32
}

func devBackgroundCBT(code, wParam, lParam uintptr) uintptr {
	if int32(code) == hcbtCreateWnd {
		c := *(**cbtCreateWnd)(unsafe.Pointer(&lParam)) // lParam points to CBT_CREATEWND
		if c.cs != nil && uint32(c.cs.style)&wsChild == 0 {
			c.insertAfter = hwndBottom
			if old, _, _ := procSetWindowLong.Call(wParam, gwlpWndProc, devBackgroundSub); old != 0 {
				devBackgroundProc[wParam] = old
			}
		}
	}
	r, _, _ := procCallNextHook.Call(0, code, wParam, lParam)
	return r
}

// devBackgroundWndProc keeps a window that is shown or moved without being activated at the
// bottom of the z-order (see keepAtBottom), then passes every message on.
func devBackgroundWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	old := devBackgroundProc[hwnd]
	switch msg {
	case wmWindowPosChanging:
		ex, _, _ := procGetWindowLong.Call(hwnd, gwlExStyle)
		keepAtBottom(*(**windowPos)(unsafe.Pointer(&lParam)), ex&wsExTopmost != 0)
	case wmNCDestroy:
		delete(devBackgroundProc, hwnd)
	}
	r, _, _ := procCallWindowProc.Call(old, hwnd, msg, wParam, lParam)
	return r
}

// keepAtBottom changes a window move that does not activate the window and would change its
// place in the z-order (or show it, which brings it to the top) into one that puts it at the
// bottom. A move that activates it (a click on it) and topmost windows are left as they are.
func keepAtBottom(p *windowPos, topmost bool) {
	if p.flags&swpNoActivateBg == 0 || topmost || p.insertAfter == hwndTopmost || p.insertAfter == hwndNoTopmost {
		return
	}
	if p.flags&swpNoZOrder == 0 || p.flags&swpShowWindow != 0 {
		p.insertAfter = hwndBottom
		p.flags &^= swpNoZOrder
	}
}

// installDevBackground turns background mode on when the environment asks for it. It must
// run on the UI thread before the first window is created; init below does that, since
// package initialisation runs on the main thread and Fyne keeps its UI there.
//
// GLFW resets the window hints when it starts, so it is started here first; Fyne's own
// glfw.Init later does nothing and the hint stays.
func installDevBackground() bool {
	if os.Getenv(devBackgroundEnv) != "1" || devBackgroundHook != 0 {
		return devBackgroundHook != 0
	}
	if err := glfw.Init(); err != nil {
		utilLog.Printf("Background mode: cannot start GLFW: %v", err)
		return false
	}
	glfw.WindowHint(glfw.FocusOnShow, glfw.False)
	h, err := hookBackgroundWindows()
	if err != nil {
		utilLog.Printf("Background mode: cannot install the window hook: %v", err)
		return false
	}
	devBackgroundHook = h
	utilLog.Printf("Background mode (%s=1): windows open behind others and do not take the focus", devBackgroundEnv)
	return true
}

var devBackgroundCBTCallback = syscall.NewCallback(devBackgroundCBT)

// hookBackgroundWindows makes the windows the calling thread creates from now on open
// behind others (see keepAtBottom). The thread must stay locked to its goroutine.
func hookBackgroundWindows() (uintptr, error) {
	h, _, err := procSetWindowsHook.Call(whCBT, devBackgroundCBTCallback, 0, uintptr(windows.GetCurrentThreadId()))
	if h == 0 {
		return 0, err
	}
	return h, nil
}

func init() { installDevBackground() }

// devBackground tells whether background mode is on.
func devBackground() bool { return devBackgroundHook != 0 }
