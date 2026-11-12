package gallery

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/jpeg"
	"io/fs"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/asset/galleries"
)

func noisyImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 7), uint8(y * 5), uint8(x ^ y), 255})
		}
	}
	return img
}

func jpegURI(t *testing.T, img image.Image, quality int) string {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}))
	// html/template writes "+" in an attribute as &#43;
	return "data:image/jpeg;base64," + strings.ReplaceAll(base64.StdEncoding.EncodeToString(buf.Bytes()), "+", "&#43;")
}

func decodeURI(t *testing.T, uri string) image.Config {
	t.Helper()
	raw, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(strings.TrimPrefix(uri, "data:image/jpeg;base64,"), "&#43;", "+"))
	require.NoError(t, err)
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	require.NoError(t, err)
	return cfg
}

func TestEncodeThumbnailScalesDownNotUp(t *testing.T) {
	big, err := EncodeThumbnail(noisyImage(1400, 900))
	require.NoError(t, err)
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(big))
	require.NoError(t, err)
	assert.Equal(t, ThumbnailMaxEdge, cfg.Width, "the long side is cut to the maximum")
	assert.InDelta(t, float64(ThumbnailMaxEdge)*900/1400, float64(cfg.Height), 1, "and the shape is kept")

	small, err := EncodeThumbnail(noisyImage(200, 300))
	require.NoError(t, err)
	cfg, err = jpeg.DecodeConfig(bytes.NewReader(small))
	require.NoError(t, err)
	assert.Equal(t, 200, cfg.Width, "a small picture is not scaled up")
	assert.Equal(t, 300, cfg.Height)
}

func TestShrinkEmbeddedImages(t *testing.T) {
	big := jpegURI(t, noisyImage(1000, 1000), 90)
	page := `<img src="` + big + `"><img src="https://example.com/x.jpg"><img src="data:image/jpeg;base64,AAAA">`

	out, n, err := ShrinkEmbeddedImages(page)
	assert.Error(t, err, "the broken image is reported")
	assert.Equal(t, 1, n, "only the real image is re-encoded")
	assert.Contains(t, out, `https://example.com/x.jpg`, "other images are left alone")
	assert.Contains(t, out, `data:image/jpeg;base64,AAAA`, "an image that cannot be read stays as it was")
	assert.Less(t, len(out), len(page)/2, "the page is much smaller")

	start := strings.Index(out, "data:image/jpeg;base64,")
	end := strings.Index(out[start:], `"`)
	cfg := decodeURI(t, out[start:start+end])
	assert.Equal(t, ThumbnailMaxEdge, cfg.Width)
	assert.NotContains(t, out[start:start+end], "+", "a plus sign is written as the page writes it")
}

// The pages built into the program carry small thumbnails (the evaluation of 2026-11-08 chose to
// keep them built in and make them small, about 20 MB instead of 104 MB). A page regenerated with
// cmd/gallerygen uses EncodeThumbnail, so it passes too; an old page does not.
func TestEmbeddedGalleriesAreSmall(t *testing.T) {
	var total int64
	images := 0
	err := fs.WalkDir(galleries.EmbeddedGalleries, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".html") {
			return err
		}
		data, err := fs.ReadFile(galleries.EmbeddedGalleries, path)
		if err != nil {
			return err
		}
		total += int64(len(data))
		for _, m := range dataURI.FindAllString(string(data), -1) {
			images++
			cfg := decodeURI(t, m)
			if cfg.Width > ThumbnailMaxEdge || cfg.Height > ThumbnailMaxEdge {
				t.Errorf("%s has a %dx%d thumbnail, more than %d px; run go run ./cmd/util/shrink_galleries", path, cfg.Width, cfg.Height, ThumbnailMaxEdge)
				break
			}
		}
		return nil
	})
	require.NoError(t, err)
	assert.Greater(t, images, 300, "the pages carry their thumbnails")
	assert.Less(t, total, int64(25_000_000), "all pages together are about 20 MB")
}
