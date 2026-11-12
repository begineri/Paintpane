//go:build !linux

package wallpaper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// museumOrFavoritesProvider reports whether a default query shows museum artworks or the user's own
// favorites. Everything else (wallhaven, Wikimedia Commons, Pexels) is not a museum source.
func museumOrFavoritesProvider(provider string) bool {
	switch provider {
	case "Favorites", "MetMuseum", "ArtInstituteChicago", "Rijksmuseum", "ClevelandMuseum",
		"Getty", "NationalPalaceMuseum", "StatensMuseumForKunst":
		return true
	}
	return false
}

// A fresh install shows only museum artworks and favorites (owner, 2026-11-11: a Wikimedia
// astronomy photo appeared on the lock screen). The wallhaven and Wikimedia Commons defaults stay in
// the list but are off, and the migrations on load must not turn them back on.
func TestDefaultConfig_OnlyMuseumsAndFavoritesActive(t *testing.T) {
	ResetConfig()
	t.Cleanup(ResetConfig)
	cfg := GetConfig(NewMockPreferences())

	var active, inactive []string
	for _, q := range cfg.Queries {
		if q.Active {
			active = append(active, q.Provider+": "+q.Description)
			assert.True(t, museumOrFavoritesProvider(q.Provider), "default query %q (%s) must be off", q.Description, q.Provider)
		} else {
			inactive = append(inactive, q.Description)
		}
	}
	assert.NotEmpty(t, active, "museum queries must be on by default")
	assert.Subset(t, inactive, []string{
		"Wikimedia Featured",
		"Wikimedia Featured Astronomy",
		"Toplist last 3 months General",
		"Random General",
	})
}

// An existing user keeps what they saved: a non-museum query they turned on stays on.
func TestDefaultConfig_SavedActiveQueryNotMigrated(t *testing.T) {
	p := NewMockPreferences()
	p.SetString(WallhavenConfigPrefKey, `{"queries": [
		{"desc": "Wikimedia Featured Astronomy", "url": "page:Commons:Featured_pictures/Astronomy", "active": true, "provider": "Wikimedia"}
	]}`)
	ResetConfig()
	t.Cleanup(ResetConfig)
	cfg := GetConfig(p)

	found := false
	for _, q := range cfg.Queries {
		if q.Provider == "Wikimedia" {
			found = true
			assert.True(t, q.Active)
		}
	}
	assert.True(t, found)
}
