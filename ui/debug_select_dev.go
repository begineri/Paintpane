//go:build !release

package ui

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
)

// POST /debug/prefs/select?name=theme|accent&index=N picks a choice in the open settings window
// the way a click does, then reports how long the switch took:
//
//	applied_ms  click until the new theme is installed (Settings().Theme() changed)
//	visible_ms  click until the UI thread is free again after the first frames that draw it
//
// The UI thread draws the frames, so a call queued on it while a frame is drawn waits for that
// frame; visible_ms is when the last such wait ended, watched for 80 ms after the theme changed.
func registerPrefsSelectRoute(handle func(string, http.HandlerFunc)) {
	handle("prefs/select", func(w http.ResponseWriter, r *http.Request) {
		if !requirePost(w, r) {
			return
		}
		name := r.URL.Query().Get("name")
		index, err := strconv.Atoi(r.URL.Query().Get("index"))
		if err != nil {
			http.Error(w, "index must be a number", http.StatusBadRequest)
			return
		}
		res, err := measurePrefsSelect(name, index)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		writeJSON(w, res)
	})
}

func measurePrefsSelect(name string, index int) (map[string]any, error) {
	sa := getInstance()
	var before fyne.Theme
	var err error
	start := time.Now()
	fyne.DoAndWait(func() {
		before = fyne.CurrentApp().Settings().Theme()
		if sa.prefsSM == nil {
			err = fmt.Errorf("preferences window is not open (POST /debug/prefs/open?page=general first)")
			return
		}
		switch c := sa.prefsSM.allWidgets[name].(type) {
		case *segmented:
			c.SetSelectedIndex(index)
		case *swatchPicker:
			c.SetSelectedIndex(index)
		default:
			err = fmt.Errorf("no segmented or swatch setting named %q", name)
		}
	})
	if err != nil {
		return nil, err
	}

	ms := func(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }
	var applied time.Duration
	for time.Since(start) < 3*time.Second {
		changed := false
		fyne.DoAndWait(func() { changed = fyne.CurrentApp().Settings().Theme() != before })
		if changed {
			applied = time.Since(start)
			break
		}
		time.Sleep(time.Millisecond)
	}
	if applied == 0 {
		return map[string]any{"applied": false, "waited_ms": ms(time.Since(start))}, nil
	}

	visible := applied
	longest := time.Duration(0)
	watch := time.Now()
	for time.Since(watch) < 80*time.Millisecond {
		t := time.Now()
		fyne.DoAndWait(func() {})
		if d := time.Since(t); d > 3*time.Millisecond {
			visible = time.Since(start)
			if d > longest {
				longest = d
			}
		}
		time.Sleep(500 * time.Microsecond)
	}
	return map[string]any{
		"applied":          true,
		"applied_ms":       ms(applied),
		"visible_ms":       ms(visible),
		"longest_block_ms": ms(longest),
	}, nil
}
