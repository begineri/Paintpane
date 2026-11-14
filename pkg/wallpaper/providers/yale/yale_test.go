package yale

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/curation"
	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
)

var (
	_ provider.ImageProvider           = (*Provider)(nil)
	_ provider.ResolutionAwareProvider = (*Provider)(nil)
	_ provider.ThumbnailProvider       = (*Provider)(nil)
	_ provider.HeaderProvider          = (*Provider)(nil)
	_ provider.PacedProvider           = (*Provider)(nil)
)

func TestIdentity(t *testing.T) {
	i18n.SetLanguage("en") // the names are translated; the OS language must not matter
	t.Cleanup(func() { i18n.SetLanguage("") })
	p := NewProvider(nil, nil)
	assert.Equal(t, "YaleUniversityArtCollections", p.ID())
	assert.Equal(t, "Yale University Art Collections", p.Name())
	assert.Equal(t, provider.TypeMuseum, p.Type())
	assert.False(t, p.SupportsUserQueries())
	assert.NotEmpty(t, p.GetProviderIcon())
	_, err := p.ParseURL("https://artgallery.yale.edu/collections/objects/12507")
	assert.Error(t, err)
}

func TestRegistered(t *testing.T) {
	// The blank import in cmd/spice relies on init registering the provider.
	assert.Contains(t, wallpaper.GetRegisteredProviders(), ProviderName)
}

func TestWithResolution(t *testing.T) {
	p := NewProvider(nil, nil)
	base := IIIFPrefix + "yuag:3b072179-2fc7-42bc-87cc-9ee4d782b270"
	tests := []struct {
		name          string
		in            string
		width, height int
		want          string
	}{
		{"full URL scaled to the screen", base + "/full/!3840,3840/0/default.jpg", 1920, 1080, base + "/full/!1920,1920/0/default.jpg"},
		{"portrait monitor uses its longer side", base + "/full/!3840,3840/0/default.jpg", 1080, 1920, base + "/full/!1920,1920/0/default.jpg"},
		{"never above 4K", base + "/full/!3840,3840/0/default.jpg", 7680, 4320, base + "/full/!3840,3840/0/default.jpg"},
		{"bare identifier", base, 2560, 1440, base + "/full/!2560,2560/0/default.jpg"},
		{"foreign URL is left alone", "https://example.org/a.jpg", 1920, 1080, "https://example.org/a.jpg"},
		{"unknown screen size", base + "/full/!3840,3840/0/default.jpg", 0, 0, base + "/full/!3840,3840/0/default.jpg"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, p.WithResolution(tt.in, tt.width, tt.height))
		})
	}
}

func TestImageFromItemSizesTheRequest(t *testing.T) {
	p := NewProvider(nil, nil)
	img, ok := p.imageFromItem(curation.PreResolvedItem{
		ID: "12507", Title: "Le café de nuit (The Night Café)", Artist: "Vincent van Gogh", Year: "c. 1669",
		ImageURL: IIIFPrefix + "yuag:abc", ViewURL: "https://artgallery.yale.edu/collections/objects/12507",
		Width: 7408, Height: 5848,
	})
	require.True(t, ok)
	assert.Equal(t, IIIFPrefix+"yuag:abc/full/!3840,3840/0/default.jpg", img.Path)
	assert.Equal(t, "Le café de nuit (The Night Café)", img.Title)
	assert.Equal(t, "Vincent van Gogh", img.Artist)
	assert.Equal(t, "Vincent van Gogh - Le café de nuit (The Night Café)", img.Attribution)
	assert.Equal(t, "YaleUniversityArtCollections", img.Provider)
	// The reported size is that of the scaled copy, so the 100 MP master check does not
	// reject a very large original that is never downloaded.
	assert.LessOrEqual(t, int64(img.Width)*int64(img.Height), int64(wallpaper.MaxMasterPixels))
	assert.Equal(t, 3840, img.Width)

	_, ok = p.imageFromItem(curation.PreResolvedItem{ID: "1"})
	assert.False(t, ok, "an item without an image URL is skipped")
}

