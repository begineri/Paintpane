package parismusees

import (
	"context"
	"encoding/json"
	stdhtml "html"
	"io/fs"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/asset/galleries"
	"github.com/dixieflatline76/Spice/v2/pkg/curation"
	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
)

var (
	_ provider.ImageProvider     = (*Provider)(nil)
	_ provider.ThumbnailProvider = (*Provider)(nil)
	_ provider.HeaderProvider    = (*Provider)(nil)
	_ provider.PacedProvider     = (*Provider)(nil)
	_ provider.TitleLocalizer    = (*Provider)(nil)
)

func useLanguage(t *testing.T, lang string) {
	t.Helper()
	i18n.SetLanguage(lang)
	t.Cleanup(func() { i18n.SetLanguage("") })
}

func TestIdentity(t *testing.T) {
	useLanguage(t, "en") // the names are translated; the OS language must not matter
	p := NewProvider(nil, nil)
	assert.Equal(t, "ParisMusees", p.ID())
	assert.Equal(t, "Paris Musées", p.Name())
	assert.Equal(t, provider.TypeMuseum, p.Type())
	assert.False(t, p.SupportsUserQueries())
	assert.NotEmpty(t, p.GetProviderIcon())
	_, err := p.ParseURL("https://www.parismuseescollections.paris.fr/")
	assert.Error(t, err)
}

func TestRegistered(t *testing.T) {
	assert.Contains(t, wallpaper.GetRegisteredProviders(), ProviderName)
}

var sample = curation.PreResolvedItem{
	ID:       "215239",
	Title:    "Les Demoiselles des bords de la Seine",
	Artist:   "Gustave Courbet",
	Year:     "1857",
	ImageURL: ImageHost + "PPP/lpdp_0001.jpg",
	ViewURL:  "https://www.parismuseescollections.paris.fr/fr/petit-palais/oeuvres/demoiselles",
	Width:    4200,
	Height:   3100,
	TitleTranslations: map[string]string{
		"en":    "Young Ladies on the Banks of the Seine",
		"zh-CN": "塞纳河畔的少女",
		"zh-TW": "塞納河畔的少女",
	},
}

// noEnglish is a work whose collection entry has no English title.
var noEnglish = curation.PreResolvedItem{
	ID:       "215240",
	Title:    "Vue de Paris",
	Artist:   "Anonyme",
	Year:     "1800",
	ImageURL: ImageHost + "PPP/lpdp_0002.jpg",
	TitleTranslations: map[string]string{
		"zh-CN": "巴黎风景",
		"zh-TW": "巴黎風景",
	},
}

// The Chinese interfaces show the Chinese title, French the museums' own French one, and every
// other language the English one; a work without an English title falls back to the French one.
func TestImageTitleFollowsTheLanguage(t *testing.T) {
	const (
		french  = "Les Demoiselles des bords de la Seine"
		english = "Young Ladies on the Banks of the Seine"
	)
	p := NewProvider(nil, nil)
	tests := []struct {
		lang            string
		want, noEnglish string
	}{
		{"en", english, "Vue de Paris"},
		{"zh", "塞纳河畔的少女", "巴黎风景"},
		{"zh-Hant", "塞納河畔的少女", "巴黎風景"},
		{"fr", french, "Vue de Paris"},
		{"de", english, "Vue de Paris"},
		{"ja", english, "Vue de Paris"},
		{"es", english, "Vue de Paris"},
		{"ru", english, "Vue de Paris"},
	}
	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			useLanguage(t, tt.lang)
			require.Equal(t, tt.lang, i18n.EffectiveLanguage())

			img, ok := p.imageFromItem(sample)
			require.True(t, ok)
			assert.Equal(t, tt.want, img.Title)
			assert.Equal(t, "Gustave Courbet - "+tt.want, img.Attribution)
			assert.Equal(t, "Gustave Courbet", img.Artist)

			img, ok = p.imageFromItem(noEnglish)
			require.True(t, ok)
			assert.Equal(t, tt.noEnglish, img.Title)
		})
	}
}

// Chinese titles may also be keyed by the app's own codes.
func TestTitleInAcceptsAppLanguageCodes(t *testing.T) {
	item := curation.PreResolvedItem{Title: "Titre", TitleTranslations: map[string]string{
		"en": "Title", "zh": "标题", "zh-Hant": "標題",
	}}
	assert.Equal(t, "标题", titleIn(item, "zh"))
	assert.Equal(t, "標題", titleIn(item, "zh-Hant"))
	assert.Equal(t, "Titre", titleIn(item, "fr"))
	assert.Equal(t, "Title", titleIn(item, "it"))
}

