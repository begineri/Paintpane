//go:build windows

package ui

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"syscall"
	"time"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/systray"

	"github.com/dixieflatline76/Spice/v2/pkg/sysinfo"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
	utilLog "github.com/dixieflatline76/Spice/v2/util/log"
)

// The app-drawn tray menu on Windows: two borderless Fyne windows (the menu and
// one submenu) that the app places next to the tray icon with Win32 calls.
// They are tool windows (no taskbar button), stay on top, and close on Esc, on
// a click outside them or when another window takes the focus.
//
// The windows exist only while they are needed: a click on the tray icon
// creates the menu window, the first submenu creates the submenu window, and
// trayReleaseAfter after the menu closes both are closed again. Each one holds
// an OpenGL context; kept hidden all the time, the two cost about 70 MB of
// memory and 70 threads (docs/lightweight-eval.md, B1). Creating one takes
// about half a second, so the submenu window, which few clicks need, is not
// made with the menu.

var (
	trayUser32               = syscall.NewLazyDLL("user32.dll")
	trayDwmapi               = syscall.NewLazyDLL("dwmapi.dll")
	procTrayGetCursorPos     = trayUser32.NewProc("GetCursorPos")
	procTrayMonitorFromPoint = trayUser32.NewProc("MonitorFromPoint")
	procTrayGetMonitorInfo   = trayUser32.NewProc("GetMonitorInfoW")
	procTraySetWindowPos     = trayUser32.NewProc("SetWindowPos")
	procTrayGetWindowLongPtr = trayUser32.NewProc("GetWindowLongPtrW")
	procTraySetWindowLongPtr = trayUser32.NewProc("SetWindowLongPtrW")
	procTrayGetForeground    = trayUser32.NewProc("GetForegroundWindow")
	procTraySetForeground    = trayUser32.NewProc("SetForegroundWindow")
	procTrayGetWindowRect    = trayUser32.NewProc("GetWindowRect")
	procTrayIsWindowVisible  = trayUser32.NewProc("IsWindowVisible")
	procTrayGetAsyncKeyState = trayUser32.NewProc("GetAsyncKeyState")
	procTrayDwmSetWindowAttr = trayDwmapi.NewProc("DwmSetWindowAttribute")
	procTrayFindWindowEx     = trayUser32.NewProc("FindWindowExW")
	procTrayGetWindowPID     = trayUser32.NewProc("GetWindowThreadProcessId")
	procTrayNotifyIconRect   = syscall.NewLazyDLL("shell32.dll").NewProc("Shell_NotifyIconGetRect")
	trayGWLExStyle           = ^uintptr(19) // GWL_EXSTYLE (-20)
	trayHWNDTopmost          = ^uintptr(0)  // HWND_TOPMOST (-1)

	// trayReleaseAfter is how long the closed menu keeps its windows. Opening it
	// again within this time is instant; after it, the windows are closed and the
	// next click creates them again.
	trayReleaseAfter = time.Minute
)

const (
	wsExToolWindow        = 0x00000080
	wsExAppWindow         = 0x00040000
	swpNoSize             = 0x0001
	swpNoActivate         = 0x0010
	monitorDefaultNearest = 2
	dwmwaCornerPreference = 33
	dwmwaBorderColor      = 34
	dwmwcpRound           = 2
)

type winPoint struct{ X, Y int32 }
type winRect struct{ L, T, R, B int32 }
type winMonitorInfo struct {
	Size    uint32
	Monitor winRect
	Work    winRect
	Flags   uint32
}

func trayCursorPos() winPoint {
	var pt winPoint
	procTrayGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	return pt
}

func packPoint(pt winPoint) uintptr { return uintptr(uint32(pt.X)) | uintptr(uint32(pt.Y))<<32 }

