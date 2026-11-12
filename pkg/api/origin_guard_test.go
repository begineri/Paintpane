package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The local API serves the program's own clients (the Favorites, Local Folder and Google Photos
// sources, which call it with Go's HTTP client) and, for /health, /add and /ws, the Spice browser
// extension. A web page the person happens to have open must not list or read their pictures,
// add collections or listen to the wallpaper feed.

// guardTestServer returns a server with a "favorites" namespace holding one picture, and records
// added URLs.
func guardTestServer(t *testing.T) (*Server, *[]string) {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "favorite_images"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "favorite_images", "a.jpg"), []byte("jpeg"), 0o644))
	s := NewServer()
	s.RegisterNamespace("favorites", root)
	added := &[]string{}
	s.SetAddCollectionHandler(func(url string) error { *added = append(*added, url); return nil })
	return s, added
}

type guardCase struct {
	name    string
	host    string
	headers map[string]string
}

var (
	localTool = guardCase{"the program's own client", "127.0.0.1:49452", nil}
	webPages  = []guardCase{
		{"a web page (Origin)", "127.0.0.1:49452", map[string]string{"Origin": "https://evil.example"}},
		{"a file:// page (Origin null)", "127.0.0.1:49452", map[string]string{"Origin": "null"}},
		{"a cross-site no-cors request", "127.0.0.1:49452", map[string]string{"Sec-Fetch-Site": "cross-site"}},
		{"a same-site page on another port", "127.0.0.1:49452", map[string]string{"Sec-Fetch-Site": "same-site"}},
		{"a page that rebinds its domain", "evil.example:49452", nil},
	}
	extensions = []guardCase{
		{"the Chrome extension", "127.0.0.1:49452", map[string]string{"Origin": "chrome-extension://abcdefghijklmnop", "Sec-Fetch-Site": "none"}},
		{"the Firefox extension", "127.0.0.1:49452", map[string]string{"Origin": "moz-extension://1234-5678"}},
	}
)

func serve(s *Server, method, target string, c guardCase, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Host = c.host
	for k, v := range c.headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	s.Handler().ServeHTTP(rr, req)
	return rr
}

func TestLocalRoutesServeOnlyTheProgram(t *testing.T) {
	s, _ := guardTestServer(t)
	for _, target := range []string{"/local/favorites/favorite_images/images", "/local/favorites/favorite_images/assets/a.jpg"} {
		rr := serve(s, http.MethodGet, target, localTool, "")
		assert.Equal(t, http.StatusOK, rr.Code, "%s from %s", target, localTool.name)
		assert.Empty(t, rr.Header().Get("Access-Control-Allow-Origin"), "no CORS header on %s", target)
		for _, c := range append(webPages, extensions...) {
			rr := serve(s, http.MethodGet, target, c, "")
			assert.Equal(t, http.StatusForbidden, rr.Code, "%s from %s", target, c.name)
			assert.NotContains(t, rr.Body.String(), "a.jpg", "%s from %s leaks the listing", target, c.name)
		}
	}
}

func TestHealthAndAddServeOnlyTheProgramAndExtensions(t *testing.T) {
	s, added := guardTestServer(t)
	for _, c := range append([]guardCase{localTool}, extensions...) {
		rr := serve(s, http.MethodGet, "/health", c, "")
		assert.Equal(t, http.StatusOK, rr.Code, "/health from %s", c.name)
		assert.NotEqual(t, "*", rr.Header().Get("Access-Control-Allow-Origin"), "/health from %s", c.name)
		rr = serve(s, http.MethodPost, "/add", c, `{"url":"https://example.org/collection"}`)
		assert.Equal(t, http.StatusOK, rr.Code, "/add from %s", c.name)
	}
	require.Len(t, *added, 1+len(extensions))
	for _, c := range webPages {
		assert.Equal(t, http.StatusForbidden, serve(s, http.MethodGet, "/health", c, "").Code, "/health from %s", c.name)
		assert.Equal(t, http.StatusForbidden, serve(s, http.MethodPost, "/add", c, `{"url":"https://evil.example/x"}`).Code, "/add from %s", c.name)
		assert.Equal(t, http.StatusForbidden, serve(s, http.MethodOptions, "/add", c, "").Code, "preflight from %s", c.name)
	}
	assert.Len(t, *added, 1+len(extensions), "no web page added a collection")
}

func TestWebSocketServesOnlyTheProgramAndExtensions(t *testing.T) {
	s, _ := guardTestServer(t)
	srv := httptest.NewServer(s.Handler())
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"

	dial := func(c guardCase) (int, error) {
		h := http.Header{}
		for k, v := range c.headers {
			h.Set(k, v)
		}
		if c.host != localTool.host {
			h.Set("Host", c.host)
		}
		conn, resp, err := websocket.DefaultDialer.Dial(wsURL, h)
		if conn != nil {
			conn.Close()
		}
		if resp == nil {
			return 0, err
		}
		return resp.StatusCode, err
	}
	for _, c := range append([]guardCase{localTool}, extensions...) {
		code, err := dial(c)
		assert.NoError(t, err, "/ws from %s", c.name)
		assert.Equal(t, http.StatusSwitchingProtocols, code, "/ws from %s", c.name)
	}
	for _, c := range webPages {
		code, err := dial(c)
		assert.Error(t, err, "/ws from %s", c.name)
		assert.Equal(t, http.StatusForbidden, code, "/ws from %s", c.name)
	}
}
