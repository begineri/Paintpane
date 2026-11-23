//go:build !linux

package wallpaper

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A fresh install showed the first wallpaper only at the first scheduled change, up to an hour
// after its first images were ready (store-feasibility 11.3 #2): the start only restored last
// time's wallpapers, and a display without one was never asked to show anything. Now a display
// that has never had a wallpaper from Paintpane shows the first image that is ready; a display
// with one from last time keeps it, as before.

// lockRecordingOS is recordingOS that also records the lock screen.
type lockRecordingOS struct {
	*recordingOS
	lockMu sync.Mutex
	locks  []string
}

func (l *lockRecordingOS) SetLockScreen(path string) error {
	l.lockMu.Lock()
	defer l.lockMu.Unlock()
	l.locks = append(l.locks, path)
	return nil
}

func (l *lockRecordingOS) lockCalls() []string {
	l.lockMu.Lock()
	defer l.lockMu.Unlock()
	return append([]string(nil), l.locks...)
}

// holdStartupWait makes the autostart wait last until the returned function is called.
func holdStartupWait(t *testing.T) (release func()) {
	t.Helper()
	ch := make(chan time.Time)
	previous := startupTimer
	startupTimer = func(time.Duration) <-chan time.Time { return ch }
	var once sync.Once
	release = func() { once.Do(func() { close(ch) }) }
	t.Cleanup(func() {
		release()
		startupTimer = previous
	})
	return release
}

// quiet is how long the tests watch for a wallpaper change that must not happen.
const quiet = 500 * time.Millisecond

func TestE2E_FreshInstallShowsTheFirstImageAsSoonAsItIsReady(t *testing.T) {
	defer ResetConfig()
	withAutostart(t, false)
	app := newE2EApp(t, t.TempDir(), NewMockPreferences(), lastShownMonitors()...)
	app.wp.cfg.SetWallpaperChangeFrequency(FrequencyHourly) // the default: the timer would take an hour
	desktop := &lockRecordingOS{recordingOS: app.os}
	app.wp.os = desktop

	app.wp.Activate()
	defer app.stop()

	time.Sleep(quiet)
	for _, m := range lastShownMonitors() {
		assert.Empty(t, app.wallpaperCalls(m.ID), "display %d: no image is ready yet", m.ID)
	}

	// The pipeline finishes the first image.
	app.seed(t, map[string][2]int{"a": {800, 500}})
	for _, m := range lastShownMonitors() {
		app.waitForWallpaperCalls(t, m.ID, 1)
		assert.Equal(t, "a", currentImage(app.wp, m.ID).ID, "display %d shows the first image", m.ID)
		assert.Equal(t, "a", app.wp.cfg.LastShownImage(m.DevicePath), "and remembers it for the next start")
	}
	require.Eventually(t, func() bool { return len(desktop.lockCalls()) > 0 }, e2eSettle, 20*time.Millisecond,
		"the lock screen gets its first image too")

	// More images arrive: they wait for the schedule, the first wallpaper stays.
	app.seed(t, map[string][2]int{"b": {500, 800}, "c": {900, 900}})
	time.Sleep(quiet)
	for _, m := range lastShownMonitors() {
		assert.Len(t, app.wallpaperCalls(m.ID), 1, "display %d changes only once", m.ID)
	}
}

// A display with a wallpaper from last time keeps it, while a display Paintpane has never used
// (a monitor plugged in since) gets its first image at once. Each display decides for itself.
func TestE2E_StartShowsFirstImageOnlyOnDisplaysWithoutOne(t *testing.T) {
	defer ResetConfig()
	withAutostart(t, false)
	root := t.TempDir()
	prefs := NewMockPreferences().(*MockPreferences)
	monitors := lastShownMonitors()

	// Last time only the landscape display was connected.
	s1 := newE2EApp(t, root, prefs, monitors[0])
	s1.seed(t, lastShownLibrary)
	s1.wp.Activate()
	s1.showFirstWallpapers(t)
	shown := currentImage(s1.wp, 0).ID
	require.NotEmpty(t, shown)
	require.Len(t, s1.wallpaperCalls(0), 1, "one wallpaper at the first start")
	s1.stop()

	s2 := newE2EApp(t, root, prefs, monitors...)
	s2.wp.Activate()
	defer s2.stop()

	// The new display's images are made at start (rebuildMissingDerivatives); the first is shown.
	s2.waitForWallpaperCalls(t, 1, 1)
	assert.NotEmpty(t, currentImage(s2.wp, 1).ID)

	s2.seed(t, map[string][2]int{"e": {700, 700}})
	time.Sleep(quiet)
	assert.Empty(t, s2.wallpaperCalls(0), "the display with last time's wallpaper keeps it")
	assert.Equal(t, shown, currentImage(s2.wp, 0).ID)
	assert.Len(t, s2.wallpaperCalls(1), 1, "the new display changes only once")
}

// After an autostart, fetching and processing wait 60–120 s (D2). The first wallpaper waits with
// them, so it cannot start a fetch of its own early, and is shown as soon as the wait is over.
func TestE2E_FirstImageAfterAutostartWaitsWithTheFetch(t *testing.T) {
	defer ResetConfig()
	withAutostart(t, true)
	release := holdStartupWait(t)
	app := newE2EApp(t, t.TempDir(), NewMockPreferences(), lastShownMonitors()...)
	app.seed(t, lastShownLibrary) // a library, so the autostart waits
	require.Positive(t, app.wp.startupDelay())

	app.wp.Activate()
	defer app.stop()

	time.Sleep(quiet)
	for _, m := range lastShownMonitors() {
		assert.Empty(t, app.wallpaperCalls(m.ID), "display %d: nothing changes during the wait", m.ID)
	}

	release()
	app.showFirstWallpapers(t)
	for _, m := range lastShownMonitors() {
		assert.Len(t, app.wallpaperCalls(m.ID), 1, "display %d", m.ID)
	}
}