// trayWorkArea is the work area (the screen without the taskbar) of the
// monitor that holds pt.
func trayWorkArea(pt winPoint) winRect {
	mon, _, _ := procTrayMonitorFromPoint.Call(packPoint(pt), monitorDefaultNearest)
	mi := winMonitorInfo{Size: uint32(unsafe.Sizeof(winMonitorInfo{}))}
	if r, _, _ := procTrayGetMonitorInfo.Call(mon, uintptr(unsafe.Pointer(&mi))); r == 0 {
		return winRect{L: pt.X - 4000, T: pt.Y - 4000, R: pt.X, B: pt.Y}
	}
	return mi.Work
}

func (r winRect) empty() bool          { return r.R <= r.L || r.B <= r.T }
func (r winRect) center() winPoint     { return winPoint{(r.L + r.R) / 2, (r.T + r.B) / 2} }
func (r winRect) has(pt winPoint) bool { return pt.X >= r.L && pt.X < r.R && pt.Y >= r.T && pt.Y < r.B }

// trayAnchor is the point the menu opens from, as Windows' own tray menus do
// (TrackPopupMenu at the click): the mouse pointer when it is on the tray
// icon, else the icon's centre (opened from the keyboard), else the pointer.
type trayAnchor struct {
	pt     winPoint
	icon   winRect // the tray icon, when Windows reports it (debug API)
	source string  // "cursor", "icon" or "point" (debug API)
}

// clampSpan keeps a span of length n starting at v within [lo, hi]; when it
// does not fit, it starts at lo.
func clampSpan(v, n, lo, hi int32) int32 {
	return max(lo, min(v, hi-n))
}

// trayMenuPos is where a w×h menu opening from pt goes, by the rule of the
// system's own tray menus (TrackPopupMenu): its top-left corner at pt; to the
// left of pt (its right edge at pt) when there is no room on the right, and
// above pt (its bottom at pt) when there is no room below. With the tray at
// the bottom of the screen that puts the menu's bottom-left corner at the
// click. It stays inside the work area.
func trayMenuPos(pt winPoint, work winRect, w, h int32) (x, y int32) {
	x, y = pt.X, pt.Y
	if x+w > work.R {
		x = pt.X - w
	}
	if y+h > work.B {
		y = pt.Y - h
	}
	return clampSpan(x, w, work.L, work.R), clampSpan(y, h, work.T, work.B)
}

// traySubPos is where a sw×sh submenu goes beside the menu rect m: to the
// right, gap away, or to the left when it does not fit on the right; its top at
// top, both kept inside the work area.
func traySubPos(m, work winRect, sw, sh, gap, top int32) (x, y int32) {
	x = m.R + gap
	if x+sw > work.R {
		x = m.L - gap - sw
		if x < work.L { // no room on either side: the side with more of it
			if work.R-m.R >= m.L-work.L {
				x = m.R + gap
			}
			x = clampSpan(x, sw, work.L, work.R)
		}
	}
	return x, clampSpan(top, sh, work.T, work.B)
}

// trayNotifyWnd is the hidden window that owns the app's tray icon (fyne.io/systray
// creates it with the class "SystrayClass" and adds the icon with ID 100).
func trayNotifyWnd() uintptr {
	class, _ := syscall.UTF16PtrFromString("SystrayClass")
	pid := uint32(syscall.Getpid())
	var h uintptr
	for i := 0; i < 64; i++ {
		h, _, _ = procTrayFindWindowEx.Call(0, h, uintptr(unsafe.Pointer(class)), 0)
		if h == 0 {
			return 0
		}
		var p uint32
		procTrayGetWindowPID.Call(h, uintptr(unsafe.Pointer(&p)))
		if p == pid {
			return h
		}
	}
	return 0
}

const trayIconID = 100 // fyne.io/systray's NOTIFYICONDATA.uID

// trayIconRect is the tray icon's place on the screen, also when it is in the
// hidden-icons panel.
func trayIconRect() (winRect, bool) {
	wnd := trayNotifyWnd()
	if wnd == 0 || procTrayNotifyIconRect.Find() != nil {
		return winRect{}, false
	}
	id := struct {
		Size uint32
		Wnd  uintptr
		ID   uint32
		GUID [16]byte
	}{Wnd: wnd, ID: trayIconID}
	id.Size = uint32(unsafe.Sizeof(id))
	var r winRect
	if hr, _, _ := procTrayNotifyIconRect.Call(uintptr(unsafe.Pointer(&id)), uintptr(unsafe.Pointer(&r))); hr != 0 || r.empty() {
		return winRect{}, false
	}
	return r, true
}

