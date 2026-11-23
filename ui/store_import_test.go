//go:build !appstore

package ui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/autostart"
	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/storeimport"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
	"github.com/dixieflatline76/Spice/v2/util"
)

// storeImportRig makes this build behave like the Store edition's first start: the import is on,
// the portable exe's settings file is (or is not) there, and importing only counts.
type storeImportRig struct {
	runs int
	err  error
}

func useStoreImport(t *testing.T, webSettings bool) *storeImportRig {
	t.Helper()
	rig := &storeImportRig{}
	web := filepath.Join(t.TempDir(), "fyne", "Paintpane", "preferences.json")
	if webSettings {
		require.NoError(t, os.MkdirAll(filepath.Dir(web), 0o755))
		require.NoError(t, os.WriteFile(web, []byte(`{"theme":"dark"}`), 0o600))
	}
	prevOn, prevPath, prevRun := storeImportEnabled, storeImportWebPrefs, storeImportRun
	storeImportEnabled = func() bool { return true }
	storeImportWebPrefs = func() string { return web }
	storeImportRun = func(*SpiceApp) error { rig.runs++; return rig.err }
	t.Cleanup(func() { storeImportEnabled, storeImportWebPrefs, storeImportRun = prevOn, prevPath, prevRun })
	useFakeAutostartAt(t, false) // accepting the licence must not touch the real Run key
	return rig
}

// importWindow returns the open import window, or nil.
func importWindow(t *testing.T, a fyne.App) *scenario {
	t.Helper()
	for _, w := range a.Driver().AllWindows() {
		if w.Title() == i18n.T("Paintpane") {
			return &scenario{t: t, w: w}
		}
	}
	return nil
}

func acceptLicence(t *testing.T, a fyne.App) {
	t.Helper()
	run := newEULARun(t, a)
	require.True(t, run.start())
	run.accept()
}

// v51 (owner, 2026-11-21): after the licence, the Store edition asks once whether to bring over the
// portable exe's data. The window has a title, one sentence, the three things and two buttons.
func TestScenarioStoreImportAsksOnceAndImports(t *testing.T) {
	rig := useStoreImport(t, true)
	a := test.NewTempApp(t)
	acceptLicence(t, a)

	s := importWindow(t, a)
	require.NotNil(t, s, "the import window opens after the licence")
	for _, text := range []string{"Import from the downloaded version?",
		"You can bring over the settings and favorites from the downloaded version.",
		"Settings", "Favorites", "Blocked images", "Don't import", "Import"} {
		assert.True(t, s.shows(text), "shows %q", text)
	}
	assert.False(t, s.shows("Downloaded wallpapers are not copied; they are downloaded again."), "no small print (v51)")

	s.tapText("Import")
	assert.Equal(t, 1, rig.runs)
	assert.True(t, a.Preferences().Bool(storeimport.AskedKey), "asked once")
	assert.Nil(t, importWindow(t, a), "the window closes")
}

func TestScenarioStoreImportDeclined(t *testing.T) {
	rig := useStoreImport(t, true)
	a := test.NewTempApp(t)
	acceptLicence(t, a)

	s := importWindow(t, a)
	require.NotNil(t, s)
	s.tapText("Don't import")
	assert.Zero(t, rig.runs)
	assert.True(t, a.Preferences().Bool(storeimport.AskedKey))
	assert.Nil(t, importWindow(t, a))

	// The next version's licence does not ask again.
	a.Preferences().SetString(util.EULAPreferenceKey, staleAcceptance)
	acceptLicence(t, a)
	assert.Nil(t, importWindow(t, a), "asked only once")
}

func TestScenarioStoreImportClosingIsDeclining(t *testing.T) {
	rig := useStoreImport(t, true)
	a := test.NewTempApp(t)
	acceptLicence(t, a)

	s := importWindow(t, a)
	require.NotNil(t, s)
	s.w.Close()
	assert.Zero(t, rig.runs)
	assert.True(t, a.Preferences().Bool(storeimport.AskedKey))
}

func TestScenarioStoreImportNotAskedWithoutThePortableExe(t *testing.T) {
	rig := useStoreImport(t, false)
	a := test.NewTempApp(t)
	acceptLicence(t, a)
	assert.Nil(t, importWindow(t, a), "nothing to import")
	assert.Zero(t, rig.runs)
	assert.True(t, a.Preferences().Bool(storeimport.AskedKey), "not looked for again")
}

