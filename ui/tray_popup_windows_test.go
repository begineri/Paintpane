//go:build windows

package ui

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	fynecanvas "fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// headlessTrayPopup is the Windows tray menu with headless windows in place of
// the borderless Win32 ones, and without the focus and mouse polling. It counts
// the windows it makes and how many of them were closed.
type headlessTrayPopup struct {
	*winTrayPopup
	made   []fyne.Window
	closed int
}

func newHeadlessTrayPopup(t *testing.T) *headlessTrayPopup {
	t.Helper()
	h := &headlessTrayPopup{winTrayPopup: &winTrayPopup{sa: &SpiceApp{}, noWatch: true}}
	h.newWindow = func(trayPalette) (fyne.Window, uintptr, error) {
		w := test.NewWindow(fynecanvas.NewRectangle(nil))
		w.SetOnClosed(func() { h.closed++ })
		h.made = append(h.made, w)
		return w, 0, nil // no Win32 window: the Win32 calls on handle 0 do nothing
	}
	items, footer := sampleTrayItems(nil)
	h.setMenu(items, footer)
	t.Cleanup(func() {
		h.close()
		h.releaseGen++ // drop the pending release
		flushUI()
		h.release()
	})
	return h
}

// open opens the menu as a click on the tray icon does.
func (h *headlessTrayPopup) open(t *testing.T) {
	t.Helper()
	h.closedAt = time.Time{}
	h.toggle()
	require.True(t, h.winTrayPopup.open, "the menu did not open")
}

func (h *headlessTrayPopup) shows(t *testing.T, label string) {
	t.Helper()
	require.NotNil(t, h.mainView)
	assert.Same(t, h.mainView, h.main.Content(), "the menu window shows the menu")
	assert.True(t, hasRow(h.mainView, label), "no row %q in %v", label, h.mainView.labels())
}

// hasRow tells whether v has a row whose label starts with label.
func hasRow(v *trayMenuView, label string) bool {
	for _, l := range v.labels() {
		if strings.HasPrefix(l, "row:"+label) {
			return true
		}
	}
	return false
}

// openDisplayMode opens the Display mode submenu, as the keyboard does.
func (h *headlessTrayPopup) openDisplayMode(t *testing.T) {
	t.Helper()
	mode := h.mainView.regionByLabel("Display mode")
	require.GreaterOrEqual(t, mode, 0)
	h.mainView.openSubmenu(mode, true)
	require.NotNil(t, h.subView, "no submenu")
	require.NotNil(t, h.sub)
	assert.Same(t, h.subView, h.sub.Content(), "the submenu window shows the submenu")
	assert.True(t, hasRow(h.subView, "Full artwork"), "submenu: %v", h.subView.labels())
}

func shortTrayRelease(t *testing.T, d time.Duration) {
	t.Helper()
	old := trayReleaseAfter
	trayReleaseAfter = d
	t.Cleanup(func() { trayReleaseAfter = old })
}

// Starting the app creates no menu windows; the first click does (B1 in
// docs/lightweight-eval.md: two hidden windows cost about 60 MB).
func TestTrayPopupCreatesWindowsOnFirstOpen(t *testing.T) {
	var h *headlessTrayPopup
	old := makeTrayPopup
	makeTrayPopup = func(*SpiceApp) trayPopup {
		h = newHeadlessTrayPopup(t)
		return h.winTrayPopup
	}
	t.Cleanup(func() { makeTrayPopup = old })

	sa := &SpiceApp{App: test.NewTempApp(t)}
	sa.installTrayPopup()
	require.NotNil(t, h)
	items, footer := sampleTrayItems(nil)
	sa.popup.setMenu(items, footer) // the tray menu is built at start and rebuilt often
	sa.popup.refreshTheme()
	flushUI()
	assert.Empty(t, h.made, "no window before the first click")
	assert.False(t, h.prepared)

	h.open(t)
	assert.Len(t, h.made, 1, "the menu window only")
	assert.Nil(t, h.sub)
	h.shows(t, "Display mode")

	// The first submenu creates the submenu window.
	h.openDisplayMode(t)
	assert.Len(t, h.made, 2)
	assert.Equal(t, 2, h.created)

	// Closing keeps the windows for a while: opening again is instant.
	h.close()
	assert.False(t, h.winTrayPopup.open)
	assert.Zero(t, h.closed)
	h.open(t)
	h.openDisplayMode(t)
	assert.Len(t, h.made, 2, "the same windows again")
	h.shows(t, "Display mode")
}

