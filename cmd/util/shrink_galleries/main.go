// shrink_galleries re-encodes the thumbnails inside the preview pages in asset/galleries with the
// size cmd/gallerygen now uses (pkg/gallery.EncodeThumbnail), without fetching anything. Run it
// after pulling pages that were generated before the thumbnails were made small:
//
//	go run ./cmd/util/shrink_galleries [-dir asset/galleries]
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dixieflatline76/Spice/v2/pkg/gallery"
)

func main() {
	dir := flag.String("dir", filepath.Join("asset", "galleries"), "folder with one sub folder of .html pages per museum")
	flag.Parse()

	pages, err := filepath.Glob(filepath.Join(*dir, "*", "*.html"))
	if err != nil || len(pages) == 0 {
		fmt.Println("no pages found in", *dir)
		os.Exit(1)
	}

	var before, after int64
	images := 0
	for _, page := range pages {
		data, err := os.ReadFile(page)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		out, n, err := gallery.ShrinkEmbeddedImages(string(data))
		if err != nil {
			fmt.Printf("%s: %v\n", page, err)
		}
		before += int64(len(data))
		after += int64(len(out))
		images += n
		if n == 0 {
			continue
		}
		if err := os.WriteFile(page, []byte(out), 0o600); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}
	fmt.Printf("%d pages, %d images re-encoded: %.1f MB -> %.1f MB\n", len(pages), images, float64(before)/1e6, float64(after)/1e6)
}