// trayClickAnchor is where a click on the tray icon (or Enter on it) opens the
// menu: see trayAnchor.
func trayClickAnchor() trayAnchor {
	cur := trayCursorPos()
	icon, ok := trayIconRect()
	switch {
	case !ok:
		return trayAnchor{pt: cur, source: "cursor"}
	case icon.has(cur):
		return trayAnchor{pt: cur, icon: icon, source: "cursor"}
	default:
		return trayAnchor{pt: icon.center(), icon: icon, source: "icon"}
	}
}

func trayWindowRect(hwnd uintptr) winRect {
	var r winRect
	procTrayGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	return r
}

func trayHWND(w fyne.Window) uintptr {
	var h uintptr
	if nw, ok := w.(driver.NativeWindow); ok {
		nw.RunNative(func(ctx any) {
			if c, ok := ctx.(driver.WindowsWindowContext); ok {
				h = c.HWND
			}
		})
	}
	return h
}

func colorRef(c color.Color) uint32 {
	r, g, b, _ := c.RGBA()
	return uint32(r>>8) | uint32(g>>8)<<8 | uint32(b>>8)<<16
}

type winTrayPopup struct {
	sa            *SpiceApp
	items, footer []schema.MenuItemSchema

	main, sub         fyne.Window
	mainView, subView *trayMenuView
	mainH, subH       uintptr
	prepared, failed  bool
	rounded           bool // Windows draws the rounded corners and the outline

	open     bool
	subOpen  bool
	anchor   trayAnchor // what the menu opens from (trayMenuPos)
	closedAt time.Time
	stop     chan struct{}

	created    int // how many times the windows were created (debug API, tests)
	releaseGen int // changes on every open and close, so a pending release can tell it is stale

	// newWindow makes one hidden menu window and returns its handle; nil means
	// a real borderless window (nativeWindow). Tests use headless windows.
	newWindow func(pal trayPalette) (fyne.Window, uintptr, error)
	noWatch   bool // tests: do not poll the focus and the mouse buttons
}

// newTrayPopup returns the app-drawn tray menu, or nil when windows cannot be
// shown (no OpenGL); the native menu is used then.
func newTrayPopup(sa *SpiceApp) trayPopup {
	if !sysinfo.CanCreateWindows() {
		return nil
	}
	if _, ok := fyne.CurrentApp().Driver().(desktop.Driver); !ok {
		return nil
	}
	return &winTrayPopup{sa: sa}
}

// makeTrayPopup is newTrayPopup; tests make a headless menu instead.
var makeTrayPopup = newTrayPopup

// installTrayPopup takes over clicks on the tray icon: both buttons open the
// app-drawn menu instead of the native one.
func (sa *SpiceApp) installTrayPopup() {
	if sa.popup != nil || sa.popupFailed {
		return
	}
	p := makeTrayPopup(sa)
	if p == nil {
		sa.popupFailed = true
		return
	}
	sa.popup = p
	click := func() { fyne.Do(p.toggle) }
	systray.SetOnTapped(click)
	systray.SetOnSecondaryTapped(click)
	sa.listenTrayTheme()
	// The windows are created on the first click (openAt), not now.
}

// fallBackToNative gives the tray back to the native menu.
func (p *winTrayPopup) fallBackToNative(err error) {
	utilLog.Printf("Tray: app-drawn menu unavailable, using the native menu: %v", err)
	p.failed = true
	systray.SetOnTapped(nil)
	systray.SetOnSecondaryTapped(nil)
}

