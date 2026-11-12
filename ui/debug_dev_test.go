//go:build !release

package ui

import (
	"encoding/json"
	"image"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/api"
)

type fakeDebugTarget struct {
	menu     *fyne.Menu
	invoked  []string
	openPage string
	logPath  string
}

func (f *fakeDebugTarget) State() any           { return map[string]string{"ok": "yes"} }
func (f *fakeDebugTarget) TrayMenu() *fyne.Menu { return f.menu }
func (f *fakeDebugTarget) Invoke(it *fyne.MenuItem) {
	f.invoked = append(f.invoked, it.Label)
	it.Action()
}
func (f *fakeDebugTarget) OpenPrefs(page string) error { f.openPage = page; return nil }
func (f *fakeDebugTarget) ClosePrefs()                 {}
func (f *fakeDebugTarget) CapturePrefs() (image.Image, error) {
	return image.NewRGBA(image.Rect(0, 0, 4, 3)), nil
}
func (f *fakeDebugTarget) LogPath() string { return f.logPath }

func newDebugTestServer(t *testing.T, target debugTarget) *httptest.Server {
	t.Helper()
	s := api.NewServer()
	registerDebugRoutes(s.HandleDebug, target)
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	return srv
}

func TestDebugRoutesTrayAndInvoke(t *testing.T) {
	ran := ""
	mode := fyne.NewMenuItem("Full Artwork", func() { ran = "full" })
	mode.Checked = true
	menu := fyne.NewMenu("",
		fyne.NewMenuItem("Next Wallpaper", func() { ran = "next" }),
		fyne.NewMenuItemSeparator(),
		&fyne.MenuItem{Label: "Artwork display", ChildMenu: fyne.NewMenu("", mode)},
		&fyne.MenuItem{Label: "Disabled", Disabled: true, Action: func() { ran = "disabled" }},
	)
	target := &fakeDebugTarget{menu: menu}
	srv := newDebugTestServer(t, target)

	resp, err := http.Get(srv.URL + "/debug/tray")
	require.NoError(t, err)
	var items []debugMenuItem
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&items))
	resp.Body.Close()
	require.Len(t, items, 4)
	assert.Equal(t, "Next Wallpaper", items[0].Label)
	assert.True(t, items[1].Separator)
	assert.True(t, items[2].Children[0].Checked)

	post := func(q string) int {
		r, err := http.Post(srv.URL+"/debug/tray/invoke?"+q, "", nil)
		require.NoError(t, err)
		r.Body.Close()
		return r.StatusCode
	}
	assert.Equal(t, http.StatusOK, post("label=Next+Wallpaper"))
	assert.Equal(t, "next", ran)
	assert.Equal(t, http.StatusOK, post("label=Artwork+display&label=Full+Artwork"))
	assert.Equal(t, "full", ran)
	assert.Equal(t, http.StatusNotFound, post("label=Nope"))
	assert.Equal(t, http.StatusConflict, post("label=Disabled"))
	assert.Equal(t, "full", ran, "a disabled item does not run")

	// Actions need POST.
	r, err := http.Get(srv.URL + "/debug/tray/invoke?label=Next+Wallpaper")
	require.NoError(t, err)
	r.Body.Close()
	assert.Equal(t, http.StatusMethodNotAllowed, r.StatusCode)
}

func TestDebugRoutesRefuseWebPages(t *testing.T) {
	srv := newDebugTestServer(t, &fakeDebugTarget{menu: fyne.NewMenu("")})
	for _, h := range []map[string]string{
		{"Origin": "https://example.com"},
		{"Sec-Fetch-Site": "cross-site"},
	} {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/debug/state", nil)
		for k, v := range h {
			req.Header.Set(k, v)
		}
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, h)
	}
	resp, err := http.Get(srv.URL + "/debug/state")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "curl-style requests are allowed")
}

func TestDebugRoutesPrefsAndLog(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "app.log")
	require.NoError(t, os.WriteFile(logPath, []byte("one\r\ntwo\r\nthree\r\n"), 0o600))
	target := &fakeDebugTarget{menu: fyne.NewMenu(""), logPath: logPath}
	srv := newDebugTestServer(t, target)

	resp, err := http.Post(srv.URL+"/debug/prefs/open?page=wallpaper", "", nil)
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, "wallpaper", target.openPage)

	resp, err = http.Get(srv.URL + "/debug/prefs.png")
	require.NoError(t, err)
	assert.Equal(t, "image/png", resp.Header.Get("Content-Type"))
	resp.Body.Close()

	resp, err = http.Get(srv.URL + "/debug/log?lines=2")
	require.NoError(t, err)
	body := make([]byte, 64)
	n, _ := resp.Body.Read(body)
	resp.Body.Close()
	assert.Equal(t, "two\nthree\n", string(body[:n]))
}
