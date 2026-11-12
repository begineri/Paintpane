package smk

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/dixieflatline76/Spice/v2/pkg/curation"
	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/setting"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
	"github.com/dixieflatline76/Spice/v2/util/log"
)

//go:embed smk.png
var iconData []byte

const (
	// ProviderName is the unique identifier used for plugin registration
	ProviderName         = "StatensMuseumForKunst"
	ProviderTitle        = "SMK"
	APIBaseURL           = "https://api.smk.dk/api/v1/art"
	CollectionHighlights = "Best of SMK"

	// maxIIIFEdge bounds the longest side of images requested from SMK's IIIF
	// server when the screen size is not known yet. SMK's "native" files can be
	// tens of thousands of pixels and hundreds of megabytes, which time out.
	maxIIIFEdge = 3840
)

type Config interface {
	GetImgShuffle() bool
	AddStatensMuseumForKunstQuery(desc, url string, active bool) (string, error)
	GetStatensMuseumForKunstQueries() []wallpaper.ImageQuery
	EnableStatensMuseumForKunstQuery(id string) error
	DisableStatensMuseumForKunstQuery(id string) error
	EnableImageQuery(id string) error
	DisableImageQuery(id string) error
	RemoveImageQuery(id string) error
}

type Provider struct {
	cfg         *wallpaper.Config
	httpClient  *http.Client // API requests
	imageClient *http.Client // image downloads (client.go)
}

func init() {
	wallpaper.RegisterProvider(ProviderName, func(cfg *wallpaper.Config, client *http.Client) provider.ImageProvider {
		return NewProvider(cfg, client)
	})
}

func NewProvider(cfg *wallpaper.Config, httpClient *http.Client) *Provider {
	return &Provider{
		cfg:         cfg,
		httpClient:  httpClient,
		imageClient: newImageClient(),
	}
}

func (p *Provider) ID() string {
	return "StatensMuseumForKunst"
}

func (p *Provider) Name() string {
	return i18n.T("Statens Museum for Kunst")
}

func (p *Provider) Title() string {
	return ProviderTitle
}

func (p *Provider) GetProviderIcon() interface{} {
	return iconData
}

// DownloadAttempts implements provider.DownloadAttemptsProvider: the image client already sends a
// second and third request when one stalls (client.go).
func (p *Provider) DownloadAttempts() int {
	return imageDownloadAttempts
}

// GetClient returns the client for image downloads, which is tuned for SMK's image server
// (see client.go). API requests keep the app's shared client.
func (p *Provider) GetClient() *http.Client {
	return p.imageClient
}

func (p *Provider) Type() provider.ProviderType {
	return provider.TypeMuseum
}

func (p *Provider) GetAttributionType() provider.AttributionType {
	return provider.AttributionBy
}

func (p *Provider) HomeURL() string {
	return "https://www.smk.dk/en/"
}

func (p *Provider) SupportsUserQueries() bool {
	return false
}

func (p *Provider) ParseURL(webURL string) (string, error) {
	return "", fmt.Errorf("user queries not supported for SMK")
}

func (p *Provider) FetchImages(ctx context.Context, apiURL string, page int) ([]provider.Image, error) {
	ids, err := p.resolveQueryToIDs(apiURL)
	if err != nil {
		return nil, err
	}

	// Calculate slice bounds
	start := (page - 1) * 50
	if start >= len(ids) {
		return nil, nil // No more images
	}

	end := start + 50
	if end > len(ids) {
		end = len(ids)
	}

	batch := ids[start:end]

	var images []provider.Image

	for _, id := range batch {
		img, err := p.fetchArtworkDetails(ctx, id)
		if err != nil {
			log.Printf("SMK: Error fetching artwork %s: %v", id, err)
			continue
		}
		if img != nil {
			images = append(images, *img)
		}
	}

	return images, nil
}

func (p *Provider) resolveQueryToIDs(query string) ([]string, error) {
	entry := curation.GetManager().GetEntry(p.ID(), query)
	if entry == nil {
		entry = curation.GetManager().GetEntry(p.ID(), CollectionHighlights)
	}
	if entry != nil && entry.Type == "curated" {
		ids := make([]string, len(entry.IDs))
		copy(ids, entry.IDs)
		return ids, nil
	}
	return nil, fmt.Errorf("query not found and fallback unavailable: %s", query)
}

