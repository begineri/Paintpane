package rijksmuseum

import (
	"context"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A2 phase 5: the real Rijksmuseum records of Melchior d' Hondecoeter's paintings, fetched from
// data.rijksmuseum.nl. Needs the network, so it only runs with A2_LIVE=1.
func TestA2_LiveHondecoeterRecordsAreDecoded(t *testing.T) {
	if os.Getenv("A2_LIVE") == "" {
		t.Skip("set A2_LIVE=1 to read the real records")
	}
	p := NewProvider(nil, &http.Client{Timeout: 30 * time.Second})
	// 200109383 is "The Floating Feather"; the others come from a search for the creator.
	for _, id := range []string{"200109383", "200117614", "200117615", "20028077", "20028078", "20028079"} {
		img, err := p.resolveObjectToImage(context.Background(), id)
		require.NoError(t, err, id)
		if img == nil {
			t.Logf("%s: no image", id)
			continue
		}
		t.Logf("%s: title=%q artist=%q attribution=%q", id, img.Title, img.Artist, img.Attribution)
		for _, s := range []string{img.Title, img.Artist, img.Attribution} {
			assert.False(t, strings.Contains(s, "&#") || strings.Contains(s, "&amp;") || strings.Contains(s, "&quot;"), "%s: %q still has a character reference", id, s)
		}
	}
}
