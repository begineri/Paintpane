package wikimedia

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/setting"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
	"github.com/dixieflatline76/Spice/v2/util/log"
)

//go:embed Wikimedia.png
var iconData []byte

// CircuitBreaker manages a temporary "open" state when rate limits are hit.
type CircuitBreaker struct {
	mu        sync.RWMutex
	openUntil time.Time
}

func (cb *CircuitBreaker) Trip(duration time.Duration) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.openUntil = time.Now().Add(duration)
}

func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return time.Now().Before(cb.openUntil)
}

func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.openUntil = time.Time{}
}

func (cb *CircuitBreaker) GetCooldownTime() time.Duration {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	if time.Now().After(cb.openUntil) {
		return 0
	}
	return time.Until(cb.openUntil)
}

// Provider implements ImageProvider for Wikimedia Commons
type Provider struct {
	cfg        *wallpaper.Config
	httpClient *http.Client
	baseURL    string
	cb         *CircuitBreaker

	queryTokens map[string]map[int]url.Values
	// scaled holds the 3840 px copy Commons offered for each listed file larger than that, by
	// page ID, for ScaledDownloadURL.
	scaled map[string]string
	mu     sync.Mutex
}

// maxScaledEntries bounds the scaled map; it is emptied when full (a file listed before that is
// then downloaded in full and shrunk after download).
const maxScaledEntries = 5000

// ScaledDownloadURL implements provider.ScaledDownloadProvider: while originals are capped, a file
// wider than 3840 px is downloaded as Commons' 3840 px copy (a standard thumbnail size), which is
// what a 4K screen shows in Full artwork mode. A wide panorama therefore comes out smaller than
// the cap's 2160 px short edge: decoding its original is what took the app past 1.5 GB
// (docs/test-report-v1.0.md §13.5).
func (p *Provider) ScaledDownloadURL(_ context.Context, img provider.Image, fit func(w, h int) (int, int)) (string, bool) {
	if w, _ := fit(img.Width, img.Height); img.Width <= 0 || w >= img.Width {
		return "", false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	u, ok := p.scaled[strings.TrimPrefix(img.ID, p.ID()+"_")]
	return u, ok
}

// NewProvider creates a new instance of Provider
func NewProvider(cfg *wallpaper.Config, client *http.Client) *Provider {
	cb := &CircuitBreaker{}

	p := &Provider{
		cfg:         cfg,
		baseURL:     WikimediaBaseURL,
		cb:          cb,
		queryTokens: make(map[string]map[int]url.Values),
	}
	// Wrap the default client with our 429 circuit breaker round tripper
	p.httpClient = &http.Client{
		Transport: &CircuitBreakerRoundTripper{
			Base: client.Transport,
			CB:   cb,
		},
		Timeout: 0, // Timeout managed per-request via context
	}
	return p
}

// ID returns the provider's unique identifier
func (p *Provider) ID() string {
	return "Wikimedia"
}

// Name returns the source's name: Wikimedia Commons, as the rest of the UI
// calls it. "Wikimedia" alone is the foundation (Donate to Wikimedia).
func (p *Provider) Name() string {
	return i18n.T("Wikimedia Commons")
}

func (p *Provider) Type() provider.ProviderType {
	return provider.TypeCommunity
}

func (p *Provider) GetAttributionType() provider.AttributionType {
	return provider.AttributionBy
}

func (p *Provider) SupportsUserQueries() bool {
	return true
}

func (p *Provider) HomeURL() string {
	return "https://commons.wikimedia.org"
}

func (p *Provider) Title() string {
	return "Wikimedia Commons"
}

func (p *Provider) GetProviderIcon() interface{} {
	return iconData
}

func (p *Provider) IsThrottled() bool {
	return p.cb.IsOpen()
}

// ParseURL determines if the input is a Search term, a Category, or a direct URL.
func (p *Provider) ParseURL(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", errors.New("input cannot be empty")
	}

	if normalized, ok := p.checkPrefixNormalization(input); ok {
		return normalized, nil
	}

	if normalized, ok, err := p.handleCategoryInput(input); ok || err != nil {
		return normalized, err
	}

	if strings.HasPrefix(input, "http") {
		return p.handleHttpInput(input)
	}

	return "search:" + input, nil
}

