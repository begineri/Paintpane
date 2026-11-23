package wallhaven

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/internal/nettest"
)

// TestLiveRateLimit prints the rate-limit headers of the real wallhaven.cc API. It checks the live
// site rather than our code, so it only runs with PAINTPANE_NETWORK_TESTS=1.
func TestLiveRateLimit(t *testing.T) {
	nettest.RequireNetwork(t)
	apiURL := "https://wallhaven.cc/api/v1/search?q=anime&sorting=random"
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(apiURL)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Print Rate Limit Headers
	t.Logf("Status: %s", resp.Status)
	t.Logf("X-Rate-Limit-Limit: %s", resp.Header.Get("X-Rate-Limit-Limit"))
	t.Logf("X-Rate-Limit-Remaining: %s", resp.Header.Get("X-Rate-Limit-Remaining"))
	t.Logf("X-Rate-Limit-Reset: %s", resp.Header.Get("X-Rate-Limit-Reset"))

	if resp.StatusCode == 429 {
		t.Error("Rate Limit Hit!")
	}
}
