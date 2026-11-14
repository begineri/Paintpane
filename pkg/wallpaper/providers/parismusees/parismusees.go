// Package parismusees is the Paris Musées wallpaper provider: the museums of the City of Paris
// (Petit Palais, Musée Carnavalet, Musée Cernuschi, ...) whose CC0 "Open Content" images are
// free to use.
//
// Paris Musées has no IIIF service and its collections API needs a registered token, which an
// open-source app cannot carry. So nothing is looked up at run time: docs/collections/
// parismusees.json carries the title (in French, English and Chinese), artist, date, original
// image address and pixel size of every work, taken from the collections website's work pages
// by musewall-tools/collection_parsers/parismusees.py (outside the repo). The provider downloads the original file, which is
// 3-5 thousand pixels on its longest side. The file addresses are the website's public image
// directory, not a documented interface; if they change, update the collection file (the app
// fetches a newer one from the repository).
package parismusees

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/dixieflatline76/Spice/v2/pkg/curation"
	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/setting"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
	"github.com/dixieflatline76/Spice/v2/util/log"
)

//go:embed parismusees.png
var iconData []byte

// Provider implements the Paris Musées provider.
type Provider struct {
	cfg    *wallpaper.Config
	client *http.Client
}

func init() {
	wallpaper.RegisterProvider(ProviderName, func(cfg *wallpaper.Config, client *http.Client) provider.ImageProvider {
		return NewProvider(cfg, client)
	})
}

// NewProvider creates a new Paris Musées provider.
func NewProvider(cfg *wallpaper.Config, client *http.Client) *Provider {
	return &Provider{cfg: cfg, client: client}
}

func (p *Provider) ID() string { return ProviderName }

func (p *Provider) Name() string { return i18n.T("Paris Musées") }

// Title is the full name too: the tray and headers show the museum by its full name.
func (p *Provider) Title() string { return i18n.T("Paris Musées") }

func (p *Provider) GetProviderIcon() interface{} { return iconData }

func (p *Provider) Type() provider.ProviderType { return provider.TypeMuseum }

func (p *Provider) GetAttributionType() provider.AttributionType { return provider.AttributionBy }

func (p *Provider) HomeURL() string { return WebBaseURL }

func (p *Provider) SupportsUserQueries() bool { return false }

func (p *Provider) ParseURL(string) (string, error) {
	return "", fmt.Errorf("user queries not supported for Paris Musées")
}

// GetClient implements provider.CustomClientProvider.
func (p *Provider) GetClient() *http.Client { return p.client }

// GetAPIPacing and GetProcessPacing implement provider.PacedProvider. The collections
// website asks robots for a 10 second delay between requests (robots.txt), so a few images are
// fetched one after the other, not in a burst.
func (p *Provider) GetAPIPacing() time.Duration { return time.Second }

func (p *Provider) GetProcessPacing() time.Duration { return 3 * time.Second }

// GetDownloadHeaders implements provider.HeaderProvider.
func (p *Provider) GetDownloadHeaders() map[string]string {
	return map[string]string{"User-Agent": "Paintpane"}
}

// FetchImages returns one page of a curated collection. Everything comes from the
// collection file; no request is made here.
func (p *Provider) FetchImages(_ context.Context, query string, page int) ([]provider.Image, error) {
	entry := curation.GetManager().GetEntry(p.ID(), query)
	if entry == nil {
		log.Printf("Paris Musées: unknown collection key %q, falling back to the highlights", query)
		entry = curation.GetManager().GetEntry(p.ID(), CollectionHighlights)
	}
	if entry == nil || entry.Type != "curated" {
		return nil, fmt.Errorf("no curated collection for %q", query)
	}

	start := (page - 1) * pageSize
	if page < 1 || start >= len(entry.Items) {
		return nil, nil
	}
	end := min(start+pageSize, len(entry.Items))

	var images []provider.Image
	for _, item := range entry.Items[start:end] {
		if img, ok := p.imageFromItem(item); ok {
			images = append(images, img)
		}
	}
	return images, nil
}

// localizedTitle is the title in the current interface language (see titleIn).
func localizedTitle(item curation.PreResolvedItem) string {
	return titleIn(item, i18n.EffectiveLanguage())
}

// titleIn is the title shown in an interface language. The collection file has the museums' own
// French title and English and Chinese ones: the Chinese interfaces show the Chinese title,
// French shows the original, and every other language shows the English title (the French
// one for a work that has no English title).
func titleIn(item curation.PreResolvedItem, lang string) string {
	pick := func(keys ...string) string {
		for _, k := range keys {
			if v := strings.TrimSpace(item.TitleTranslations[k]); v != "" {
				return v
			}
		}
		return strings.TrimSpace(item.Title)
	}
	switch lang {
	case "zh":
		return pick("zh", "zh-CN")
	case "zh-Hant":
		return pick("zh-Hant", "zh-TW")
	case "fr":
		return pick()
	default:
		return pick("en")
	}
}