// With High-resolution originals on, the work is requested at its full size, or within the
// 100 MP master limit when it is larger.
func TestImageFromItemHighResOriginals(t *testing.T) {
	p := NewProvider(nil, nil)
	assert.False(t, p.highRes(), "no config reads off")
	p.highRes = func() bool { return true }

	img, ok := p.imageFromItem(curation.PreResolvedItem{ID: "ycba-432", ImageURL: IIIFPrefix + "ycba:abc", Width: 10104, Height: 7479})
	require.True(t, ok)
	assert.Equal(t, IIIFPrefix+"ycba:abc/full/max/0/default.jpg", img.Path)
	assert.Equal(t, 10104, img.Width)
	assert.Equal(t, 7479, img.Height)

	img, ok = p.imageFromItem(curation.PreResolvedItem{ID: "big", ImageURL: IIIFPrefix + "yuag:big", Width: 13994, Height: 9000})
	require.True(t, ok)
	assert.Contains(t, img.Path, "/full/!")
	assert.LessOrEqual(t, int64(img.Width)*int64(img.Height), int64(wallpaper.MaxMasterPixels))

	img, ok = p.imageFromItem(curation.PreResolvedItem{ID: "nosize", ImageURL: IIIFPrefix + "yuag:x"})
	require.True(t, ok)
	assert.Equal(t, IIIFPrefix+"yuag:x/full/!3840,3840/0/default.jpg", img.Path, "without a size: the 4K request")

	assert.Equal(t, img.Path, p.WithResolution(img.Path, 1920, 1080), "not scaled down to the screen")
}

func TestFetchImagesPaginatesTheEmbeddedCollection(t *testing.T) {
	p := NewProvider(nil, nil)
	entry := curation.GetManager().GetEntry(ProviderName, "yale_european")
	require.NotNil(t, entry)
	require.Greater(t, len(entry.Items), pageSize, "the test needs more than one page")

	page1, err := p.FetchImages(context.Background(), "yale_european", 1)
	require.NoError(t, err)
	assert.Len(t, page1, pageSize)

	page2, err := p.FetchImages(context.Background(), "yale_european", 2)
	require.NoError(t, err)
	assert.Len(t, page2, len(entry.Items)-pageSize)

	end, err := p.FetchImages(context.Background(), "yale_european", 3)
	require.NoError(t, err)
	assert.Empty(t, end)

	fallback, err := p.FetchImages(context.Background(), "no-such-collection", 1)
	require.NoError(t, err)
	assert.NotEmpty(t, fallback, "an unknown key falls back to the highlights")
}

func TestFetchThumbnails(t *testing.T) {
	p := NewProvider(nil, nil)
	raw, err := json.Marshal(curation.PreResolvedItem{
		ID: "12507", Title: "Le café de nuit (The Night Café)", Artist: "Vincent van Gogh", ImageURL: IIIFPrefix + "yuag:abc",
	})
	require.NoError(t, err)
	thumbs, err := p.FetchThumbnails(context.Background(), []string{string(raw), "not json", `{"id":"2"}`})
	require.NoError(t, err)
	require.Len(t, thumbs, 1)
	assert.Equal(t, IIIFPrefix+"yuag:abc/full/!800,800/0/default.jpg", thumbs[0].URL)
	assert.Equal(t, IIIFPrefix+"yuag:abc/full/!3840,3840/0/default.jpg", thumbs[0].FallbackURL)
}