func (p *Provider) checkPrefixNormalization(input string) (string, bool) {
	lowerInput := strings.ToLower(input)
	if strings.HasPrefix(lowerInput, "search:") {
		return "search:" + input[7:], true
	}
	if strings.HasPrefix(lowerInput, "category:") {
		return "category:" + input[9:], true
	}
	if strings.HasPrefix(lowerInput, "page:") {
		return "page:" + input[5:], true
	}
	if strings.HasPrefix(lowerInput, "file:") {
		if strings.HasPrefix(lowerInput, "file:file:") {
			return input, true
		}
		return "file:" + input, true
	}
	return "", false
}

func (p *Provider) handleCategoryInput(input string) (string, bool, error) {
	catRegex := regexp.MustCompile(WikimediaCategoryRegexp)
	if !catRegex.MatchString(input) {
		return "", false, nil
	}

	if strings.HasPrefix(input, "http") {
		u, err := url.Parse(input)
		if err != nil {
			return "", false, err
		}
		parts := strings.Split(u.Path, "/")
		if len(parts) > 0 {
			input = parts[len(parts)-1]
		}
	}
	lowerInput := strings.ToLower(input)
	if strings.HasPrefix(lowerInput, "category:") {
		return "category:" + input[9:], true, nil
	}
	if strings.HasPrefix(lowerInput, "category%3a") {
		return "category:" + input[11:], true, nil
	}
	return "category:" + strings.TrimPrefix(input, "Category:"), true, nil
}

func (p *Provider) handleHttpInput(input string) (string, error) {
	domainRegex := regexp.MustCompile(WikimediaDomainRegexp)
	if !domainRegex.MatchString(input) {
		return "", errors.New("invalid Wikimedia URL: must be commons.wikimedia.org")
	}

	u, err := url.Parse(input)
	if err != nil {
		return "", err
	}

	if u.Path == "/w/index.php" || strings.Contains(u.Path, "Special:MediaSearch") {
		searchParam := u.Query().Get("search")
		if searchParam != "" {
			return "search:" + searchParam, nil
		}
	}

	if strings.Contains(u.Path, "/wiki/File:") {
		parts := strings.Split(u.Path, "/wiki/")
		if len(parts) > 1 {
			return "file:" + parts[1], nil
		}
	}

	if strings.Contains(u.Path, "/wiki/") {
		parts := strings.Split(u.Path, "/wiki/")
		if len(parts) > 1 {
			return "page:" + parts[1], nil
		}
	}

	return "", errors.New(i18n.T("only 'Category:', 'File:' or component Search URLs are currently supported directly"))
}

type wikimediaResponse struct {
	BatchComplete string `json:"batchcomplete"`
	// Continue holds every parameter the next request must repeat. Besides the generator's
	// (gcmcontinue, gimcontinue, gsroffset) there is iicontinue: imageinfo with a scaled copy
	// covers 50 files per request, so the rest of a batch of 100 comes with iicontinue and the
	// generator's own parameter only after that.
	Continue map[string]json.RawMessage `json:"continue"`
	Query    struct {
		Pages map[string]struct {
			PageID    int    `json:"pageid"`
			Title     string `json:"title"`
			ImageInfo []struct {
				URL            string `json:"url"`
				DescriptionURL string `json:"descriptionurl"`
				Width          int    `json:"width"`
				Height         int    `json:"height"`
				ThumbURL       string `json:"thumburl"`
				ThumbWidth     int    `json:"thumbwidth"`
				ThumbHeight    int    `json:"thumbheight"`
				ExtMetadata    struct {
					ObjectName struct {
						Value string `json:"value"`
					} `json:"ObjectName"`
					Artist struct {
						Value string `json:"value"`
					} `json:"Artist"`
					LicenseShortName struct {
						Value string `json:"value"`
					} `json:"LicenseShortName"`
				} `json:"extmetadata"`
			} `json:"imageinfo"`
		} `json:"pages"`
	} `json:"query"`
	Error *struct {
		Code string `json:"code"`
		Info string `json:"info"`
	} `json:"error"`
}

func (p *Provider) WithResolution(query string, width, height int) string {
	constraint := fmt.Sprintf(" filew:>%d fileh:>%d", width, height)
	if strings.HasPrefix(query, "search:") {
		return query + constraint
	}
	if strings.HasPrefix(query, "category:") {
		catName := strings.TrimPrefix(query, "category:")
		return fmt.Sprintf("search:incategory:\"%s\"%s", catName, constraint)
	}
	return query
}

