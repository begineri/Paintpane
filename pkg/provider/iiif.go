package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// iiifImageRe matches an IIIF Image API request for the whole image:
// {base}/full/{size}/0/{quality}.{format}.
var iiifImageRe = regexp.MustCompile(`^(https?://.+)/full/([^/]+)/0/(default|color|gray|bitonal)\.(jpg|png)$`)

// iiifInfoTimeout bounds the info.json request made to learn an image's size.
const iiifInfoTimeout = 20 * time.Second

// iiifInfo is the part of an IIIF info.json that says how large the image is and how large the
// server will serve it (maxWidth and maxArea are IIIF 3; IIIF 2 puts them in profile[1]). The
// profile is a list in IIIF 2 and a single string ("level2") in IIIF 3.
type iiifInfo struct {
	Width     int             `json:"width"`
	Height    int             `json:"height"`
	MaxWidth  int             `json:"maxWidth"`
	MaxHeight int             `json:"maxHeight"`
	MaxArea   int64           `json:"maxArea"`
	Profile   json.RawMessage `json:"profile"`
}

// limits returns the largest width, height and area the server serves, 0 when unlimited.
func (i iiifInfo) limits() (w, h int, area int64) {
	w, h, area = i.MaxWidth, i.MaxHeight, i.MaxArea
	var profile []json.RawMessage
	_ = json.Unmarshal(i.Profile, &profile) // stays empty for IIIF 3's string
	for _, raw := range profile {
		var p struct {
			MaxWidth  int   `json:"maxWidth"`
			MaxHeight int   `json:"maxHeight"`
			MaxArea   int64 `json:"maxArea"`
		}
		if json.Unmarshal(raw, &p) != nil {
			continue // a compliance level URI
		}
		if w == 0 {
			w = p.MaxWidth
		}
		if h == 0 {
			h = p.MaxHeight
		}
		if area == 0 {
			area = p.MaxArea
		}
	}
	return w, h, area
}

// IIIFScaledURL turns an IIIF Image API URL for the whole image into one for the image scaled to
// the size fit gives, as "{base}/full/{w},/0/default.jpg". width and height are the full size;
// when either is unknown it is read from the image's info.json with client. It returns false when
// the URL is not such a request, the size cannot be learned, the image is no larger than fit
// allows, or the server already serves it smaller than that.
func IIIFScaledURL(ctx context.Context, client *http.Client, imageURL string, width, height int, fit func(w, h int) (int, int)) (string, bool) {
	m := iiifImageRe.FindStringSubmatch(imageURL)
	if m == nil {
		return "", false
	}
	base, quality, format := m[1], m[3], m[4]
	var info iiifInfo
	if width <= 0 || height <= 0 {
		var err error
		info, err = fetchIIIFInfo(ctx, client, base)
		if err != nil || info.Width <= 0 || info.Height <= 0 {
			return "", false
		}
		width, height = info.Width, info.Height
	}
	w, h := fit(width, height)
	if w <= 0 || h <= 0 || w >= width {
		return "", false
	}
	if mw, mh, ma := info.limits(); (mw > 0 && mw < w) || (mh > 0 && mh < h) || (ma > 0 && ma < int64(w)*int64(h)) {
		return "", false // "max" is already smaller than the cap
	}
	return fmt.Sprintf("%s/full/%d,/0/%s.%s", base, w, quality, format), true
}

func fetchIIIFInfo(ctx context.Context, client *http.Client, base string) (iiifInfo, error) {
	if client == nil {
		client = http.DefaultClient
	}
	ctx, cancel := context.WithTimeout(ctx, iiifInfoTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(base, "/")+"/info.json", nil)
	if err != nil {
		return iiifInfo{}, err
	}
	req.Header.Set("Accept", "application/json, application/ld+json")
	resp, err := client.Do(req)
	if err != nil {
		return iiifInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return iiifInfo{}, fmt.Errorf("info.json: status %d", resp.StatusCode)
	}
	var info iiifInfo
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&info); err != nil {
		return iiifInfo{}, err
	}
	return info, nil
}
