package getty

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/dixieflatline76/Spice/v2/pkg/curation"
	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/setting"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
	"github.com/dixieflatline76/Spice/v2/util/log"
)

// Provider implements the Getty Museum wallpaper provider.
type Provider struct {
	cfg    *wallpaper.Config
	client *http.Client
	mu     sync.RWMutex
}

func init() {
	wallpaper.RegisterProvider(ProviderName, func(cfg *wallpaper.Config, client *http.Client) provider.ImageProvider {
		return NewProvider(cfg, client)
	})
}

// NewProvider creates a new Getty provider.
func NewProvider(cfg *wallpaper.Config, client *http.Client) *Provider {
	return &Provider{cfg: cfg, client: client}
}

func (p *Provider) ID() string      { return ProviderName }
func (p *Provider) HomeURL() string { return "https://www.getty.edu/art/collection/" }

func (p *Provider) Name() string {
	return i18n.T("The Getty")
}

func (p *Provider) Title() string { return "The Getty" }

//go:embed getty.png
var iconData []byte

func (p *Provider) GetProviderIcon() interface{} { return iconData }

func (p *Provider) Type() provider.ProviderType {
	return provider.TypeMuseum
}

func (p *Provider) GetAttributionType() provider.AttributionType {
	return provider.AttributionBy
}

func (p *Provider) SupportsUserQueries() bool { return false }

func (p *Provider) ParseURL(url string) (string, error) {
	return url, nil
}

// GetAPIPacing implements the PacedProvider interface.
func (p *Provider) GetAPIPacing() time.Duration {
	return 200 * time.Millisecond
}

// GetProcessPacing implements the PacedProvider interface.
func (p *Provider) GetProcessPacing() time.Duration {
	return 200 * time.Millisecond
}

// FetchImages fetches images sequentially based on the page number and curated UUIDs.
func (p *Provider) FetchImages(ctx context.Context, query string, page int) ([]provider.Image, error) {
	entry := curation.GetManager().GetEntry(p.ID(), query)
	if entry == nil || entry.Type != "curated" || len(entry.IDs) == 0 {
		return nil, nil
	}
	ids := entry.IDs

	const pageSize = 10
	start := (page - 1) * pageSize
	if start >= len(ids) {
		return nil, nil
	}
	end := start + pageSize
	if end > len(ids) {
		end = len(ids)
	}

	pageIDs := ids[start:end]
	var images []provider.Image

	for _, uuid := range pageIDs {
		img, err := p.fetchObjectByUUID(ctx, uuid)
		if err != nil {
			log.Printf("Getty: Error fetching artwork %s: %v", uuid, err)
			continue
		}
		if img != nil {
			images = append(images, *img)
		}
	}
	return images, nil
}

func (p *Provider) fetchObjectByUUID(ctx context.Context, uuid string) (*provider.Image, error) {
	manifestURL := fmt.Sprintf(apiEndpoint, uuid)

	req, err := http.NewRequestWithContext(ctx, "GET", manifestURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Paintpane")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status from API: %s", resp.Status)
	}

	var obj linkedArtObject
	if err := json.NewDecoder(resp.Body).Decode(&obj); err != nil {
		return nil, err
	}

	img, err := parseObject(obj)
	if err != nil {
		return nil, err
	}

	img.ID = uuid
	if img.ViewURL == "" {
		img.ViewURL = fmt.Sprintf("https://www.getty.edu/art/collection/object/%s", uuid)
	}
	img.Provider = ProviderName

	return img, nil
}

// FetchThumbnails implements provider.ThumbnailProvider.
func (p *Provider) FetchThumbnails(ctx context.Context, ids []string) ([]provider.Thumbnail, error) {
	thumbnails := make([]provider.Thumbnail, len(ids))
	var wg sync.WaitGroup
	limiter := rate.NewLimiter(rate.Every(p.GetAPIPacing()), 1)

	for i, id := range ids {
		wg.Add(1)
		go func(index int, artworkID string) {
			defer wg.Done()
			_ = limiter.Wait(ctx)
			img, err := p.fetchObjectByUUID(ctx, artworkID)
			if err != nil {
				log.Printf("Getty: Failed to fetch %s for thumbnails: %v", artworkID, err)
				return
			}
			if img != nil && img.Path != "" {
				// Prefer a smaller image for gallery preview; fall back to max if resize fails.
				thumbURL := strings.ReplaceAll(img.Path, "/full/max/0/default.jpg", "/full/800,/0/default.jpg")
				thumbnails[index] = provider.Thumbnail{
					ID:          artworkID,
					URL:         thumbURL,
					FallbackURL: img.Path, // /full/max/ as fallback if /800,/ returns 400
					ViewURL:     img.ViewURL,
					Title:       img.Title,
					Artist:      img.Artist,
					Year:        img.Year,
				}
			}
		}(i, id)
	}
	wg.Wait()

	var validThumbnails []provider.Thumbnail
	for _, t := range thumbnails {
		if t.URL != "" {
			validThumbnails = append(validThumbnails, t)
		}
	}
	return validThumbnails, nil
}

