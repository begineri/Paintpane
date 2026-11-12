package ui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
)

// The settings window's wording after the prototype review (docs/ui_refresh.md §12):
// renamed labels, sentence-case English, and no help line under rows that explain
// themselves.
func TestScenarioSettingsWording(t *testing.T) {
	inLanguages(t, func(t *testing.T, lang string) {
		want := map[string]struct {
			general, wallpaper, advanced        string
			personalization, appearance, accent string
			picture, displayMode, frameSize     string
			nsfwHelp                            string
		}{
			"en": {"General", "Wallpaper", "Advanced", "Personalization", "Appearance", "Accent color",
				"Picture", "Display mode", "Frame size",
				"When on, shows nude collections, wallhaven images not rated SFW, and images you marked as NSFW."},
			"zh": {"常规", "壁纸", "高级", "个性化", "外观", "主题色",
				"画面", "显示方式", "画框大小",
				"开启后，显示裸体系列、Wallhaven 未标为 SFW 的图片，以及你标记为 NSFW 的图片。"},
		}[lang]
		s := newScenarioWith(t, wallpaper.GetInstance())

		s.tapText(want.general)
		for _, text := range []string{want.personalization, want.appearance, want.accent} {
			assert.True(t, s.shows(text), "General shows %q", text)
		}

		s.tapText(want.wallpaper)
		for _, text := range []string{want.picture, want.displayMode} {
			assert.True(t, s.shows(text), "Wallpaper shows %q", text)
		}
		assert.False(t, s.shows(want.nsfwHelp), "the help under Show NSFW images is gone (S2-16)")
		assert.False(t, s.shows("Download new wallpapers each night."), "the nightly refresh help is gone")
		assert.False(t, s.shows("Switching rebuilds your wallpapers in the background."), "the display mode help is gone")

		// Advanced explains nothing under its rows, and does not call the build unofficial.
		s.tapText(want.advanced)
		require.Equal(t, pageAdvanced, s.sa.prefsNav.Current())
		for _, o := range shownObjects(s.w.Canvas().Content()) {
			if text, ok := labelText(o); ok {
				assert.NotContains(t, text, "unofficial build of Spice")
				assert.NotContains(t, text, "Write verbose debug entries")
				assert.NotContains(t, text, "Delete every downloaded picture")
			}
		}
	})
}

// Advanced shows what the cache holds and names it in the clear dialog (S4-4C, S4-4E). It has no
// displays row (S4-9); General keeps the Single-display shortcuts switch under its new name (S3-11).
func TestScenarioAdvancedCacheLineAndDialog(t *testing.T) {
	inLanguages(t, func(t *testing.T, lang string) {
		want := map[string]struct{ page, general, summary, message, single string }{
			"en": {"Advanced", "General", "No images", "This deletes 0 downloaded images (0 B). You will need internet to see new wallpapers.",
				"Single-display shortcuts (hold Alt + display number)"},
			"zh": {"高级", "常规", "0 张", "将删除 0 张已下载的图片（0 B）。之后需要联网才能看到新壁纸。",
				"单台显示器快捷键（按住 Alt + 显示器编号）"},
		}[lang]
		s := newScenarioWith(t, wallpaper.GetInstance())

		s.tapText(want.general)
		_, hasSwitch := s.sa.prefsSM.allWidgets["enableTargetedShortcuts"]
		assert.True(t, hasSwitch, "the single-display shortcuts switch stays")
		assert.True(t, s.shows(want.single), "and reads %q", want.single)
		assert.False(t, s.sa.prefsSM.allWidgets["enableTargetedShortcuts"].(*toggleSwitch).Checked, "off by default")

		s.tapText(want.page)
		require.Eventually(t, func() bool { return s.shows(want.summary) }, 2*time.Second, 20*time.Millisecond,
			"the cache line replaces Calculating…")
		for _, o := range shownObjects(s.w.Canvas().Content()) {
			if text, ok := labelText(o); ok {
				assert.NotContains(t, text, "Detect displays", "no displays row on Advanced")
				assert.NotContains(t, text, "Refresh displays", "no displays row on Advanced")
				assert.NotContains(t, text, "重新检测显示器", "no displays row on Advanced")
			}
		}

		test.Tap(s.sa.prefsSM.allWidgets["clearCache"].(fyne.Tappable))
		assert.True(t, overlayShows(s, want.message), "the dialog says how much it deletes")
		buttons := dialogButtons(s)
		require.Len(t, buttons, 2)
		test.Tap(buttons[0])
	})
}

// View all shortcuts opens the Shortcuts page inside the window (S3-12, S3-S): two groups, key
// caps, and a link back to General.
func TestScenarioShortcutsPage(t *testing.T) {
	inLanguages(t, func(t *testing.T, lang string) {
		want := map[string]struct{ general, link, title, all, one, next, back string }{
			"en": {"General", "View all shortcuts →", "Shortcuts", "All displays", "One display", "Next wallpaper", "General"},
			"zh": {"常规", "查看所有快捷键 →", "快捷键", "所有显示器", "单台显示器", "下一张壁纸", "常规"},
		}[lang]
		s := newScenarioWith(t, wallpaper.GetInstance())

		s.tapText(want.general)
		require.True(t, s.shows(want.link))
		s.tapText(want.link)

		assert.True(t, s.shows(want.title), "the page title")
		assert.True(t, s.shows(want.all), "the All displays group")
		assert.True(t, s.shows(want.one), "the One display group")
		assert.True(t, s.shows(want.next), "a row")
		assert.True(t, s.shows("Ctrl") && s.shows("Alt") || s.shows("Cmd"), "keys are drawn as caps")
		assert.False(t, s.shows(want.link), "the General page is replaced")

		// The back link returns to General.
		s.tapText(want.back)
		assert.True(t, s.shows(want.link), "back on General")
		assert.False(t, s.shows(want.all))
	})
}

func TestSettingsEnglishIsSentenceCase(t *testing.T) {
	// A few of the strings that used to be Title Case.
	i18n.SetLanguage("en")
	for _, key := range []string{"Clear cache", "Curated collections", "Refresh displays", "Add new collection", "Paintpane settings", "Tune image", "Local folders", "System default"} {
		assert.Equal(t, key, i18n.T(key))
	}
}
