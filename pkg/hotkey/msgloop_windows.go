//go:build windows

package hotkey

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/dixieflatline76/Spice/v2/util/log"
	"golang.org/x/sys/windows"
)

// All global hotkeys share one OS thread that blocks in GetMessage. RegisterHotKey with a nil
// window posts WM_HOTKEY to the registering thread's queue, so that thread needs no window and no
// timer: it sleeps until a hotkey is pressed or another goroutine asks it to (un)register one.
//
// golang.design/x/hotkey, which this replaces on Windows, gives every hotkey its own locked thread
// that wakes every 10 ms to call PeekMessage. With 7 to 22 hotkeys that kept the idle app at about
// 5% of a core (docs/test-report-v1.0.md 14.6).

var (
	procRegisterHotKey     = user32.NewProc("RegisterHotKey")
	procUnregisterHotKey   = user32.NewProc("UnregisterHotKey")
	procGetMessageW        = user32.NewProc("GetMessageW")
	procPeekMessageW       = user32.NewProc("PeekMessageW")
	procPostThreadMessageW = user32.NewProc("PostThreadMessageW")
)

const (
	wmQuit   = 0x0012
	wmHotkey = 0x0312
	wmUser   = 0x0400
	// wmRunCommands wakes the loop to run the closures queued by do.
	wmRunCommands = 0x8000 + 1 // WM_APP + 1

	pmNoRemove = 0x0000

	// modNoRepeat stops keyboard auto-repeat from firing the hotkey again while it is held, so one
	// press is one action. The old library effectively queued the repeats and replayed them as a
	// burst after the key was released.
	modNoRepeat = 0x4000

	// Application hotkey ids must be in 0x0000-0xBFFF.
	maxHotkeyID = 0xBFFF

	// bindingQueue is how many presses of one hotkey can wait while its action is still running.
	// Further presses are dropped rather than blocking the message loop.
	bindingQueue = 8
)

// winMSG is the Win32 MSG structure.
type winMSG struct {
	HWnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       struct{ X, Y int32 }
	LPrivate uint32
}

// binding is one registered hotkey. Its action runs on its own goroutine, so a slow action never
// holds up the message loop or another hotkey, and presses of the same hotkey run in order.
type binding struct {
	events chan struct{}
}

func newBinding(name string, action func()) *binding {
	b := &binding{events: make(chan struct{}, bindingQueue)}
	go func() {
		for range b.events {
			runAction(name, action)
		}
	}()
	return b
}

func runAction(name string, action func()) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Hotkey] PANIC in action for %s: %v", name, r)
		}
	}()
	action()
}

// msgLoop owns the hotkey thread. The zero value is ready to use; the thread starts on the first
// register call and runs until close.
type msgLoop struct {
	startMu sync.Mutex
	started bool
	closed  bool
	tid     uint32
	exited  chan struct{}

	queueMu sync.Mutex
	queue   []func()

	// Only touched on the loop thread.
	bindings map[uintptr]*binding

	// wakeups counts how often GetMessage returned; tests use it to prove the loop does not poll.
	wakeups atomic.Uint64
}

var errLoopClosed = errors.New("hotkey message loop is closed")

// ensureStarted starts the loop thread once and returns its thread id.
func (l *msgLoop) ensureStarted() (uint32, error) {
	l.startMu.Lock()
	defer l.startMu.Unlock()
	if l.closed {
		return 0, errLoopClosed
	}
	if !l.started {
		ready := make(chan uint32)
		l.exited = make(chan struct{})
		l.bindings = make(map[uintptr]*binding)
		go l.run(ready)
		l.tid = <-ready
		l.started = true
	}
	return l.tid, nil
}

