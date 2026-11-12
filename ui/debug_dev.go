//go:build !release

package ui

// Dev-build debug routes on the local API server (127.0.0.1:49452), so the
// running app can be checked and driven from a terminal without clicking or
// taking desktop screenshots:
//
//	curl -s localhost:49452/debug/state
//	curl -s localhost:49452/debug/tray
//	curl -s -X POST "localhost:49452/debug/tray/invoke?label=Next wallpaper"
//	curl -s -X POST localhost:49452/debug/traymenu/open      # the app-drawn menu (Windows)
//	curl -s localhost:49452/debug/traymenu                    # its place, focus and items
//	curl -s -o tray.png localhost:49452/debug/traymenu.png
//	curl -s -X POST "localhost:49452/debug/prefs/open?page=wallpaper"
//	curl -s -o prefs.png localhost:49452/debug/prefs.png
//	curl -s -X POST "localhost:49452/debug/prefs/select?name=theme&index=2"   # timed, see debug_select_dev.go
//	curl -s "localhost:49452/debug/log?lines=100"
//
// Release builds compile debug_release.go instead and register nothing.

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"

	"github.com/dixieflatline76/Spice/v2/config"
	"github.com/dixieflatline76/Spice/v2/pkg/api"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
)

// RegisterDebugHandlers adds the dev-only /debug/ routes to the local API server.
func RegisterDebugHandlers(s *api.Server) {
	registerDebugRoutes(s.HandleDebug, &appDebugTarget{})
	registerTuneDebugRoutes(s.HandleDebug)
}

// debugTarget is what the debug routes drive. The app implements it; tests use a fake.
type debugTarget interface {
	State() any
	TrayMenu() *fyne.Menu
	Invoke(item *fyne.MenuItem)
	OpenPrefs(page string) error
	ClosePrefs()
	CapturePrefs() (image.Image, error)
	LogPath() string
}

// debugMenuItem is a tray menu item as JSON.
type debugMenuItem struct {
	Label     string          `json:"label,omitempty"`
	Checked   bool            `json:"checked,omitempty"`
	Disabled  bool            `json:"disabled,omitempty"`
	Separator bool            `json:"separator,omitempty"`
	Children  []debugMenuItem `json:"children,omitempty"`
}

func menuToDebug(m *fyne.Menu) []debugMenuItem {
	if m == nil {
		return nil
	}
	out := make([]debugMenuItem, 0, len(m.Items))
	for _, it := range m.Items {
		d := debugMenuItem{Label: it.Label, Checked: it.Checked, Disabled: it.Disabled, Separator: it.IsSeparator}
		if it.ChildMenu != nil {
			d.Children = menuToDebug(it.ChildMenu)
		}
		out = append(out, d)
	}
	return out
}

// findMenuItem follows a path of labels through the menu and its submenus.
func findMenuItem(m *fyne.Menu, path []string) *fyne.MenuItem {
	for i, label := range path {
		if m == nil {
			return nil
		}
		var found *fyne.MenuItem
		for _, it := range m.Items {
			if !it.IsSeparator && it.Label == label {
				found = it
				break
			}
		}
		if found == nil {
			return nil
		}
		if i == len(path)-1 {
			return found
		}
		m = found.ChildMenu
	}
	return nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func requirePost(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost {
		http.Error(w, "use POST", http.StatusMethodNotAllowed)
		return false
	}
	return true
}

func registerDebugRoutes(handle func(string, http.HandlerFunc), t debugTarget) {
	handle("state", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, t.State())
	})

	handle("tray", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, menuToDebug(t.TrayMenu()))
	})

	// POST /debug/tray/invoke?label=A[&label=B...] runs the item at that label path.
	handle("tray/invoke", func(w http.ResponseWriter, r *http.Request) {
		if !requirePost(w, r) {
			return
		}
		path := r.URL.Query()["label"]
		if len(path) == 0 {
			http.Error(w, "missing label", http.StatusBadRequest)
			return
		}
		it := findMenuItem(t.TrayMenu(), path)
		switch {
		case it == nil:
			http.Error(w, "no such menu item: "+strings.Join(path, " > "), http.StatusNotFound)
		case it.Disabled || it.Action == nil:
			http.Error(w, "menu item is disabled or has no action", http.StatusConflict)
		default:
			t.Invoke(it)
			writeJSON(w, map[string]string{"invoked": strings.Join(path, " > ")})
		}
	})

	registerTrayPopupRoutes(handle, t)
	registerPrefsSelectRoute(handle)

	// POST /debug/prefs/open?page=sources|wallpaper|general|advanced
	handle("prefs/open", func(w http.ResponseWriter, r *http.Request) {
		if !requirePost(w, r) {
			return
		}
		if err := t.OpenPrefs(r.URL.Query().Get("page")); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]string{"opened": r.URL.Query().Get("page")})
	})

	handle("prefs/close", func(w http.ResponseWriter, r *http.Request) {
		if !requirePost(w, r) {
			return
		}
		t.ClosePrefs()
		writeJSON(w, map[string]bool{"closed": true})
	})

	// GET /debug/prefs.png renders the preferences window as the app draws it.
	handle("prefs.png", func(w http.ResponseWriter, _ *http.Request) {
		img, err := t.CapturePrefs()
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
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

	// GET /debug/log?lines=N returns the last N lines of the app log (default 200).
	handle("log", func(w http.ResponseWriter, r *http.Request) {
		n, err := strconv.Atoi(r.URL.Query().Get("lines"))
		if err != nil || n <= 0 {
			n = 200
		}
		lines, err := tailLines(t.LogPath(), n)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, strings.Join(lines, "\n")+"\n")
	})
}