func (p *Provider) fetchArtworkDetails(ctx context.Context, id string) (*provider.Image, error) {
	url := fmt.Sprintf("%s/?object_number=%s", APIBaseURL, id)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		Items []struct {
			Titles []struct {
				Title    string `json:"title"`
				Language string `json:"language"`
			} `json:"titles"`
			Artist         []string `json:"artist"`
			ImageThumbnail string   `json:"image_thumbnail"`
			ImageNative    string   `json:"image_native"`
			ImageIIIFID    string   `json:"image_iiif_id"`
			ImageWidth     int      `json:"image_width"`
			ImageHeight    int      `json:"image_height"`
			FrontendURL    string   `json:"frontend_url"`
			Colors         []string `json:"colors"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Items) == 0 {
		return nil, fmt.Errorf("no item found for id %s", id)
	}

	item := result.Items[0]

	if item.ImageIIIFID == "" && item.ImageNative == "" && item.ImageThumbnail == "" {
		return nil, nil // No image available
	}

	// Prefer a size-bounded IIIF request; the native download is the full scan.
	imgURL := iiif4KImageURL(item.ImageIIIFID, item.ImageWidth, item.ImageHeight)
	if p.highResOriginals() {
		imgURL = iiifLargeImageURL(item.ImageIIIFID, item.ImageWidth, item.ImageHeight)
	}
	if imgURL == "" {
		imgURL = item.ImageNative
	}
	if imgURL == "" {
		imgURL = item.ImageThumbnail
	}

	title := "Unknown"
	if len(item.Titles) > 0 {
		title = item.Titles[0].Title
		for _, t := range item.Titles {
			if t.Language == "en" || t.Language == "engelsk" {
				title = t.Title
			}
		}
	}

	artist := "Unknown"
	if len(item.Artist) > 0 {
		artist = item.Artist[0]
	}

	attribution := fmt.Sprintf("%s, %s", title, artist)

	viewURL := item.FrontendURL
	if viewURL == "" {
		viewURL = fmt.Sprintf("https://open.smk.dk/artwork/image/%s", id)
	}

	return &provider.Image{
		ID:          id,
		Path:        imgURL,
		ViewURL:     viewURL,
		Attribution: attribution,
		Provider:    p.ID(),
		FileType:    "image/jpeg",
	}, nil
}

// highResOriginals reports whether the user asked for originals larger than about 4K (the
// "high-resolution originals" setting, off by default). The setting belongs to the wallpaper
// config; SMK asks for it by method name so it works as soon as the config has it, and reads
// "off" until then. The choice is made when an image is listed, so images already listed keep the
// size they were listed with.
func (p *Provider) highResOriginals() bool {
	if p.cfg == nil {
		return false
	}
	return highResSetting(p.cfg)
}

// highResSetting reads the setting from a config that has it; anything else reads "off".
func highResSetting(cfg any) bool {
	hr, ok := cfg.(interface{ GetHighResOriginals() bool })
	return ok && hr.GetHighResOriginals()
}

// iiif4KImageURL is the default request: the image scaled to fit maxIIIFEdge, or its original size
// when that is smaller. SMK's server enlarges a smaller original to the requested box (a 1834×2263
// original came back as 3112×3840, 8.0 MB instead of 3.8 MB), which adds bytes but no detail.
func iiif4KImageURL(base string, width, height int) string {
	if base == "" {
		return ""
	}
	if width > 0 && height > 0 && max(width, height) <= maxIIIFEdge {
		return strings.TrimRight(base, "/") + "/full/max/0/default.jpg"
	}
	return iiifImageURL(base, maxIIIFEdge)
}

// iiifLargeImageURL is the request for the original size, or for the largest size within
// wallpaper.MaxMasterPixels when the original is larger (originals above that are not
// downloaded at all). Without a known size it falls back to the 4K request.
func iiifLargeImageURL(base string, width, height int) string {
	if base == "" {
		return ""
	}
	if width <= 0 || height <= 0 {
		return iiifImageURL(base, maxIIIFEdge)
	}
	base = strings.TrimRight(base, "/")
	if int64(width)*int64(height) <= wallpaper.MaxMasterPixels {
		return base + "/full/max/0/default.jpg"
	}
	scale := math.Sqrt(float64(wallpaper.MaxMasterPixels) / (float64(width) * float64(height)))
	return fmt.Sprintf("%s/full/!%d,%d/0/default.jpg", base, int(float64(width)*scale), int(float64(height)*scale))
}

// WithResolution implements ResolutionAwareProvider to dynamically scale IIIF images.
// The request is a square box on the screen's longer side, so a portrait work
// still has enough pixels when it is cropped to a landscape screen.
func (p *Provider) WithResolution(apiURL string, width, height int) string {
	edge := max(width, height)
	if edge <= 0 {
		return apiURL
	}
	if base := iiifBase(apiURL); base != "" {
		return iiifImageURL(base, edge)
	}
	return apiURL
}

// iiifImageURL is the IIIF request for an image scaled to fit an edge×edge box.
func iiifImageURL(base string, edge int) string {
	if base == "" {
		return ""
	}
	return fmt.Sprintf("%s/full/!%d,%d/0/default.jpg", strings.TrimRight(base, "/"), edge, edge)
}

// iiifBase returns the IIIF image identifier (".../iiif/jp2/<file>.jp2") from
// an SMK IIIF image URL or from an SMK download URL, which wraps the IIIF
// native-size URL in base64 JSON. Images stored before IIIF requests were used
// carry download URLs. It returns "" for anything else.
func iiifBase(rawURL string) string {
	if i := strings.Index(rawURL, "/api/v1/download/"); i >= 0 {
		rest := rawURL[i+len("/api/v1/download/"):]
		if j := strings.IndexByte(rest, '/'); j >= 0 {
			rest = rest[:j]
		}
		decoded, err := base64.StdEncoding.DecodeString(rest)
		if err != nil {
			decoded, err = base64.RawURLEncoding.DecodeString(strings.TrimRight(rest, "="))
		}
		if err != nil {
			return ""
		}
		var wrapped []struct {
			ImgURL string `json:"img_url"`
		}
		if json.Unmarshal(decoded, &wrapped) != nil || len(wrapped) == 0 {
			return ""
		}
		rawURL = wrapped[0].ImgURL
	}
	if !strings.Contains(rawURL, "/iiif/") {
		return ""
	}
	if i := strings.Index(rawURL, "/full/"); i >= 0 {
		return rawURL[:i]
	}
	return ""
}

// FetchThumbnails implements provider.ThumbnailProvider.
// It fetches the IIIF thumbnail URLs directly rather than the unreliable download API.
func (p *Provider) FetchThumbnails(ctx context.Context, ids []string) ([]provider.Thumbnail, error) {
	var thumbnails []provider.Thumbnail
	for _, id := range ids {
		url := fmt.Sprintf("%s/?object_number=%s", APIBaseURL, id)
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			log.Printf("SMK: Failed to create request for %s: %v", id, err)
			continue
		}

		resp, err := p.httpClient.Do(req)
		if err != nil {
			log.Printf("SMK: Failed to fetch %s for thumbnails: %v", id, err)
			continue
		}

		var result struct {
			Items []struct {
				ImageThumbnail string `json:"image_thumbnail"`
				FrontendURL    string `json:"frontend_url"`
				Titles         []struct {
					Title    string `json:"title"`
					Language string `json:"language"`
				} `json:"titles"`
				Artist []string `json:"artist"`
			} `json:"items"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			log.Printf("SMK: Failed to decode %s: %v", id, err)
			continue
		}
		resp.Body.Close()

		if len(result.Items) == 0 {
			continue
		}

		item := result.Items[0]
		thumbURL := item.ImageThumbnail
		if thumbURL == "" {
			continue
		}

		// Use IIIF thumbnail URL with 800px max dimension for fast, reliable loading
		thumbURL = p.WithResolution(thumbURL, 800, 800)

		viewURL := item.FrontendURL
		if viewURL == "" {
			viewURL = fmt.Sprintf("https://open.smk.dk/artwork/image/%s", id)
		}

		title := "Unknown"
		if len(item.Titles) > 0 {
			title = item.Titles[0].Title
			for _, t := range item.Titles {
				if t.Language == "en" || t.Language == "engelsk" {
					title = t.Title
				}
			}
		}

		artist := "Unknown"
		if len(item.Artist) > 0 {
			artist = item.Artist[0]
		}

		thumbnails = append(thumbnails, provider.Thumbnail{
			ID:      id,
			URL:     thumbURL,
			ViewURL: viewURL,
			Title:   title,
			Artist:  artist,
		})
	}
	return thumbnails, nil
}

