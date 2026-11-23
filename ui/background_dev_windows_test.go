//go:build !release && windows

package ui

import (
	"runtime"
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

var (
	procBgRegisterClass = bgUser32.NewProc("RegisterClassExW")
	procBgCreateWindow  = bgUser32.NewProc("CreateWindowExW")
	procBgDestroyWindow = bgUser32.NewProc("DestroyWindow")
	procBgShowWindow    = bgUser32.NewProc("ShowWindow")
	procBgDefWindowProc = bgUser32.NewProc("DefWindowProcW")
	procBgGetWindow     = bgUser32.NewProc("GetWindow")
	procBgForeground    = bgUser32.NewProc("GetForegroundWindow")
	procBgUnhook        = bgUser32.NewProc("UnhookWindowsHookEx")
)

func TestKeepAtBottom(t *testing.T) {
	const noMove, noSize, noActivate, noZ, show = 0x2, 0x1, swpNoActivateBg, swpNoZOrder, swpShowWindow
	for _, c := range []struct {
		name      string
		after     uintptr
		flags     uint32
		topmost   bool
		wantAfter uintptr
		wantNoZ   bool
	}{
		{"shown without activation (GLFW's SW_SHOWNA)", 0, show | noActivate | noMove | noSize | noZ, false, hwndBottom, false},
		{"raised without activation", 0, noActivate | noMove | noSize, false, hwndBottom, false},
		{"moved without a z-order change", 0, noActivate | noSize | noZ, false, 0, true},
		{"activated (a click on it)", 0, noMove | noSize, false, 0, false},
		{"made topmost (the tray menu)", hwndTopmost, noActivate | noMove | noSize, false, hwndTopmost, false},
		{"a topmost window shown", 0, show | noActivate | noMove | noSize | noZ, true, 0, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			p := windowPos{insertAfter: c.after, flags: c.flags}
			keepAtBottom(&p, c.topmost)
			assert.Equal(t, c.wantAfter, p.insertAfter)
			assert.Equal(t, c.wantNoZ, p.flags&swpNoZOrder != 0)
		})
	}
}

// A window made and shown the way GLFW does it (hidden, then SW_SHOWNA) on a thread with the
// background hook stays below windows shown before it and does not take the focus; without
// the hook a window shown last comes out on top. The test compares only its own windows, never
// the desktop's foreground window, which the person at the PC may change at any moment. The
// windows are tool windows far off the screen, so nothing appears on the desktop or the taskbar.
func TestBackgroundModeKeepsWindowsBehind(t *testing.T) {
	done := make(chan struct{})
	skip := ""
	go func() {
		defer close(done)
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()

		// Two plain windows: the second one, shown last, must come out above the first.
		ref := makeTestWindow(t, "reference")
		defer procBgDestroyWindow.Call(ref)
		plain := makeTestWindow(t, "plain")
		defer procBgDestroyWindow.Call(plain)
		procBgShowWindow.Call(ref, 8) // SW_SHOWNA, as GLFW shows windows
		procBgShowWindow.Call(plain, 8)
		if !isBelow(ref, plain) {
			skip = "a window shown without activation did not come out above the one shown before it, so this desktop cannot show what the hook changes"
			return
		}

		// A window made with the hook and shown last stays below both.
		hook, err := hookBackgroundWindows()
		require.NoError(t, err)
		behind := makeTestWindow(t, "behind")
		procBgUnhook.Call(hook)
		defer procBgDestroyWindow.Call(behind)
		_, hooked := devBackgroundProc[behind]
		assert.True(t, hooked, "the hook subclassed the new window")
		procBgShowWindow.Call(behind, 8)
		assert.True(t, isBelow(behind, ref), "the window shown in background mode is below the windows shown before it")
		assert.True(t, isBelow(behind, plain), "the window shown in background mode is below the windows shown before it")
		fg, _, _ := procBgForeground.Call()
		assert.NotEqual(t, behind, fg, "the window shown in background mode did not take the focus")
	}()
	<-done
	if skip != "" {
		t.Skip(skip)
	}
	assert.Empty(t, devBackgroundProc, "destroyed windows are forgotten")
}

func makeTestWindow(t *testing.T, title string) uintptr {
	t.Helper()
	class, _ := windows.UTF16PtrFromString("PaintpaneBackgroundTest")
	wc := struct {
		size, style            uint32
		proc                   uintptr
		clsExtra, wndExtra     int32
		inst, icon, cur, bg, m uintptr
		name                   *uint16
		iconSm                 uintptr
	}{proc: procBgDefWindowProc.Addr(), name: class}
	wc.size = uint32(unsafe.Sizeof(wc))
	procBgRegisterClass.Call(uintptr(unsafe.Pointer(&wc))) // fails harmlessly the second time
	name, _ := windows.UTF16PtrFromString(title)
	const wsOverlappedWindow, wsExToolWindow = 0x00CF0000, 0x80
	off := int32(-20000)
	h, _, err := procBgCreateWindow.Call(wsExToolWindow, uintptr(unsafe.Pointer(class)), uintptr(unsafe.Pointer(name)),
		wsOverlappedWindow, uintptr(off), uintptr(off), 300, 200, 0, 0, 0, 0)
	require.NotZero(t, h, "CreateWindowEx: %v", err)
	return h
}

// isBelow tells whether a comes after b in the z-order.
func isBelow(a, b uintptr) bool {
	const gwHwndNext = 2
	for w, _, _ := procBgGetWindow.Call(b, gwHwndNext); w != 0; w, _, _ = procBgGetWindow.Call(w, gwHwndNext) {
		if w == a {
			return true
		}
	}
	return false
}
