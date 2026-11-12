//go:build !release

package ui

// Dev-build debug routes for the Tune Image window:
//
//	GET  /debug/tune                     open?, applying?, Reset shown?, window size
//	GET  /debug/tune.png                 the window as drawn
//	GET  /debug/tune/preview.png         the preview image alone, at the size it was rendered
//	POST /debug/tune/drag?d=N            drag the preview N logical pixels along the cropped axis
//	                                     and let go (the picture follows the pointer)
//	POST /debug/tune/mode?i=N            pick this image's display mode: 0 Full artwork,
//	                                     1 Museum frame, 2 Smart crop (GET /debug/tune has "mode")
//	POST /debug/tune/reset               tap Reset
//	POST /debug/tune/close               close the window
//
// The POSTs take &wait=1 to return only when the window has finished applying (the overlay is
// gone), with the time it took in ms.

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"net/http"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
)

func registerTuneDebugRoutes(handle func(string, http.HandlerFunc)) {
	handle("tune", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, tuneDebugState())
	})

	handle("tune.png", func(w http.ResponseWriter, _ *http.Request) {
		var img image.Image
		fyne.DoAndWait(func() {
			if win := getInstance().tuneWindow; win != nil {
				img = win.Canvas().Capture()
			}
		})
		writeDebugPNG(w, img, errors.New("Tune Image is not open"))
	})

	handle("tune/preview.png", func(w http.ResponseWriter, _ *http.Request) {
		var img image.Image
		fyne.DoAndWait(func() {
			if win := getInstance().tuneWindow; win != nil {
				for _, o := range debugObjects(win.Canvas().Content()) {
					if c, ok := o.(*canvas.Image); ok && c.CornerRadius == tunePreviewRadius && c.Image != nil {
						img = c.Image
					}
				}
			}
		})
		writeDebugPNG(w, img, errors.New("Tune Image is not open or shows no picture"))
	})

	handle("tune/drag", func(w http.ResponseWriter, r *http.Request) {
		if !requirePost(w, r) {
			return
		}
		d, err := strconv.ParseFloat(r.URL.Query().Get("d"), 32)
		if err != nil {
			http.Error(w, "d must be a number of logical pixels", http.StatusBadRequest)
			return
		}
		ok := false
		fyne.DoAndWait(func() {
			win := getInstance().tuneWindow
			if win == nil {
				return
			}
			for _, o := range debugObjects(win.Canvas().Content()) {
				if s, isSurface := o.(*dragSurface); isSurface && s.allowed() {
					ev := &fyne.DragEvent{}
					if s.axis == provider.CropVertical {
						ev.Dragged.DY = float32(d)
					} else {
						ev.Dragged.DX = float32(d)
					}
					s.Dragged(ev)
					s.DragEnd()
					ok = true
				}
			}
		})
		if !ok {
			http.Error(w, "Tune Image is not open or its picture cannot be dragged", http.StatusConflict)
			return
		}
		tuneDebugWait(w, r)
	})

	handle("tune/mode", func(w http.ResponseWriter, r *http.Request) {
		if !requirePost(w, r) {
			return
		}
		i, err := strconv.Atoi(r.URL.Query().Get("i"))
		if err != nil || i < 0 || i > 2 {
			http.Error(w, "i must be 0 (Full artwork), 1 (Museum frame) or 2 (Smart crop)", http.StatusBadRequest)
			return
		}
		ok := false
		fyne.DoAndWait(func() {
			win := getInstance().tuneWindow
			if win == nil {
				return
			}
			for _, o := range debugObjects(win.Canvas().Content()) {
				if s, isSeg := o.(*segmented); isSeg {
					s.SetSelectedIndex(i)
					ok = true
				}
			}
		})
		if !ok {
			http.Error(w, "Tune Image is not open or shows no display mode control", http.StatusConflict)
			return
		}
		tuneDebugWait(w, r)
	})
	handle("tune/close", func(w http.ResponseWriter, r *http.Request) {
		if !requirePost(w, r) {
			return
		}
		fyne.DoAndWait(func() {
			if win := getInstance().tuneWindow; win != nil {
				win.Close()
			}
		})
		writeJSON(w, map[string]bool{"closed": true})
	})

	handle("tune/reset", func(w http.ResponseWriter, r *http.Request) {
		if !requirePost(w, r) {
			return
		}
		label := i18n.TMap("Reset", map[string]string{"zh": "恢复", "zh-Hant": "恢復"})
		ok := false
		fyne.DoAndWait(func() {
			win := getInstance().tuneWindow
			if win == nil {
				return
			}
			for _, o := range debugObjects(win.Canvas().Content()) {
				if b, isButton := o.(*widget.Button); isButton && b.Text == label && b.OnTapped != nil {
					b.OnTapped()
					ok = true
					return
				}
			}
		})
		if !ok {
			http.Error(w, "Tune Image is not open or shows no Reset", http.StatusConflict)
			return
		}
		tuneDebugWait(w, r)
	})
}

// tuneDebugWait answers a POST, after the window has finished applying when the request has wait=1.
func tuneDebugWait(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if r.URL.Query().Get("wait") == "1" {
		deadline := start.Add(60 * time.Second)
		for time.Now().Before(deadline) {
			time.Sleep(20 * time.Millisecond)
			if st := tuneDebugState(); st["applying"] != true {
				break
			}
		}
	}
	state := tuneDebugState()
	state["tookMs"] = time.Since(start).Milliseconds()
	writeJSON(w, state)
}

func tuneDebugState() map[string]any {
	st := map[string]any{"open": false}
	fyne.DoAndWait(func() {
		win := getInstance().tuneWindow
		if win == nil {
			return
		}
		st["open"] = true
		st["title"] = win.Title()
		size := win.Canvas().Size()
		st["size"] = []float32{size.Width, size.Height}
		// newTunePanel returns a stack of the window body and the "Applying..." overlay.
		if root, ok := win.Content().(*fyne.Container); ok && len(root.Objects) == 2 {
			st["applying"] = root.Objects[1].Visible()
		}
		label := i18n.TMap("Reset", map[string]string{"zh": "恢复", "zh-Hant": "恢復"})
		st["resetShown"] = false
		for _, o := range debugObjects(win.Content()) {
			if b, ok := o.(*widget.Button); ok && b.Text == label {
				st["resetShown"] = true
			}
			if s, ok := o.(*segmented); ok && s.Selected >= 0 && s.Selected < len(s.Options) {
				st["mode"] = s.Options[s.Selected]
			}
		}
	})
	return st
}

// debugObjects lists the visible objects under o through its containers. The Tune Image window
// keeps everything these routes look for (preview, drag layer, buttons) directly in containers.
func debugObjects(o fyne.CanvasObject) []fyne.CanvasObject {
	if o == nil || !o.Visible() {
		return nil
	}
	out := []fyne.CanvasObject{o}
	if c, ok := o.(*fyne.Container); ok {
		for _, child := range c.Objects {
			out = append(out, debugObjects(child)...)
		}
	}
	return out
}

func writeDebugPNG(w http.ResponseWriter, img image.Image, missing error) {
	if img == nil {
		http.Error(w, missing.Error(), http.StatusConflict)
		return
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(buf.Bytes())
}
