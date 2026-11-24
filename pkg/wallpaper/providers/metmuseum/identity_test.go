package metmuseum

import (
	"context"
	"image"
	_ "image/jpeg"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/internal/nettest"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
)

// The shared client calls itself a desktop browser. The Met's API and image servers are told who
// is asking instead, for API calls and image downloads alike.
func TestMetRequestsNameTheApplication(t *testing.T) {
	var mu sync.Mutex
	var agents []string
	replay := replayClient(t)
	recording := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		mu.Lock()
		agents = append(agents, r.Header.Get("User-Agent"))
		mu.Unlock()
		return replay.Transport.RoundTrip(r)
	})
	// Wrapped like the app's shared client.
	shared := &http.Client{Transport: &wallpaper.UserAgentTransport{
		RoundTripper: recording,
		UserAgent:    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0.0.0",
	}}
	p := NewProvider(nil, shared)

	img, err := p.fetchObjectDetails(context.Background(), 436535)
	require.NoError(t, err)
	require.NotNil(t, img)

	var cp provider.CustomClientProvider = p
	req, _ := http.NewRequest(http.MethodGet, img.Path, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (set by the downloader)")
	if resp, err := cp.GetClient().Do(req); err == nil {
		resp.Body.Close() // no image fixture: 404 is fine, only the request matters
	}

	require.Len(t, agents, 2)
	for _, ua := range agents {
		assert.Equal(t, ClientUserAgent, ua)
	}
}

func TestIdentifiedClientKeepsTimeoutAndTransport(t *testing.T) {
	inner := nettest.Replay(t, nil).Transport
	c := identifiedClient(&http.Client{Timeout: 42, Transport: inner})
	assert.EqualValues(t, 42, c.Timeout)
	ua, ok := c.Transport.(*wallpaper.UserAgentTransport)
	require.True(t, ok)
	assert.Equal(t, ClientUserAgent, ua.UserAgent)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// TestLiveMetImageDownloads asks the real API for one object and downloads its small image with
// the provider's client: two requests. It runs only with PAINTPANE_NETWORK_TESTS=1.
func TestLiveMetImageDownloads(t *testing.T) {
	nettest.RequireNetwork(t)
	p := NewProvider(nil, &http.Client{Timeout: 60 * time.Second})
	img, err := p.fetchObjectDetails(context.Background(), 436535)
	require.NoError(t, err)
	require.NotNil(t, img)
	url := img.DerivativePaths["thumbnail"]
	require.NotEmpty(t, url)

	resp, err := p.GetClient().Get(url)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "image/jpeg", resp.Header.Get("Content-Type"), "an image, not a bot-protection page")
	cfg, _, err := image.DecodeConfig(resp.Body)
	require.NoError(t, err)
	t.Logf("%s: %dx%d", url, cfg.Width, cfg.Height)
}