// EnrichImage is a no-op
func (p *Provider) EnrichImage(_ context.Context, img provider.Image) (provider.Image, error) {
	return img, nil
}

func (p *Provider) GetDownloadHeaders() map[string]string {
	return map[string]string{
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Accept":          "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8",
		"Accept-Language": "en-US,en;q=0.9",
	}
}

// --- UI Implementation (Pure Go) ---

// CreateSettingsPanel returns the declarative UI for SMK settings.
func (p *Provider) CreateSettingsPanel(sm setting.SettingsManager) *schema.PanelSchema {
	return schema.CreateMuseumSettingsPanel(schema.MuseumSettingsConfig{
		ID:          "StatensMuseumForKunst",
		Title:       ProviderTitle,
		Location:    i18n.T("Copenhagen, Denmark"),
		LicenseURL:  "https://www.smk.dk/en/article/smk-open/",
		Description: i18n.T("Denmark's largest art museum, featuring outstanding collections of Danish and international art from the past seven centuries."),
		MapQuery:    "Statens Museum for Kunst",
		WebsiteURL:  "https://www.smk.dk/en/",
		DonateURL:   "https://www.smk.dk/en/article/donations/",
	}, sm.OpenURL)
}

func (p *Provider) CreateQueryPanel(sm setting.SettingsManager, _ string) *schema.PanelSchema {
	return wallpaper.CreateCuratedQueryPanel(p, sm, p.cfg)
}
