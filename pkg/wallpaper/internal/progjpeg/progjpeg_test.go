package progjpeg

import (
	"bytes"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

var samples = []string{
	"video-001.q50.420.progressive.jpeg",
	"video-001.q50.444.progressive.jpeg",
	"video-005.gray.q50.progressive.jpeg",
	"video-001.separate.dc.progression.progressive.jpeg",
	"video-001.q50.420.jpeg",
	"video-001.q50.444.jpeg",
	"video-005.gray.q50.jpeg",
}

func read(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "jpeg", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// The decoder gives exactly the image image/jpeg gives, progressive or not.
func TestDecodeMatchesImageJPEG(t *testing.T) {
	for _, name := range samples {
		data := read(t, name)
		want, err := jpeg.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%s: image/jpeg: %v", name, err)
		}
		got, err := Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%s: progjpeg: %v", name, err)
		}
		if got.Bounds() != want.Bounds() {
			t.Fatalf("%s: bounds %v, want %v", name, got.Bounds(), want.Bounds())
		}
		b := want.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				if got.At(x, y) != want.At(x, y) {
					t.Fatalf("%s: pixel (%d,%d) is %v, want %v", name, x, y, got.At(x, y), want.At(x, y))
				}
			}
		}
		cfg, err := DecodeConfig(bytes.NewReader(data))
		if err != nil || cfg.Width != b.Dx() || cfg.Height != b.Dy() {
			t.Errorf("%s: DecodeConfig %+v, %v", name, cfg, err)
		}
	}
}

// A progressive image's coefficients take half the memory: 2 bytes per sample instead of 4.
func TestProgressiveDecodeUsesLessMemory(t *testing.T) {
	data := read(t, "video-001.q50.444.progressive.jpeg")
	allocated := func(decode func() (image.Image, error)) uint64 {
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		if _, err := decode(); err != nil {
			t.Fatal(err)
		}
		runtime.ReadMemStats(&after)
		return after.TotalAlloc - before.TotalAlloc
	}
	std := allocated(func() (image.Image, error) { return jpeg.Decode(bytes.NewReader(data)) })
	ours := allocated(func() (image.Image, error) { return Decode(bytes.NewReader(data)) })
	// 150x103 at 4:4:4: about 190 KB of coefficients in image/jpeg, 95 KB here, plus 46 KB for
	// the image and some buffers in both.
	if float64(ours) > 0.8*float64(std) {
		t.Errorf("progjpeg allocated %d bytes, image/jpeg %d; want well under", ours, std)
	}
}
