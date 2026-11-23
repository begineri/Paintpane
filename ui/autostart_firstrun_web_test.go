//go:build !appstore && !msstore

package ui

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/autostart"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
	"github.com/dixieflatline76/Spice/v2/util"
)

// These tests check Start with Windows in the portable exe. The Store builds have no such switch
// (Windows starts them through the package's StartupTask), so they are left out there.

// The portable exe: the first time someone accepts the licence, the program registers itself, so
// the Start with Windows switch is on by default (S3-6, D1).
func TestScenarioFirstRunRegistersStartWithWindows(t *testing.T) {
	store, exe := useFakeAutostartAt(t, false)
	a := test.NewTempApp(t)

	run := newEULARun(t, a)
	require.True(t, run.start(), "a first run shows the licence")
	_, registered, _ := store.Command()
	assert.False(t, registered, "nothing is registered before Accept")
	run.accept()

	cmd, registered, _ := store.Command()
	assert.True(t, registered, "accepting registers the program")
	assert.Equal(t, autostart.Command(exe), cmd)
	assert.True(t, a.Preferences().Bool(autostartFirstRunKey), "and records that it did")

	// The settings window shows the switch on.
	s := newScenarioWith(t, wallpaper.GetInstance())
	s.tapText("General")
	assert.True(t, s.switchOn("startWithWindows"), "on by default")
}

// It happens once: after the person turns the switch off, neither a restart nor the next version's
// licence registers the program again.
func TestScenarioFirstRunRegistersOnlyOnce(t *testing.T) {
	store, _ := useFakeAutostartAt(t, false)
	a := test.NewTempApp(t)
	run := newEULARun(t, a)
	require.True(t, run.start())
	run.accept()
	_, registered, _ := store.Command()
	require.True(t, registered)

	// The person turns Start with Windows off.
	s := newScenarioWith(t, wallpaper.GetInstance())
	s.tapText("General")
	s.toggle("startWithWindows")
	s.sa.prefsSM.FlushPending()
	_, registered, _ = store.Command()
	require.False(t, registered)

	// Next start: no licence, and refreshing creates nothing.
	assert.False(t, newEULARun(t, a).start(), "this version was accepted")
	refreshAutostart()
	_, registered, _ = store.Command()
	assert.False(t, registered, "stays off after a restart")

	// A new version asks for the licence again.
	a.Preferences().SetString(util.EULAPreferenceKey, staleAcceptance)
	run2 := newEULARun(t, a)
	require.True(t, run2.start())
	run2.accept()
	_, registered, _ = store.Command()
	assert.False(t, registered, "turned off stays off after an upgrade")

	// Even with the first-run mark lost, the earlier acceptance says it is not a first run.
	a.Preferences().SetString(util.EULAPreferenceKey, staleAcceptance)
	a.Preferences().SetBool(autostartFirstRunKey, false)
	run3 := newEULARun(t, a)
	require.True(t, run3.start())
	run3.accept()
	_, registered, _ = store.Command()
	assert.False(t, registered, "an earlier acceptance alone keeps it off")
}

// Someone who accepted an earlier version and upgrades is not registered all of a sudden.
func TestScenarioUpgradeDoesNotRegister(t *testing.T) {
	store, _ := useFakeAutostartAt(t, false)
	a := test.NewTempApp(t)
	a.Preferences().SetString(util.EULAPreferenceKey, staleAcceptance)

	run := newEULARun(t, a)
	require.True(t, run.start(), "the new version shows the licence again")
	run.accept()

	_, registered, _ := store.Command()
	assert.False(t, registered, "an upgrade is not a first run")
	assert.True(t, a.Preferences().Bool(autostartFirstRunKey), "and it is never reconsidered")
	assert.True(t, util.HasAcceptedEULA(a.Preferences()), "the acceptance itself is recorded")
}

// A registry that cannot be written is logged, and not tried again at the next licence.
func TestFirstRunFailureIsNotRetried(t *testing.T) {
	store, _ := useFakeAutostartAt(t, false)
	a := test.NewTempApp(t)
	store.FailWith(assert.AnError)
	registerAutostartOnFirstRun(a.Preferences(), false)
	store.FailWith(nil)
	assert.True(t, a.Preferences().Bool(autostartFirstRunKey))
	registerAutostartOnFirstRun(a.Preferences(), false)
	_, registered, _ := store.Command()
	assert.False(t, registered, "once means once")
}