func TestImageFromItem(t *testing.T) {
	useLanguage(t, "en")
	p := NewProvider(nil, nil)
	img, ok := p.imageFromItem(sample)
	require.True(t, ok)
	assert.Equal(t, sample.ImageURL, img.Path, "the original file is downloaded as is: there is no resizing service")
	assert.Equal(t, sample.ViewURL, img.ViewURL)
	assert.Equal(t, "ParisMusees", img.Provider)
	assert.Equal(t, 4200, img.Width)
	assert.Equal(t, 3100, img.Height)
	assert.LessOrEqual(t, int64(img.Width)*int64(img.Height), int64(wallpaper.MaxMasterPixels))

	_, ok = p.imageFromItem(curation.PreResolvedItem{ID: "1"})
	assert.False(t, ok, "an item without an image URL is skipped")
}

func TestFetchImagesPaginatesTheEmbeddedCollection(t *testing.T) {
	p := NewProvider(nil, nil)
	col := curation.GetManager().GetCollection(ProviderName)
	require.NotNil(t, col)

	var entry *curation.CollectionEntry
	for i := range col.Entries {
		if len(col.Entries[i].Items) > pageSize {
			entry = &col.Entries[i]
			break
		}
	}
	require.NotNil(t, entry, "the test needs a series with more than one page")

	page1, err := p.FetchImages(context.Background(), entry.Key, 1)
	require.NoError(t, err)
	assert.Len(t, page1, pageSize)

	page2, err := p.FetchImages(context.Background(), entry.Key, 2)
	require.NoError(t, err)
	assert.Len(t, page2, len(entry.Items)-pageSize)

	end, err := p.FetchImages(context.Background(), entry.Key, 3)
	require.NoError(t, err)
	assert.Empty(t, end)

	fallback, err := p.FetchImages(context.Background(), "no-such-collection", 1)
	require.NoError(t, err)
	assert.NotEmpty(t, fallback, "an unknown key falls back to the highlights")
}

// The preview page is made once and opened in any language: it shows the English title and
// carries the Chinese and French ones, whatever the language was when it was made.
func TestFetchThumbnails(t *testing.T) {
	for _, lang := range []string{"en", "zh", "de"} {
		t.Run(lang, func(t *testing.T) {
			useLanguage(t, lang)
			p := NewProvider(nil, nil)
			raw, err := json.Marshal(sample)
			require.NoError(t, err)
			rawNoEnglish, err := json.Marshal(noEnglish)
			require.NoError(t, err)
			thumbs, err := p.FetchThumbnails(context.Background(), []string{string(raw), "not json", `{"id":"2"}`, string(rawNoEnglish)})
			require.NoError(t, err)
			require.Len(t, thumbs, 2)
			assert.Equal(t, sample.ImageURL, thumbs[0].URL)
			assert.Equal(t, "Young Ladies on the Banks of the Seine", thumbs[0].Title)
			assert.Equal(t, map[string]string{
				"zh":      "塞纳河畔的少女",
				"zh-Hant": "塞納河畔的少女",
				"fr":      "Les Demoiselles des bords de la Seine",
			}, thumbs[0].TitleTranslations)

			assert.Equal(t, "Vue de Paris", thumbs[1].Title, "no English title: the French one")
			assert.Equal(t, map[string]string{"zh": "巴黎风景", "zh-Hant": "巴黎風景"}, thumbs[1].TitleTranslations,
				"French is the default already, so it is not repeated")
		})
	}
}

// The embedded preview pages were made from the collection file: each title label shows the
// English title and carries the Chinese and French ones under its key. If this fails after the
// collection titles change, regenerate the pages with cmd/gallerygen.
func TestEmbeddedPreviewTitles(t *testing.T) {
	col := curation.GetManager().GetCollection(ProviderName)
	require.NotNil(t, col)
	byID := map[string]curation.PreResolvedItem{}
	for _, e := range col.Entries {
		for _, it := range e.Items {
			byID[it.ID] = it
		}
	}

	dir := strings.ToLower(ProviderName)
	pages, err := fs.ReadDir(galleries.EmbeddedGalleries, dir)
	require.NoError(t, err)
	require.Len(t, pages, len(col.Entries))

	labelRe := regexp.MustCompile(`<div class="title"( data-i18n="([^"]*)")?>([^<]*)</div>`)
	transRe := regexp.MustCompile(`const translations = (.*);\r?\n`)
	for _, page := range pages {
		labels := 0
		b, err := galleries.EmbeddedGalleries.ReadFile(dir + "/" + page.Name())
		require.NoError(t, err)
		html := string(b)

		m := transRe.FindStringSubmatch(html)
		require.NotNil(t, m, page.Name())
		var trans map[string]map[string]string
		require.NoError(t, json.Unmarshal([]byte(m[1]), &trans), page.Name())

		for _, l := range labelRe.FindAllStringSubmatch(html, -1) {
			labels++
			key := l[2]
			require.True(t, strings.HasPrefix(key, "artwork:"), "%s: label %q has no key", page.Name(), l[3])
			item, ok := byID[strings.TrimPrefix(key, "artwork:")]
			require.True(t, ok, "%s: %s is not in the collection", page.Name(), key)

			title, others := previewTitles(item)
			assert.Equal(t, title, stdhtml.UnescapeString(l[3]), "%s: %s", page.Name(), key)
			for _, lang := range previewLanguages {
				assert.Equal(t, others[lang], trans[lang][key], "%s: %s in %s", page.Name(), key, lang)
			}
			for _, lang := range []string{"de", "ja", "en"} {
				assert.Empty(t, trans[lang][key], "%s: %s in %s shows the English title", page.Name(), key, lang)
			}
		}
		assert.Equal(t, strings.Count(html, `class="artwork-container"`), labels, "%s: every work has a labelled title", page.Name())
	}
}

