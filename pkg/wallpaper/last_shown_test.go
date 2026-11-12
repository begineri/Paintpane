//go:build !linux

package wallpaper

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
)

// A start keeps last time's wallpapers, so it must know which they are (A2 §14.9 #2): the tray's
// first row, Tune Image, favourite and block act on them before the first change.

func lastShownMonitors() []Monitor {
	m0 := e2eMonitor(0, 800, 500)
	m0.DevicePath = `\\.\DISPLAY1`
	m1 := e2eMonitor(1, 500, 800)
	m1.DevicePath = `\\.\DISPLAY2`
	return []Monitor{m0, m1}
}

var lastShownLibrary = map[string][2]int{"a": {800, 500}, "b": {500, 800}, "c": {900, 900}, "d": {1000, 600}}

func currentImage(wp *Plugin, monitorID int) provider.Image {
	wp.monMu.RLock()
	mc := wp.Monitors[monitorID]
	wp.monMu.RUnlock()
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return mc.State.CurrentImage
}

// nowShowingRows returns the tray's first row of each display, in display order.
func nowShowingRows(wp *Plugin) []schema.MenuItemSchema {
	var rows []schema.MenuItemSchema
	var walk func(items []schema.MenuItemSchema)
	walk = func(items []schema.MenuItemSchema) {
		for _, it := range items {
			if it.Kind == schema.MenuItemNowShowing {
				rows = append(rows, it)
			}
			if it.SubMenu != nil {
				walk(it.SubMenu.Items)
			}
		}
	}
	walk(wp.CreateTrayMenu().Items)
	return rows
}

// firstSession shows a wallpaper on every display and quits, like a day of use.
func firstSession(t *testing.T, root string, prefs *MockPreferences) map[int]string {
	t.Helper()
	s1 := newE2EApp(t, root, prefs, lastShownMonitors()...)
	s1.seed(t, lastShownLibrary)
	s1.wp.Activate()
	s1.showFirstWallpapers(t)
	shown := map[int]string{}
	for _, m := range lastShownMonitors() {
		shown[m.ID] = shownID(s1.wallpaperCalls(m.ID)[0])
		require.Equal(t, shown[m.ID], currentImage(s1.wp, m.ID).ID)
	}
	s1.stop()
	return shown
}

func TestE2E_StartRestoresEachDisplaysLastWallpaper(t *testing.T) {
	defer ResetConfig()
	root := t.TempDir()
	prefs := NewMockPreferences().(*MockPreferences)
	shown := firstSession(t, root, prefs)

	s2 := newE2EApp(t, root, prefs, lastShownMonitors()...)
	s2.wp.Activate()
	defer s2.stop()

	for _, m := range lastShownMonitors() {
		img := currentImage(s2.wp, m.ID)
		assert.Equal(t, shown[m.ID], img.ID, "display %d knows last time's wallpaper", m.ID)
		assert.NotEmpty(t, img.FilePath, "with the file this display shows")
	}

	rows := nowShowingRows(s2.wp)
	require.Len(t, rows, 2)
	for i, row := range rows {
		img := currentImage(s2.wp, i)
		assert.Equal(t, s2.wp.nowShowingLabel(img), row.Label, "display %d's first row names its artwork", i)
		assert.NotEqual(t, i18n.T("Paintpane"), row.Label)
		assert.NotNil(t, row.Action, "the row opens the artwork's page")
	}

	time.Sleep(500 * time.Millisecond)
	for _, m := range lastShownMonitors() {
		assert.Empty(t, s2.wallpaperCalls(m.ID), "restoring does not set the wallpaper again")
	}

	// Favourite acts on the restored image.
	favs := make(chan string, 1)
	s2.wp.monMu.RLock()
	mc1 := s2.wp.Monitors[1]
	s2.wp.monMu.RUnlock()
	mc1.mu.Lock()
	mc1.OnFavoriteRequest = func(img provider.Image) { favs <- img.ID }
	mc1.mu.Unlock()
	s2.wp.TriggerFavorite(1)
	select {
	case id := <-favs:
		assert.Equal(t, shown[1], id)
	case <-time.After(5 * time.Second):
		t.Fatal("favourite did nothing")
	}

	// Tune Image re-crops the restored image and shows it.
	s2.wp.SetTuningOptions(0, provider.TuningOptions{Anchor: provider.AnchorTopCenter})
	s2.waitForWallpaperCalls(t, 0, 1)
	assert.Equal(t, shown[0], shownID(s2.wallpaperCalls(0)[0]), "the restored image is tuned")

	// Previous wallpaper comes back to it after a change.
	s2.wp.SetNextWallpaper(1, true)
	s2.waitForWallpaperCalls(t, 1, 1)
	require.NotEqual(t, shown[1], currentImage(s2.wp, 1).ID)
	s2.wp.SetPreviousWallpaper(1, true)
	s2.waitForWallpaperCalls(t, 1, 2)
	assert.Equal(t, shown[1], currentImage(s2.wp, 1).ID, "previous returns to last time's wallpaper")

	// Don't show this image again blocks the restored image and shows another.
	before := currentImage(s2.wp, 0).ID
	s2.wp.DeleteCurrentImage(0)
	require.Eventually(t, func() bool { return s2.wp.cfg.InAvoidSet(before) }, 5*time.Second, 20*time.Millisecond,
		"the restored image is blocked")
	require.Eventually(t, func() bool { return currentImage(s2.wp, 0).ID != before }, 5*time.Second, 20*time.Millisecond)
}

