package wallpaper

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/disintegration/imaging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
)

func TestCappedSize(t *testing.T) {
	cases := []struct {
		name         string
		w, h         int
		wantW, wantH int
	}{
		{"16:9 to 4K", 7680, 4320, 3840, 2160},
		{"4:3 by the long edge", 8000, 6000, 3840, 2880},
		{"portrait 3:4", 6000, 8000, 2880, 3840},
		{"square", 6000, 6000, 3840, 3840},
		{"2:1 keeps a 2160 short edge", 8000, 4000, 4320, 2160},
		{"63 MP panorama bounded by pixels", 16875, 3750, 8640, 1920},
		{"already small", 2560, 1600, 2560, 1600},
		{"exactly 4K", 3840, 2160, 3840, 2160},
		{"long scroll bounded by pixels", 12000, 1500, 11520, 1440},
		{"short-edged scroll within the pixel bound is left", 9000, 1200, 9000, 1200},
		{"unknown size", 0, 0, 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w, h := cappedSize(c.w, c.h)
			assert.Equal(t, [2]int{c.wantW, c.wantH}, [2]int{w, h})
			if c.w > 0 {
				assert.LessOrEqual(t, w*h, capMaxPixels+c.w) // rounding
			}
		})
	}
}

// SMK reads the switch from the config by method name (as any), the way this test does.
func TestHighResOriginalsReadableByMethodName(t *testing.T) {
	app := newCapApp(t, false)
	var cfg any = app.wp.cfg
	hr, ok := cfg.(interface{ GetHighResOriginals() bool })
	require.True(t, ok)
	assert.False(t, hr.GetHighResOriginals(), "off by default")
	app.wp.cfg.SetHighResOriginals(true)
	assert.True(t, hr.GetHighResOriginals())
}

func writeQuadrantJPEG(t *testing.T, path string, w, h int) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	f, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, jpeg.Encode(f, quadrantImage(w, h), &jpeg.Options{Quality: 90}))
	require.NoError(t, f.Close())
}

func imageSize(t *testing.T, path string) (int, int) {
	t.Helper()
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	require.NoError(t, err)
	return cfg.Width, cfg.Height
}

func TestCapMaster_ShrinksLargeOriginalInPlace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "big.jpg")
	writeQuadrantJPEG(t, path, 6000, 4500)

	w, h, err := capMaster(context.Background(), path)
	require.NoError(t, err)
	assert.Equal(t, [2]int{3840, 2880}, [2]int{w, h})
	gotW, gotH := imageSize(t, path)
	assert.Equal(t, [2]int{3840, 2880}, [2]int{gotW, gotH}, "the file on disk is the capped one")

	// The whole picture is kept, only smaller.
	img, err := imaging.Open(path)
	require.NoError(t, err)
	assertNearColor(t, quadTL, img.At(20, 20), "top-left")
	assertNearColor(t, quadBR, img.At(3820, 2860), "bottom-right")
	leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(path), "*tmp*"))
	assert.Empty(t, leftovers, "no temporary file is left")
}

func TestCapMaster_KeepsOriginalsWithinTheCapOrSlightlyOver(t *testing.T) {
	for _, size := range [][2]int{{3000, 2000}, {4000, 3000}} { // 4000x3000 is 8% over 3840x2880
		path := filepath.Join(t.TempDir(), "m.jpg")
		writeQuadrantJPEG(t, path, size[0], size[1])
		before, err := os.ReadFile(path)
		require.NoError(t, err)

		w, h, err := capMaster(context.Background(), path)
		require.NoError(t, err)
		assert.Equal(t, size, [2]int{w, h})
		after, _ := os.ReadFile(path)
		assert.True(t, bytes.Equal(before, after), "%v: the file is not rewritten", size)
	}
}

// capTestServer serves a quadrant JPEG of the given size at /full.jpg and a small one at
// /scaled.jpg, and records the paths requested.
type capTestServer struct {
	*httptest.Server
	mu       sync.Mutex
	requests []string
}

