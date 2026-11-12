package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A web page that rebinds its own name to 127.0.0.1 makes same-origin GETs with neither an
// Origin nor a Sec-Fetch-Site header (browsers send Fetch Metadata only to secure origins). Its
// Host header still names the page's domain, so that is what refuses it.
func TestHandleDebugRefusesForeignHost(t *testing.T) {
	s := NewServer()
	s.HandleDebug("state", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	for host, want := range map[string]int{
		"localhost:49452":        http.StatusOK,
		"127.0.0.1:49452":        http.StatusOK,
		"[::1]:49452":            http.StatusOK,
		"LOCALHOST:49452":        http.StatusOK,
		"localhost":              http.StatusOK,
		"evil.example:49452":     http.StatusForbidden,
		"127.0.0.1.nip.io:49452": http.StatusForbidden,
		"":                       http.StatusForbidden,
	} {
		req := httptest.NewRequest(http.MethodGet, "/debug/state", nil)
		req.Host = host
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, req)
		assert.Equal(t, want, rr.Code, "Host %q", host)
	}
}
