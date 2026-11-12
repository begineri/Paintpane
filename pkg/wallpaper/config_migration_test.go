//go:build !linux

package wallpaper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigMigration_LegacyToUnified(t *testing.T) {
	// 1. Setup Legacy JSON
	legacyJSON := `{
		"query_urls": [
			{"url": "http://wallhaven.cc/legacy1", "active": true}
		]
	}`

	// 2. Setup Mock Preferences
	p := NewMockPreferences()
	p.SetString("wallhaven_image_queries", legacyJSON)

	// 3. Load Config
	// GetConfig calls loadFromPrefs internally
	ResetConfig() // Ensure fresh instance
	cfg := GetConfig(p)

	// 4. Verify Migration
	assert.NotEmpty(t, cfg.Queries)

	// Check content
	foundWallhaven := false

	for _, q := range cfg.Queries {
		if q.URL == "http://wallhaven.cc/legacy1" {
			foundWallhaven = true
			assert.Equal(t, "Wallhaven", q.Provider)
			assert.NotEmpty(t, q.ID)
		}
	}

	assert.True(t, foundWallhaven, "Legacy Wallhaven query not migrated")

	// Verify Favorites was added
	foundFav := false
	for _, q := range cfg.Queries {
		if q.ID == FavoritesQueryID {
			foundFav = true
			break
		}
	}
	assert.True(t, foundFav, "Favorites query should be auto-added")
}

func metQueries(cfg *Config) (urls []string) {
	for _, q := range cfg.Queries {
		if q.Provider == "MetMuseum" {
			urls = append(urls, q.URL)
		}
	}
	return urls
}

func TestRenameMetBestOfStep_RenamesOldKey(t *testing.T) {
	cfg := &Config{Queries: []ImageQuery{
		{ID: "a", Description: "Best of The Met", URL: "spice_melange", Active: true, Provider: "MetMuseum"},
		{ID: "b", URL: "spice_melange", Provider: "Wallhaven"}, // other providers are left alone
	}}

	changed, err := RenameMetBestOfStep(cfg)
	assert.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, []string{"metmuseum_best_of"}, metQueries(cfg))
	assert.True(t, cfg.Queries[0].Active, "the user's on/off choice survives")
	assert.Equal(t, "spice_melange", cfg.Queries[1].URL)
}

func TestRenameMetBestOfStep_NoDuplicate(t *testing.T) {
	cfg := &Config{Queries: []ImageQuery{
		{ID: "a", URL: "spice_melange", Provider: "MetMuseum"},
		{ID: "b", URL: "metmuseum_best_of", Active: true, Provider: "MetMuseum"},
	}}

	changed, err := RenameMetBestOfStep(cfg)
	assert.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, []string{"metmuseum_best_of"}, metQueries(cfg))
	assert.True(t, cfg.Queries[0].Active, "the existing metmuseum_best_of query is the one kept")

	changed, err = RenameMetBestOfStep(cfg)
	assert.NoError(t, err)
	assert.False(t, changed, "second run changes nothing")
}

func TestConfigMigration_MetOldKeyOnLoad(t *testing.T) {
	p := NewMockPreferences()
	p.SetString(WallhavenConfigPrefKey, `{"queries": [
		{"desc": "Best of The Met", "url": "spice_melange", "active": true, "provider": "MetMuseum"}
	]}`)

	ResetConfig()
	cfg := GetConfig(p)

	assert.Equal(t, []string{"metmuseum_best_of"}, metQueries(cfg))
}