func (l *msgLoop) run(ready chan<- uint32) {
	// Never unlocked: when this goroutine returns, Go ends the thread with it, and Windows drops
	// any hotkey still registered to that thread.
	runtime.LockOSThread()
	defer close(l.exited)

	var msg winMSG
	// Make Windows create this thread's message queue before anyone posts to it.
	procPeekMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, wmUser, wmUser, pmNoRemove)
	ready <- windows.GetCurrentThreadId()

	for {
		r, _, err := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		l.wakeups.Add(1)
		switch int32(r) {
		case 0: // WM_QUIT
			l.runQueued()
			l.unregisterAll()
			return
		case -1:
			log.Printf("[Hotkey] GetMessage failed, stopping the hotkey loop: %v", err)
			l.runQueued()
			l.unregisterAll()
			return
		}
		switch msg.Message {
		case wmHotkey:
			if b := l.bindings[msg.WParam]; b != nil {
				select {
				case b.events <- struct{}{}:
				default:
					log.Debugf("[Hotkey] Action for hotkey %d still busy; press dropped", msg.WParam)
				}
			}
		case wmRunCommands:
			l.runQueued()
		}
	}
}

func (l *msgLoop) runQueued() {
	l.queueMu.Lock()
	q := l.queue
	l.queue = nil
	l.queueMu.Unlock()
	for _, f := range q {
		f()
	}
}

func (l *msgLoop) unregisterAll() {
	for id, b := range l.bindings {
		procUnregisterHotKey.Call(0, id)
		close(b.events)
		delete(l.bindings, id)
	}
}

// do runs f on the loop thread and waits for it.
func (l *msgLoop) do(f func()) error {
	tid, err := l.ensureStarted()
	if err != nil {
		return err
	}
	done := make(chan struct{})
	l.queueMu.Lock()
	l.queue = append(l.queue, func() { f(); close(done) })
	l.queueMu.Unlock()
	if r, _, err := procPostThreadMessageW.Call(uintptr(tid), wmRunCommands, 0, 0); r == 0 {
		// The closure stays queued and runs on the loop's next wakeup; do not wait for it.
		return fmt.Errorf("wake hotkey loop: %w", err)
	}
	select {
	case <-done:
		return nil
	case <-l.exited:
		return errLoopClosed
	}
}

// register registers mods+key as a system-wide hotkey that calls action on each press.
// mods are MOD_ALT/MOD_CONTROL/... bits; key is a virtual-key code.
func (l *msgLoop) register(mods, key uint32, name string, action func()) (uintptr, error) {
	var id uintptr
	var regErr error
	err := l.do(func() {
		for id = 1; id <= maxHotkeyID && l.bindings[id] != nil; id++ {
		}
		if id > maxHotkeyID {
			regErr = errors.New("no free hotkey id")
			return
		}
		r, _, e := procRegisterHotKey.Call(0, id, uintptr(mods|modNoRepeat), uintptr(key))
		if r == 0 {
			regErr = e
			return
		}
		l.bindings[id] = newBinding(name, action)
	})
	if err != nil {
		return 0, err
	}
	if regErr != nil {
		return 0, regErr
	}
	return id, nil
}

// unregister releases the hotkey with this id. Presses already queued for it may still run.
func (l *msgLoop) unregister(id uintptr) error {
	var unregErr error
	err := l.do(func() {
		b := l.bindings[id]
		if b == nil {
			unregErr = fmt.Errorf("hotkey %d is not registered", id)
			return
		}
		if r, _, e := procUnregisterHotKey.Call(0, id); r == 0 {
			unregErr = e
		}
		close(b.events)
		delete(l.bindings, id)
	})
	if err != nil {
		return err
	}
	return unregErr
}

// close unregisters every hotkey and ends the loop thread. The loop cannot be used afterwards.
func (l *msgLoop) close() {
	l.startMu.Lock()
	if l.closed {
		l.startMu.Unlock()
		return
	}
	l.closed = true
	started, tid, exited := l.started, l.tid, l.exited
	l.startMu.Unlock()
	if !started {
		return
	}
	procPostThreadMessageW.Call(uintptr(tid), wmQuit, 0, 0)
	<-exited
}