// prepare creates the hidden menu window, unless it exists already. The
// submenu window comes later (ensureSub).
func (p *winTrayPopup) prepare() (err error) {
	if p.prepared || p.failed {
		if p.failed {
			return errors.New("tray menu window failed earlier")
		}
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
		if err != nil {
			if p.main != nil {
				p.main.Close()
			}
			p.main, p.mainH = nil, 0
			p.fallBackToNative(err)
		}
	}()
	if p.main, p.mainH, err = p.makeWindow(); err != nil {
		return err
	}
	p.main.SetCloseIntercept(p.close)
	p.main.Canvas().SetOnTypedKey(func(e *fyne.KeyEvent) { p.key(e.Name, false) })
	p.prepared = true
	p.created++
	return nil
}

// ensureSub creates the hidden submenu window, unless it exists already. It
// reports whether there is one.
func (p *winTrayPopup) ensureSub() (ok bool) {
	if p.sub != nil {
		return true
	}
	defer func() {
		if r := recover(); r != nil {
			utilLog.Printf("Tray: cannot create the submenu window: %v", r)
			ok = false
		}
	}()
	w, h, err := p.makeWindow()
	if err != nil {
		utilLog.Printf("Tray: cannot create the submenu window: %v", err)
		return false
	}
	w.SetCloseIntercept(p.close)
	w.Canvas().SetOnTypedKey(func(e *fyne.KeyEvent) { p.key(e.Name, true) })
	p.sub, p.subH = w, h
	p.created++
	// Showing the new window to create it may have taken the focus from the
	// menu: give it back, and watch the new window too. The old watcher's
	// pending close, if any, is dropped with it.
	procTraySetForeground.Call(p.mainH)
	if p.open {
		p.watch()
	}
	return true
}

func (p *winTrayPopup) makeWindow() (fyne.Window, uintptr, error) {
	mk := p.newWindow
	if mk == nil {
		mk = p.nativeWindow
	}
	return mk(currentTrayPalette())
}

// nativeWindow creates one hidden borderless tool window with rounded corners.
func (p *winTrayPopup) nativeWindow(pal trayPalette) (fyne.Window, uintptr, error) {
	drv, ok := fyne.CurrentApp().Driver().(desktop.Driver)
	if !ok {
		return nil, 0, errors.New("no desktop driver")
	}
	w := drv.CreateSplashWindow()
	w.SetContent(canvas.NewRectangle(color.Transparent))
	w.Resize(fyne.NewSize(1, 1))
	w.Show() // creates the native window; hidden again at once
	w.Hide()
	h := trayHWND(w)
	if h == 0 {
		w.Close()
		return nil, 0, errors.New("no window handle")
	}
	ex, _, _ := procTrayGetWindowLongPtr.Call(h, trayGWLExStyle)
	procTraySetWindowLongPtr.Call(h, trayGWLExStyle, (ex|wsExToolWindow)&^wsExAppWindow)
	pref := uint32(dwmwcpRound)
	r, _, _ := procTrayDwmSetWindowAttr.Call(h, dwmwaCornerPreference, uintptr(unsafe.Pointer(&pref)), 4)
	p.rounded = r == 0
	p.setBorderColor(h, pal)
	return w, h, nil
}

// release closes the windows of a closed menu; the next open creates them again.
// Fyne keeps running without them: its system tray has a hidden window of its own.
func (p *winTrayPopup) release() {
	if !p.prepared || p.open {
		return
	}
	p.main.Close()
	if p.sub != nil {
		p.sub.Close()
	}
	p.main, p.sub = nil, nil
	p.mainH, p.subH = 0, 0
	p.mainView, p.subView = nil, nil
	p.prepared = false
}

// releaseLater releases the windows trayReleaseAfter from now, unless the menu
// has been opened (or closed again) since.
func (p *winTrayPopup) releaseLater() {
	p.releaseGen++
	gen := p.releaseGen
	time.AfterFunc(trayReleaseAfter, func() {
		onUIThread(func() {
			if p.releaseGen == gen {
				p.release()
			}
		})
	})
}

func (p *winTrayPopup) setBorderColor(h uintptr, pal trayPalette) {
	c := colorRef(pal.sep)
	procTrayDwmSetWindowAttr.Call(h, dwmwaBorderColor, uintptr(unsafe.Pointer(&c)), 4)
}

