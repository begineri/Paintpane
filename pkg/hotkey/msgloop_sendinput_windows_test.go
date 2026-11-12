//go:build windows

package hotkey

import (
	"os"
	"testing"
	"time"
	"unsafe"
)

// keyInput is a Win32 INPUT holding a KEYBDINPUT (40 bytes on 64-bit Windows).
type keyInput struct {
	Type      uint32
	_         uint32
	Vk        uint16
	Scan      uint16
	Flags     uint32
	Time      uint32
	_         uint32
	ExtraInfo uintptr
	_         [8]byte
}

var procSendInput = user32.NewProc("SendInput")

func sendKeys(t *testing.T, keys ...keyInput) {
	t.Helper()
	n, _, err := procSendInput.Call(uintptr(len(keys)), uintptr(unsafe.Pointer(&keys[0])), unsafe.Sizeof(keys[0]))
	if int(n) != len(keys) {
		t.Fatalf("SendInput sent %d of %d: %v", n, len(keys), err)
	}
}

func down(vk uint16) keyInput { return keyInput{Type: 1, Vk: vk} }
func up(vk uint16) keyInput   { return keyInput{Type: 1, Vk: vk, Flags: 2} } // KEYEVENTF_KEYUP

// TestMsgLoopRealKeypress types Ctrl+Alt+Shift+F13 into the real desktop session, so it only runs
// when PAINTPANE_HOTKEY_SENDINPUT=1. It checks that Windows delivers the hotkey to the loop thread
// and that holding the key (repeated key-down events) still counts as one press.
func TestMsgLoopRealKeypress(t *testing.T) {
	if os.Getenv("PAINTPANE_HOTKEY_SENDINPUT") != "1" {
		t.Skip("types into the desktop session; set PAINTPANE_HOTKEY_SENDINPUT=1 to run")
	}
	l := newTestLoop(t)
	fired, action := signal()
	if _, err := l.register(testMods, testKey(0), "real", action); err != nil {
		t.Fatalf("register: %v", err)
	}
	const ctrl, alt, shift, f13 = 0x11, 0x12, 0x10, vkF13
	sendKeys(t, down(ctrl), down(alt), down(shift),
		down(f13), down(f13), down(f13), down(f13), // auto-repeat while held
		up(f13), up(shift), up(alt), up(ctrl))
	waitFor(t, fired, "real hotkey")
	select {
	case <-fired:
		t.Fatal("held key fired the hotkey more than once")
	case <-time.After(300 * time.Millisecond):
	}
}
