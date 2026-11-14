// Package nga is the National Gallery of Art (Washington) wallpaper provider.
//
// NGA publishes its open data as CC0 CSV files and serves images through IIIF, but has no
// search API. The collections in docs/collections/nga.json therefore carry the title, artist,
// date, IIIF image address and pixel size of every work (built by
// musewall-tools/collection_parsers/nga.py, outside the repo), and the provider only asks the
// IIIF server for a scaled copy of the image. The works can be as large as 30,000 pixels a side,
// so the longest side requested never exceeds maxIIIFEdge. High-resolution originals does not
// change that: NGA's image server sends at most 4096 pixels a side, even for "full/max".
package nga

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"time"

	"github.com/dixieflatline76/Spice/v2/pkg/curation"
	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/setting"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
	"github.com/dixieflatline76/Spice/v2/util/log"
)

//go:embed nga.png
var iconData []byte

// Provider implements the National Gallery of Art provider.
type Provider struct {
	cfg    *wallpaper.Config
	client *http.Client
}

func init() {
	wallpaper.RegisterProvider(ProviderName, func(cfg *wallpaper.Config, client *http.Client) provider.ImageProvider {
		return NewProvider(cfg, client)
	})
}

// NewProvider creates a new National Gallery of Art provider.
func NewProvider(cfg *wallpaper.Config, client *http.Client) *Provider {
	return &Provider{cfg: cfg, client: client}
}

func (p *Provider) ID() string { return ProviderName }

func (p *Provider) Name() string { return i18n.T("National Gallery of Art") }

// Title is the full name too: the tray and headers show the museum by its full name.
func (p *Provider) Title() string { return i18n.T("National Gallery of Art") }

func (p *Provider) GetProviderIcon() interface{} { return iconData }

func (p *Provider) Type() provider.ProviderType { return provider.TypeMuseum }

func (p *Provider) GetAttributionType() provider.AttributionType { return provider.AttributionBy }

func (p *Provider) HomeURL() string { return WebBaseURL }

func (p *Provider) SupportsUserQueries() bool { return false }

func (p *Provider) ParseURL(string) (string, error) {
	return "", fmt.Errorf("user queries not supported for the National Gallery of Art")
}

// GetClient implements provider.CustomClientProvider.
func (p *Provider) GetClient() *http.Client { return p.client }

// GetAPIPacing and GetProcessPacing implement provider.PacedProvider. NGA publishes no rate
// limit for its image service, so requests are spaced out a little.
func (p *Provider) GetAPIPacing() time.Duration { return 200 * time.Millisecond }

func (p *Provider) GetProcessPacing() time.Duration { return 500 * time.Millisecond }

// GetDownloadHeaders implements provider.HeaderProvider.
func (p *Provider) GetDownloadHeaders() map[string]string {
	return map[string]string{"User-Agent": "Paintpane"}
}

// FetchImages returns one page of a curated collection. Everything comes from the
// collection file; no request is made here.
func (p *Provider) FetchImages(_ context.Context, query string, page int) ([]provider.Image, error) {
	entry := curation.GetManager().GetEntry(p.ID(), query)
	if entry == nil {
		log.Printf("NGA: unknown collection key %q, falling back to the highlights", query)
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

func (p *Provider) imageFromItem(item curation.PreResolvedItem) (provider.Image, bool) {
	if item.ImageURL == "" {
		return provider.Image{}, false
	}
	w, h := provider.IIIFScaledSize(item.Width, item.Height, maxIIIFEdge)
	return provider.Image{
		ID:          item.ID,
		Path:        provider.IIIFImageURL(item.ImageURL, maxIIIFEdge),
		ViewURL:     item.ViewURL,
		Attribution: fmt.Sprintf("%s - %s", item.Artist, item.Title),
		Title:       item.Title,
		Artist:      item.Artist,
		Year:        item.Year,
		Provider:    p.ID(),
		FileType:    "image/jpeg",
		Width:       w,
		Height:      h,
	}, true
}

// WithResolution implements provider.ResolutionAwareProvider. The request is a square box on
// the screen's longer side, so a portrait work still has enough pixels when it is cropped to
// a landscape screen; it never goes beyond maxIIIFEdge.
func (p *Provider) WithResolution(apiURL string, width, height int) string {
	edge := min(max(width, height), maxIIIFEdge)
	if edge <= 0 {
		return apiURL
	}
	if base := provider.IIIFBase(apiURL, IIIFPrefix); base != "" {
		return provider.IIIFImageURL(base, edge)
	}
	return apiURL
}

// EnrichImage is a no-op: all metadata comes from the collection file.
func (p *Provider) EnrichImage(_ context.Context, img provider.Image) (provider.Image, error) {
	return img, nil
}

// FetchThumbnails implements provider.ThumbnailProvider from the collection file. Each id is a
// JSON-encoded curation.PreResolvedItem, as the gallery generator passes for pre-resolved
// providers.
func (p *Provider) FetchThumbnails(_ context.Context, ids []string) ([]provider.Thumbnail, error) {
	var thumbnails []provider.Thumbnail
	for _, raw := range ids {
		var item curation.PreResolvedItem
		if err := jsonUnmarshal(raw, &item); err != nil {
			log.Printf("NGA: bad pre-resolved item: %v", err)
			continue
		}
		if item.ImageURL == "" {
			continue
		}
		thumbnails = append(thumbnails, provider.Thumbnail{
			ID:          item.ID,
			URL:         provider.IIIFImageURL(item.ImageURL, thumbnailEdge),
			FallbackURL: provider.IIIFImageURL(item.ImageURL, maxIIIFEdge),
			ViewURL:     item.ViewURL,
			Title:       item.Title,
			Artist:      item.Artist,
			Year:        item.Year,
		})
	}
	return thumbnails, nil
}

// --- UI ---

// CreateSettingsPanel returns the museum info panel.
func (p *Provider) CreateSettingsPanel(sm setting.SettingsManager) *schema.PanelSchema {
	return schema.CreateMuseumSettingsPanel(schema.MuseumSettingsConfig{
		ID:          ProviderName,
		Title:       i18n.T("National Gallery of Art"),
		Location:    i18n.T("Washington, DC, USA"),
		LicenseURL:  LicenseURL,
		Description: i18n.T("The national art museum of the United States, with European and American paintings from the Middle Ages to the present, including the only painting by Leonardo da Vinci in the Americas."),
		MapQuery:    "National Gallery of Art Washington DC",
		WebsiteURL:  WebBaseURL,
		DonateURL:   "https://www.nga.gov/support",
	}, sm.OpenURL)
}

// CreateQueryPanel creates the collection toggle panel.
func (p *Provider) CreateQueryPanel(sm setting.SettingsManager, _ string) *schema.PanelSchema {
	return wallpaper.CreateCuratedQueryPanel(p, sm, p.cfg)
}
