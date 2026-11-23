//go:build !appstore

package ui

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The first wallpaper of a fresh install waits for the licence: the wallpaper plugin is told when
// Accept is clicked, not before (wallpaper.LicenceAccepted).
func TestScenarioAcceptingTheLicenceLetsTheFirstWallpaperShow(t *testing.T) {
	useFakeAutostartAt(t, false)
	calls := 0
	previous := licenceAccepted
	licenceAccepted = func() { calls++ }
	t.Cleanup(func() { licenceAccepted = previous })

	a := test.NewTempApp(t)
	run := newEULARun(t, a)
	require.True(t, run.start(), "a first run shows the licence")
	assert.Zero(t, calls, "not while the licence is waiting")

	run.accept()
	assert.Equal(t, 1, calls, "accepting tells the wallpaper plugin")
}
