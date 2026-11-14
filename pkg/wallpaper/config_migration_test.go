//go:build !linux

package wallpaper

import (
	"encoding/json"
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

func queryFor(cfg *Config, provider, url string) *ImageQuery {
	for i := range cfg.Queries {
		if cfg.Queries[i].Provider == provider && cfg.Queries[i].URL == url {
			return &cfg.Queries[i]
		}
	}
	return nil
}

func TestAddFirstNewMuseumsStep_TurnsOnThreeMuseumsOnce(t *testing.T) {
	cfg := &Config{Queries: []ImageQuery{
		{ID: "a", URL: "metmuseum_best_of", Active: true, Provider: "MetMuseum"},
	}}

	changed, err := AddFirstNewMuseumsStep(cfg)
	assert.NoError(t, err)
	assert.True(t, changed)
	assert.Len(t, cfg.Queries, 4)
	for _, want := range firstNewMuseumQueries {
		q := queryFor(cfg, want.Provider, want.URL)
		if assert.NotNil(t, q, want.Provider) {
			assert.True(t, q.Active, want.Provider)
			assert.Equal(t, GenerateQueryID(want.Provider+":"+want.URL), q.ID)
		}
	}
	assert.True(t, cfg.migrationApplied(firstNewMuseumsMigration))

	// The user turns one museum off and removes another: neither comes back.
	queryFor(cfg, "NationalGalleryOfArt", "nga_masterpieces").Active = false
	var kept []ImageQuery
	for _, q := range cfg.Queries {
		if q.Provider != "ParisMusees" {
			kept = append(kept, q)
		}
	}
	cfg.Queries = kept

	changed, err = AddFirstNewMuseumsStep(cfg)
	assert.NoError(t, err)
	assert.False(t, changed, "second run changes nothing")
	assert.False(t, queryFor(cfg, "NationalGalleryOfArt", "nga_masterpieces").Active)
	assert.Nil(t, queryFor(cfg, "ParisMusees", "pm_masterpieces"))
}

func TestAddFirstNewMuseumsStep_KeepsAnExistingQuery(t *testing.T) {
	cfg := &Config{Queries: []ImageQuery{
		{ID: "x", URL: "nga_masterpieces", Active: false, Provider: "NationalGalleryOfArt"},
	}}

	changed, err := AddFirstNewMuseumsStep(cfg)
	assert.NoError(t, err)
	assert.True(t, changed)
	assert.Len(t, cfg.Queries, 3, "Yale and Paris Musées are added; NGA is not duplicated")
	nga := queryFor(cfg, "NationalGalleryOfArt", "nga_masterpieces")
	assert.False(t, nga.Active, "an existing choice is not overridden")
}

// A museum the user already has another collection of is the user's choice: its highlights are
// not added on top.
func TestAddFirstNewMuseumsStep_KeepsAMuseumWithAnotherCollection(t *testing.T) {
	cfg := &Config{Queries: []ImageQuery{
		{ID: "y", URL: "yale_british_art", Active: false, Provider: "YaleUniversityArtCollections"},
	}}

	changed, err := AddFirstNewMuseumsStep(cfg)
	assert.NoError(t, err)
	assert.True(t, changed)
	assert.Nil(t, queryFor(cfg, "YaleUniversityArtCollections", "yale_masterpieces"))
	assert.False(t, queryFor(cfg, "YaleUniversityArtCollections", "yale_british_art").Active)
	assert.NotNil(t, queryFor(cfg, "NationalGalleryOfArt", "nga_masterpieces"))
	assert.NotNil(t, queryFor(cfg, "ParisMusees", "pm_masterpieces"))
}

func TestAddFirstNewMuseumsStep_ExistingUserOnLoad(t *testing.T) {
	p := NewMockPreferences()
	p.SetString(WallhavenConfigPrefKey, `{"queries": [
		{"desc": "Best of The Met", "url": "metmuseum_best_of", "active": true, "provider": "MetMuseum"}
	]}`)

	ResetConfig()
	cfg := GetConfig(p)

	for _, want := range firstNewMuseumQueries {
		q := queryFor(cfg, want.Provider, want.URL)
		if assert.NotNil(t, q, want.Provider) {
			assert.True(t, q.Active, want.Provider)
		}
	}
	assert.True(t, cfg.migrationApplied(firstNewMuseumsMigration))
}

func TestNewUsersGetTheMuseumsFromTheDefaultConfig(t *testing.T) {
	ResetConfig()
	cfg := GetConfig(NewMockPreferences())

	for _, want := range firstNewMuseumQueries {
		q := queryFor(cfg, want.Provider, want.URL)
		if assert.NotNil(t, q, want.Provider+" is in default_config.json") {
			assert.True(t, q.Active, want.Provider)
		}
	}
	// The migration found them already there and only recorded itself.
	assert.True(t, cfg.migrationApplied(firstNewMuseumsMigration))
	count := 0
	for _, q := range cfg.Queries {
		if q.Provider == "NationalGalleryOfArt" {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

// The marker must survive a save and load, or the museums would be switched on again at every start.
func TestAddFirstNewMuseumsStep_DoesNotReturnAfterRestart(t *testing.T) {
	p := NewMockPreferences()
	p.SetString(WallhavenConfigPrefKey, `{"queries": [
		{"desc": "Best of The Met", "url": "metmuseum_best_of", "active": true, "provider": "MetMuseum"}
	]}`)
	ResetConfig()
	GetConfig(p) // first start after the update: the three museums are added and saved

	saved := p.String(WallhavenConfigPrefKey)
	assert.Contains(t, saved, firstNewMuseumsMigration, "the marker is saved")

	// The user turns the National Gallery of Art off and removes Paris Musées, then restarts.
	var stored struct {
		Queries []map[string]any `json:"queries"`
		Rest    map[string]any   `json:"-"`
	}
	assert.NoError(t, json.Unmarshal([]byte(saved), &stored))
	var kept []map[string]any
	for _, q := range stored.Queries {
		switch q["provider"] {
		case "ParisMusees":
			continue
		case "NationalGalleryOfArt":
			q["active"] = false
		}
		kept = append(kept, q)
	}
	var whole map[string]any
	assert.NoError(t, json.Unmarshal([]byte(saved), &whole))
	whole["queries"] = kept
	edited, err := json.Marshal(whole)
	assert.NoError(t, err)
	p.SetString(WallhavenConfigPrefKey, string(edited))

	ResetConfig()
	cfg := GetConfig(p)

	nga := queryFor(cfg, "NationalGalleryOfArt", "nga_masterpieces")
	if assert.NotNil(t, nga) {
		assert.False(t, nga.Active, "a museum the user turned off stays off")
	}
	assert.Nil(t, queryFor(cfg, "ParisMusees", "pm_masterpieces"), "a removed museum stays removed")
}