const (
	// scaledWidth is the width of the scaled copy requested from Commons (a standard thumbnail size).
	scaledWidth = 3840
	// maxOriginalPixels is the largest original that is downloaded as is; bigger ones use the scaled
	// copy. Matches wallpaper.MaxMasterPixels.
	maxOriginalPixels = 100_000_000
)

// unsupportedExts are Commons file types the wallpaper pipeline cannot decode (vector, animated,
// document, audio/video, and TIFF, whose LZW variants the decoder rejects).
var unsupportedExts = map[string]bool{
	".svg": true, ".gif": true, ".pdf": true, ".ogg": true, ".ogv": true, ".webm": true,
	".tif": true, ".tiff": true,
}

// isUnsupportedFormat reports whether the file at rawURL is a format we cannot decode. It looks at
// the URL path only: the API returns URLs such as ".../Jovian_Tempest.tif?utm_source=commons..." and
// a suffix check on the whole string would miss them, so the download would fail after the fact.
func isUnsupportedFormat(rawURL string) bool {
	path := rawURL
	if u, err := url.Parse(rawURL); err == nil {
		path = u.Path
	}
	i := strings.LastIndex(path, ".")
	return i >= 0 && unsupportedExts[strings.ToLower(path[i:])]
}

func (p *Provider) FetchImages(ctx context.Context, query string, page int) ([]provider.Image, error) {
	var allImages []provider.Image
	isPageQuery := strings.HasPrefix(query, "page:")

	p.mu.Lock()
	if p.queryTokens[query] == nil {
		p.queryTokens[query] = make(map[int]url.Values)
	}

	var continueParams url.Values
	if page > 1 {
		tokens, ok := p.queryTokens[query][page]
		if !ok {
			p.mu.Unlock()
			return nil, nil
		}
		continueParams = tokens
	}
	p.mu.Unlock()

	var apiURL string
	var params url.Values

	if strings.HasPrefix(query, "category:") {
		catTitle := strings.TrimPrefix(query, "category:")
		apiURL = p.baseURL
		params = url.Values{}
		params.Set("action", "query")
		params.Set("generator", "categorymembers")
		params.Set("gcmtitle", "Category:"+catTitle)
		params.Set("gcmtype", "file")
		params.Set("gcmlimit", "100")
		params.Set("prop", "imageinfo")
		params.Set("iiprop", "url|size|extmetadata")
		params.Set("format", "json")
	} else if strings.HasPrefix(query, "search:") {
		searchTerm := strings.TrimPrefix(query, "search:")
		apiURL = p.baseURL
		params = url.Values{}
		params.Set("action", "query")
		params.Set("generator", "search")
		params.Set("gsrsearch", searchTerm)
		params.Set("gsrnamespace", "6")
		params.Set("gsrlimit", "100")
		params.Set("prop", "imageinfo")
		params.Set("iiprop", "url|size|extmetadata")
		params.Set("format", "json")
	} else if strings.HasPrefix(query, "file:") {
		fileTitle := strings.TrimPrefix(query, "file:")
		apiURL = p.baseURL
		params = url.Values{}
		params.Set("action", "query")
		params.Set("titles", fileTitle)
		params.Set("prop", "imageinfo")
		params.Set("iiprop", "url|size|extmetadata")
		params.Set("format", "json")
	} else if strings.HasPrefix(query, "page:") {
		pageTitle := strings.TrimPrefix(query, "page:")
		apiURL = p.baseURL
		params = url.Values{}
		params.Set("action", "query")
		params.Set("titles", pageTitle)
		params.Set("generator", "images")
		params.Set("gimlimit", "200")
		params.Set("prop", "imageinfo")
		params.Set("iiprop", "url|size|extmetadata")
		params.Set("format", "json")
	} else {
		return nil, errors.New("unknown query format")
	}

	// Ask for a scaled copy next to the original: some Commons files are 200 MP and 200 MB.
	params.Set("iiurlwidth", strconv.Itoa(scaledWidth))

	for k, v := range continueParams {
		params.Set(k, v[0])
	}

	fullURL := apiURL + "?" + params.Encode()
	var result wikimediaResponse
	err := p.doWithRetry(ctx, fullURL, &result)
	if err != nil {
		return nil, err
	}
	if result.Error != nil {
		if page > 1 {
			// A cursor saved in an earlier run can go stale ("badcontinue"). Nothing on this page
			// tells the app to start the query over from page 1.
			log.Printf("Wikimedia: page %d of %q failed (%s: %s); starting the query over", page, query, result.Error.Code, result.Error.Info)
			p.mu.Lock()
			delete(p.queryTokens[query], page)
			p.mu.Unlock()
			return nil, nil
		}
		return nil, fmt.Errorf("wikimedia API error %s: %s", result.Error.Code, result.Error.Info)
	}

	for _, pageData := range result.Query.Pages {
		if len(pageData.ImageInfo) == 0 {
			continue
		}
		info := pageData.ImageInfo[0]
		if isUnsupportedFormat(info.URL) {
			continue
		}

		if isPageQuery && info.Width > 0 && info.Height > 0 {
			aspect := float64(info.Width) / float64(info.Height)
			if aspect < 1.1 {
				continue
			}
		}

		artist := sanitizeAttribution(info.ExtMetadata.Artist.Value)
		if isUnknownAuthor(artist) {
			artist = "" // the tray then shows no artist (A2 §12.9 #2)
		}
		attribution := joinAttribution(artist, sanitizeAttribution(info.ExtMetadata.LicenseShortName.Value))
		title := imageTitle(info.ExtMetadata.ObjectName.Value, pageData.Title)

		path, width, height := info.URL, info.Width, info.Height
		if int64(width)*int64(height) > maxOriginalPixels {
			// Too big to decode safely. Download the scaled copy instead, or skip the file when
			// Commons did not offer one.
			if info.ThumbURL == "" || info.ThumbWidth <= 0 || info.ThumbHeight <= 0 {
				continue
			}
			path, width, height = info.ThumbURL, info.ThumbWidth, info.ThumbHeight
		} else if info.ThumbURL != "" && info.ThumbWidth > 0 && info.ThumbWidth < width {
			p.mu.Lock()
			if p.scaled == nil || len(p.scaled) >= maxScaledEntries {
				p.scaled = make(map[string]string)
			}
			p.scaled[strconv.Itoa(pageData.PageID)] = info.ThumbURL
			p.mu.Unlock()
		}

		allImages = append(allImages, provider.Image{
			ID:          strconv.Itoa(pageData.PageID),
			Provider:    p.ID(),
			Path:        path,
			ViewURL:     FilePageURL(info.DescriptionURL, pageData.PageID),
			Attribution: attribution,
			Title:       title,
			Artist:      artist,
			FileType:    "image/jpeg",
			Width:       width,
			Height:      height,
		})
	}

	if nextParams := nextPageParams(result.Continue); len(nextParams) > 0 {
		p.mu.Lock()
		p.queryTokens[query][page+1] = nextParams
		p.mu.Unlock()
	}

	return allImages, nil
}

