package wikimedia

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
)

func TestIsUnsupportedFormat(t *testing.T) {
	cases := map[string]bool{
		"https://upload.wikimedia.org/wikipedia/commons/c/c6/Jovian_Tempest.tif?utm_source=commons.wikimedia.org&utm_campaign=imageinfo&utm_content=original": true,
		"https://upload.wikimedia.org/wikipedia/commons/a/a1/Flag.svg?utm_source=commons.wikimedia.org":                                                       true,
		"https://upload.wikimedia.org/wikipedia/commons/a/a1/Scan.TIFF":                                                                                       true,
		"https://upload.wikimedia.org/wikipedia/commons/a/a1/Anim.gif":                                                                                        true,
		"https://upload.wikimedia.org/wikipedia/commons/a/a1/Painting.jpg?utm_source=commons.wikimedia.org":                                                   false,
		"https://upload.wikimedia.org/wikipedia/commons/a/a1/Painting.png":                                                                                    false,
		"https://upload.wikimedia.org/wikipedia/commons/a/a1/Painting.jpeg":                                                                                   false,
		"https://upload.wikimedia.org/wikipedia/commons/a/a1/NoExtension":                                                                                     false,
	}
	for u, want := range cases {
		if got := isUnsupportedFormat(u); got != want {
			t.Errorf("isUnsupportedFormat(%q) = %v, want %v", u, got, want)
		}
	}
}

func TestFetchImages_OversizedOriginalsUseScaledCopy(t *testing.T) {
	var asked string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r.URL.Query().Get("iiurlwidth")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"query":{"pages":{
			"1":{"pageid":1,"imageinfo":[{"url":"https://upload.wikimedia.org/normal.jpg","descriptionurl":"https://commons.wikimedia.org/wiki/File:Normal.jpg","width":6000,"height":4000,
				"thumburl":"https://thumb.wikimedia.org/normal-3840.jpg","thumbwidth":3840,"thumbheight":2560}]},
			"2":{"pageid":2,"imageinfo":[{"url":"https://upload.wikimedia.org/huge.jpg","width":42208,"height":9870,
				"thumburl":"https://thumb.wikimedia.org/huge-3840.jpg","thumbwidth":3840,"thumbheight":898}]},
			"3":{"pageid":3,"imageinfo":[{"url":"https://upload.wikimedia.org/huge-no-thumb.jpg","width":20000,"height":10000}]},
			"4":{"pageid":4,"imageinfo":[{"url":"https://upload.wikimedia.org/edge.jpg","width":10000,"height":10000,
				"thumburl":"https://thumb.wikimedia.org/edge-3840.jpg","thumbwidth":3840,"thumbheight":3840}]}
		}}}`))
	}))
	defer ts.Close()

	p := NewProvider(&wallpaper.Config{}, ts.Client())
	p.baseURL = ts.URL
	images, err := p.FetchImages(context.Background(), "category:Space", 1)
	if err != nil {
		t.Fatal(err)
	}

	if asked != "3840" {
		t.Errorf("iiurlwidth = %q, want 3840", asked)
	}
	byID := map[string]provider.Image{}
	for _, img := range images {
		byID[img.ID] = img
	}
	if len(byID) != 3 {
		t.Fatalf("got images %v, want 1, 2 and 4 (3 has no scaled copy and is skipped)", byID)
	}
	if got := byID["1"]; got.Path != "https://upload.wikimedia.org/normal.jpg" || got.Width != 6000 {
		t.Errorf("a normal image keeps its original: %+v", got)
	}
	if got := byID["4"]; got.Path != "https://upload.wikimedia.org/edge.jpg" {
		t.Errorf("exactly 100 MP is still downloaded as is: %+v", got)
	}
	huge := byID["2"]
	if huge.Path != "https://thumb.wikimedia.org/huge-3840.jpg" || huge.Width != 3840 || huge.Height != 898 {
		t.Errorf("a 417 MP image must use the scaled copy and its size: %+v", huge)
	}
	// The view link is the file's page on Commons, not the file (2026-11-12): the description
	// page the API gives, or the page by its ID when it gives none.
	if got := byID["1"].ViewURL; got != "https://commons.wikimedia.org/wiki/File:Normal.jpg" {
		t.Errorf("the view link is the file's description page: %q", got)
	}
	if huge.ViewURL != "https://commons.wikimedia.org/w/index.php?curid=2" {
		t.Errorf("without a description URL the view link is the page by ID: %q", huge.ViewURL)
	}
}

// While originals are capped, a file wider than 3840 px is downloaded as the 3840 px copy Commons
// listed next to it; a file no wider keeps its original.
func TestScaledDownloadURL_UsesTheListedScaledCopy(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"query":{"pages":{
			"1":{"pageid":1,"imageinfo":[{"url":"https://upload.wikimedia.org/big.jpg","width":8000,"height":6000,
				"thumburl":"https://thumb.wikimedia.org/big-3840.jpg","thumbwidth":3840,"thumbheight":2880}]},
			"2":{"pageid":2,"imageinfo":[{"url":"https://upload.wikimedia.org/tall.jpg","width":3000,"height":9000,
				"thumburl":"https://upload.wikimedia.org/tall.jpg","thumbwidth":3000,"thumbheight":9000}]}
		}}}`))
	}))
	defer ts.Close()

	p := NewProvider(&wallpaper.Config{}, ts.Client())
	p.baseURL = ts.URL
	images, err := p.FetchImages(context.Background(), "category:Art", 1)
	if err != nil {
		t.Fatal(err)
	}
	fit := func(w, h int) (int, int) { return w / 2, h / 2 } // any cap that shrinks
	for _, img := range images {
		img.ID = p.ID() + "_" + img.ID // as the downloader sees it
		u, ok := p.ScaledDownloadURL(context.Background(), img, fit)
		switch img.Path {
		case "https://upload.wikimedia.org/big.jpg":
			if !ok || u != "https://thumb.wikimedia.org/big-3840.jpg" {
				t.Errorf("big: got %q, %v; want the 3840 px copy", u, ok)
			}
		case "https://upload.wikimedia.org/tall.jpg":
			if ok {
				t.Errorf("tall: got %q; a file no wider than 3840 px has no smaller copy", u)
			}
		}
	}
	noShrink := func(w, h int) (int, int) { return w, h }
	if _, ok := p.ScaledDownloadURL(context.Background(), provider.Image{ID: "Wikimedia_1", Width: 8000, Height: 6000}, noShrink); ok {
		t.Error("an image within the cap is downloaded as is")
	}
}
