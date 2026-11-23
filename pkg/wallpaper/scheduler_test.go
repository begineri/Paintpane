package wallpaper

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dixieflatline76/Spice/v2/pkg/curation"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/util"
	"github.com/stretchr/testify/assert"
)

func TestCheckAndRunRefresh_Logic(t *testing.T) {
	tempDir := t.TempDir()

	wp := &Plugin{
		cfg:                GetConfig(NewMockPreferences()),
		fetchingInProgress: util.NewSafeBool(),
		queryPages:         make(map[string]*util.SafeCounter),
		providers:          make(map[string]provider.ImageProvider),
		httpClient:         &http.Client{Transport: noNetworkTransport{}}, // the connectivity check fails at once
		store:              NewImageStore(),
		fm:                 NewFileManager(tempDir),
	}

	// Setup Store for Maintenance
	cachePath := filepath.Join(tempDir, "image_cache_map.json")
	wp.store.SetFileManager(wp.fm, cachePath)

	// Initially, it should NOT run unless it's midnight
	t1 := time.Date(2026, 3, 24, 10, 0, 0, 0, time.UTC)
	lastDay := wp.checkAndRunRefresh(t1, -1, true)
	assert.Equal(t, 24, lastDay)

	// Day change to 25
	t2 := time.Date(2026, 3, 25, 0, 1, 0, 0, time.UTC)

	// We want to verify that when it runs, it doesn't crash
	// and that it preserves our queryPages.
	wp.queryPages["q1"] = util.NewSafeIntWithValue(10)

	// We expect this might try to hit the network for connectivity check,
	// which will likely fail or timeout.
	fmt.Println("Running refresh check with simulated day change...")
	newLastDay := wp.checkAndRunRefresh(t2, lastDay, false)

	// If network is unavailable (likely in test), it won't run but will return the same lastDay?
	// No, if shouldRun is true, it tries to run. If network fails, it returns the OLD lastDay.

	// Let's check if our page counter is Still 10 (it should be regardless of whether it ran or failed)
	assert.Equal(t, 10, wp.queryPages["q1"].Value(), "Query page should never be reset in refresh")

	if newLastDay == 25 {
		fmt.Println("Refresh successfully triggered and completed (or bypassed network check).")
	} else {
		fmt.Println("Refresh skipped (likely network check failed).")
	}
}

func TestCheckAndRunRefresh_OTAIntegration(t *testing.T) {
	tempDir := t.TempDir()

	wp := &Plugin{
		cfg:                GetConfig(NewMockPreferences()),
		fetchingInProgress: util.NewSafeBool(),
		queryPages:         make(map[string]*util.SafeCounter),
		providers:          make(map[string]provider.ImageProvider),
		httpClient: &http.Client{
			Timeout:   10 * time.Millisecond,
			Transport: &mockTransport{},
		},
		store: NewImageStore(),
		fm:    NewFileManager(tempDir),
	}

	// Enable OTA in config. GetConfig is shared by the whole test binary: put it back.
	otaWas := wp.cfg.GetMuseumCollectionOTA()
	wp.cfg.SetMuseumCollectionOTA(true)
	t.Cleanup(func() { wp.cfg.SetMuseumCollectionOTA(otaWas) })

	// Set up mock remote server for Curation Manager
	remoteJSON := `{
		"version": "v9.9.9",
		"description": "Mock Remote Collection",
		"collections": []
	}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(remoteJSON))
	}))
	defer ts.Close()

	// A curation manager of this test's own. SyncAll replaces every provider's collection with
	// the mock's empty one; on the shared manager that was left for the tests after this one, so
	// a second run (-count=2) found Getty's nudes gone (TestIsImageHidden) and testprov already at
	// v9.9.9, with no cache file written.
	curation.GetManager()
	sharedManager := curation.GlobalManager
	cm := curation.NewManager()
	curation.GlobalManager = cm
	cm.RemoteBaseURL = ts.URL + "/"
	cm.CacheDir = filepath.Join(tempDir, "curation_cache")

	// Create a dummy provider so SyncAll triggers an update
	curation.ProviderIDToFilename["testprov"] = "testprov.json"
	t.Cleanup(func() {
		curation.GlobalManager = sharedManager
		delete(curation.ProviderIDToFilename, "testprov")
	})

	// Fast forward time to trigger a new day refresh
	now := time.Now()
	lastRefreshDay := now.Add(-48 * time.Hour).Day()

	newRefreshDay := wp.checkAndRunRefresh(now, lastRefreshDay, true)

	assert.Equal(t, now.Day(), newRefreshDay, "Expected refresh day to update")

	// Verify OTA was actually triggered by checking if the cache file was written. SyncAll runs
	// inside checkAndRunRefresh, so the file is there once it returns.
	cachePath := filepath.Join(cm.CacheDir, "testprov.json")
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Errorf("Expected OTA curation cache file to be written at %s", cachePath)
	}
}

// noNetworkTransport fails every request at once, like a machine without a network. The nightly
// refresh tests use it so the connectivity check (connectivitycheck.gstatic.com) never leaves the
// machine and always takes the "network unavailable" path; with a real client and a 10 ms timeout
// it went to the real host and, on a fast enough connection, could take the other path.
type noNetworkTransport struct{}

func (noNetworkTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("no network in this test: %s", r.URL)
}

type mockTransport struct{}

func (t *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: 204,
		Body:       http.NoBody,
	}, nil
}
