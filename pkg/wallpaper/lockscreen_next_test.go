package wallpaper

import (
	"testing"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// T15 (prototype v44): "Next lock screen picture" changes only the lock screen. It picks an image
// that may be shown and is neither the desktop's nor the lock screen's current one.
func TestNextLockScreen_ChangesOnlyLockScreen(t *testing.T) {
	osMock := newLockScreenOS()
	mc := newNudeTestController(t, 0, newNudeTestConfig(t), osMock,
		sfwImage("desktop"), sfwImage("lock"), nsfwImage("hidden"), sfwImage("new"))
	mc.State.CurrentImage = sfwImage("desktop")
	mc.lockImageID = "lock"

	mc.handleCommand(CmdNextLockScreen)

	assert.Equal(t, "new.jpg", nextLockScreenPath(t, osMock))
	assert.Equal(t, "new", mc.lockImageID)
	assert.Equal(t, "desktop", mc.State.CurrentImage.ID, "the desktop stays")
	osMock.AssertNotCalled(t, "SetWallpaper", mock.Anything, mock.Anything)
}

// With nothing else to show, the lock screen keeps its image; it never falls back to the desktop's.
func TestNextLockScreen_NoOtherImageDoesNothing(t *testing.T) {
	osMock := newLockScreenOS()
	mc := newNudeTestController(t, 0, newNudeTestConfig(t), osMock,
		sfwImage("desktop"), sfwImage("lock"), nsfwImage("hidden"))
	mc.State.CurrentImage = sfwImage("desktop")
	mc.lockImageID = "lock"

	mc.handleCommand(CmdNextLockScreen)

	assertNoLockScreenUpdate(t, osMock)
	assert.Equal(t, "lock", mc.lockImageID)
}

// Only the primary monitor sets the lock screen, so the command does nothing elsewhere.
func TestNextLockScreen_OnlyPrimaryMonitor(t *testing.T) {
	osMock := newLockScreenOS()
	mc := newNudeTestController(t, 1, newNudeTestConfig(t), osMock, sfwImage("a"), sfwImage("b"))
	mc.State.CurrentImage = sfwImage("a")

	assert.False(t, mc.SupportsLockScreen())
	mc.handleCommand(CmdNextLockScreen)

	assertNoLockScreenUpdate(t, osMock)
}

// The tray shows the row once, for the primary monitor, and only where the lock screen can be set.
// Using it sends the command to the primary monitor and leaves the menu open.
func TestTrayMenu_NextLockScreenRow(t *testing.T) {
	cfg := newNudeTestConfig(t)
	plugin := func(monitors ...*MonitorController) *Plugin {
		wp := &Plugin{cfg: cfg, Monitors: map[int]*MonitorController{}}
		for _, mc := range monitors {
			wp.Monitors[mc.ID] = mc
		}
		return wp
	}
	rows := func(wp *Plugin) []schema.MenuItemSchema {
		var found []schema.MenuItemSchema
		var walk func(items []schema.MenuItemSchema)
		walk = func(items []schema.MenuItemSchema) {
			for _, it := range items {
				if it.Label == i18n.T("Next lock screen picture") {
					found = append(found, it)
				}
				if it.SubMenu != nil {
					walk(it.SubMenu.Items)
				}
			}
		}
		walk(wp.CreateTrayMenu().Items)
		return found
	}

	primary := newNudeTestController(t, 0, cfg, newLockScreenOS(), sfwImage("a"))
	second := newNudeTestController(t, 1, cfg, newLockScreenOS(), sfwImage("b"))
	found := rows(plugin(primary, second))
	require.Len(t, found, 1)
	row := found[0]
	assert.Equal(t, "lock", row.Glyph)
	assert.True(t, row.KeepOpen)
	assert.False(t, row.ChangesWallpaper, "the desktop does not change, so Tune Image stays open")

	row.Action()
	select {
	case cmd := <-primary.Commands:
		assert.Equal(t, CmdNextLockScreen, cmd)
	default:
		assert.Fail(t, "no command sent to the primary monitor")
	}
	assert.Empty(t, second.Commands)

	assert.Empty(t, rows(plugin(newNudeTestController(t, 0, cfg, newRecordingOS(), sfwImage("a")))), "no lock screen support, no row")
}
