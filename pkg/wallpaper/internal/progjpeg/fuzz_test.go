package progjpeg

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// FuzzDecode feeds the decoder damaged and hostile files. Masters come from museum servers, and a
// panic in a pipeline worker would take the whole program down, so a bad file must only ever give
// an error. Run longer with: go test -run=^$ -fuzz=FuzzDecode -fuzztime=5m ./pkg/wallpaper/internal/progjpeg
func FuzzDecode(f *testing.F) {
	for _, name := range samples {
		data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "jpeg", name))
		if err != nil {
			f.Fatal(err)
		}
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		cfg, err := DecodeConfig(bytes.NewReader(data))
		if err != nil {
			return
		}
		// Skip sizes that would only measure memory, not correctness.
		if int64(cfg.Width)*int64(cfg.Height) > 4<<20 {
			return
		}
		_, _ = Decode(bytes.NewReader(data))
	})
}