func TestScenarioStoreImportNotAskedOnAnUpgrade(t *testing.T) {
	useStoreImport(t, true)
	a := test.NewTempApp(t)
	a.Preferences().SetString(util.EULAPreferenceKey, staleAcceptance) // an earlier version was accepted
	acceptLicence(t, a)
	assert.Nil(t, importWindow(t, a), "only on the first start")
}

func TestScenarioStoreImportNotInThePortableExe(t *testing.T) {
	useStoreImport(t, true)
	storeImportEnabled = func() bool { return false }
	a := test.NewTempApp(t)
	acceptLicence(t, a)
	assert.Nil(t, importWindow(t, a))
	assert.False(t, a.Preferences().Bool(storeimport.AskedKey))
}

func TestScenarioStoreImportFailureKeepsTheWindow(t *testing.T) {
	rig := useStoreImport(t, true)
	rig.err = errors.New("disk full")
	a := test.NewTempApp(t)
	acceptLicence(t, a)

	s := importWindow(t, a)
	require.NotNil(t, s)
	s.tapText("Import")
	require.NotNil(t, importWindow(t, a), "the window stays so the person can try again or close it")
	require.NotNil(t, s.w.Canvas().Overlays().Top(), "the error is shown")

	rig.err = nil
	s.w.Canvas().Overlays().Top().Hide()
	s.w.Canvas().Overlays().Remove(s.w.Canvas().Overlays().Top())
	s.tapText("Import")
	assert.Equal(t, 2, rig.runs, "trying again works")
}

// The keys the import leaves out are the ones this program uses for them.
func TestStoreImportLeavesOutTheLicenceAndAutostart(t *testing.T) {
	assert.Contains(t, storeimport.NotImported, util.EULAPreferenceKey)
	assert.Contains(t, storeimport.NotImported, autostartFirstRunKey)
}

// v51 (owner chose B): the Store edition's Running group has one row that says where Windows keeps
// the switch, with a button to that page. The portable exe has its own switch instead.
func TestScenarioStoreRunningRow(t *testing.T) {
	useFakeAutostart(t)
	opened := 0
	prevVisible, prevOpen := storeStartupRowVisible, openStartupSettings
	storeStartupRowVisible = func() bool { return true }
	openStartupSettings = func(*SpiceApp) { opened++ }
	t.Cleanup(func() { storeStartupRowVisible, openStartupSettings = prevVisible, prevOpen })
	autostartStore = func() autostart.Store { return nil } // no Run key switch, as in the Store build

	s := newScenarioWith(t, wallpaper.GetInstance())
	s.tapText("General")
	assert.True(t, s.shows("Running"))
	assert.True(t, s.shows("Start with Windows"))
	assert.True(t, s.shows("Turn it on or off in Windows Settings › Apps › Startup."))
	s.tapText("Open Settings")
	assert.Equal(t, 1, opened)
}

func TestScenarioPortableExeHasNoStoreRunningRow(t *testing.T) {
	useFakeAutostart(t)
	prev := storeStartupRowVisible
	storeStartupRowVisible = func() bool { return false }
	t.Cleanup(func() { storeStartupRowVisible = prev })

	s := newScenarioWith(t, wallpaper.GetInstance())
	s.tapText("General")
	assert.False(t, s.shows("Open Settings"))
	assert.False(t, s.shows("Turn it on or off in Windows Settings › Apps › Startup."))
}

// Accepting the licence on the Store edition's first start does both: the wallpaper plugin hears
// that the licence is accepted (a fresh install's first wallpaper waits for it), and the import
// window opens (merge of 722139f with v51).
func TestScenarioStoreImportAfterTheLicenceNotice(t *testing.T) {
	useStoreImport(t, true)
	notices := 0
	previous := licenceAccepted
	licenceAccepted = func() { notices++ }
	t.Cleanup(func() { licenceAccepted = previous })

	a := test.NewTempApp(t)
	acceptLicence(t, a)
	assert.Equal(t, 1, notices, "the plugin is told once")
	assert.NotNil(t, importWindow(t, a), "and the import question still comes")
}
