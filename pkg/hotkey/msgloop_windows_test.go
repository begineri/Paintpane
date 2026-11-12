//go:build windows

package hotkey

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// Combos nobody types: Ctrl+Alt+Shift with F13-F24. Registering them really registers a
// system-wide hotkey for the length of the test.
const (
	testMods = 0x1 | 0x2 | 0x4 // MOD_ALT | MOD_CONTROL | MOD_SHIFT
	vkF13    = 0x7C
)

func testKey(i int) uint32 { return vkF13 + uint32(i) } // i in 0..11

func newTestLoop(t *testing.T) *msgLoop {
	t.Helper()
	l := &msgLoop{}
	t.Cleanup(l.close)
	return l
}

// press makes the loop see a WM_HOTKEY for id, as Windows posts it when the keys are pressed.
func press(t *testing.T, l *msgLoop, id uintptr) {
	t.Helper()
	if r, _, err := procPostThreadMessageW.Call(uintptr(l.tid), wmHotkey, id, 0); r == 0 {
		t.Fatalf("PostThreadMessage: %v", err)
	}
}

func waitFor(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

func signal() (chan struct{}, func()) {
	ch := make(chan struct{}, 16)
	return ch, func() { ch <- struct{}{} }
}

func TestMsgLoopRunsActionOnHotkey(t *testing.T) {
	l := newTestLoop(t)
	fired, action := signal()
	id, err := l.register(testMods, testKey(0), "test", action)
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	press(t, l, id)
	waitFor(t, fired, "action")
	press(t, l, id)
	waitFor(t, fired, "second action")
}

func TestMsgLoopRegistersWithWindows(t *testing.T) {
	a := newTestLoop(t)
	b := newTestLoop(t)

	id, err := a.register(testMods, testKey(1), "a", func() {})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	// Windows refuses a combo that is already taken, which proves the first one is registered.
	if _, err := b.register(testMods, testKey(1), "b", func() {}); !errors.Is(err, windows.Errno(1409)) { // ERROR_HOTKEY_ALREADY_REGISTERED
		t.Fatalf("second register of the same combo: got %v, want ERROR_HOTKEY_ALREADY_REGISTERED", err)
	}
	if err := a.unregister(id); err != nil {
		t.Fatalf("unregister: %v", err)
	}
	if _, err := b.register(testMods, testKey(1), "b", func() {}); err != nil {
		t.Fatalf("register after the other loop unregistered: %v", err)
	}
}

func TestMsgLoopDoesNotWakeWhenIdle(t *testing.T) {
	l := newTestLoop(t)
	for i := 0; i < 8; i++ {
		if _, err := l.register(testMods, testKey(i), "idle", func() {}); err != nil {
			t.Fatalf("register %d: %v", i, err)
		}
	}
	before := l.wakeups.Load()
	time.Sleep(500 * time.Millisecond)
	// The old library woke each hotkey's thread every 10 ms: 8 hotkeys would be about 400 here.
	if n := l.wakeups.Load() - before; n > 2 {
		t.Fatalf("idle loop woke %d times in 500 ms; it should block in GetMessage", n)
	}
}

func TestMsgLoopSlowActionDoesNotBlockOthers(t *testing.T) {
	l := newTestLoop(t)
	release := make(chan struct{})
	defer close(release)
	slowStarted, markSlow := signal()
	fastFired, fast := signal()

	slowID, err := l.register(testMods, testKey(2), "slow", func() { markSlow(); <-release })
	if err != nil {
		t.Fatal(err)
	}
	fastID, err := l.register(testMods, testKey(3), "fast", fast)
	if err != nil {
		t.Fatal(err)
	}
	press(t, l, slowID)
	waitFor(t, slowStarted, "slow action to start")
	press(t, l, fastID)
	waitFor(t, fastFired, "fast action while the slow one is still running")
	// Registering still works while an action is blocked.
	if _, err := l.register(testMods, testKey(4), "more", func() {}); err != nil {
		t.Fatalf("register while an action is blocked: %v", err)
	}
}

func TestMsgLoopActionPanicKeepsHotkeyWorking(t *testing.T) {
	l := newTestLoop(t)
	fired, mark := signal()
	var calls atomic.Int32
	id, err := l.register(testMods, testKey(5), "panicky", func() {
		mark()
		if calls.Add(1) == 1 {
			panic("boom")
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	press(t, l, id)
	waitFor(t, fired, "first (panicking) action")
	press(t, l, id)
	waitFor(t, fired, "action after a panic")
}

func TestMsgLoopUnregisterStopsActions(t *testing.T) {
	l := newTestLoop(t)
	fired, action := signal()
	id, err := l.register(testMods, testKey(6), "gone", action)
	if err != nil {
		t.Fatal(err)
	}
	if err := l.unregister(id); err != nil {
		t.Fatalf("unregister: %v", err)
	}
	press(t, l, id) // a WM_HOTKEY that was already queued when the hotkey went away
	// Round-trip through the loop so the press above has been handled.
	if err := l.do(func() {}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-fired:
		t.Fatal("action ran after unregister")
	case <-time.After(100 * time.Millisecond):
	}
	if err := l.unregister(id); err == nil {
		t.Fatal("second unregister succeeded")
	}
}

func TestMsgLoopReusesIDs(t *testing.T) {
	l := newTestLoop(t)
	for i := 0; i < 200; i++ {
		id, err := l.register(testMods, testKey(7), "cycle", func() {})
		if err != nil {
			t.Fatalf("register #%d: %v", i, err)
		}
		if id != 1 {
			t.Fatalf("register #%d got id %d; freed ids should be reused", i, id)
		}
		if err := l.unregister(id); err != nil {
			t.Fatalf("unregister #%d: %v", i, err)
		}
	}
}

func TestMsgLoopCloseReleasesHotkeys(t *testing.T) {
	a := &msgLoop{}
	for i := 8; i < 10; i++ {
		if _, err := a.register(testMods, testKey(i), "closing", func() {}); err != nil {
			t.Fatal(err)
		}
	}
	a.close()
	a.close() // second close is a no-op

	if _, err := a.register(testMods, testKey(8), "after close", func() {}); !errors.Is(err, errLoopClosed) {
		t.Fatalf("register after close: got %v, want errLoopClosed", err)
	}
	b := newTestLoop(t)
	for i := 8; i < 10; i++ {
		if _, err := b.register(testMods, testKey(i), "reuse", func() {}); err != nil {
			t.Fatalf("combo %d still taken after close: %v", i, err)
		}
	}
}

func TestMsgLoopConcurrentRegister(t *testing.T) {
	l := newTestLoop(t)
	var wg sync.WaitGroup
	for g := 0; g < 6; g++ {
		wg.Add(1)
		go func(key uint32) {
			defer wg.Done()
			for i := 0; i < 25; i++ {
				id, err := l.register(testMods|0x8, key, "concurrent", func() {}) // + MOD_WIN
				if err != nil {
					t.Errorf("register: %v", err)
					return
				}
				if err := l.unregister(id); err != nil {
					t.Errorf("unregister: %v", err)
					return
				}
			}
		}(testKey(g))
	}
	wg.Wait()
}
