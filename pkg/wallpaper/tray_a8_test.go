//go:build !linux

package wallpaper

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
)

// A8 acceptance (docs/prd.md): the tray menu follows docs/ui_refresh.md section 12, T-S2 and
// T7/T8 option B: the artwork, one row of icon buttons (previous, pause, next | favourite,
// tune), the two text rows for this image, then the extra displays and the global items.
// English labels are in sentence case (T-R2).

func trayLabels(items []schema.MenuItemSchema) []string {
	var out []string
	for _, it := range items {
		if it.IsSeparator {
			out = append(out, "---")
			continue
		}
		label := it.Label
		if it.IsChecked {
			label += " [x]"
		}
		if it.SubMenu != nil {
			label += " ▸(" + strings.Join(trayLabels(it.SubMenu.Items), " | ") + ")"
		}
		out = append(out, label)
	}
	return out
}

func TestA8_TrayMenuContentAndOrder(t *testing.T) {
	useRealDisplayModeDefault(t)
	defer ResetConfig()
	app := newE2EApp(t, t.TempDir(), NewMockPreferences(), e2eMonitor(0, 1920, 1080), e2eMonitor(1, 1080, 1920))
	app.seed(t, map[string][2]int{"a": {2000, 1500}, "b": {1500, 2000}})
	app.wp.Activate()
	defer app.stop()
	app.showFirstWallpapers(t)
	app.wp.cfg.SetWallpaperChangeFrequency(FrequencyHourly) // the harness default is Never, which hides Pause

	menu := app.wp.CreateTrayMenu()
	require.NotNil(t, menu)
	got := trayLabels(menu.Items)
	t.Logf("two displays, default settings:\n  %s", strings.Join(got, "\n  "))

	// The primary display, in order (the header text depends on the image).
	require.Greater(t, len(got), 8)
	assert.Equal(t, schema.MenuItemNowShowing, menu.Items[0].Kind, "the artwork block comes first")
	assert.Equal(t, []string{"Previous wallpaper", "Pause rotation", "Next wallpaper",
		"---", "Tune image", "Don't show this image again", "Mark as NSFW"}, got[1:8],
		"icon row (playback; no favourite here), then the three text rows for this image (v45)")
	for _, i := range []int{1, 2, 3} {
		assert.Equal(t, schema.MenuItemIconButton, menu.Items[i].Kind, got[i])
	}
	for _, i := range []int{5, 6, 7} {
		assert.Equal(t, schema.MenuItemRow, menu.Items[i].Kind, "%s is a text row with an icon", got[i])
	}
	assert.Equal(t, "back", menu.Items[1].Glyph)
	assert.Equal(t, "crop", menu.Items[5].Glyph, "tune image: crop")
	assert.Equal(t, "block", menu.Items[6].Glyph, "don't show again: circle-slash")
	assert.Equal(t, "eyeOff", menu.Items[7].Glyph, "mark as NSFW: eye-off")

	// The global block ends the menu, in this order.
	tail := got[len(got)-3:]
	assert.True(t, strings.HasPrefix(tail[0], "Display 2"), "the second display has its own submenu: %s", tail[0])
	assert.Equal(t, []string{"Display mode ▸(Full artwork [x] | Museum frame | Smart crop)", "Show NSFW images"}, tail[1:],
		"the display mode submenu, then the NSFW switch (off by default)")
	last := menu.Items[len(menu.Items)-1]
	assert.Equal(t, schema.MenuItemSwitch, last.Kind)
	assert.Empty(t, last.Glyph, "Show NSFW images has no icon, so eye-off means one thing")
	assert.Equal(t, "Full artwork", menu.Items[len(menu.Items)-2].Detail, "the current choice shows on the row")

	// T-L: items that change the image or the wallpaper close Tune Image first.
	var closes []string
	for i, it := range menu.Items[:8] {
		if it.ChangesWallpaper {
			closes = append(closes, got[i])
		}
	}
	assert.Equal(t, []string{"Previous wallpaper", "Next wallpaper", "Tune image", "Don't show this image again", "Mark as NSFW"}, closes,
		"pause and favourite leave Tune Image open")
	assert.True(t, last.ChangesWallpaper, "Show NSFW images")
	for _, it := range menu.Items[len(menu.Items)-2].SubMenu.Items {
		assert.True(t, it.ChangesWallpaper, it.Label)
	}
}

func TestA8_PauseHiddenWhenRotationIsNever(t *testing.T) {
	useRealDisplayModeDefault(t)
	defer ResetConfig()
	app := newE2EApp(t, t.TempDir(), NewMockPreferences(), e2eMonitor(0, 1920, 1080))
	app.seed(t, map[string][2]int{"a": {2000, 1500}})
	app.wp.Activate()
	defer app.stop()
	app.showFirstWallpapers(t)

	app.wp.cfg.SetWallpaperChangeFrequency(FrequencyNever)
	assert.NotContains(t, trayLabels(app.wp.CreateTrayMenu().Items), "Pause rotation", "no pause item when the frequency is Never")
	app.wp.cfg.SetWallpaperChangeFrequency(FrequencyHourly)
	assert.Contains(t, trayLabels(app.wp.CreateTrayMenu().Items), "Pause rotation")
}
