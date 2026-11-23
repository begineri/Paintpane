// Package nettest keeps `go test ./...` off the real network.
//
// Provider code builds its request URLs from fixed hosts (digitalarchive.npm.gov.tw,
// openaccess-api.clevelandart.org, ...). Replay gives a test an *http.Client that sends every
// request, whatever its host, to a local httptest server answering from recorded files, so the
// provider runs unchanged and the URLs it produces are still the real ones.
//
// Tests that only make sense against the real sites call RequireNetwork, which skips them unless
// PAINTPANE_NETWORK_TESTS=1.
package nettest

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Env is the environment variable that turns on tests needing the real network.
const Env = "PAINTPANE_NETWORK_TESTS"

// RequireNetwork skips t unless PAINTPANE_NETWORK_TESTS=1 (or one of the older per-area switches
// passed in legacy is set), so CI never depends on a museum's server being up.
func RequireNetwork(t testing.TB, legacy ...string) {
	t.Helper()
	if os.Getenv(Env) == "1" {
		return
	}
	for _, name := range legacy {
		if os.Getenv(name) != "" {
			return
		}
	}
	t.Skipf("needs the real network; set %s=1 to run", Env)
}

// Replay returns a client that answers every request from fixtures, without leaving the machine.
//
// Keys are the requested host plus path and query, without the scheme, exactly as the provider
// sends them ("digitalarchive.npm.gov.tw/Integrate/GetJson?cid=2383&dept=U"). Values are file
// paths relative to the test's package directory (normally under testdata/). A request with no
// fixture gets 404 Not Found, which is also how the tests check the "unknown ID" paths.
func Replay(t testing.TB, fixtures map[string]string) *http.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Host + r.URL.RequestURI()
		file, ok := fixtures[key]
		if !ok {
			t.Logf("nettest: no fixture for %s, answering 404", key)
			http.NotFound(w, r)
			return
		}
		body, err := os.ReadFile(file)
		if err != nil {
			t.Errorf("nettest: fixture for %s: %v", key, err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if ct := contentType(file); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)

	target, _ := url.Parse(srv.URL)
	return &http.Client{Transport: redirect{target: target, next: srv.Client().Transport}}
}

// redirect sends a request to target but keeps the original host in the Host header, which the
// replay server uses to pick the fixture.
type redirect struct {
	target *url.URL
	next   http.RoundTripper
}

func (rt redirect) RoundTrip(req *http.Request) (*http.Response, error) {
	out := req.Clone(req.Context())
	out.Host = req.URL.Host
	out.URL.Scheme = rt.target.Scheme
	out.URL.Host = rt.target.Host
	resp, err := rt.next.RoundTrip(out)
	if resp != nil {
		resp.Request = req // the caller sees the URL it asked for
	}
	return resp, err
}

func contentType(file string) string {
	switch strings.ToLower(filepath.Ext(file)) {
	case ".json":
		return "application/json"
	case ".jsonld":
		return "application/ld+json"
	case ".html":
		return "text/html; charset=utf-8"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	}
	return ""
}
