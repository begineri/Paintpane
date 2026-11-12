//go:build !appstore

package ui

import (
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/asset"
	"github.com/dixieflatline76/Spice/v2/config"
	"github.com/dixieflatline76/Spice/v2/pkg/autostart"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
	"github.com/dixieflatline76/Spice/v2/util"
)

// noopOS stands in for the platform hooks the licence window calls.
type noopOS struct{}

func (noopOS) TransformToForeground()                 {}
func (noopOS) TransformToBackground()                 {}
func (noopOS) SetupLifecycle(_ fyne.App, _ *SpiceApp) {}

// useFakeAutostartAt is useFakeAutostart with the exe in a folder of the test's own, with or
// without the installer's setup.ini next to it.
func useFakeAutostartAt(t *testing.T, installed bool) (*autostart.MemoryStore, string) {
	t.Helper()
	store := useFakeAutostart(t)
	exe := filepath.Join(t.TempDir(), "paintpane.exe")
	if installed {
		require.NoError(t, os.WriteFile(filepath.Join(filepath.Dir(exe), autostart.SetupMarker), []byte("[Paintpane]\r\nStartWithWindows=0\r\n"), 0o644))
	}
	autostartExecutable = func() (string, error) { return exe, nil }
	return store, exe
}

// eulaRun is one start of the program up to the licence window: the real displayEULAAcceptance
// on a headless app whose preferences carry over between runs.
type eulaRun struct {
	t  *testing.T
	a  fyne.App
	sa *SpiceApp
}

func newEULARun(t *testing.T, a fyne.App) *eulaRun {
	return &eulaRun{t: t, a: a, sa: &SpiceApp{App: a, appConfig: config.NewAppConfig(a.Preferences()),
		assetMgr: asset.NewManager(), os: noopOS{}}}
}

// start does verifyEULA's check: the licence window opens when this version was not accepted. It
// reports whether it opened.
func (r *eulaRun) start() bool {
	if util.HasAcceptedEULA(r.a.Preferences()) {
		return false
	}
	r.sa.displayEULAAcceptance()
	return true
}

// accept taps Accept in the open licence window.
func (r *eulaRun) accept() {
	r.t.Helper()
	var w fyne.Window
	for _, win := range r.a.Driver().AllWindows() {
		if win.Title() == "License agreement" {
			w = win
		}
	}
	require.NotNil(r.t, w, "the licence window is open")
	w.Resize(fyne.NewSize(800, 600))
	s := &scenario{t: r.t, sa: r.sa, w: w}
	s.tapText("Accept")
}

// staleAcceptance is what an earlier version left in the preferences: the next version asks for
// the licence again.
const staleAcceptance = `{"eula_version":"0.9.0","acceptance_timestamp":"2026-11-01T10:00:00Z","hash":"from-an-older-version"}`

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

// Switched off in Windows' Startup apps page: the program does not switch it back on later.
func TestScenarioFirstRunRespectsWindowsStartupApps(t *testing.T) {
	store, _ := useFakeAutostartAt(t, false)
	a := test.NewTempApp(t)
	run := newEULARun(t, a)
	require.True(t, run.start())
	run.accept()

	store.SetDisabledByUser(true) // Task Manager › Startup apps › Disable
	refreshAutostart()
	a.Preferences().SetString(util.EULAPreferenceKey, staleAcceptance)
	run2 := newEULARun(t, a)
	require.True(t, run2.start())
	run2.accept()

	on, err := autostart.Enabled(store)
	require.NoError(t, err)
	assert.False(t, on, "still switched off")
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

// Installed with the installer: its Start with Windows task already asked, so an unticked task
// stays unticked.
func TestScenarioInstallerAnswerStands(t *testing.T) {
	store, _ := useFakeAutostartAt(t, true)
	a := test.NewTempApp(t)
	run := newEULARun(t, a)
	require.True(t, run.start())
	run.accept()
	_, registered, _ := store.Command()
	assert.False(t, registered, "the installer's task was unticked: not registered")
}

// Where there is no switch (outside Windows, the Microsoft Store build) nothing is registered and
// nothing is recorded.
func TestFirstRunWithoutTheSwitchDoesNothing(t *testing.T) {
	useFakeAutostartAt(t, false)
	autostartStore = func() autostart.Store { return nil }
	a := test.NewTempApp(t)
	registerAutostartOnFirstRun(a.Preferences(), false)
	assert.False(t, a.Preferences().Bool(autostartFirstRunKey))
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
