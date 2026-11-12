//go:build !release

package ui

import (
	"bytes"
	"image"
	"image/png"
	"net/http"
	"strconv"

	"fyne.io/fyne/v2"
)

// trayPopupTarget is implemented by targets that have an app-drawn tray menu.
type trayPopupTarget interface {
	TrayPopup() trayPopupDebug
}

// registerTrayPopupRoutes adds the routes for the app-drawn tray menu, so it can
// be checked without clicking the tray:
//
//	POST /debug/traymenu/open[?x=&y=]   open it as a click on the tray icon would
//	                                    (from the pointer when it is on the icon,
//	                                    else from the icon's centre; with x,y, as a
//	                                    click at x,y)
//	POST /debug/traymenu/close
//	GET  /debug/traymenu                place, focus, window styles and items
//	GET  /debug/traymenu.png            the menu (and an open submenu) as drawn
//	POST /debug/traymenu/use?label=[&sub=1]   use an item of the menu or submenu
//	POST /debug/traymenu/hover?label=   move the mouse onto an item
//	POST /debug/traymenu/key?name=Down[&sub=1]
func registerTrayPopupRoutes(handle func(string, http.HandlerFunc), t debugTarget) {
	popup := func(w http.ResponseWriter) trayPopupDebug {
		pt, ok := t.(trayPopupTarget)
		if !ok {
			http.Error(w, "no app-drawn tray menu", http.StatusNotFound)
			return nil
		}
		p := pt.TrayPopup()
		if p == nil {
			http.Error(w, "no app-drawn tray menu on this platform or display", http.StatusNotFound)
		}
		return p
	}
	onMain := func(fn func()) { fyne.DoAndWait(fn) }
	post := func(w http.ResponseWriter, r *http.Request) trayPopupDebug {
		if !requirePost(w, r) {
			return nil
		}
		return popup(w)
	}

	handle("traymenu", func(w http.ResponseWriter, _ *http.Request) {
		if p := popup(w); p != nil {
			var st map[string]any
			onMain(func() { st = p.debugState() })
			writeJSON(w, st)
		}
	})
	handle("traymenu/open", func(w http.ResponseWriter, r *http.Request) {
		p := post(w, r)
		if p == nil {
			return
		}
		x, errX := strconv.Atoi(r.URL.Query().Get("x"))
		y, errY := strconv.Atoi(r.URL.Query().Get("y"))
		var err error
		onMain(func() { err = p.debugOpen(x, y, errX == nil && errY == nil) })
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		writeJSON(w, map[string]bool{"open": true})
	})
	handle("traymenu/close", func(w http.ResponseWriter, r *http.Request) {
		if p := post(w, r); p != nil {
			onMain(func() { p.debugKey(fyne.KeyEscape, false) })
			writeJSON(w, map[string]bool{"closed": true})
		}
	})
	handle("traymenu.png", func(w http.ResponseWriter, _ *http.Request) {
		p := popup(w)
		if p == nil {
			return
		}
		var img image.Image
		onMain(func() { img = p.debugCapture() })
		if img == nil {
			http.Error(w, "tray menu is not open (POST /debug/traymenu/open first)", http.StatusConflict)
			return
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(buf.Bytes())
	})
	handle("traymenu/use", func(w http.ResponseWriter, r *http.Request) {
		if p := post(w, r); p != nil {
			label := r.URL.Query().Get("label")
			var err error
			onMain(func() { err = p.debugUse(label, r.URL.Query().Get("sub") != "") })
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			writeJSON(w, map[string]string{"used": label})
		}
	})
	handle("traymenu/hover", func(w http.ResponseWriter, r *http.Request) {
		if p := post(w, r); p != nil {
			label := r.URL.Query().Get("label")
			var err error
			onMain(func() { err = p.debugHover(label) })
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			writeJSON(w, map[string]string{"hovered": label})
		}
	})
	handle("traymenu/key", func(w http.ResponseWriter, r *http.Request) {
		if p := post(w, r); p != nil {
			name := fyne.KeyName(r.URL.Query().Get("name"))
			onMain(func() { p.debugKey(name, r.URL.Query().Get("sub") != "") })
			writeJSON(w, map[string]string{"key": string(name)})
		}
	})
}

// TrayPopup returns the app-drawn tray menu, or nil.
func (appDebugTarget) TrayPopup() trayPopupDebug {
	var p trayPopupDebug
	fyne.DoAndWait(func() {
		if d, ok := getInstance().popup.(trayPopupDebug); ok {
			p = d
		}
	})
	return p
}
