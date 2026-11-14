// Package yale is the Yale University art collections wallpaper provider: the Yale University
// Art Gallery and the Yale Center for British Art.
//
// Both museums publish their records in the LUX portal and their images through IIIF, all
// without a key. To keep the app free of search and parsing code, the collections in
// docs/collections/yale.json carry the title, artist, date, IIIF image address and pixel size
// of every work (built by musewall-tools/collection_parsers/yale.py, outside the repo, from LUX
// and the IIIF manifests, which also state the CC0 rights), and the provider only asks the IIIF server for a scaled
// copy of the image. The longest side requested is at most maxIIIFEdge, unless High-resolution
// originals is on: then the full size, or the largest copy within wallpaper.MaxMasterPixels.
package yale

import (
	"context"
	_ "embed"
	"fmt"
	"net"
	"net/http"
	"net/url"
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

//go:embed yale.png
var iconData []byte

// Provider implements the Yale University Art Collections provider.
type Provider struct {
	cfg    *wallpaper.Config
	client *http.Client

	// highRes reports whether High-resolution originals is on; read when a work is listed.
	highRes func() bool
}

func init() {
	wallpaper.RegisterProvider(ProviderName, func(cfg *wallpaper.Config, client *http.Client) provider.ImageProvider {
		return NewProvider(cfg, client)
	})
}

// NewProvider creates a new Yale University Art Collections provider.
func NewProvider(cfg *wallpaper.Config, _ *http.Client) *Provider {
	return &Provider{cfg: cfg, client: newImageClient(), highRes: cfg.GetHighResOriginals}
}

// newImageClient is the HTTP client for Yale's image server: the app's usual connection limits,
// a scaled request that is slow or fails taken again at full size (maxFallback).
func newImageClient() *http.Client {
	return &http.Client{
		Timeout: imageRequestTimeout,
		Transport: maxFallback{prefix: IIIFPrefix, next: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   wallpaper.HTTPClientDialerTimeout,
				KeepAlive: wallpaper.HTTPClientKeepAlive,
			}).DialContext,
			TLSHandshakeTimeout:   wallpaper.HTTPClientTLSHandshakeTimeout,
			ResponseHeaderTimeout: imageHeaderTimeout,
			MaxIdleConnsPerHost:   2,
		}},
	}
}

// maxFallback asks again for the full-size image ("full/max") when Yale's server does not send a
// scaled copy: an error (such as no answer within imageHeaderTimeout) or a 5xx status. The full
// size is stored and comes at once; the app then shrinks it while originals are capped at about 4K.
type maxFallback struct {
	prefix string
	next   http.RoundTripper
}

func (t maxFallback) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := t.next.RoundTrip(r)
	if err == nil && resp.StatusCode < 500 {
		return resp, nil
	}
	alt, ok := fullMaxURL(r.URL.String(), t.prefix)
	if !ok || r.Method != http.MethodGet || r.Context().Err() != nil {
		return resp, err
	}
	u, perr := url.Parse(alt)
	if perr != nil {
		return resp, err
	}
	why := fmt.Sprint(err)
	if resp != nil {
		why = resp.Status
		resp.Body.Close()
	}
	log.Printf("Yale: no scaled copy of %s (%s); taking the full-size image", r.URL.Path, why)
	r2 := r.Clone(r.Context())
	r2.URL, r2.Host = u, u.Host
	return t.next.RoundTrip(r2)
}

// fullMaxURL turns a scaled IIIF request under prefix into the full-size one. It returns false
// for another server, or a request that already asks for the full size.
func fullMaxURL(raw, prefix string) (string, bool) {
	if !strings.HasPrefix(raw, prefix) {
		return "", false
	}
	i := strings.Index(raw, "/full/")
	if i < 0 {
		return "", false
	}
	rest := raw[i+len("/full/"):]
	j := strings.Index(rest, "/")
	if j <= 0 || rest[:j] == "max" || rest[:j] == "full" {
		return "", false
	}
	return raw[:i] + "/full/max" + rest[j:], true
}

func (p *Provider) ID() string { return ProviderName }

func (p *Provider) Name() string { return i18n.T("Yale University Art Collections") }

// Title is the full name too: the tray and headers show the museum by its full name.
func (p *Provider) Title() string { return i18n.T("Yale University Art Collections") }

func (p *Provider) GetProviderIcon() interface{} { return iconData }

func (p *Provider) Type() provider.ProviderType { return provider.TypeMuseum }

func (p *Provider) GetAttributionType() provider.AttributionType { return provider.AttributionBy }

func (p *Provider) HomeURL() string { return WebBaseURL }

func (p *Provider) SupportsUserQueries() bool { return false }

func (p *Provider) ParseURL(string) (string, error) {
	return "", fmt.Errorf("user queries not supported for the Yale University Art Collections")
}

// GetClient implements provider.CustomClientProvider.
func (p *Provider) GetClient() *http.Client { return p.client }

// GetAPIPacing and GetProcessPacing implement provider.PacedProvider. Yale publishes no rate
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
		log.Printf("Yale: unknown collection key %q, falling back to the highlights", query)
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
	path := provider.IIIFImageURL(item.ImageURL, maxIIIFEdge)
	w, h := provider.IIIFScaledSize(item.Width, item.Height, maxIIIFEdge)
	if p.highRes != nil && p.highRes() {
		// Yale's image server sends the full size (unlike NGA's, which stops at 4096 pixels).
		if u, lw, lh := provider.IIIFLargeImageURL(item.ImageURL, item.Width, item.Height, wallpaper.MaxMasterPixels); u != "" {
			path, w, h = u, lw, lh
		}
	}
	return provider.Image{
		ID:          item.ID,
		Path:        path,
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
// a landscape screen; it never goes beyond maxIIIFEdge. A high-resolution request is left as it is.
func (p *Provider) WithResolution(apiURL string, width, height int) string {
	if p.highRes != nil && p.highRes() {
		return apiURL
	}
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
			log.Printf("Yale: bad pre-resolved item: %v", err)
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
		Title:       i18n.T("Yale University Art Collections"),
		Location:    i18n.T("New Haven, CT, USA"),
		LicenseURL:  LicenseURL,
		Description: i18n.T("The Yale University Art Gallery, the oldest university art museum in the Americas, and the Yale Center for British Art, home to the largest collection of British art outside the United Kingdom."),
		MapQuery:    "Yale University Art Gallery New Haven",
		WebsiteURL:  WebBaseURL,
		DonateURL:   "https://artgallery.yale.edu/support",
	}, sm.OpenURL)
}

// CreateQueryPanel creates the collection toggle panel.
func (p *Provider) CreateQueryPanel(sm setting.SettingsManager, _ string) *schema.PanelSchema {
	return wallpaper.CreateCuratedQueryPanel(p, sm, p.cfg)
}