// nextPageParams turns the API's "continue" object into the parameters of the next request, all of
// them as the API gave them (strings, or numbers such as gsroffset).
func nextPageParams(c map[string]json.RawMessage) url.Values {
	next := url.Values{}
	for k, raw := range c {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			s = strings.TrimSpace(string(raw)) // a number
		}
		if s != "" {
			next.Set(k, s)
		}
	}
	if next.Get("continue") == "" {
		return url.Values{} // not a continuation the API understands
	}
	return next
}

// PageCursor implements provider.PageCursorProvider: the continue parameters for page of query.
func (p *Provider) PageCursor(query string, page int) (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	v, ok := p.queryTokens[query][page]
	if !ok {
		return "", false
	}
	return v.Encode(), true
}

// SetPageCursor implements provider.PageCursorProvider.
func (p *Provider) SetPageCursor(query string, page int, cursor string) {
	v, err := url.ParseQuery(cursor)
	if err != nil || page <= 1 || len(v) == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.queryTokens[query] == nil {
		p.queryTokens[query] = make(map[int]url.Values)
	}
	p.queryTokens[query][page] = v
}

func (p *Provider) doWithRetry(ctx context.Context, fullURL string, target interface{}) error {
	return p.doRequest(ctx, fullURL, target)
}

func (p *Provider) doRequest(ctx context.Context, fullURL string, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", WikimediaUserAgent)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return err
	}
	return nil
}

func (p *Provider) GetClient() *http.Client {
	return p.httpClient
}

const (
	WikimediaAPIPacing       = 5 * time.Second
	WikimediaMediaPacing     = 60 * time.Second
	WikimediaDefaultCooldown = 15 * time.Minute
)

