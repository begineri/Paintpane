//go:build !linux

package wallpaper

import (
	"sync"
	"testing"
)

// ResetConfig resets the config singleton for testing purposes.
func init() {
	// Most tests exercise cropping and predate display modes, so they keep it unless they opt in.
	defaultArtworkDisplayMode = ArtworkDisplaySmartCrop
}

// useRealDisplayModeDefault gives a test the default that users get.
func useRealDisplayModeDefault(t *testing.T) {
	t.Helper()
	previous := defaultArtworkDisplayMode
	defaultArtworkDisplayMode = ArtworkDisplayFull
	t.Cleanup(func() { defaultArtworkDisplayMode = previous })
}

func ResetConfig() {
	cfgInstance = nil
	cfgOnce = sync.Once{}
}
