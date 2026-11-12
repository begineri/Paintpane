package wallpaper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// The two modes that never crop come first, in the settings page and in the tray.
func TestArtworkDisplayModeOrder(t *testing.T) {
	want := []string{ArtworkDisplayFull.String(), ArtworkDisplayFramed.String(), ArtworkDisplaySmartCrop.String()}
	assert.Equal(t, want, GetArtworkDisplayModes())

	cfg := GetConfig(NewMockPreferences())
	cfg.SetArtworkDisplayMode(ArtworkDisplayFull)
	wp := &Plugin{cfg: cfg}
	var labels []string
	for _, item := range wp.artworkDisplayMenuItem().SubMenu.Items {
		labels = append(labels, item.Label)
		assert.Equal(t, item.Label == trayDisplayModeLabel(ArtworkDisplayFull), item.IsChecked, "only the current mode is checked")
	}
	trayWant := []string{"Full artwork", "Museum frame", "Smart crop"}
	assert.Equal(t, trayWant, labels, "the tray submenu uses the same order, in sentence case (T-R2)")
}

// Reordering the list must not change what is saved: the stored values stay as they were.
func TestArtworkDisplayModeSavedValuesUnchanged(t *testing.T) {
	assert.Equal(t, 0, int(ArtworkDisplaySmartCrop))
	assert.Equal(t, 1, int(ArtworkDisplayFull))
	assert.Equal(t, 2, int(ArtworkDisplayFramed))

	for _, mode := range []ArtworkDisplayMode{ArtworkDisplaySmartCrop, ArtworkDisplayFull, ArtworkDisplayFramed} {
		assert.Equal(t, mode, artworkDisplayModeAt(artworkDisplayModeIndex(mode)), "position and mode map both ways")
	}
	assert.Equal(t, ArtworkDisplayFull, artworkDisplayModeAt(-1), "out of range falls back to the default")
	assert.Equal(t, ArtworkDisplayFull, artworkDisplayModeAt(99))
}
