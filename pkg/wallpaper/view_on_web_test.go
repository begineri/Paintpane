//go:build !linux

package wallpaper

import (
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
)

// The tray's first row opens the page of the artwork it names (2026-11-12: a click on
// "Flower Pyramid" seemed to do nothing).

// openedURLs records the pages the plugin asks the app to open.
func openedURLs(t *testing.T, wp *Plugin) chan string {
	t.Helper()
	pm, ok := wp.manager.(*MockPluginManager)
	require.True(t, ok)
	opened := make(chan string, 4)
	pm.On("OpenURL", mock.Anything).Run(func(args mock.Arguments) {
		opened <- args.Get(0).(*url.URL).String()
	}).Return(nil)
	return opened
}

func setCurrentImage(wp *Plugin, monitorID int, img provider.Image) {
	wp.monMu.RLock()
	mc := wp.Monitors[monitorID]
	wp.monMu.RUnlock()
	mc.mu.Lock()
	mc.State.CurrentID = img.ID
	mc.State.CurrentImage = img
	mc.mu.Unlock()
}

func TestTrayFirstRowOpensTheShownArtworksPage(t *testing.T) {
	defer ResetConfig()
	app := newE2EApp(t, t.TempDir(), NewMockPreferences(), e2eMonitor(0, 1920, 1080))
	app.seed(t, map[string][2]int{"a": {2000, 1500}, "b": {2100, 1500}})
	app.wp.Activate()
	defer app.stop()
	app.showFirstWallpapers(t)
	opened := openedURLs(t, app.wp)

	flower := provider.Image{ID: "ClevelandMuseum_675579", Provider: "ClevelandMuseum",
		Title: "Flower Pyramid", Artist: "Adrianus Kocx", ViewURL: "https://clevelandart.org/art/2024.27"}
	setCurrentImage(app.wp, 0, flower)
	rows := nowShowingRows(app.wp)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].Action)

	// The hourly change comes while the menu is open: the row still names Flower Pyramid, so a
	// click opens its page, not the new wallpaper's.
	setCurrentImage(app.wp, 0, provider.Image{ID: "MetMuseum_1", Provider: "MetMuseum",
		ViewURL: "https://www.metmuseum.org/art/collection/search/1"})
	rows[0].Action()
	select {
	case got := <-opened:
		assert.Equal(t, flower.ViewURL, got)
	case <-time.After(5 * time.Second):
		t.Fatal("a click on the first row opened nothing")
	}
}

func TestViewCurrentImageOnWeb(t *testing.T) {
	defer ResetConfig()
	app := newE2EApp(t, t.TempDir(), NewMockPreferences(), e2eMonitor(0, 1920, 1080))
	app.seed(t, map[string][2]int{"a": {2000, 1500}})
	app.wp.Activate()
	defer app.stop()
	app.showFirstWallpapers(t)
	opened := openedURLs(t, app.wp)

	setCurrentImage(app.wp, 0, provider.Image{ID: "Rijksmuseum_1", ViewURL: "https://www.rijksmuseum.nl/en/collection/SK-C-5"})
	app.wp.ViewCurrentImageOnWeb(0)
	assert.Equal(t, "https://www.rijksmuseum.nl/en/collection/SK-C-5", <-opened)

	// No page: nothing opens (and the log says why).
	setCurrentImage(app.wp, 0, provider.Image{ID: "LocalFolder_x_1"})
	app.wp.ViewCurrentImageOnWeb(0)
	app.wp.ViewCurrentImageOnWeb(7) // no such display
	select {
	case got := <-opened:
		t.Fatalf("opened %q for an image without a page", got)
	case <-time.After(200 * time.Millisecond):
	}
}
