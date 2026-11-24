package metmuseum

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/internal/nettest"
)

// The v1 search (collectionapi.metmuseum.org/public/collection/v1/search) answers 410 Gone since
// its retirement on 2026-10-01. The provider now uses the paged v1.1 search. The fixtures in
// testdata/search were recorded from v1.1 on 2026-11-24.

const searchHost = "collectionapi.metmuseum.org/public/collection/v1.1/search?"

func noSearchPause(t *testing.T) {
	old := searchPagePause
	searchPagePause = 0
	t.Cleanup(func() { searchPagePause = old })
}

func TestSearchHighlightsUsesV11(t *testing.T) {
	client := nettest.Replay(t, map[string]string{
		searchHost + "hasImages=true&isHighlight=true&isPublicDomain=true&limit=500&offset=0&q=American+Paintings": "testdata/search/american-paintings.json",
	})
	ids, err := NewProvider(nil, client).fetchSearchHighlights(context.Background(), "American Paintings")
	require.NoError(t, err)
	assert.Len(t, ids, 99)
	assert.Contains(t, ids, 10159)
}

func TestDepartmentHighlightsUsesV11(t *testing.T) {
	client := nettest.Replay(t, map[string]string{
		searchHost + "departmentId=10&hasImages=true&isHighlight=true&isPublicDomain=true&limit=500&offset=0": "testdata/search/department-10.json",
	})
	ids, err := NewProvider(nil, client).fetchDepartmentHighlights(context.Background(), DeptEgyptianArt)
	require.NoError(t, err)
	assert.Len(t, ids, 114)
}

// European Paintings has more works than one page holds: every page is fetched, and no more.
func TestDepartmentSearchReadsEveryPage(t *testing.T) {
	noSearchPause(t)
	dir := t.TempDir()
	page := func(name string, from, n, total int) string {
		ids := make([]int, n)
		for i := range ids {
			ids[i] = from + i
		}
		data, _ := json.Marshal(map[string]any{"total": total, "objectIDs": ids})
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, data, 0o644))
		return path
	}
	base := searchHost + "departmentId=11&hasImages=true&isHighlight=false&isPublicDomain=true&limit=500&medium=Paintings&offset="
	var calls atomic.Int32
	replay := nettest.Replay(t, map[string]string{
		base + "0":   page("p1.json", 1, 500, 700),
		base + "500": page("p2.json", 501, 200, 700),
	})
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return replay.Transport.RoundTrip(r)
	})}

	ids, err := NewProvider(nil, client).fetchDepartmentHighlights(context.Background(), DeptEuropeanPaintings)
	require.NoError(t, err)
	assert.Len(t, ids, 700)
	assert.Equal(t, 1, ids[0])
	assert.Equal(t, 700, ids[699])
	assert.EqualValues(t, 2, calls.Load(), "the second page is the last")
}

// A search that fails is an error, not a collection that silently comes up empty.
func TestSearchFailureIsAnError(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusGone, Status: "410 Gone", Body: http.NoBody, Header: http.Header{}, Request: r}, nil
	})}
	p := NewProvider(nil, client)

	_, err := p.fetchSearchHighlights(context.Background(), "American Paintings")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "410")

	_, err = p.fetchDepartmentHighlights(context.Background(), DeptAsianArt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "410")
}

func TestSearchUnreadableAnswerIsAnError(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Status: "200 OK", Body: http.NoBody, Header: http.Header{}, Request: r}, nil
	})}
	_, err := NewProvider(nil, client).fetchSearchHighlights(context.Background(), "x")
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "met search"), err.Error())
}
