//go:build release

package ui

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dixieflatline76/Spice/v2/pkg/api"
)

// A release build must not answer on any debug route: run with go test -tags release ./ui/.
func TestReleaseBuildHasNoDebugRoutes(t *testing.T) {
	s := api.NewServer()
	RegisterDebugHandlers(s)
	for _, path := range []string{"/debug/state", "/debug/tray", "/debug/log", "/debug/prefs.png", "/debug/tune"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Host = "localhost:49452"
		rr := httptest.NewRecorder()
		s.Handler().ServeHTTP(rr, req)
		assert.Equal(t, http.StatusNotFound, rr.Code, path)
	}
}