// withoutLicence makes the app one whose licence is not accepted yet (newE2EApp accepts it).
func withoutLicence(app *e2eApp) {
	app.wp.licenceMu.Lock()
	app.wp.licenceAccepted = false
	app.wp.licenceMu.Unlock()
}

// Until the licence is accepted, the desktop and the lock screen are left alone, though images are
// downloaded and made ready. Accepting shows the first image at once.
func TestE2E_FirstWallpaperWaitsForTheLicence(t *testing.T) {
	defer ResetConfig()
	withAutostart(t, false)
	app := newE2EApp(t, t.TempDir(), NewMockPreferences(), lastShownMonitors()...)
	withoutLicence(app)
	desktop := &lockRecordingOS{recordingOS: app.os}
	app.wp.os = desktop

	app.wp.Activate()
	defer app.stop()
	app.seed(t, map[string][2]int{"a": {800, 500}, "b": {500, 800}})

	time.Sleep(quiet)
	for _, m := range lastShownMonitors() {
		assert.Empty(t, app.wallpaperCalls(m.ID), "display %d: not before the licence is accepted", m.ID)
		assert.Empty(t, currentImage(app.wp, m.ID).ID)
	}
	assert.Empty(t, desktop.lockCalls(), "nor the lock screen")

	app.wp.LicenceAccepted()
	for _, m := range lastShownMonitors() {
		app.waitForWallpaperCalls(t, m.ID, 1)
	}
	require.Eventually(t, func() bool { return len(desktop.lockCalls()) > 0 }, e2eSettle, 20*time.Millisecond,
		"the lock screen gets its first image with the desktop")

	app.wp.LicenceAccepted() // another call changes nothing
	app.seed(t, map[string][2]int{"c": {900, 900}})
	time.Sleep(quiet)
	for _, m := range lastShownMonitors() {
		assert.Len(t, app.wallpaperCalls(m.ID), 1, "display %d changes once", m.ID)
	}
}

// The licence accepted before any image is ready: the first image is shown when it arrives.
func TestE2E_LicenceAcceptedBeforeTheFirstImageIsReady(t *testing.T) {
	defer ResetConfig()
	withAutostart(t, false)
	app := newE2EApp(t, t.TempDir(), NewMockPreferences(), lastShownMonitors()...)
	withoutLicence(app)
	app.wp.Activate()
	defer app.stop()

	app.wp.LicenceAccepted()
	time.Sleep(quiet)
	for _, m := range lastShownMonitors() {
		assert.Empty(t, app.wallpaperCalls(m.ID), "display %d: no image yet", m.ID)
	}

	app.seed(t, map[string][2]int{"a": {800, 500}})
	app.showFirstWallpapers(t)
	for _, m := range lastShownMonitors() {
		assert.Equal(t, "a", currentImage(app.wp, m.ID).ID)
	}
}

// Every version asks for the licence again, so a person who upgrades accepts it with wallpapers
// from last time on screen. They are kept, before and after Accept.
func TestE2E_AcceptingTheLicenceKeepsLastTimesWallpapers(t *testing.T) {
	defer ResetConfig()
	withAutostart(t, false)
	root := t.TempDir()
	prefs := NewMockPreferences().(*MockPreferences)
	shown := firstSession(t, root, prefs)

	s2 := newE2EApp(t, root, prefs, lastShownMonitors()...)
	withoutLicence(s2)
	s2.wp.Activate()
	defer s2.stop()
	s2.wp.LicenceAccepted()

	time.Sleep(quiet)
	for _, m := range lastShownMonitors() {
		assert.Empty(t, s2.wallpaperCalls(m.ID), "display %d keeps last time's wallpaper", m.ID)
		assert.Equal(t, shown[m.ID], currentImage(s2.wp, m.ID).ID)
	}
}

// Unit level: the command does nothing on a display that has a wallpaper, from this run or from
// last time, and on a paused one; otherwise it waits for images when none is ready.
func TestShowFirstWallpaperOnlyWithoutAWallpaper(t *testing.T) {
	defer ResetConfig()
	app := newE2EApp(t, t.TempDir(), NewMockPreferences(), lastShownMonitors()...)
	m := lastShownMonitors()[0]

	newMC := func() *MonitorController {
		return NewMonitorController(m.ID, m, app.store, app.fm, app.os, app.wp.cfg, app.wp.imgProcessor)
	}

	mc := newMC()
	mc.showFirstWallpaper()
	assert.True(t, mc.State.WaitingForImages, "no image yet: waits for the first one")
	assert.False(t, mc.State.ManualRecovery, "as an automatic change")

	app.wp.cfg.SetLastShownImage(m.DevicePath, "gone")
	mc = newMC()
	mc.showFirstWallpaper()
	assert.False(t, mc.State.WaitingForImages, "a wallpaper from last time is kept, even one no longer in the library")

	app.wp.cfg.RemoveValue(lastShownPrefKey)
	mc = newMC()
	mc.State.Paused = true
	mc.showFirstWallpaper()
	assert.False(t, mc.State.WaitingForImages, "a paused display is left alone")
	assert.Empty(t, app.wallpaperCalls(m.ID))
}
