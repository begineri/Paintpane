package wikimedia

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
)

var _ provider.PageCursorProvider = (*Provider)(nil)

const pageWithMore = `{"continue":{"continue":"gcmcontinue||","gcmcontinue":"file|ABC|42"},
 "query":{"pages":{"7":{"pageid":7,"title":"File:A.jpg","imageinfo":[{"url":"https://upload.wikimedia.org/A.jpg","width":4000,"height":3000}]}}}}`

const pageLast = `{"query":{"pages":{"8":{"pageid":8,"title":"File:B.jpg","imageinfo":[{"url":"https://upload.wikimedia.org/B.jpg","width":4000,"height":3000}]}}}}`

const badContinue = `{"error":{"code":"badcontinue","info":"Invalid continue param."}}`

// Commons pages with a cursor from the page before. The app saves the cursor of a query's next
// page and gives it back after a restart (provider.PageCursorProvider).
func TestWikimediaPageCursorSurvivesARestart(t *testing.T) {
	var mu sync.Mutex
	var continues []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		continues = append(continues, r.URL.Query().Get("gcmcontinue"))
		mu.Unlock()
		if r.URL.Query().Get("gcmcontinue") == "" {
			_, _ = w.Write([]byte(pageWithMore))
			return
		}
		_, _ = w.Write([]byte(pageLast))
	}))
	defer ts.Close()

	first := NewProvider(&wallpaper.Config{}, ts.Client())
	first.baseURL = ts.URL
	_, err := first.FetchImages(context.Background(), "category:Paintings", 1)
	require.NoError(t, err)
	cursor, ok := first.PageCursor("category:Paintings", 2)
	require.True(t, ok)
	_, ok = first.PageCursor("category:Paintings", 3)
	assert.False(t, ok)

	// After a restart, a new provider has no cursor: page 2 is empty until it is given back.
	second := NewProvider(&wallpaper.Config{}, ts.Client())
	second.baseURL = ts.URL
	images, err := second.FetchImages(context.Background(), "category:Paintings", 2)
	require.NoError(t, err)
	assert.Empty(t, images)

	second.SetPageCursor("category:Paintings", 2, cursor)
	images, err = second.FetchImages(context.Background(), "category:Paintings", 2)
	require.NoError(t, err)
	require.Len(t, images, 1)
	assert.Equal(t, "8", images[0].ID)
	assert.Equal(t, []string{"", "file|ABC|42"}, continues, "page 2 was asked for with the saved cursor")

	// Nonsense is ignored.
	second.SetPageCursor("category:Other", 1, "gcmcontinue=x")
	second.SetPageCursor("category:Other", 2, "%%%")
	_, ok = second.PageCursor("category:Other", 2)
	assert.False(t, ok)
}

// A cursor saved long ago can be refused ("badcontinue"). The page counts as the end of the query,
// so the app starts it over at page 1, instead of failing on that page at every start.
func TestWikimediaStaleCursorStartsTheQueryOver(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("gcmcontinue") != "" {
			_, _ = w.Write([]byte(badContinue))
			return
		}
		_, _ = w.Write([]byte(pageWithMore))
	}))
	defer ts.Close()

	p := NewProvider(&wallpaper.Config{}, ts.Client())
	p.baseURL = ts.URL
	p.SetPageCursor("category:Paintings", 5, "continue=gcmcontinue%7C%7C&gcmcontinue=file%7COLD%7C1")
	images, err := p.FetchImages(context.Background(), "category:Paintings", 5)
	require.NoError(t, err, "not an error that would keep the query on this page")
	assert.Empty(t, images)
	_, ok := p.PageCursor("category:Paintings", 5)
	assert.False(t, ok, "the stale cursor is dropped")

	// On page 1 an API error is an error.
	ts2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"error":{"code":"internal_api_error","info":"boom"}}`))
	}))
	defer ts2.Close()
	p.baseURL = ts2.URL
	_, err = p.FetchImages(context.Background(), "category:Paintings", 1)
	assert.Error(t, err)
}

// Imageinfo with a scaled copy covers 50 files per request: Commons answers the first half of a
// batch of 100 with iicontinue and the generator's own continuation only later. Page 2 must carry
// iicontinue; without it every page asked for the first 50 files again.
func TestWikimediaNextPageCarriesEveryContinueParameter(t *testing.T) {
	var got []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		got = append(got, q.Get("continue")+" ii="+q.Get("iicontinue")+" gsr="+q.Get("gsroffset"))
		switch {
		case q.Get("iicontinue") == "":
			_, _ = w.Write([]byte(`{"continue":{"iicontinue":"Flamingo.jpg|2024","continue":"||"},` +
				`"query":{"pages":{"7":{"pageid":7,"title":"File:A.jpg","imageinfo":[{"url":"https://u/A.jpg","width":4000,"height":3000}]}}}}`))
		default:
			_, _ = w.Write([]byte(`{"continue":{"gsroffset":100,"continue":"gsroffset||"},` +
				`"query":{"pages":{"8":{"pageid":8,"title":"File:B.jpg","imageinfo":[{"url":"https://u/B.jpg","width":4000,"height":3000}]}}}}`))
		}
	}))
	defer ts.Close()

	p := NewProvider(&wallpaper.Config{}, ts.Client())
	p.baseURL = ts.URL
	for page := 1; page <= 2; page++ {
		images, err := p.FetchImages(context.Background(), "search:sunset", page)
		require.NoError(t, err)
		require.Len(t, images, 1)
	}
	assert.Equal(t, []string{" ii= gsr=", "|| ii=Flamingo.jpg|2024 gsr="}, got)
	c, ok := p.PageCursor("search:sunset", 3)
	require.True(t, ok)
	assert.Contains(t, c, "gsroffset=100", "a number is carried too")
}
