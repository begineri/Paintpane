package wikimedia

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
)

// A2 phase 5: credits of real Commons files. Needs the network, so it only runs with A2_LIVE=1.
func TestA2_LiveCreditsAreReadable(t *testing.T) {
	if os.Getenv("A2_LIVE") == "" {
		t.Skip("set A2_LIVE=1 to read real Commons data")
	}
	p := NewProvider(&wallpaper.Config{}, &http.Client{Timeout: 60 * time.Second})
	queries := []string{
		"category:Featured_pictures_on_Wikimedia_Commons",
		"page:Commons:Featured_pictures/Astronomy",
		"search:Unknown author painting 17th century",
		"search:Anonymous portrait oil on canvas",
	}
	for _, q := range queries {
		for page := 1; page <= 2; page++ {
			images, err := p.FetchImages(context.Background(), q, page)
			if err != nil {
				t.Logf("%s page %d: %v", q, page, err)
				continue
			}
			noAuthor := 0
			for _, img := range images {
				// What the fix is meant to remove: a credit that starts with the placeholder, and
				// character references left in the text.
				bad := strings.HasPrefix(img.Attribution, "Unknown author") || strings.Contains(img.Attribution, "&#") || strings.Contains(img.Attribution, "&amp;") || strings.Contains(img.Artist, "&")
				assert.False(t, bad, "%s: attribution %q artist %q", img.ID, img.Attribution, img.Artist)
				// Left over: "Anonymous" followed by the placeholder (A2 phase 5, report 13.9 #1).
				if strings.Contains(img.Artist, "Unknown author") {
					t.Logf("  placeholder inside the credit: %s artist=%q", img.ID, img.Artist)
				}
				if img.Artist == "" {
					noAuthor++
					t.Logf("  no author: %s attribution=%q", img.ID, img.Attribution)
				}
			}
			t.Logf("%s page %d: %d images, %d without an author", q, page, len(images), noAuthor)
		}
	}
}