// A closed menu closes its windows after trayReleaseAfter, and the next click
// creates new ones, as often as the menu is opened and closed.
func TestTrayPopupReleasesWindowsAfterClose(t *testing.T) {
	shortTrayRelease(t, 30*time.Millisecond)
	h := newHeadlessTrayPopup(t)

	for i := 1; i <= 5; i++ {
		h.open(t)
		assert.Equal(t, 2*(i-1), h.closed, "cycle %d: the earlier windows are closed", i)
		h.shows(t, "Display mode")

		// The submenu still opens, in a new window too.
		assert.Len(t, h.made, 2*i-1, "cycle %d: the menu window only", i)
		h.openDisplayMode(t)

		h.close()
		assert.Equal(t, 2*(i-1), h.closed, "cycle %d: not closed at once", i)
		require.Eventually(t, func() bool { flushUI(); return !h.prepared }, 2*time.Second, 10*time.Millisecond,
			"cycle %d: the windows were not released", i)
		assert.Equal(t, 2*i, h.closed, "cycle %d: both windows closed", i)
		assert.Nil(t, h.main)
		assert.Nil(t, h.sub)
		assert.Nil(t, h.mainView)
	}
	assert.Equal(t, 10, h.created)
}

// Opening the menu again before the release is due keeps the windows; the
// release counts from the last close.
func TestTrayPopupReopenKeepsWindows(t *testing.T) {
	shortTrayRelease(t, 150*time.Millisecond)
	h := newHeadlessTrayPopup(t)

	h.open(t)
	h.close()
	time.Sleep(100 * time.Millisecond)
	h.open(t) // before the release
	time.Sleep(200 * time.Millisecond)
	flushUI() // the first close's release runs now and must do nothing
	assert.True(t, h.prepared, "an open menu keeps its windows")
	assert.Zero(t, h.closed)
	h.shows(t, "Display mode")

	h.close()
	time.Sleep(50 * time.Millisecond)
	flushUI()
	assert.True(t, h.prepared, "released too early")
	require.Eventually(t, func() bool { flushUI(); return !h.prepared }, 2*time.Second, 10*time.Millisecond)
	assert.Equal(t, 1, h.closed)
	assert.Len(t, h.made, 1)
}

// A menu whose windows were released still takes a new menu and a theme
// change, and shows them when it opens.
func TestTrayPopupReleasedTakesUpdates(t *testing.T) {
	shortTrayRelease(t, 10*time.Millisecond)
	h := newHeadlessTrayPopup(t)
	h.open(t)
	h.close()
	require.Eventually(t, func() bool { flushUI(); return !h.prepared }, 2*time.Second, 10*time.Millisecond)

	release, _ := url.Parse("https://example.com/releases/v1.0.1")
	items, footer := sampleTrayItems(nil)
	items = append(items, (&SpiceApp{update: &trayUpdate{version: "v1.0.1", url: release}}).updateMenuItems()...)
	h.setMenu(items, footer)
	h.refreshTheme()
	assert.False(t, h.prepared, "updates do not create windows")
	assert.Len(t, h.made, 1)

	h.open(t)
	h.shows(t, "Update to v1.0.1")
}

// A 1920×1080 screen at 100 % with a 48 px taskbar at the bottom.
var trayTestWork = winRect{0, 0, 1920, 1032}

const trayTestW, trayTestH = 256, 420

