package gallery

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png" // embedded thumbnails may be PNG
	"regexp"
	"strings"

	"github.com/disintegration/imaging"
)

// The preview pages are built into the program, so their thumbnails are kept small: they are
// only there to look at in a browser. At 560 px on the long side and JPEG quality 70 a thumbnail
// is about 40 KB, a fifth of the 1000 px quality 90 ones the pages used to carry.
const (
	ThumbnailMaxEdge = 560
	ThumbnailQuality = 70
)

// EncodeThumbnail scales img down to ThumbnailMaxEdge on its long side (never up) and encodes it
// as a JPEG of ThumbnailQuality.
func EncodeThumbnail(img image.Image) ([]byte, error) {
	b := img.Bounds()
	if b.Dx() > ThumbnailMaxEdge || b.Dy() > ThumbnailMaxEdge {
		img = imaging.Fit(img, ThumbnailMaxEdge, ThumbnailMaxEdge, imaging.Lanczos)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: ThumbnailQuality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// The pages are written by html/template, which writes "+" inside an attribute as "&#43;".
var dataURI = regexp.MustCompile(`data:image/(?:jpeg|png);base64,((?:[A-Za-z0-9/=]|\+|&#43;)+)`)

// ShrinkEmbeddedImages re-encodes every base64 image of a generated gallery page with
// EncodeThumbnail, so pages made before the thumbnails were made small can be shrunk without
// fetching the images again. It returns the new page and the number of images it changed; an
// image that cannot be decoded is left as it is.
func ShrinkEmbeddedImages(page string) (string, int, error) {
	changed := 0
	var firstErr error
	out := dataURI.ReplaceAllStringFunc(page, func(uri string) string {
		m := dataURI.FindStringSubmatch(uri)
		raw, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(m[1], "&#43;", "+"))
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("decoding an embedded image: %w", err)
			}
			return uri
		}
		img, _, err := image.Decode(bytes.NewReader(raw))
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("reading an embedded image: %w", err)
			}
			return uri
		}
		small, err := EncodeThumbnail(img)
		if err != nil || len(small) >= len(raw) {
			return uri
		}
		changed++
		return "data:image/jpeg;base64," + strings.ReplaceAll(base64.StdEncoding.EncodeToString(small), "+", "&#43;")
	})
	return out, changed, firstErr
}
