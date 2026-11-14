package nga

import (
	"context"
	"encoding/json"
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
	assert.Equal(t, "NationalGalleryOfArt", p.ID())
	assert.Equal(t, "National Gallery of Art", p.Name())
	assert.Equal(t, provider.TypeMuseum, p.Type())
	assert.False(t, p.SupportsUserQueries())
	assert.NotEmpty(t, p.GetProviderIcon())
	_, err := p.ParseURL("https://www.nga.gov/artworks/60")
	assert.Error(t, err)
}

func TestRegistered(t *testing.T) {
	// The blank import in cmd/spice relies on init registering the provider.
	assert.Contains(t, wallpaper.GetRegisteredProviders(), ProviderName)
}

func TestWithResolution(t *testing.T) {
	p := NewProvider(nil, nil)
	base := IIIFPrefix + "00007f61-4922-417b-8f27-893ea328206c"
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
		ID: "60", Title: "Girl with the Red Hat", Artist: "Johannes Vermeer", Year: "c. 1669",
		ImageURL: IIIFPrefix + "abc", ViewURL: "https://www.nga.gov/artworks/60",
		Width: 12070, Height: 15257,
	})
	require.True(t, ok)
	assert.Equal(t, IIIFPrefix+"abc/full/!3840,3840/0/default.jpg", img.Path)
	assert.Equal(t, "Girl with the Red Hat", img.Title)
	assert.Equal(t, "Johannes Vermeer", img.Artist)
	assert.Equal(t, "Johannes Vermeer - Girl with the Red Hat", img.Attribution)
	assert.Equal(t, "NationalGalleryOfArt", img.Provider)
	// The reported size is that of the scaled copy, so the 100 MP master check does not
	// reject a 184 MP original that is never downloaded.
	assert.LessOrEqual(t, int64(img.Width)*int64(img.Height), int64(wallpaper.MaxMasterPixels))
	assert.Equal(t, 3840, img.Height)

	_, ok = p.imageFromItem(curation.PreResolvedItem{ID: "1"})
	assert.False(t, ok, "an item without an image URL is skipped")
}

func TestFetchImagesPaginatesTheEmbeddedCollection(t *testing.T) {
	p := NewProvider(nil, nil)
	entry := curation.GetManager().GetEntry(ProviderName, "nga_impressionism")
	require.NotNil(t, entry)
	require.Greater(t, len(entry.Items), pageSize, "the test needs more than one page")

	page1, err := p.FetchImages(context.Background(), "nga_impressionism", 1)
	require.NoError(t, err)
	assert.Len(t, page1, pageSize)

	page2, err := p.FetchImages(context.Background(), "nga_impressionism", 2)
	require.NoError(t, err)
	assert.Len(t, page2, len(entry.Items)-pageSize)

	end, err := p.FetchImages(context.Background(), "nga_impressionism", 3)
	require.NoError(t, err)
	assert.Empty(t, end)

	fallback, err := p.FetchImages(context.Background(), "no-such-collection", 1)
	require.NoError(t, err)
	assert.NotEmpty(t, fallback, "an unknown key falls back to the highlights")
}

func TestFetchThumbnails(t *testing.T) {
	p := NewProvider(nil, nil)
	raw, err := json.Marshal(curation.PreResolvedItem{
		ID: "60", Title: "Girl with the Red Hat", Artist: "Johannes Vermeer", ImageURL: IIIFPrefix + "abc",
	})
	require.NoError(t, err)
	thumbs, err := p.FetchThumbnails(context.Background(), []string{string(raw), "not json", `{"id":"2"}`})
	require.NoError(t, err)
	require.Len(t, thumbs, 1)
	assert.Equal(t, IIIFPrefix+"abc/full/!800,800/0/default.jpg", thumbs[0].URL)
	assert.Equal(t, IIIFPrefix+"abc/full/!3840,3840/0/default.jpg", thumbs[0].FallbackURL)
}

// TestCollectionData checks the curated file itself: the series the user sees, and that every
// work has what the provider needs.
func TestCollectionData(t *testing.T) {
	col := curation.GetManager().GetCollection(ProviderName)
	require.NotNil(t, col)
	assert.GreaterOrEqual(t, len(col.Entries), 5)
	assert.LessOrEqual(t, len(col.Entries), 7)

	var nudes int
	for _, e := range col.Entries {
		assert.Equal(t, "curated", e.Type, e.Key)
		assert.GreaterOrEqual(t, len(e.Items), 15, e.Key)
		assert.LessOrEqual(t, len(e.Items), 30, e.Key)
		for _, lang := range []string{"zh-CN", "zh-TW"} {
			assert.NotEmpty(t, e.NameTranslations[lang], "%s has no %s name", e.Key, lang)
		}
		if strings.Contains(strings.ToLower(e.Name), "nude") {
			nudes++
		}

		seen := map[string]bool{}
		for _, it := range e.Items {
			assert.False(t, seen[it.ID], "%s repeats %s", e.Key, it.ID)
			seen[it.ID] = true
			assert.True(t, strings.HasPrefix(it.ImageURL, IIIFPrefix), it.ID)
			assert.NotEmpty(t, it.Title, it.ID)
			assert.NotEmpty(t, it.Artist, it.ID)
			assert.Positive(t, it.Width, it.ID)
			assert.Positive(t, it.Height, it.ID)
			assert.True(t, strings.HasSuffix(it.ViewURL, "/"+it.ID), it.ID)
		}
	}
	assert.Equal(t, 1, nudes, "one nudes series, so the nudes filter can hide it")
}

func TestNudesFilterCoversTheNudeSeries(t *testing.T) {
	m := curation.GetManager()
	assert.True(t, m.IsNudeID(ProviderName+"_41"), "Titian's Venus with a Mirror is in the nudes series")
	assert.False(t, m.IsNudeID(ProviderName+"_60"), "Vermeer's Girl with the Red Hat is not")
}
