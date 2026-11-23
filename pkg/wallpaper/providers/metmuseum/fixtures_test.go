package metmuseum

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dixieflatline76/Spice/v2/internal/nettest"
)

// replayClient answers Met Collection API object requests from testdata/objects/<id>.json,
// recorded from collectionapi.metmuseum.org on 2026-11-18, so the tests never touch the real API.
// An object without a file gets 404, which the provider skips like any failed object.
func replayClient(t *testing.T) *http.Client {
	t.Helper()
	files, err := filepath.Glob("testdata/objects/*.json")
	if err != nil || len(files) == 0 {
		t.Fatalf("no Met fixtures: %v", err)
	}
	fixtures := make(map[string]string, len(files))
	for _, f := range files {
		id := strings.TrimSuffix(filepath.Base(f), ".json")
		fixtures["collectionapi.metmuseum.org/public/collection/v1/objects/"+id] = filepath.ToSlash(f)
	}
	return nettest.Replay(t, fixtures)
}