// TestCollectionData checks the curated file itself: the series the user sees, and that every
// work has what the provider needs.
func TestCollectionData(t *testing.T) {
	col := curation.GetManager().GetCollection(ProviderName)
	require.NotNil(t, col)
	assert.GreaterOrEqual(t, len(col.Entries), 5)
	assert.LessOrEqual(t, len(col.Entries), 7)

	var nudes int
	seenAnywhere := map[string]string{}
	for _, e := range col.Entries {
		assert.Equal(t, "curated", e.Type, e.Key)
		// The nudes series holds the few nude works found among the picks; they were moved there
		// from the other series so the nudes filter hides them.
		minItems := 15
		if strings.Contains(strings.ToLower(e.Name), "nude") {
			minItems = 5
			nudes++
		}
		assert.GreaterOrEqual(t, len(e.Items), minItems, e.Key)
		assert.LessOrEqual(t, len(e.Items), 30, e.Key)
		for _, lang := range []string{"zh-CN", "zh-TW"} {
			assert.NotEmpty(t, e.NameTranslations[lang], "%s has no %s name", e.Key, lang)
		}

		seen := map[string]bool{}
		for _, it := range e.Items {
			assert.False(t, seen[it.ID], "%s repeats %s", e.Key, it.ID)
			seen[it.ID] = true
			if other, dup := seenAnywhere[it.ID]; dup {
				t.Errorf("%s is in both %s and %s", it.ID, other, e.Key)
			}
			seenAnywhere[it.ID] = e.Key
			assert.True(t, strings.HasPrefix(it.ImageURL, IIIFPrefix), it.ID)
			assert.NotEmpty(t, it.Title, it.ID)
			assert.NotEmpty(t, it.Artist, it.ID)
			assert.Positive(t, it.Width, it.ID)
			assert.Positive(t, it.Height, it.ID)
			assert.NotEmpty(t, it.ViewURL, it.ID)
		}
	}
	assert.Equal(t, 1, nudes, "one nudes series, so the nudes filter can hide it")
}

// The nude works found by hand among Yale's picks (docs/sources-expansion.md section 7) are hidden
// while NSFW images are off; the other works are not.
func TestNudesFilterCoversTheNudeSeries(t *testing.T) {
	m := curation.GetManager()
	for id, title := range map[string]string{
		"43619":     "Rubens, Hero and Leander",
		"9448":      "Gauguin, Paradise Lost (?)",
		"132":       "Trumbull, Maternal Affection, Tenderness, or Love",
		"ycba-1123": "James Ward, Diana at the Bath",
		"ycba-1124": "James Ward, Venus Rising from her Couch",
	} {
		assert.True(t, m.IsNudeID(ProviderName+"_"+id), "%s is in the nudes series", title)
	}
	assert.False(t, m.IsNudeID(ProviderName+"_ycba-432"), "Stubbs's Bulls Fighting is not")
}

// Yale's server sometimes takes a minute or answers 504 for a scaled copy, while the full-size
// image comes at once: a scaled request that fails is asked again as full/max.
func TestImageClientFallsBackToTheFullSize(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.URL.Path)
		if strings.Contains(r.URL.Path, "/full/max/") {
			_, _ = w.Write([]byte("full"))
			return
		}
		w.WriteHeader(http.StatusGatewayTimeout)
	}))
	defer srv.Close()

	c := &http.Client{Transport: maxFallback{prefix: srv.URL + "/iiif/2/", next: http.DefaultTransport}}
	resp, err := c.Get(srv.URL + "/iiif/2/yuag:abc/full/!3840,3840/0/default.jpg")
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, []string{"/iiif/2/yuag:abc/full/!3840,3840/0/default.jpg", "/iiif/2/yuag:abc/full/max/0/default.jpg"}, seen)

	seen = nil
	resp, err = c.Get(srv.URL + "/other/full/!3840,3840/0/default.jpg")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusGatewayTimeout, resp.StatusCode, "another server is not rewritten")
	assert.Len(t, seen, 1)

	_, ok := fullMaxURL(IIIFPrefix+"yuag:abc/full/max/0/default.jpg", IIIFPrefix)
	assert.False(t, ok, "already the full size")
	tr, ok := NewProvider(nil, nil).GetClient().Transport.(maxFallback)
	require.True(t, ok)
	assert.Equal(t, IIIFPrefix, tr.prefix)
}
