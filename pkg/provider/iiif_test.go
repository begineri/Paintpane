package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

// fit4K is the 4K cap's rule for the cases here: long edge 3840.
func fit4K(w, h int) (int, int) {
	if w <= 3840 && h <= 3840 {
		return w, h
	}
	if w >= h {
		return 3840, h * 3840 / w
	}
	return w * 3840 / h, 3840
}

func TestIIIFScaledURL_KnownSize(t *testing.T) {
	u, ok := IIIFScaledURL(context.Background(), nil, "https://iiif.example/abc/full/max/0/default.jpg", 8000, 6000, fit4K)
	assert.True(t, ok)
	assert.Equal(t, "https://iiif.example/abc/full/3840,/0/default.jpg", u)
}

func TestIIIFScaledURL_ReadsInfoJSONWhenTheSizeIsUnknown(t *testing.T) {
	var infoCalls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/iiif/abc/info.json" {
			infoCalls.Add(1)
			_, _ = w.Write([]byte(`{"@context":"http://iiif.io/api/image/2/context.json","width":6000,"height":9000,
				"profile":["http://iiif.io/api/image/2/level2.json",{"formats":["jpg"]}]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	u, ok := IIIFScaledURL(context.Background(), srv.Client(), srv.URL+"/iiif/abc/full/max/0/default.jpg", 0, 0, fit4K)
	assert.True(t, ok)
	assert.Equal(t, srv.URL+"/iiif/abc/full/2560,/0/default.jpg", u)
	assert.EqualValues(t, 1, infoCalls.Load())
}

// IIIF 3 (the Rijksmuseum's server) gives the profile as a string, not a list.
func TestIIIFScaledURL_ReadsIIIF3InfoJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"@context":"http://iiif.io/api/image/3/context.json","id":"https://iiif.micr.io/aehHx",
			"type":"ImageService3","protocol":"http://iiif.io/api/image","profile":"level2",
			"formats":["jpg","png","webp"],"width":5456,"height":6517,"tileSize":1024}`))
	}))
	defer srv.Close()

	u, ok := IIIFScaledURL(context.Background(), srv.Client(), srv.URL+"/aehHx/full/max/0/default.jpg", 0, 0, fit4K)
	assert.True(t, ok)
	assert.Equal(t, srv.URL+"/aehHx/full/3214,/0/default.jpg", u)
}

func TestIIIFScaledURL_LeavesTheURLAlone(t *testing.T) {
	limited := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// IIIF 3: the server serves at most 3000 px wide, so "max" is already under the cap.
		_, _ = w.Write([]byte(`{"width":9000,"height":6000,"maxWidth":3000}`))
	}))
	defer limited.Close()
	broken := httptest.NewServer(http.NotFoundHandler())
	defer broken.Close()

	cases := map[string]struct {
		url  string
		w, h int
	}{
		"not IIIF":                {"https://images.example/original/abc.jpg", 8000, 6000},
		"a region, not the whole": {"https://iiif.example/abc/0,0,100,100/max/0/default.jpg", 8000, 6000},
		"already small":           {"https://iiif.example/abc/full/max/0/default.jpg", 3000, 2000},
		"server limit under cap":  {limited.URL + "/abc/full/max/0/default.jpg", 0, 0},
		"no info.json":            {broken.URL + "/abc/full/max/0/default.jpg", 0, 0},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, ok := IIIFScaledURL(context.Background(), http.DefaultClient, c.url, c.w, c.h, fit4K)
			assert.False(t, ok)
		})
	}
}

func TestIIIFImageURL(t *testing.T) {
	assert.Equal(t, "https://iiif.example/abc/full/!3840,3840/0/default.jpg", IIIFImageURL("https://iiif.example/abc", 3840))
	assert.Equal(t, "https://iiif.example/abc/full/!800,800/0/default.jpg", IIIFImageURL("https://iiif.example/abc/", 800))
}

func TestIIIFBase(t *testing.T) {
	const prefix = "https://iiif.example/"
	assert.Equal(t, "https://iiif.example/abc", IIIFBase("https://iiif.example/abc/full/!3840,3840/0/default.jpg", prefix))
	assert.Equal(t, "https://iiif.example/abc", IIIFBase("https://iiif.example/abc", prefix))
	assert.Equal(t, "https://iiif.example/abc", IIIFBase("https://iiif.example/abc/", prefix))
	assert.Empty(t, IIIFBase("https://other.example/abc/full/max/0/default.jpg", prefix), "another service is not rewritten")
	assert.Empty(t, IIIFBase("https://iiif.example/abc", ""))
}

func TestIIIFScaledSize(t *testing.T) {
	w, h := IIIFScaledSize(12070, 15257, 3840)
	assert.Equal(t, 3840, h)
	assert.InDelta(t, 3038, w, 1)

	w, h = IIIFScaledSize(30000, 10000, 3840)
	assert.Equal(t, 3840, w)
	assert.InDelta(t, 1280, h, 1)

	w, h = IIIFScaledSize(2000, 1500, 3840) // never enlarged
	assert.Equal(t, 2000, w)
	assert.Equal(t, 1500, h)

	w, h = IIIFScaledSize(0, 1500, 3840)
	assert.Zero(t, w)
	assert.Zero(t, h)
}

func TestIIIFLargeImageURL(t *testing.T) {
	u, w, h := IIIFLargeImageURL("https://iiif.example/abc/", 10104, 7479, 100_000_000)
	assert.Equal(t, "https://iiif.example/abc/full/max/0/default.jpg", u, "within the limit: the full size")
	assert.Equal(t, 10104, w)
	assert.Equal(t, 7479, h)

	u, w, h = IIIFLargeImageURL("https://iiif.example/abc", 13994, 9000, 100_000_000)
	assert.Equal(t, "https://iiif.example/abc/full/!12469,8019/0/default.jpg", u, "over the limit: the largest copy within it")
	assert.LessOrEqual(t, int64(w)*int64(h), int64(100_000_000))
	assert.Equal(t, 12469, w)
	assert.Equal(t, 8019, h)

	u, w, h = IIIFLargeImageURL("https://iiif.example/abc", 0, 9000, 100_000_000)
	assert.Empty(t, u)
	assert.Zero(t, w)
	assert.Zero(t, h)
}
