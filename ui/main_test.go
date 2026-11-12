package ui

import (
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/zalando/go-keyring"
)

// The headless driver runs fyne.Do on the calling goroutine, so a widget changed from a background
// goroutine would race with layout on the test goroutine. Tests queue such changes instead and
// the test goroutine applies them (flushUI) whenever it looks at the window.
var (
	uiQueueMu sync.Mutex
	uiQueue   []func()
)

func queueForTest(f func()) {
	uiQueueMu.Lock()
	uiQueue = append(uiQueue, f)
	uiQueueMu.Unlock()
}

// flushUI applies the queued UI changes on the calling (test) goroutine.
func flushUI() {
	uiQueueMu.Lock()
	pending := uiQueue
	uiQueue = nil
	uiQueueMu.Unlock()
	for _, f := range pending {
		f()
	}
}

// TestMain pins the language because tests assert English UI strings,
// and i18n otherwise follows the OS locale of whoever runs them.
//
// It also points LOCALAPPDATA (Windows) and XDG_CACHE_HOME (Linux) at a temporary
// folder: tests that initialise the wallpaper plugin create its working
// directories under the user cache dir, which must not be the user's real one.
func TestMain(m *testing.M) {
	i18n.SetLanguage("en")
	onUIThread = queueForTest

	// Config reads the wallhaven and Pexels API keys from the OS credential store. Without a mock,
	// a test run (and the screenshots) would show the keys of whoever runs it.
	keyring.MockInit()

	dir, err := os.MkdirTemp("", "paintpane-ui-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "cannot create temp cache dir:", err)
		os.Exit(1)
	}
	if os.Getenv("SCREENSHOT_DIR") == "" { // the screenshot run sets its own
		_ = os.Setenv("LOCALAPPDATA", dir)
		_ = os.Setenv("XDG_CACHE_HOME", dir)
	}

	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
