package wallpaper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The paper mat is off for new users (J9, v26); a value someone saved is kept.
func TestPaperMatDefaultsToOffForNewUsers(t *testing.T) {
	ResetConfig()
	cfg := GetConfig(NewMockPreferences())
	assert.False(t, cfg.VirtualPaperMatting, "a new user starts without a paper mat")
	assert.InDelta(t, 0.85, cfg.VirtualFrameSize, 0.0001)
}

// The crop strategy is fixed at Flexibility (S2-9, v31): a saved Quality or Disabled is replaced
// when the config loads, and new users already have it.
func TestSavedCropStrategyIsReplacedByFlexibility(t *testing.T) {
	for _, saved := range []SmartFitMode{SmartFitNormal, SmartFitOff} {
		ResetConfig()
		prefs := NewMockPreferences()
		prefs.SetInt(SmartFitModePrefKey, int(saved))

		cfg := GetConfig(prefs)
		assert.Equal(t, SmartFitAggressive, cfg.GetSmartFitMode(), "saved %v", saved)
	}

	ResetConfig()
	assert.Equal(t, SmartFitAggressive, GetConfig(NewMockPreferences()).GetSmartFitMode(), "new users")
}

func TestPaperMatSavedValueIsKept(t *testing.T) {
	ResetConfig()
	prefs := NewMockPreferences()
	cfg := GetConfig(prefs)
	cfg.VirtualPaperMatting = true
	cfg.save()

	ResetConfig()
	assert.True(t, GetConfig(prefs).VirtualPaperMatting, "an earlier choice of a mat survives")
}