func (p *winTrayPopup) setMenu(items, footer []schema.MenuItemSchema) {
	p.items, p.footer = items, footer
	if !p.open || p.mainView == nil {
		return
	}
	p.hideSub()
	p.mainView.setItems(items, footer)
	p.main.Resize(p.mainView.MinSize())
	p.place()
}

func (p *winTrayPopup) refreshTheme() {
	if !p.prepared {
		return
	}
	pal := currentTrayPalette()
	p.setBorderColor(p.mainH, pal)
	if p.sub != nil {
		p.setBorderColor(p.subH, pal)
	}
	if p.mainView != nil {
		p.mainView.refreshPalette()
	}
	if p.subView != nil {
		p.subView.refreshPalette()
	}
}

func (p *winTrayPopup) toggle() {
	if p.open {
		p.close()
		return
	}
	// The press that closed the menu (on the tray icon) is followed by this click.
	if time.Since(p.closedAt) < 400*time.Millisecond {
		return
	}
	p.openAt(trayClickAnchor())
}

func (p *winTrayPopup) newView(items, footer []schema.MenuItemSchema, minWidth float32) *trayMenuView {
	v := newTrayMenuView(items, footer, minWidth)
	v.drawBorder = !p.rounded
	v.iconFor = p.sa.trayIconResource
	v.onUse = p.use
	return v
}

func (p *winTrayPopup) openAt(a trayAnchor) {
	if err := p.prepare(); err != nil {
		return
	}
	p.releaseGen++ // keep the windows
	p.anchor = a
	p.mainView = p.newView(p.items, p.footer, trayMinWidth)
	p.mainView.onSubmenu = p.showSub
	p.main.SetContent(p.mainView)
	p.main.Resize(p.mainView.MinSize())
	p.place()
	p.main.Show()
	procTraySetForeground.Call(p.mainH)
	p.open = true
	p.watch()
	// A monitor with another scale resizes the window once it is there.
	time.AfterFunc(80*time.Millisecond, func() {
		onUIThread(func() {
			if p.open {
				p.place()
			}
		})
	})
}

// place puts the menu at its anchor the way the system's tray menus open
// (trayMenuPos), inside the work area of the anchor's monitor.
func (p *winTrayPopup) place() {
	r := trayWindowRect(p.mainH)
	x, y := trayMenuPos(p.anchor.pt, trayWorkArea(p.anchor.pt), r.R-r.L, r.B-r.T)
	procTraySetWindowPos.Call(p.mainH, trayHWNDTopmost, uintptr(x), uintptr(y), 0, 0, swpNoSize|swpNoActivate)
}

// showSub opens a row's submenu to the right of the menu (to the left when
// there is no room), its first item level with the row.
func (p *winTrayPopup) showSub(it *schema.MenuItemSchema, top float32, byKey bool) {
	if it == nil {
		p.hideSub()
		procTraySetForeground.Call(p.mainH)
		return
	}
	if !p.ensureSub() {
		return
	}
	p.subView = p.newView(it.SubMenu.Items, nil, subMenuWidth(it.SubMenu.Items))
	p.sub.SetContent(p.subView)
	p.sub.Resize(p.subView.MinSize())

	mr := trayWindowRect(p.mainH)
	scale := float32(mr.R-mr.L) / p.mainView.MinSize().Width
	sr := trayWindowRect(p.subH)
	sw, sh := sr.R-sr.L, sr.B-sr.T
	gap := int32(traySubGap*scale + 0.5)
	x, y := traySubPos(mr, trayWorkArea(mr.center()), sw, sh, gap, mr.T+int32((top-trayInset)*scale+0.5))
	procTraySetWindowPos.Call(p.subH, trayHWNDTopmost, uintptr(x), uintptr(y), 0, 0, swpNoSize|swpNoActivate)
	p.sub.Show()
	p.subOpen = true
	if byKey {
		procTraySetForeground.Call(p.subH)
		p.subView.highlightFirst()
	} else {
		procTraySetForeground.Call(p.mainH) // the keyboard stays in the menu
	}
}

