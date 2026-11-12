package getty

import (
	"context"
	"net/http"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
)

var a2ObjectNumber = regexp.MustCompile(`\(\s*\d[\d.]*[A-Za-z.]*\d*[a-z]?\s*\)\s*$|\(\d{2}\.[A-Z]{2}\.\d+`)

// A2 phase 5: titles of real Getty objects have no object number. Needs the network, so it
// only runs with A2_LIVE=1.
func TestA2_LiveTitlesHaveNoObjectNumber(t *testing.T) {
	if os.Getenv("A2_LIVE") == "" {
		t.Skip("set A2_LIVE=1 to read real Getty data")
	}
	p := NewProvider(&wallpaper.Config{}, &http.Client{Timeout: 60 * time.Second})
	total := 0
	for page := 1; page <= 3; page++ {
		images, err := p.FetchImages(context.Background(), "getty_masterpieces", page)
		if err != nil {
			t.Logf("page %d: %v", page, err)
			continue
		}
		for _, img := range images {
			total++
			t.Logf("  %s | %q | artist %q", img.ID, img.Title, img.Artist)
			assert.False(t, a2ObjectNumber.MatchString(img.Title), "%s: %q ends with an object number", img.ID, img.Title)
		}
	}
	t.Logf("%d Getty titles read", total)
	assert.Greater(t, total, 0)
}