func newCapTestServer(t *testing.T, w, h int) *capTestServer {
	t.Helper()
	full := encodeJPEG(t, w, h)
	sw, sh := cappedSize(w, h)
	scaled := encodeJPEG(t, sw, sh)
	s := &capTestServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.requests = append(s.requests, r.URL.Path)
		s.mu.Unlock()
		switch r.URL.Path {
		case "/full.jpg":
			_, _ = rw.Write(full)
		case "/scaled.jpg":
			_, _ = rw.Write(scaled)
		default:
			http.NotFound(rw, r)
		}
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *capTestServer) requested() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.requests...)
}

func encodeJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, quadrantImage(w, h), &jpeg.Options{Quality: 90}))
	return buf.Bytes()
}

// scalingProvider is a source whose server can scale (like IIIF), at scaledPath.
type scalingProvider struct {
	MockProvider
	scaledURL string
}

func (p *scalingProvider) ScaledDownloadURL(_ context.Context, img provider.Image, fit func(w, h int) (int, int)) (string, bool) {
	if w, _ := fit(img.Width, img.Height); w >= img.Width {
		return "", false
	}
	return p.scaledURL, true
}

func newCapApp(t *testing.T, highRes bool) *e2eApp {
	t.Helper()
	app := newE2EApp(t, t.TempDir(), NewMockPreferences(), e2eMonitor(0, 2560, 1600))
	t.Cleanup(ResetConfig)
	app.wp.httpClient = http.DefaultClient
	app.wp.cfg.SetHighResOriginals(highRes)
	return app
}

func TestProcessImageJob_CapsDownloadedOriginal(t *testing.T) {
	for _, highRes := range []bool{false, true} {
		t.Run(fmt.Sprintf("highRes=%v", highRes), func(t *testing.T) {
			srv := newCapTestServer(t, 6000, 4500)
			app := newCapApp(t, highRes)
			img, err := app.wp.ProcessImageJob(context.Background(), DownloadJob{
				Ctx:      context.Background(),
				Image:    provider.Image{ID: "Mock_big", Path: srv.URL + "/full.jpg", SourceQueryID: "q1"},
				Provider: &MockProvider{IDVal: "Mock"},
			})
			require.NoError(t, err)
			master, err := app.wp.masterPathFor(img)
			require.NoError(t, err)
			w, h := imageSize(t, master)
			if highRes {
				assert.Equal(t, [2]int{6000, 4500}, [2]int{w, h}, "on: the original is kept at full size")
			} else {
				assert.Equal(t, [2]int{3840, 2880}, [2]int{w, h}, "off: the original is capped")
			}
			assert.Equal(t, [2]int{w, h}, [2]int{img.Width, img.Height}, "the image takes the size on disk")
			assertWholeOnScreen(t, img.DerivativePaths["2560x1600"], w, h)
		})
	}
}

func TestProcessImageJob_AsksAScalingServerForTheSmallerCopy(t *testing.T) {
	for _, highRes := range []bool{false, true} {
		t.Run(fmt.Sprintf("highRes=%v", highRes), func(t *testing.T) {
			srv := newCapTestServer(t, 6000, 4500)
			app := newCapApp(t, highRes)
			src := &scalingProvider{MockProvider: MockProvider{IDVal: "Iiif"}, scaledURL: srv.URL + "/scaled.jpg"}
			img, err := app.wp.ProcessImageJob(context.Background(), DownloadJob{
				Ctx:      context.Background(),
				Image:    provider.Image{ID: "Iiif_1", Path: srv.URL + "/full.jpg", Width: 6000, Height: 4500, SourceQueryID: "q1"},
				Provider: src,
			})
			require.NoError(t, err)
			if highRes {
				assert.Equal(t, []string{"/full.jpg"}, srv.requested())
				assert.Equal(t, [2]int{6000, 4500}, [2]int{img.Width, img.Height})
			} else {
				assert.Equal(t, []string{"/scaled.jpg"}, srv.requested(), "the full image is never downloaded")
				assert.Equal(t, [2]int{3840, 2880}, [2]int{img.Width, img.Height}, "the size reported by the source is replaced")
			}
		})
	}
}

