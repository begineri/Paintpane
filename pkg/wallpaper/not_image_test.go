package wallpaper

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/internal/nettest"
)

// On 2026-11-23 images.metmuseum.org answered every image request with Imperva's "Pardon Our
// Interruption" page, sent as 200 text/html. The page was saved as the master and every Met
// image failed later with "image: unknown format". These tests replay such answers offline.

const metImageURL = "https://images.metmuseum.org/CRDImages/ep/original/DP-42549-001.jpg"

// countingClient counts the requests that reach client's transport.
func countingClient(client *http.Client, calls *atomic.Int32) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return client.Transport.RoundTrip(r)
	})}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDownloadMasterFile_BotCheckPageIsNotSavedOrRetried(t *testing.T) {
	fastRetries(t)
	var calls atomic.Int32
	client := countingClient(nettest.Replay(t, map[string]string{
		"images.metmuseum.org/CRDImages/ep/original/DP-42549-001.jpg": "testdata/blocked/met-imperva.html",
	}), &calls)
	wp := &Plugin{}
	dir := t.TempDir()
	dest := filepath.Join(dir, "MetMuseum_436535.jpg")

	_, err := wp.downloadMasterFile(context.Background(), client, metImageURL, dest, &MockPacedProvider{id: "MetMuseum"})

	var ni *notImageError
	require.True(t, errors.As(err, &ni), "a web page is reported as such, not as a broken image: %v", err)
	assert.True(t, ni.botCheck, "the challenge page is recognised")
	assert.Contains(t, err.Error(), "bot-protection page")
	assert.EqualValues(t, 1, calls.Load(), "asking again at once would get the same page")
	entries, _ := os.ReadDir(dir)
	assert.Empty(t, entries, "nothing is written to the cache, not even a .part file")
	assert.InDelta(t, blockedFirstPause, wp.downloadBackoff().pausedFor("MetMuseum"), float64(time.Minute), "the source is paused (real clock)")
	assert.Zero(t, wp.downloadBackoff().pausedFor("SMK"), "other sources go on")
}

func TestDownloadMasterFile_OtherWebPageIsNotSavedButDoesNotPause(t *testing.T) {
	fastRetries(t)
	var calls atomic.Int32
	client := countingClient(nettest.Replay(t, map[string]string{
		"images.metmuseum.org/CRDImages/ep/original/DP-42549-001.jpg": "testdata/blocked/plain-page.html",
	}), &calls)
	wp := &Plugin{}
	dest := filepath.Join(t.TempDir(), "img.jpg")

	_, err := wp.downloadMasterFile(context.Background(), client, metImageURL, dest, &MockPacedProvider{id: "MetMuseum"})

	var ni *notImageError
	require.True(t, errors.As(err, &ni), "%v", err)
	assert.False(t, ni.botCheck)
	assert.EqualValues(t, 1, calls.Load())
	assert.NoFileExists(t, dest)
	assert.Zero(t, wp.downloadBackoff().pausedFor("MetMuseum"), "one odd page is about that image only")
}

func TestDownloadMasterFile_ImageStillDownloads(t *testing.T) {
	jpeg := "testdata/jpeg/video-001.q50.420.jpeg"
	client := nettest.Replay(t, map[string]string{
		"images.metmuseum.org/CRDImages/ep/original/DP-42549-001.jpg": jpeg,
	})
	dest := filepath.Join(t.TempDir(), "img.jpg")

	got, err := (&Plugin{}).downloadMasterFile(context.Background(), client, metImageURL, dest, &MockPacedProvider{id: "MetMuseum"})

	require.NoError(t, err)
	want, _ := os.ReadFile(jpeg)
	data, _ := os.ReadFile(got)
	assert.Equal(t, want, data, "the bytes looked at first are kept in the file")
}

// The page is recognised by the headers and cookies the service sets, also when the body does
// not say so and the server does not call it HTML.
func TestCheckImageResponse_BotCheckSigns(t *testing.T) {
	cases := map[string]func(w http.ResponseWriter){
		"Imperva cookie": func(w http.ResponseWriter) {
			w.Header().Add("Set-Cookie", "visid_incap_1661977=abc; path=/")
			w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		},
		"Imperva header": func(w http.ResponseWriter) {
			w.Header().Set("X-Iinfo", "8-12596061-0 2NNN")
			w.Header().Set("Content-Type", "text/html")
		},
		"sniffed HTML": func(w http.ResponseWriter) {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Cf-Mitigated", "challenge")
		},
	}
	for name, setHeaders := range cases {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				setHeaders(w)
				_, _ = w.Write([]byte("<html><body>Please wait</body></html>"))
			}))
			defer srv.Close()
			wp := &Plugin{}
			dest := filepath.Join(t.TempDir(), "img.jpg")
			_, err := wp.downloadMasterFile(context.Background(), srv.Client(), srv.URL, dest, &MockPacedProvider{id: "X"})
			var ni *notImageError
			require.True(t, errors.As(err, &ni), "%v", err)
			assert.True(t, ni.botCheck)
			assert.NoFileExists(t, dest)
		})
	}
}

// A2 §12.9 #4 pauses a source whose server cannot be reached; a source behind a bot check is
// paused at once, for an hour, then longer each time it is still blocked, up to 12 hours.
func TestDownloadBackoff_BlockedSourcePausesAndDoubles(t *testing.T) {
	b, now := newTestBackoff()

	assert.Equal(t, time.Hour, b.blocked("MetMuseum"))
	assert.Equal(t, time.Hour, b.pausedFor("MetMuseum"))
	assert.Zero(t, b.blocked("MetMuseum"), "downloads already under way do not extend the pause")
	assert.Zero(t, b.failed("MetMuseum"), "nor do unreachable-server failures while paused")

	want := []time.Duration{2 * time.Hour, 4 * time.Hour, 8 * time.Hour, 12 * time.Hour, 12 * time.Hour}
	for _, w := range want {
		*now = now.Add(b.pausedFor("MetMuseum") + time.Second)
		assert.Equal(t, w, b.blocked("MetMuseum"), "the first image after a pause is turned away again")
	}

	*now = now.Add(13 * time.Hour)
	b.succeeded("MetMuseum")
	assert.Equal(t, time.Hour, b.blocked("MetMuseum"), "an image that downloads starts over")
}
