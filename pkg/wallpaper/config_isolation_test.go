package wallpaper

import (
	"encoding/json"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/dixieflatline76/Spice/v2/asset"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSerializationIsolation(t *testing.T) {
	// Goal: Prove that primitive toggles are completely invisible to the JSON marshaler.
	prefs := test.NewApp().Preferences()
	mgr := &asset.Manager{}
	cfg := &Config{
		Preferences: prefs,
		assetMgr:    mgr,
	}

	// 1. Set primitive toggles via native setters
	cfg.SetShortcutsDisabled(true)
	cfg.SetWallhavenSyncEnabled(true)

	// 2. Trigger a save
	cfg.save()

	// 3. Inspect the stored JSON blob in Fyne Preferences
	jsonStr := prefs.String(WallhavenConfigPrefKey)
	require.NotEmpty(t, jsonStr, "JSON config blob should not be empty after save")

	var rawMap map[string]interface{}
	err := json.Unmarshal([]byte(jsonStr), &rawMap)
	require.NoError(t, err, "Should be able to unmarshal the saved JSON")

	// 4. Assert that isolated keys strictly do NOT exist in the map
	restrictedKeys := []string{
		"shortcuts_disabled",
		"wallhaven_sync_enabled",
		"wallhaven_api_key",
		"wallhaven_username",
		"logLevel",
		"maxConcurrentProcessors",
	}

	for _, key := range restrictedKeys {
		_, exists := rawMap[key]
		assert.False(t, exists, "Key '%s' should NOT exist in the JSON blob (it must be isolated)", key)
	}
}

func TestAntiClobberStateSurvival(t *testing.T) {
	// Goal: Prove that complex data saves do not clobber native primitive states.
	prefs := test.NewApp().Preferences()
	mgr := &asset.Manager{}
	cfg := &Config{
		Preferences: prefs,
		assetMgr:    mgr,
	}

	// 1. Set ShortcutsDisabled to true (Native state)
	cfg.SetShortcutsDisabled(true)
	require.True(t, cfg.GetShortcutsDisabled())

	// 2. Add a new dummy ImageQuery (triggers c.save() and internal state sync)
	_, err := cfg.AddImageQuery("Anti-Clobber Test", "https://example.com/api", true)
	require.NoError(t, err)

	// 3. Assert it still returns true
	// Previously, the stale struct field (default false) would have been marshaled into JSON,
	// then potentially re-loaded or shadowed during the process.
	assert.True(t, cfg.GetShortcutsDisabled(), "ShortcutsDisabled must survive a JSON save/sync cycle")

	// Double check Wallhaven Sync too
	cfg.SetWallhavenSyncEnabled(true)
	_, _ = cfg.AddImageQuery("Another Query", "https://example.com/2", true)
	assert.True(t, cfg.GetWallhavenSyncEnabled(), "WallhavenSyncEnabled must survive a JSON save/sync cycle")
}

// The single-display shortcuts switch is off by default, kept apart from Global shortcuts, and
// out of the JSON config (S3-11, v21).
func TestTargetedShortcutsSwitch(t *testing.T) {
	prefs := test.NewApp().Preferences()
	cfg := &Config{Preferences: prefs, assetMgr: &asset.Manager{}}
	assert.True(t, cfg.GetTargetedShortcutsDisabled(), "off by default")
	assert.False(t, cfg.GetShortcutsDisabled(), "global shortcuts are on by default")

	cfg.SetTargetedShortcutsDisabled(false)
	assert.False(t, cfg.GetTargetedShortcutsDisabled())
	assert.False(t, cfg.GetShortcutsDisabled(), "the two switches are independent")

	cfg.save()
	var raw map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(prefs.String(WallhavenConfigPrefKey)), &raw))
	_, exists := raw["targeted_shortcuts_disabled"]
	assert.False(t, exists, "the switch is not part of the JSON blob")
}

func TestNativeIORoundTrip(t *testing.T) {
	// Goal: Prove that setters write to Fyne and getters read from Fyne, persisting across instances.
	prefs := test.NewApp().Preferences() // Shared mock storage
	mgr := &asset.Manager{}

	// Instance A
	cfgA := &Config{
		Preferences: prefs,
		assetMgr:    mgr,
	}
	cfgA.SetShortcutsDisabled(true)

	// Instance B (Fresh instance, same storage)
	cfgB := &Config{
		Preferences: prefs,
		assetMgr:    mgr,
	}

	assert.True(t, cfgB.GetShortcutsDisabled(), "Fresh instance MUST read true from storage")

	// Complex check: change in B, read in A
	cfgB.SetShortcutsDisabled(false)
	assert.False(t, cfgA.GetShortcutsDisabled(), "Instance A MUST see changes made by Instance B via shared storage")
}
