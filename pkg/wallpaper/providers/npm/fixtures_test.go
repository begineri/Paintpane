package npm

import (
	"net/http"
	"testing"

	"github.com/dixieflatline76/Spice/v2/internal/nettest"
)

// replayClient answers the National Palace Museum IIIF manifest requests from testdata, recorded
// from digitalarchive.npm.gov.tw on 2026-11-18, so the tests never touch the real site.
func replayClient(t *testing.T) *http.Client {
	return nettest.Replay(t, map[string]string{
		"digitalarchive.npm.gov.tw/Integrate/GetJson?cid=2383&dept=U": "testdata/manifest-2383.json",
		"digitalarchive.npm.gov.tw/Integrate/GetJson?cid=299&dept=U":  "testdata/manifest-299.json",
	})
}
