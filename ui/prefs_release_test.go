package ui

import (
	"runtime"
	"testing"
	"time"
	"weak"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/asset"
	"github.com/dixieflatline76/Spice/v2/config"
	spiceui "github.com/dixieflatline76/Spice/v2/pkg/ui"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
)

// prefsReleaseHarness opens and closes a headless settings window the way CreatePreferencesWindow
// does (same close handler), with a short release delay.
type prefsReleaseHarness struct {
	t            *testing.T
	sa           *SpiceApp
	plugin       *scenarioPlugin
	themeChanges int
}

func newPrefsReleaseHarness(t *testing.T) *prefsReleaseHarness {
	t.Helper()
	old := prefsReleaseAfter
	prefsReleaseAfter = 30 * time.Millisecond
	t.Cleanup(func() { prefsReleaseAfter = old })

	a := test.NewTempApp(t)
	initWallpaperOnce.Do(func() {
		wallpaper.GetInstance().Init(&scenarioManager{prefs: a.Preferences(), am: asset.NewManager()})
	})
	p := &scenarioPlugin{applied: map[string]bool{}}
	h := &prefsReleaseHarness{t: t, plugin: p, sa: &SpiceApp{App: a, os: noopOS{},
		appConfig: config.NewAppConfig(a.Preferences()), assetMgr: asset.NewManager(), plugins: []spiceui.Plugin{p}}}
	a.Settings().AddListener(func(fyne.Settings) { h.themeChanges++ })
	t.Cleanup(func() {
		for _, w := range a.Driver().AllWindows() {
			w.Close()
		}
	})
	return h
}

// open does what CreatePreferencesWindow does for a window that is not open yet.
func (h *prefsReleaseHarness) open(page string) fyne.Window {
	h.t.Helper()
	sa := h.sa
	require.Nil(h.t, sa.prefsWindow, "the window is already open")
	sa.cancelPrefsRelease()
	w := test.NewWindow(nil)
	w.Resize(fyne.NewSize(defaultPrefsWindowWidth, defaultPrefsWindowHeight))
	sa.prefsWindow = w
	sa.RebuildPreferencesContent("")
	sa.prefsNav.Select(page) // as the debug API and the tray's page items do
	w.SetOnClosed(func() { sa.prefsClosed(w) })
	flushUI()
	return w
}

// wait lets pending release timers fire and runs what they hand to the UI thread.
func (h *prefsReleaseHarness) wait() {
	time.Sleep(3 * prefsReleaseAfter)
	flushUI()
	time.Sleep(prefsReleaseAfter / 2)
	flushUI()
}

func TestPrefsRelease_ThemeReappliedOnCloseMemoryReturnedAMinuteLater(t *testing.T) {
	h := newPrefsReleaseHarness(t)
	w := h.open(pageWallpaper)
	h.sa.prefsSM.allWidgets["change_on_start"].(*toggleSwitch).SetChecked(false)
	h.sa.prefsSM.checkAndEnableApply()
	theme := h.sa.Settings().Theme()

	w.Close()
	assert.Equal(t, map[string]bool{"change_on_start": false}, h.plugin.applied, "a change made just before closing applies")
	assert.Nil(t, h.sa.prefsWindow)
	assert.Nil(t, h.sa.prefsSM)
	assert.Nil(t, h.sa.prefsNav)
	assert.Equal(t, 1, h.themeChanges, "closing sets the same theme again, so Fyne drops the window's text textures and font cache")
	assert.Same(t, theme, h.sa.Settings().Theme(), "the theme itself is unchanged")
	assert.Zero(t, h.sa.prefsReleased, "memory is returned to Windows only after the delay")

	h.wait()
	assert.Equal(t, 1, h.sa.prefsReleased)
}

func TestPrefsRelease_ReopenedBeforeTheDelayKeepsMemory(t *testing.T) {
	h := newPrefsReleaseHarness(t)
	h.open(pageSources).Close()
	w := h.open(pageGeneral)
	h.wait()
	assert.Zero(t, h.sa.prefsReleased, "the window is open again")
	assert.Equal(t, pageGeneral, h.sa.prefsNav.Current())

	w.Close()
	h.wait()
	assert.Equal(t, 1, h.sa.prefsReleased, "released once after the last close")
	assert.Equal(t, 2, h.themeChanges, "once per close")
}

// Opening and closing many times, with releases in between, leaves no window, manager or
// goroutine behind and every reopened window works.
func TestPrefsRelease_RepeatedOpenCloseLeavesNothingBehind(t *testing.T) {
	h := newPrefsReleaseHarness(t)
	runtime.GC()
	goroutines := runtime.NumGoroutine()
	for i := 0; i < 10; i++ {
		page := []string{pageSources, pageWallpaper, pageGeneral, pageAdvanced}[i%4]
		w := h.open(page)
		assert.Equal(t, page, h.sa.prefsNav.Current(), "cycle %d opens on the page asked for", i)
		if page == pageWallpaper {
			sw := h.sa.prefsSM.allWidgets["change_on_start"].(*toggleSwitch)
			sw.SetChecked(!sw.Checked)
			h.sa.prefsSM.checkAndEnableApply()
		}
		w.Close()
		if i%2 == 1 {
			h.wait()
		}
	}
	h.wait()
	assert.Nil(t, h.sa.prefsSM)
	assert.Equal(t, 5, h.sa.prefsReleased, "one release per close that waited out the delay")
	assert.Equal(t, 10, h.themeChanges, "one theme re-apply per close")
	assert.Contains(t, h.plugin.applied, "change_on_start", "changes made in reopened windows apply")
	time.Sleep(50 * time.Millisecond)
	assert.LessOrEqual(t, runtime.NumGoroutine(), goroutines+2, "no goroutines left running")
}

// After the release the closed window's settings manager, and with it every widget and callback of
// its pages, can be collected; a window still open keeps working.
func TestPrefsRelease_ClosedWindowIsCollected(t *testing.T) {
	h := newPrefsReleaseHarness(t)
	var managers []weak.Pointer[SettingsManager]
	for _, page := range []string{pageSources, pageWallpaper, pageGeneral, pageAdvanced} {
		w := h.open(page)
		managers = append(managers, weak.Make(h.sa.prefsSM))
		w.Close()
	}
	h.sa.tuneWindow = test.NewWindow(nil) // another window, open while the release runs
	other := newToggleSwitch(false, nil)
	h.sa.tuneWindow.SetContent(other)
	h.wait()
	require.Equal(t, 1, h.sa.prefsReleased)

	runtime.GC()
	runtime.GC()
	for i, m := range managers {
		assert.Nil(t, m.Value(), "the manager of window %d is collected", i)
	}
	test.Tap(other)
	assert.True(t, other.Checked, "the open window still works")
	h.sa.tuneWindow.Close()
	h.sa.tuneWindow = nil
}