// previewLanguages are the interface languages whose titles can differ from the English one the
// preview page shows by default.
var previewLanguages = []string{"zh", "zh-Hant", "fr"}

// previewTitles is the title a preview page shows by default (the one for every other language)
// and the titles it switches to in previewLanguages, where they differ.
func previewTitles(item curation.PreResolvedItem) (string, map[string]string) {
	title := titleIn(item, "en")
	var others map[string]string
	for _, lang := range previewLanguages {
		if t := titleIn(item, lang); t != title {
			if others == nil {
				others = map[string]string{}
			}
			others[lang] = t
		}
	}
	return title, others
}

// LocalizedTitle implements provider.TitleLocalizer: the title of a listed image in the current
// interface language, looked up in the collection file by the image's ID (bare, or namespaced
// "ParisMusees_<id>" as the image store keeps it).
func (p *Provider) LocalizedTitle(img provider.Image) (string, bool) {
	id := strings.TrimPrefix(img.ID, ProviderName+"_")
	col := curation.GetManager().GetCollection(ProviderName)
	if col == nil || id == "" {
		return "", false
	}
	for _, e := range col.Entries {
		for _, item := range e.Items {
			if item.ID == id {
				return localizedTitle(item), true
			}
		}
	}
	return "", false
}

func (p *Provider) imageFromItem(item curation.PreResolvedItem) (provider.Image, bool) {
	if item.ImageURL == "" {
		return provider.Image{}, false
	}
	title := localizedTitle(item)
	return provider.Image{
		ID:          item.ID,
		Path:        item.ImageURL,
		ViewURL:     item.ViewURL,
		Attribution: fmt.Sprintf("%s - %s", item.Artist, title),
		Title:       title,
		Artist:      item.Artist,
		Year:        item.Year,
		Provider:    p.ID(),
		FileType:    "image/jpeg",
		Width:       item.Width,
		Height:      item.Height,
	}, true
}

// EnrichImage is a no-op: all metadata comes from the collection file.
func (p *Provider) EnrichImage(_ context.Context, img provider.Image) (provider.Image, error) {
	return img, nil
}

// FetchThumbnails implements provider.ThumbnailProvider from the collection file. Each id is a
// JSON-encoded curation.PreResolvedItem. There is no resizing service, so the thumbnail is the
// original file. The preview page is made once and opened in any language, so the titles do not
// follow the current one: Title is the English title and TitleTranslations has the others.
func (p *Provider) FetchThumbnails(_ context.Context, ids []string) ([]provider.Thumbnail, error) {
	var thumbnails []provider.Thumbnail
	for _, raw := range ids {
		var item curation.PreResolvedItem
		if err := json.Unmarshal([]byte(raw), &item); err != nil {
			log.Printf("Paris Musées: bad pre-resolved item: %v", err)
			continue
		}
		if item.ImageURL == "" {
			continue
		}
		title, others := previewTitles(item)
		thumbnails = append(thumbnails, provider.Thumbnail{
			ID:                item.ID,
			URL:               item.ImageURL,
			ViewURL:           item.ViewURL,
			Title:             title,
			TitleTranslations: others,
			Artist:            item.Artist,
			Year:              item.Year,
		})
	}
	return thumbnails, nil
}

// --- UI ---

// CreateSettingsPanel returns the museum info panel.
func (p *Provider) CreateSettingsPanel(sm setting.SettingsManager) *schema.PanelSchema {
	return schema.CreateMuseumSettingsPanel(schema.MuseumSettingsConfig{
		ID:          ProviderName,
		Title:       i18n.T("Paris Musées"),
		Location:    i18n.T("Paris, France"),
		LicenseURL:  LicenseURL,
		Description: i18n.T("The network of the City of Paris museums, including the Petit Palais, the Musée Carnavalet and the Musée Cernuschi, with Courbet, Monet, Sisley and the history of Paris."),
		MapQuery:    "Petit Palais Paris",
		WebsiteURL:  WebBaseURL,
		DonateURL:   "https://parismusees.paris.fr/en/support-us",
	}, sm.OpenURL)
}

// CreateQueryPanel creates the collection toggle panel.
func (p *Provider) CreateQueryPanel(sm setting.SettingsManager, _ string) *schema.PanelSchema {
	return wallpaper.CreateCuratedQueryPanel(p, sm, p.cfg)
}