// TestCollectionData checks the curated file itself: the series the user sees, and that every
// work has what the provider needs, including titles in every language the app requires.
func TestCollectionData(t *testing.T) {
	col := curation.GetManager().GetCollection(ProviderName)
	require.NotNil(t, col)
	assert.GreaterOrEqual(t, len(col.Entries), 5)
	assert.LessOrEqual(t, len(col.Entries), 7)

	var nudes int
	seenAnywhere := map[string]string{}
	for _, e := range col.Entries {
		assert.Equal(t, "curated", e.Type, e.Key)
		// The nudes series is short on purpose: only that many open-licence nude paintings
		// were found among the museums' CC0 paintings.
		minItems := 15
		if strings.Contains(strings.ToLower(e.Name), "nude") {
			minItems = 8
		}
		assert.GreaterOrEqual(t, len(e.Items), minItems, e.Key)
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
			if other, dup := seenAnywhere[it.ID]; dup {
				t.Errorf("%s is in both %s and %s", it.ID, other, e.Key)
			}
			seenAnywhere[it.ID] = e.Key

			assert.True(t, strings.HasPrefix(it.ImageURL, ImageHost), it.ID)
			assert.True(t, strings.HasSuffix(it.ImageURL, ".jpg"), it.ID)
			assert.NotEmpty(t, it.Title, it.ID)
			assert.NotEmpty(t, it.Artist, it.ID)
			assert.NotEmpty(t, it.Year, it.ID)
			assert.Positive(t, it.Width, it.ID)
			assert.Positive(t, it.Height, it.ID)
			assert.LessOrEqual(t, int64(it.Width)*int64(it.Height), int64(wallpaper.MaxMasterPixels), it.ID)
			assert.True(t, strings.HasPrefix(it.ViewURL, "https://www.parismuseescollections.paris.fr/"), it.ID)
			for _, lang := range []string{"en", "zh-CN", "zh-TW"} {
				assert.NotEmpty(t, it.TitleTranslations[lang], "%s (%s) has no %s title", it.ID, it.Title, lang)
			}
		}
	}
	assert.Equal(t, 1, nudes, "one nudes series, so the nudes filter can hide it")
}

// An image listed in one language gives its title in the language the interface has now, so the
// tray follows a change of language for images already in the library.
func TestLocalizedTitleFollowsTheCurrentLanguage(t *testing.T) {
	p := NewProvider(nil, nil)
	useLanguage(t, "zh")
	img, ok := p.imageFromItem(curation.PreResolvedItem{ID: "les-demoiselles-des-bords-de-la-seine-3-9t-3-9", ImageURL: "x"})
	require.True(t, ok)
	img.ID = ProviderName + "_" + img.ID // as the image store keeps it

	useLanguage(t, "en")
	title, ok := p.LocalizedTitle(img)
	require.True(t, ok)
	assert.Equal(t, "Young Ladies on the Banks of the Seine (Summer)", title)

	for lang, want := range map[string]string{
		"zh":      "塞纳河畔的少女们（夏）",
		"zh-Hant": "塞納河畔的少女們（夏）",
		"fr":      "Les demoiselles des bords de la Seine (été)",
		"de":      "Young Ladies on the Banks of the Seine (Summer)",
		"ja":      "Young Ladies on the Banks of the Seine (Summer)",
	} {
		useLanguage(t, lang)
		title, ok = p.LocalizedTitle(img)
		require.True(t, ok, lang)
		assert.Equal(t, want, title, lang)
	}

	_, ok = p.LocalizedTitle(provider.Image{ID: ProviderName + "_not-in-the-collection"})
	assert.False(t, ok)
}