func TestE2E_StartWhenLastWallpaperIsGone(t *testing.T) {
	defer ResetConfig()
	root := t.TempDir()
	prefs := NewMockPreferences().(*MockPreferences)
	shown := firstSession(t, root, prefs)

	// Display 1's image left the library (blocked), display 0's file went with a cache trim.
	s2 := newE2EApp(t, root, prefs, lastShownMonitors()...)
	s2.wp.cfg.AddToAvoidSet(shown[1])
	require.NoError(t, s2.store.LoadCache())
	s2.store.Remove(shown[0])
	s2.store.SaveCache()

	s2.wp.Activate()
	defer s2.stop()

	for _, m := range lastShownMonitors() {
		assert.Empty(t, currentImage(s2.wp, m.ID).ID, "display %d has no current image", m.ID)
	}
	for i, row := range nowShowingRows(s2.wp) {
		assert.Equal(t, i18n.T("Paintpane"), row.Label, "display %d shows the app's name, not \"initializing\"", i)
		assert.Nil(t, row.Action)
	}
	time.Sleep(500 * time.Millisecond)
	for _, m := range lastShownMonitors() {
		assert.Empty(t, s2.wallpaperCalls(m.ID), "the start still does not change the wallpaper")
	}

	// The next change shows and remembers a new image as usual.
	s2.wp.SetNextWallpaper(0, true)
	s2.waitForWallpaperCalls(t, 0, 1)
	assert.Equal(t, currentImage(s2.wp, 0).ID, s2.wp.cfg.LastShownImage(`\\.\DISPLAY1`))
}

func TestLastShownImagePerDisplay(t *testing.T) {
	cfg := GetConfig(NewMockPreferences())
	t.Cleanup(func() { cfg.RemoveValue(lastShownPrefKey) })
	cfg.RemoveValue(lastShownPrefKey)

	cfg.SetLastShownImage(`\\.\DISPLAY1`, "Met_1")
	cfg.SetLastShownImage(`\\.\DISPLAY2`, "Getty_2")
	cfg.SetLastShownImage(`\\.\DISPLAY1`, "Met_3")
	cfg.SetLastShownImage("", "ignored")
	cfg.SetLastShownImage(`\\.\DISPLAY3`, "")
	assert.Equal(t, "Met_3", cfg.LastShownImage(`\\.\DISPLAY1`))
	assert.Equal(t, "Getty_2", cfg.LastShownImage(`\\.\DISPLAY2`))
	assert.Empty(t, cfg.LastShownImage(`\\.\DISPLAY3`))

	cfg.SetString(lastShownPrefKey, "{not json")
	assert.Empty(t, cfg.LastShownImage(`\\.\DISPLAY1`), "an unreadable value is ignored")

	mc := &MonitorController{ID: 2}
	assert.Equal(t, "display-2", mc.monitorKey(), "a display without a device path is known by its number")
}