func (p *winTrayPopup) hideSub() {
	if p.subOpen {
		p.sub.Hide()
		p.subOpen = false
	}
	p.subView = nil
}

func (p *winTrayPopup) close() {
	if !p.open {
		return
	}
	p.hideSub()
	p.main.Hide()
	p.open = false
	p.closedAt = time.Now()
	if p.stop != nil {
		close(p.stop)
		p.stop = nil
	}
	p.releaseLater()
}

// use runs an item: the menu closes first unless the item keeps it open.
func (p *winTrayPopup) use(it schema.MenuItemSchema, keepOpen bool) {
	if !keepOpen {
		p.close()
	}
	if it.Action != nil {
		fyne.Do(it.Action)
	}
}

func (p *winTrayPopup) key(name fyne.KeyName, inSub bool) {
	switch {
	case inSub && (name == fyne.KeyEscape || name == fyne.KeyRight || name == fyne.KeyLeft):
		if p.mainView != nil {
			p.mainView.closeSubmenu()
		}
	case name == fyne.KeyEscape:
		p.close()
	case inSub && p.subView != nil:
		p.subView.typedKey(name)
	case p.mainView != nil:
		p.mainView.typedKey(name)
	}
}

// watch closes the menu when another window takes the focus or the mouse is
// pressed outside the menu and its submenu.
func (p *winTrayPopup) watch() {
	if p.stop != nil {
		close(p.stop)
	}
	stop := make(chan struct{})
	p.stop = stop
	if p.noWatch {
		return
	}
	mainH, subH := p.mainH, p.subH
	go func() {
		t := time.NewTicker(30 * time.Millisecond)
		defer t.Stop()
		hadFocus := false
		wasDown := false
		for {
			select {
			case <-stop:
				return
			case <-t.C:
			}
			fg, _, _ := procTrayGetForeground.Call()
			ours := fg == mainH || (subH != 0 && fg == subH)
			if ours {
				hadFocus = true
			}
			down := false
			for _, vk := range []uintptr{0x01, 0x02, 0x04} { // left, right, middle button
				if s, _, _ := procTrayGetAsyncKeyState.Call(vk); s&0x8000 != 0 {
					down = true
				}
			}
			outside := false
			if down && !wasDown {
				pt := trayCursorPos()
				outside = !inWindow(mainH, pt) && !inWindow(subH, pt)
			}
			wasDown = down
			if outside || (hadFocus && !ours) {
				onUIThread(func() {
					select {
					case <-stop:
					default:
						p.close()
					}
				})
				return
			}
		}
	}()
}

func inWindow(h uintptr, pt winPoint) bool {
	if v, _, _ := procTrayIsWindowVisible.Call(h); v == 0 {
		return false
	}
	r := trayWindowRect(h)
	return pt.X >= r.L && pt.X < r.R && pt.Y >= r.T && pt.Y < r.B
}

// --- dev-build debug API ---

func (p *winTrayPopup) debugOpen(x, y int, at bool) error {
	if p.open {
		p.close()
	}
	a := trayClickAnchor() // as a click on the icon (or, with the pointer elsewhere, Enter on it)
	if at {
		a = trayAnchor{pt: winPoint{int32(x), int32(y)}, source: "point"} // as a click at x,y
	}
	p.closedAt = time.Time{}
	p.openAt(a)
	if !p.open {
		return errors.New("the app-drawn tray menu could not be shown")
	}
	return nil
}

