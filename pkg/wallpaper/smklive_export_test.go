//go:build smklive

package wallpaper

import (
	"context"
	"net"
	"net/http"
	"time"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
)

// Hooks for smk_live_test.go, which downloads from the real SMK servers. Only built with
// -tags smklive, so `go test ./...` never touches the network.

// LiveDownloader is a Plugin reduced to the download path.
type LiveDownloader struct{ wp *Plugin }

func NewLiveDownloader() *LiveDownloader { return &LiveDownloader{wp: &Plugin{}} }

// Download runs the same download, retry and backoff code as the app.
func (d *LiveDownloader) Download(ctx context.Context, client *http.Client, url, path string, p provider.ImageProvider) (string, error) {
	return d.wp.downloadMasterFile(ctx, client, url, path, p)
}

// PausedFor reports the source's backoff, which makes the app's dispatcher hold its jobs.
func (d *LiveDownloader) PausedFor(source string) time.Duration {
	return d.wp.downloadBackoff().pausedFor(source)
}

// AppHTTPClient is the client the app hands to every provider (see getPlugin).
func AppHTTPClient() *http.Client {
	return &http.Client{
		Timeout: HTTPClientRequestTimeout,
		Transport: &UserAgentTransport{
			RoundTripper: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   HTTPClientDialerTimeout,
					KeepAlive: HTTPClientKeepAlive,
				}).DialContext,
				ResponseHeaderTimeout: HTTPClientResponseHeaderTimeout,
				TLSHandshakeTimeout:   HTTPClientTLSHandshakeTimeout,
			},
			UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		},
	}
}