func TestProcessImageJob_FallsBackWhenTheScaledSizeIsRefused(t *testing.T) {
	srv := newCapTestServer(t, 6000, 4500)
	app := newCapApp(t, false)
	src := &scalingProvider{MockProvider: MockProvider{IDVal: "Iiif"}, scaledURL: srv.URL + "/refused.jpg"}
	img, err := app.wp.ProcessImageJob(context.Background(), DownloadJob{
		Ctx:      context.Background(),
		Image:    provider.Image{ID: "Iiif_2", Path: srv.URL + "/full.jpg", Width: 6000, Height: 4500, SourceQueryID: "q1"},
		Provider: src,
	})
	require.NoError(t, err)
	assert.Equal(t, []string{"/refused.jpg", "/full.jpg"}, srv.requested())
	assert.Equal(t, [2]int{3840, 2880}, [2]int{img.Width, img.Height}, "the full image is shrunk after download")
}

// An original kept from before the cap (or from while the switch was on) is shrunk the next time
// it is processed, e.g. when the display mode changes; until then it stays as it is.
func TestProcessImageJob_CapsAnExistingOriginalWhenRebuilt(t *testing.T) {
	app := newCapApp(t, false)
	master, err := app.fm.GetMasterPath("old", ".jpg")
	require.NoError(t, err)
	writeQuadrantJPEG(t, master, 6000, 4500)

	img, err := app.wp.ProcessImageJob(context.Background(), DownloadJob{
		Ctx:   context.Background(),
		Image: provider.Image{ID: "old", Path: "http://example.invalid/old.jpg", Width: 6000, Height: 4500, SourceQueryID: "q1"},
	})
	require.NoError(t, err)
	w, h := imageSize(t, master)
	assert.Equal(t, [2]int{3840, 2880}, [2]int{w, h})
	assert.Equal(t, [2]int{3840, 2880}, [2]int{img.Width, img.Height})
}

// Smart crop with face detection, and Tune image's dragged position, work on the capped original.
func TestProcessImageJob_SmartCropOnCappedOriginal(t *testing.T) {
	srv := newCapTestServer(t, 9000, 4500) // 2:1, capped to 4320x2160
	app := newCapApp(t, false)
	app.wp.cfg.SetArtworkDisplayMode(ArtworkDisplaySmartCrop)
	app.wp.cfg.SetSmartFitMode(SmartFitAggressive)
	app.wp.cfg.SetFaceCropEnabled(true)
	img, err := app.wp.ProcessImageJob(context.Background(), DownloadJob{
		Ctx:      context.Background(),
		Image:    provider.Image{ID: "Mock_wide", Path: srv.URL + "/full.jpg", SourceQueryID: "q1"},
		Provider: &MockProvider{IDVal: "Mock"},
	})
	require.NoError(t, err)
	assert.Equal(t, [2]int{4320, 2160}, [2]int{img.Width, img.Height})
	path := img.DerivativePaths["2560x1600"]
	require.NotEmpty(t, path)
	w, h := imageSize(t, path)
	assert.Equal(t, [2]int{2560, 1600}, [2]int{w, h}, "the crop fills the screen without upscaling past the original")

	// Tune image: keep the left edge.
	master, err := app.wp.masterPathFor(img)
	require.NoError(t, err)
	src, err := openMaster(master)
	require.NoError(t, err)
	opts := provider.TuningOptions{HasCropOffset: true, CropOffset: 0}
	out, err := app.wp.imgProcessor.FitImage(context.Background(), src, 2560, 1600, opts)
	require.NoError(t, err)
	assertNearColor(t, quadTL, out.At(20, 20), "the left part is kept")
}