type CircuitBreakerRoundTripper struct {
	Base http.RoundTripper
	CB   *CircuitBreaker
}

func (t *CircuitBreakerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.CB.IsOpen() {
		return nil, fmt.Errorf("wikimedia circuit breaker: open")
	}

	var resp *http.Response
	var err error

	if t.Base != nil {
		resp, err = t.Base.RoundTrip(req)
	} else {
		resp, err = http.DefaultTransport.RoundTrip(req)
	}

	if err != nil {
		return nil, err
	}

	if resp.StatusCode == 429 {
		resp.Body.Close()
		coolDown := WikimediaDefaultCooldown
		if ra := resp.Header.Get("Retry-After"); ra != "" {
			if seconds, err := strconv.Atoi(ra); err == nil {
				coolDown = time.Duration(seconds) * time.Second
			}
		}
		t.CB.Trip(coolDown)
		return nil, fmt.Errorf("wikimedia circuit breaker: rate limited (429)")
	}

	return resp, nil
}

func (p *Provider) GetAPIPacing() time.Duration {
	return WikimediaAPIPacing
}

func (p *Provider) GetProcessPacing() time.Duration {
	return WikimediaMediaPacing
}

func (p *Provider) EnrichImage(ctx context.Context, img provider.Image) (provider.Image, error) {
	return img, nil
}

func (p *Provider) GetDownloadHeaders() map[string]string {
	return map[string]string{
		"User-Agent": WikimediaUserAgent,
		"Referer":    "https://github.com/begineri/Paintpane",
	}
}

// --- UI Integration (Pure Go) ---

// CreateSettingsPanel returns the declarative UI for Wikimedia settings.
func (p *Provider) CreateSettingsPanel(_ setting.SettingsManager) *schema.PanelSchema {
	return &schema.PanelSchema{
		Sections: []schema.SectionSchema{
			{
				Title:   i18n.T("Wikimedia Commons"),
				Compact: true,
				Items: []schema.ItemSchema{
					schema.HyperlinkItem{
						Text: i18n.T("Donate to Wikimedia"),
						URL:  "https://donate.wikimedia.org/",
					},
				},
			},
		},
	}
}

// CreateQueryPanel creates the image query management panel.
func (p *Provider) CreateQueryPanel(sm setting.SettingsManager, pendingUrl string) *schema.PanelSchema {
	addCfg := schema.AddQueryConfig{
		Title:           i18n.T("Add Wikimedia collection"),
		URLLabel:        i18n.T("Category or search term"),
		URLPlaceholder:  i18n.T("Category:Nature or Search:Pacific Ocean"),
		URLValidator:    `^.*$`, // Wikimedia is flexible: anything is accepted, so there is no message
		DescPlaceholder: i18n.T("e.g. Nature"),
		AddHandler: func(desc, url string, active bool) (string, error) {
			apiURL, err := p.ParseURL(url)
			if err != nil {
				return "", err
			}
			return p.cfg.AddWikimediaQuery(desc, apiURL, active)
		},
	}

	if pendingUrl != "" {
		sm.ShowAddQueryDialog(addCfg, pendingUrl, "", sm.RefreshUI)
	}

	return &schema.PanelSchema{
		Sections: []schema.SectionSchema{
			{
				Title:   i18n.T("Wikimedia queries"),
				Compact: true,
				Items: []schema.ItemSchema{
					schema.ButtonItem{
						Name:       "wikimedia_add",
						ButtonText: i18n.T("Add new collection"),
						IconName:   "add",
						OnPressed: func() {
							sm.ShowAddQueryDialog(addCfg, "", "", sm.RefreshUI)
						},
					},
					schema.QueryListItem{
						GetQueries: func() []schema.Query {
							queries := p.cfg.GetQueries()
							var abstracts []schema.Query
							for _, q := range queries {
								if q.Provider == p.ID() {
									abstracts = append(abstracts, schema.Query{
										ID:          q.ID,
										URL:         q.URL,
										Description: q.Description,
										Active:      q.Active,
										Managed:     q.Managed,
									})
								}
							}
							return abstracts
						},
						EnableQuery:  p.cfg.EnableImageQuery,
						DisableQuery: p.cfg.DisableImageQuery,
						RemoveQuery:  p.cfg.RemoveImageQuery,
						GetDisplayURL: func(q schema.Query) *url.URL {
							return p.getDisplayURL(wallpaper.ImageQuery{URL: q.URL})
						},
					},
				},
			},
		},
	}
}

