package wallpaper

import (
	"bufio"
	"bytes"
	"fmt"
	"mime"
	"net/http"
	"strings"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
)

// sniffLen is how much of a download is looked at before it is saved (as much as
// http.DetectContentType reads).
const sniffLen = 512

// notImageError is returned when a server answers an image request with a web page instead of
// the image. Asking again at once gets the same page, so the download is not retried, and nothing
// is written to the cache.
type notImageError struct {
	provider    string
	contentType string
	// botCheck is set when the page comes from a bot-protection service (Imperva, Cloudflare).
	// The service answers every image of the source the same way, so the source is paused.
	botCheck bool
}

func (e *notImageError) Error() string {
	ct := e.contentType
	if ct == "" {
		ct = "no content type"
	}
	if e.botCheck {
		return fmt.Sprintf("%s: the image server answered with a bot-protection page (%s) instead of the image", e.provider, ct)
	}
	return fmt.Sprintf("%s: the image server answered with a web page (%s) instead of the image", e.provider, ct)
}

// checkImageResponse returns a *notImageError when resp, a 200 answer to an image request, carries
// an HTML page. It only peeks at body, so the caller still copies the whole download from it.
// Other bodies pass: some servers send images as application/octet-stream or without a type, and
// whether the bytes decode is checked later.
func checkImageResponse(resp *http.Response, body *bufio.Reader, imgProvider provider.ImageProvider) error {
	peek, _ := body.Peek(sniffLen) // a short body returns what there is
	declared := resp.Header.Get("Content-Type")
	mediaType, _, _ := mime.ParseMediaType(declared)
	isHTML := mediaType == "text/html" || mediaType == "application/xhtml+xml" ||
		strings.HasPrefix(http.DetectContentType(peek), "text/html")
	if !isHTML {
		return nil
	}
	name := "Unknown"
	if imgProvider != nil {
		name = imgProvider.ID()
	}
	return &notImageError{provider: name, contentType: declared, botCheck: isBotCheck(resp, peek)}
}

// isBotCheck reports whether an HTML answer comes from a bot-protection service, by the headers
// and cookies such services set or the words of their challenge page.
func isBotCheck(resp *http.Response, peek []byte) bool {
	if resp.Header.Get("X-Iinfo") != "" || resp.Header.Get("Cf-Mitigated") != "" {
		return true
	}
	for _, c := range resp.Header.Values("Set-Cookie") {
		if strings.HasPrefix(c, "visid_incap_") || strings.HasPrefix(c, "incap_ses_") {
			return true
		}
	}
	for _, marker := range [][]byte{[]byte("Pardon Our Interruption"), []byte("Incapsula"), []byte("Just a moment...")} {
		if bytes.Contains(peek, marker) {
			return true
		}
	}
	return false
}
