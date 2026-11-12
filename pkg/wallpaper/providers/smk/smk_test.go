package smk

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
)

func TestParseURL(t *testing.T) {
	p := &Provider{}

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "Any URL",
			url:     "https://www.smk.dk/en/",
			wantErr: true, // we don't support arbitrary URLs yet since everything is curated
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := p.ParseURL(tt.url)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWithResolution(t *testing.T) {
	p := &Provider{}

	tests := []struct {
		name     string
		url      string
		width    int
		height   int
		expected string
	}{
		{
			name:     "IIIF Full Region",
			url:      "https://iip.smk.dk/iiif/jp2/KMS1621.tif.jp2/full/!2048,2048/0/default.jpg",
			width:    1920,
			height:   1080,
			expected: "https://iip.smk.dk/iiif/jp2/KMS1621.tif.jp2/full/!1920,1920/0/default.jpg",
		},
		{
			name:  "Native IIIF URL",
			url:   "https://iip.smk.dk/iiif/jp2/3197xr831_KMS3696.tif.jp2/full/full/0/native.jpg",
			width: 2560, height: 1600,
			expected: "https://iip.smk.dk/iiif/jp2/3197xr831_KMS3696.tif.jp2/full/!2560,2560/0/default.jpg",
		},
		{
			// Download URLs wrap the native-size IIIF URL in base64 JSON. Images
			// stored by earlier versions still carry them.
			name:  "Download API URL",
			url:   "https://api.smk.dk/api/v1/download/W3siaW1nX3VybCI6Imh0dHBzOi8vaWlwLnNtay5kay9paWlmL2pwMi8zMTk3eHI4MzFfS01TMzY5Ni50aWYuanAyL2Z1bGwvZnVsbC8wL25hdGl2ZS5qcGciLCJwdWJsaWNfZG9tYWluIjp0cnVlLCJvYmplY3RfbnVtYmVyIjoiS01TMzY5NiIsIm51bSI6IiJ9XQ==/KMS3696.jpg",
			width: 2560, height: 1600,
			expected: "https://iip.smk.dk/iiif/jp2/3197xr831_KMS3696.tif.jp2/full/!2560,2560/0/default.jpg",
		},
		{
			name:  "Thumbnail URL",
			url:   "https://iip-thumb.smk.dk/iiif/jp2/3197xr831_KMS3696.tif.jp2/full/!1024,/0/default.jpg",
			width: 800, height: 800,
			expected: "https://iip-thumb.smk.dk/iiif/jp2/3197xr831_KMS3696.tif.jp2/full/!800,800/0/default.jpg",
		},
		{
			name:     "Unknown size",
			url:      "https://iip.smk.dk/iiif/jp2/KMS1621.tif.jp2/full/full/0/native.jpg",
			expected: "https://iip.smk.dk/iiif/jp2/KMS1621.tif.jp2/full/full/0/native.jpg",
		},
		{
			name:     "Non IIIF URL",
			url:      "https://example.com/image.jpg",
			width:    1920,
			height:   1080,
			expected: "https://example.com/image.jpg", // unchanged
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.WithResolution(tt.url, tt.width, tt.height)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestLargeImageURL(t *testing.T) {
	base := "https://iip.smk.dk/iiif/jp2/9g54xm869_KMS1-cropped.tif.jp2"
	// 7174×5536 is 40 MP: the original.
	assert.Equal(t, base+"/full/max/0/default.jpg", iiifLargeImageURL(base, 7174, 5536))
	// 20000×10000 is 200 MP: the largest size within 100 MP.
	assert.Equal(t, base+"/full/!14142,7071/0/default.jpg", iiifLargeImageURL(base+"/", 20000, 10000))
	// Unknown size: the 4K request.
	assert.Equal(t, base+"/full/!3840,3840/0/default.jpg", iiifLargeImageURL(base, 0, 0))
	assert.Equal(t, "", iiifLargeImageURL("", 100, 100))
}

type highResConfig struct{ on bool }

func (c highResConfig) GetHighResOriginals() bool { return c.on }

// The provider follows the high-resolution setting once the wallpaper config has it, and stays at
// about 4K while it does not.
func TestHighResOriginalsSetting(t *testing.T) {
	assert.False(t, (&Provider{}).highResOriginals(), "no config")
	assert.False(t, (&Provider{cfg: &wallpaper.Config{}}).highResOriginals(), "config without the setting yet")
	assert.True(t, highResSetting(highResConfig{on: true}))
	assert.False(t, highResSetting(highResConfig{on: false}))
}

func Test4KImageURL(t *testing.T) {
	base := "https://iip.smk.dk/iiif/jp2/x.tif.jp2"
	assert.Equal(t, base+"/full/!3840,3840/0/default.jpg", iiif4KImageURL(base, 7174, 5536))
	// Smaller originals are not enlarged by the server.
	assert.Equal(t, base+"/full/max/0/default.jpg", iiif4KImageURL(base+"/", 1834, 2263))
	assert.Equal(t, base+"/full/max/0/default.jpg", iiif4KImageURL(base, 3840, 2000))
	assert.Equal(t, base+"/full/!3840,3840/0/default.jpg", iiif4KImageURL(base, 0, 0))
	assert.Equal(t, "", iiif4KImageURL("", 100, 100))
}