func (p *winTrayPopup) debugState() map[string]any {
	st := map[string]any{"open": p.open, "prepared": p.prepared, "failed": p.failed, "rounded": p.rounded,
		"created": p.created, "subWindow": p.sub != nil, "releaseAfter": trayReleaseAfter.String()}
	if !p.prepared {
		return st
	}
	fg, _, _ := procTrayGetForeground.Call()
	rect := func(h uintptr) []int32 { r := trayWindowRect(h); return []int32{r.L, r.T, r.R - r.L, r.B - r.T} }
	ex, _, _ := procTrayGetWindowLongPtr.Call(p.mainH, trayGWLExStyle)
	ltrb := func(r winRect) []int32 { return []int32{r.L, r.T, r.R, r.B} }
	st["anchor"] = map[string]any{"source": p.anchor.source, "point": []int32{p.anchor.pt.X, p.anchor.pt.Y}, "icon": ltrb(p.anchor.icon)}
	st["workArea"] = ltrb(trayWorkArea(p.anchor.pt))
	st["rect"] = rect(p.mainH)
	st["foreground"] = map[bool]string{true: "menu", false: "other"}[fg == p.mainH]
	if p.sub != nil && fg == p.subH {
		st["foreground"] = "submenu"
	}
	st["toolWindow"] = ex&wsExToolWindow != 0
	st["topmost"] = ex&0x8 != 0 // WS_EX_TOPMOST
	cur := trayCursorPos()
	st["cursor"] = []int32{cur.X, cur.Y}
	if p.mainView != nil {
		p.mainView.mu.Lock()
		if a := p.mainView.active; a >= 0 && a < len(p.mainView.regions) {
			st["active"] = p.mainView.regions[a].item.Label
		}
		st["hasMouse"] = p.mainView.hasMouse
		p.mainView.mu.Unlock()
		st["items"] = p.mainView.labels()
		st["size"] = []float32{p.mainView.MinSize().Width, p.mainView.MinSize().Height}
	}
	if p.subOpen && p.subView != nil {
		st["subRect"] = rect(p.subH)
		st["subItems"] = p.subView.labels()
	}
	return st
}

// debugCapture renders the menu, with an open submenu beside it, as drawn.
func (p *winTrayPopup) debugCapture() image.Image {
	if !p.open {
		return nil
	}
	mainImg := p.main.Canvas().Capture()
	if !p.subOpen {
		return mainImg
	}
	subImg := p.sub.Canvas().Capture()
	mr, sr := trayWindowRect(p.mainH), trayWindowRect(p.subH)
	left, top := min(mr.L, sr.L), min(mr.T, sr.T)
	right, bottom := max(mr.L+int32(mainImg.Bounds().Dx()), sr.L+int32(subImg.Bounds().Dx())),
		max(mr.T+int32(mainImg.Bounds().Dy()), sr.T+int32(subImg.Bounds().Dy()))
	out := image.NewRGBA(image.Rect(0, 0, int(right-left), int(bottom-top)))
	draw.Draw(out, mainImg.Bounds().Add(image.Pt(int(mr.L-left), int(mr.T-top))), mainImg, image.Point{}, draw.Src)
	draw.Draw(out, subImg.Bounds().Add(image.Pt(int(sr.L-left), int(sr.T-top))), subImg, image.Point{}, draw.Src)
	return out
}

func (p *winTrayPopup) debugView(sub bool) (*trayMenuView, error) {
	v := p.mainView
	if sub {
		v = p.subView
	}
	if !p.open || v == nil {
		return nil, errors.New("menu not open")
	}
	return v, nil
}

func (p *winTrayPopup) debugUse(label string, sub bool) error {
	v, err := p.debugView(sub)
	if err != nil {
		return err
	}
	i := v.regionByLabel(label)
	if i < 0 {
		return fmt.Errorf("no item %q", label)
	}
	v.use(i, false)
	return nil
}

func (p *winTrayPopup) debugHover(label string) error {
	v, err := p.debugView(false)
	if err != nil {
		return err
	}
	i := v.regionByLabel(label)
	if i < 0 {
		return fmt.Errorf("no item %q", label)
	}
	v.mu.Lock()
	r := v.regions[i]
	v.mu.Unlock()
	v.MouseMoved(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: r.pos.Add(fyne.NewPos(r.size.Width/2, r.size.Height/2))}})
	return nil
}

func (p *winTrayPopup) debugKey(name fyne.KeyName, sub bool) { p.key(name, sub) }
