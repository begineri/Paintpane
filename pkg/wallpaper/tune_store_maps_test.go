//go:build !linux

package wallpaper

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
)

// Tune Image works on the monitor's copy of the current image. Store.GetByID copies images
// shallowly, so that copy's Tuning and ProcessingFlags maps were the store's own: writing them
// without the store's lock while a store save encodes them is a concurrent map read and write,
// which ends the program. After tuning, the monitor must hold maps of its own, and the store must
// still learn the tuning and whether the render was framed.
func TestTuneDoesNotShareMapsWithStore(t *testing.T) {
	defer ResetConfig()
	app := newE2EApp(t, t.TempDir(), NewMockPreferences(), e2eMonitor(0, 800, 500))
	app.seed(t, map[string][2]int{"pic": {700, 600}})
	app.wp.Activate()
	defer app.stop()
	app.wp.SetArtworkDisplayMode(ArtworkDisplayFull)
	app.waitUntilSettled(t, filepath.Join(FittedRootDir, FullArtDir), []string{"pic"})

	tune := func(opts provider.TuningOptions) {
		app.os.mu.Lock()
		calls := len(app.os.set[0])
		app.os.mu.Unlock()
		app.wp.SetTuningOptions(0, opts)
		require.Eventually(t, func() bool {
			app.os.mu.Lock()
			defer app.os.mu.Unlock()
			return len(app.os.set[0]) > calls
		}, 10*time.Second, 20*time.Millisecond)
	}
	same := func(a, b any) bool {
		va, vb := reflect.ValueOf(a), reflect.ValueOf(b)
		return !va.IsNil() && va.Pointer() == vb.Pointer()
	}

	tune(provider.TuningOptions{CropOffset: 20, HasCropOffset: true})
	tune(provider.TuningOptions{Display: provider.DisplayFramed})

	mc := app.wp.Monitors[0]
	mc.mu.RLock()
	cur := mc.State.CurrentImage
	mc.mu.RUnlock()
	stored, ok := app.store.GetByID("pic")
	require.True(t, ok)

	assert.False(t, same(cur.ProcessingFlags, stored.ProcessingFlags), "the monitor writes ProcessingFlags without the store lock")
	assert.False(t, same(cur.Tuning, stored.Tuning), "the monitor writes Tuning without the store lock")
	assert.Equal(t, provider.DisplayFramed, stored.GetTuning("800x500").Display, "the store keeps the last tuning")
	assert.Equal(t, cur.ProcessingFlags["VirtualFramed:800x500"], stored.ProcessingFlags["VirtualFramed:800x500"],
		"the store records whether the render was framed")
	assert.True(t, cur.ProcessingFlags["VirtualFramed:800x500"], "a framed tuning is drawn with a frame")
}

// Copies handed out by GetByID keep the maps they were given; the store replaces its own.
func TestStoreTuningAndOutcomeFlagsCopyOnWrite(t *testing.T) {
	store := NewImageStore()
	store.SetAsyncSave(false)
	store.Add(provider.Image{ID: "a", ProcessingFlags: map[string]bool{"SmartFit": true}})
	require.True(t, store.SetTuningOptions("a", "800x500", provider.TuningOptions{CropOffset: 10, HasCropOffset: true}))

	before, _ := store.GetByID("a")
	require.True(t, store.SetTuningOptions("a", "800x500", provider.TuningOptions{CropOffset: 90, HasCropOffset: true}))
	require.True(t, store.SetOutcomeFlag("a", "VirtualFramed:800x500", true))

	after, _ := store.GetByID("a")
	assert.Equal(t, 10.0, before.Tuning["800x500"].CropOffset, "an earlier copy is not changed under its reader")
	assert.False(t, before.ProcessingFlags["VirtualFramed:800x500"])
	assert.Equal(t, 90.0, after.Tuning["800x500"].CropOffset)
	assert.True(t, after.ProcessingFlags["VirtualFramed:800x500"])
	assert.True(t, after.ProcessingFlags["SmartFit"], "other flags are kept")

	require.True(t, store.SetOutcomeFlag("a", "VirtualFramed:800x500", false))
	cleared, _ := store.GetByID("a")
	_, has := cleared.ProcessingFlags["VirtualFramed:800x500"]
	assert.False(t, has)
	assert.False(t, store.SetOutcomeFlag("missing", "VirtualFramed:800x500", true))
}