// tailLines returns the last n lines of a file, reading at most its last 1 MiB.
func tailLines(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	const maxRead = 1 << 20
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	off := st.Size() - maxRead
	if off < 0 {
		off = 0
	}
	if _, err := f.Seek(off, io.SeekStart); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimRight(string(data), "\r\n"), "\n")
	if off > 0 && len(lines) > 0 {
		lines = lines[1:] // the first line was probably cut
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], "\r")
	}
	return lines, nil
}

// appDebugTarget drives the running app. UI work runs on Fyne's main thread.
type appDebugTarget struct{}

func (appDebugTarget) State() any {
	sa := getInstance()
	prefs := map[string]any{"open": false}
	fyne.DoAndWait(func() {
		if sa.prefsWindow != nil {
			prefs["open"] = true
			if sa.prefsNav != nil {
				prefs["page"] = sa.prefsNav.Current()
			}
			size := sa.prefsWindow.Canvas().Size()
			prefs["size"] = []float32{size.Width, size.Height}
		}
	})
	return map[string]any{
		"version":   config.AppVersion,
		"wallpaper": wallpaper.GetInstance().DebugState(),
		"prefs":     prefs,
	}
}

func (appDebugTarget) TrayMenu() *fyne.Menu {
	var m *fyne.Menu
	fyne.DoAndWait(func() { m = getInstance().trayMenu })
	return m
}

func (appDebugTarget) Invoke(item *fyne.MenuItem) {
	// Menu actions already hop to the main thread where they need to; run them
	// the way the tray does, off the HTTP goroutine.
	go item.Action()
}

func (appDebugTarget) OpenPrefs(page string) error {
	switch page {
	case "", pageSources, pageWallpaper, pageGeneral, pageAdvanced:
	default:
		return fmt.Errorf("unknown page %q (use sources, wallpaper, general or advanced)", page)
	}
	sa := getInstance()
	fyne.DoAndWait(func() {
		sa.CreatePreferencesWindow("")
		if page != "" && sa.prefsNav != nil {
			sa.prefsNav.Select(page)
		}
	})
	return nil
}

func (appDebugTarget) ClosePrefs() {
	sa := getInstance()
	fyne.DoAndWait(func() {
		if sa.prefsWindow != nil {
			sa.prefsWindow.Close()
		}
	})
}

func (appDebugTarget) CapturePrefs() (image.Image, error) {
	sa := getInstance()
	var img image.Image
	fyne.DoAndWait(func() {
		if sa.prefsWindow != nil {
			img = sa.prefsWindow.Canvas().Capture()
		}
	})
	if img == nil {
		return nil, errors.New("preferences window is not open (POST /debug/prefs/open first)")
	}
	return img, nil
}

func (appDebugTarget) LogPath() string {
	return filepath.Join(config.GetAppDir(), config.AppName+config.LogExt)
}