// A click in the hidden-icons panel opens the menu as Windows' own tray menus
// do (the Bluetooth icon's, for one): its bottom-left corner at the click, so
// it may cover the top of the panel.
func TestTrayMenuPosHiddenIconsPanel(t *testing.T) {
	click := winPoint{1652, 902} // on the app's icon in the panel, above the taskbar
	x, y := trayMenuPos(click, trayTestWork, trayTestW, trayTestH)
	assert.Equal(t, click.X, x, "the left edge at the click")
	assert.Equal(t, click.Y, y+trayTestH, "the bottom at the click")
}

func TestTrayMenuPosTaskbar(t *testing.T) {
	// A click on an icon pinned to the taskbar, below the work area: above the
	// taskbar, the left edge at the click.
	x, y := trayMenuPos(winPoint{1600, 1056}, trayTestWork, trayTestW, trayTestH)
	assert.Equal(t, int32(1600), x)
	assert.Equal(t, trayTestWork.B-trayTestH, y, "kept inside the work area")

	// An auto-hidden taskbar: the work area is the whole screen, the click is
	// on the shown taskbar.
	screen := winRect{0, 0, 2560, 1600}
	x, y = trayMenuPos(winPoint{1980, 1550}, screen, 432, 474)
	assert.Equal(t, int32(1980), x)
	assert.Equal(t, int32(1550-474), y)

	// A taskbar at the top: the menu opens downwards from the click.
	work := winRect{0, 48, 1920, 1080}
	x, y = trayMenuPos(winPoint{1600, 24}, work, trayTestW, trayTestH)
	assert.Equal(t, int32(1600), x)
	assert.Equal(t, work.T, y)

	// A monitor left of the main one, at 150 %.
	work = winRect{-2880, 0, 0, 1548}
	x, y = trayMenuPos(winPoint{-500, 1400}, work, 384, 630)
	assert.Equal(t, int32(-500), x)
	assert.Equal(t, int32(1400-630), y)
}

func TestTrayMenuPosScreenEdges(t *testing.T) {
	// No room on the right: the right edge at the click.
	x, _ := trayMenuPos(winPoint{1800, 1000}, trayTestWork, trayTestW, trayTestH)
	assert.Equal(t, int32(1800-trayTestW), x)

	// Room below: the top at the click (Windows does the same).
	_, y := trayMenuPos(winPoint{1000, 300}, trayTestWork, trayTestW, trayTestH)
	assert.Equal(t, int32(300), y)

	// Room on neither side or neither way: inside the work area.
	x, y = trayMenuPos(winPoint{100, 500}, winRect{0, 0, 300, 800}, 280, 600)
	assert.Equal(t, int32(0), x)
	assert.Equal(t, int32(0), y)
	_, y = trayMenuPos(winPoint{1000, 1000}, trayTestWork, trayTestW, 1100)
	assert.Equal(t, trayTestWork.T, y, "taller than the work area: from its top")
}

func TestTraySubPos(t *testing.T) {
	menu := winRect{1528, 438, 1784, 858}
	x, y := traySubPos(menu, trayTestWork, 120, 100, 1, 600)
	assert.Equal(t, menu.R+1, x, "to the right by default")
	assert.Equal(t, int32(600), y)

	menu = winRect{1652, 600, 1908, 1020}
	x, _ = traySubPos(menu, trayTestWork, 150, 100, 1, 700)
	assert.Equal(t, menu.L-1-150, x, "to the left when the right has no room")

	_, y = traySubPos(menu, trayTestWork, 150, 100, 1, 1000)
	assert.Equal(t, trayTestWork.B-100, y, "kept inside the work area")

	// No room on either side: inside the work area, on the side with more room.
	x, _ = traySubPos(winRect{100, 0, 1900, 400}, trayTestWork, 150, 100, 1, 0)
	assert.Equal(t, trayTestWork.L, x, "more room on the left")
	x, _ = traySubPos(winRect{20, 0, 1800, 400}, trayTestWork, 150, 100, 1, 0)
	assert.Equal(t, trayTestWork.R-150, x, "more room on the right")
}