// linkedArtObject is the part of a Getty object record (Linked Art, data.getty.edu) that Paintpane
// reads. The record is plain JSON in Linked Art's fixed shape, so it is decoded directly: framing
// it as JSON-LD first (json-gold) gave the same fields but loaded two context documents over the
// network for every record and took most of the CPU of a Getty fetch.
type linkedArtObject struct {
	Type           string                  `json:"type"`
	Label          string                  `json:"_label"`
	ProducedBy     oneOrMany[linkedArtProduction] `json:"produced_by"`
	Representation oneOrMany[linkedArtRef] `json:"representation"`
	SubjectOf      oneOrMany[linkedArtRef] `json:"subject_of"`
}

type linkedArtProduction struct {
	CarriedOutBy oneOrMany[linkedArtRef] `json:"carried_out_by"`
	Timespan     oneOrMany[linkedArtRef] `json:"timespan"`
}

type linkedArtRef struct {
	ID     string `json:"id"`
	Label  string `json:"_label"`
	Format string `json:"format"`
}

// oneOrMany reads a JSON-LD property written either as one value or as a list of them.
type oneOrMany[T any] []T

func (m *oneOrMany[T]) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) > 0 && b[0] == '[' {
		var list []T
		if err := json.Unmarshal(b, &list); err != nil {
			return err
		}
		*m = list
		return nil
	}
	if bytes.Equal(b, []byte("null")) {
		*m = nil
		return nil
	}
	var one T
	if err := json.Unmarshal(b, &one); err != nil {
		return err
	}
	*m = oneOrMany[T]{one}
	return nil
}

// parseObject turns a Getty object record into an image: the first representation is the
// picture (asked for at full size), the text/html page it is the subject of is where to view it.
func parseObject(obj linkedArtObject) (*provider.Image, error) {
	if obj.Type != "HumanMadeObject" {
		return nil, fmt.Errorf("not an object record (type %q)", obj.Type)
	}
	title, author := objectNames(obj)

	imageURL := ""
	if len(obj.Representation) > 0 {
		// Convert IIIF default to max resolution
		imageURL = strings.ReplaceAll(obj.Representation[0].ID, "/full/full/0/default.jpg", "/full/max/0/default.jpg")
	}
	if imageURL == "" {
		return nil, errors.New("no image representation found")
	}

	viewURL := ""
	for _, so := range obj.SubjectOf {
		if so.Format == "text/html" && so.ID != "" {
			viewURL = so.ID
			break
		}
	}

	year := ""
	if len(obj.ProducedBy) > 0 && len(obj.ProducedBy[0].Timespan) > 0 {
		year = obj.ProducedBy[0].Timespan[0].Label
	}

	return &provider.Image{
		Path:        imageURL,
		Attribution: attribution(author, title),
		Title:       title,
		Artist:      author,
		Year:        year,
		ViewURL:     viewURL,
	}, nil
}

// EnrichImage is a no-op
// ScaledDownloadURL implements provider.ScaledDownloadProvider: the IIIF image server sends the
// image already scaled while originals are capped at about 4K.
func (p *Provider) ScaledDownloadURL(ctx context.Context, img provider.Image, fit func(w, h int) (int, int)) (string, bool) {
	return provider.IIIFScaledURL(ctx, p.client, img.Path, img.Width, img.Height, fit)
}

func (p *Provider) EnrichImage(_ context.Context, img provider.Image) (provider.Image, error) {
	return img, nil
}

// --- UI Implementation ---

func (p *Provider) CreateSettingsPanel(sm setting.SettingsManager) *schema.PanelSchema {
	return schema.CreateMuseumSettingsPanel(schema.MuseumSettingsConfig{
		ID:          "Getty",
		Title:       i18n.T("The J. Paul Getty Museum"),
		Location:    i18n.T("Los Angeles, CA, USA"),
		LicenseURL:  "https://www.getty.edu/about/open-content-program/",
		Description: i18n.T("The J. Paul Getty Museum features European paintings, drawings, sculpture, illuminated manuscripts, decorative arts, and photography from its beginnings to the present, gathered internationally."),
		MapQuery:    "J. Paul Getty Museum Los Angeles",
		WebsiteURL:  "https://www.getty.edu/visit/",
		DonateURL:   "https://www.getty.edu/about/join-and-give/",
	}, sm.OpenURL)
}

func (p *Provider) CreateQueryPanel(sm setting.SettingsManager, _ string) *schema.PanelSchema {
	return wallpaper.CreateCuratedQueryPanel(p, sm, p.cfg)
}