func (p *Provider) getDisplayURL(q wallpaper.ImageQuery) *url.URL {
	queryURL := q.URL
	lowerURL := strings.ToLower(queryURL)
	var displayURL string
	if strings.HasPrefix(lowerURL, "category:") {
		catName := queryURL[9:]
		displayURL = "https://commons.wikimedia.org/wiki/Category:" + url.PathEscape(catName)
	} else if strings.HasPrefix(lowerURL, "search:") {
		displayURL = "https://commons.wikimedia.org/w/index.php?search=" + url.QueryEscape(queryURL[7:])
	} else if strings.HasPrefix(lowerURL, "page:") {
		pageName := queryURL[5:]
		displayURL = "https://commons.wikimedia.org/wiki/" + pageName
	} else {
		displayURL = queryURL
	}
	if u, err := url.Parse(displayURL); err == nil && u.Scheme != "" {
		return u
	}
	return nil
}

func init() {
	wallpaper.RegisterProvider("Wikimedia", func(cfg *wallpaper.Config, client *http.Client) provider.ImageProvider {
		return NewProvider(cfg, client)
	})
}

var (
	htmlTagRe    = regexp.MustCompile("<[^>]*>")
	whiteSpaceRe = regexp.MustCompile(`[\s\p{Zs}]+`) // also &nbsp;
	fileExtRe    = regexp.MustCompile(`\.[A-Za-z0-9]{2,5}$`)
	// cameraNameRe matches file names a camera or phone gives, which say
	// nothing about the work: "IMG_1234", "DSC01234", "20190512_123456".
	cameraNameRe = regexp.MustCompile(`(?i)^(img|dsc[nf]?|dcim|pict|photo|image|p)?[\d\s_.()\-]*$`)
)

// imageTitle is the work's title: the ObjectName Commons gives, else the file
// name without "File:" and its extension. A name that is only a camera's file
// number is no title, so the tray shows its text for a missing title.
func imageTitle(objectName, pageTitle string) string {
	if t := usableTitle(sanitizeAttribution(objectName)); t != "" {
		return t
	}
	name := strings.TrimPrefix(strings.TrimSpace(pageTitle), "File:")
	name = fileExtRe.ReplaceAllString(name, "")
	name = strings.ReplaceAll(name, "_", " ")
	return usableTitle(whiteSpaceRe.ReplaceAllString(name, " "))
}

func usableTitle(t string) string {
	t = strings.TrimSpace(t)
	if cameraNameRe.MatchString(t) {
		return ""
	}
	return t
}

// sanitizeAttribution turns an HTML credit from Commons into plain text. A tag
// becomes a space, so text in separate elements does not run together
// ("Unknown author" + "Unknown author or not provided").
func sanitizeAttribution(input string) string {
	output := htmlTagRe.ReplaceAllString(input, " ")
	output = html.UnescapeString(output)
	output = whiteSpaceRe.ReplaceAllString(output, " ")
	return strings.TrimSpace(output)
}

// isUnknownAuthor reports whether a credit names no artist: it says the author
// is not known, as Commons' {{Unknown|author}} template does, or that the work
// is anonymous ("Anonymous Unknown author", "Anonymous ( Spain ) Unknown
// author 17 th century …"). The tray then shows no artist.
func isUnknownAuthor(artist string) bool {
	a := strings.ToLower(strings.TrimSpace(artist))
	return a == "" || strings.HasPrefix(a, "unknown") || strings.HasPrefix(a, "anonymous")
}

// joinAttribution writes "artist (licence)", leaving out what is missing.
func joinAttribution(artist, license string) string {
	switch {
	case artist == "":
		if license == "" {
			return ""
		}
		return "(" + license + ")"
	case license == "":
		return artist
	}
	return artist + " (" + license + ")"
}

// FilePageURL is the Commons page of a file: its description page, with the title, author,
// licence and source. Without one from the API it is the page by its page ID. It used to be the
// file itself (imageinfo's url), so the tray's first row opened the bare picture, several
// megabytes of it, instead of the page about it.
func FilePageURL(descriptionURL string, pageID int) string {
	if descriptionURL != "" {
		return descriptionURL
	}
	return fmt.Sprintf("https://commons.wikimedia.org/w/index.php?curid=%d", pageID)
}
