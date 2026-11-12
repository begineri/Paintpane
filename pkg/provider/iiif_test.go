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
