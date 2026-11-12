package ui

import (
	"errors"
	"testing"

	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
)

// The Save collection dialog of the (hidden) Google Photos picker uses the settings window's
// dialog style, not Fyne's form dialog, and its suggested name has full-width brackets in Chinese.
func TestScenarioSaveCollectionDialog(t *testing.T) {
	inLanguages(t, func(t *testing.T, lang string) {
		want := map[string]struct{ title, name, active, cancel, save string }{
			"en": {"Save collection", "Name", "Active", "Cancel", "Save"},
			"zh": {"保存合集", "名称", "已启用", "取消", "保存"},
		}[lang]
		s := newScenario(t)

		var savedName string
		var savedActive, cancelled bool
		saveErr := errors.New("disk full")
		s.sa.prefsSM.showSaveCollectionDialog("Collection Nov 08 10:00 (3 items)", func(name string, active bool) error {
			savedName, savedActive = name, active
			return saveErr
		}, func() { cancelled = true })
		require.NotNil(t, s.w.Canvas().Overlays().Top(), "the dialog opens")

		for _, text := range []string{want.title, want.name, want.active} {
			assert.True(t, overlayShows(s, text), "the dialog shows %q", text)
		}
		for _, old := range []string{"Internal ID:", "内部 ID：", "Description:", "描述：", "googlephotos://"} {
			assert.NotContains(t, overlayTexts(s), old, "the old form text is gone")
		}
		buttons := dialogButtons(s)
		require.Len(t, buttons, 2)
		assert.Equal(t, want.cancel, buttons[0].Text)
		assert.Equal(t, want.save, buttons[1].Text)
		assert.Equal(t, buttonPrimary, buttons[1].kind)
		entries := overlayEntries(s)
		require.Len(t, entries, 1)
		assert.Equal(t, "Collection Nov 08 10:00 (3 items)", entries[0].Text, "the suggested name is filled in")

		// An empty name cannot be saved.
		entries[0].SetText("  ")
		assert.True(t, buttons[1].Disabled())
		entries[0].SetText("Holiday")
		assert.False(t, buttons[1].Disabled())

		// A failed save shows the error and keeps the dialog open; a good one closes it.
		test.Tap(buttons[1])
		assert.True(t, overlayShows(s, "disk full"))
		require.NotNil(t, s.w.Canvas().Overlays().Top())
		saveErr = nil
		test.Tap(buttons[1])
		assert.Nil(t, s.w.Canvas().Overlays().Top(), "Save closes the dialog")
		assert.Equal(t, "Holiday", savedName)
		assert.True(t, savedActive, "Active is on by default")
		assert.False(t, cancelled)

		// Cancel closes it and lets the source clean up its download.
		s.sa.prefsSM.showSaveCollectionDialog("x", func(string, bool) error { return nil }, func() { cancelled = true })
		test.Tap(dialogButtons(s)[0])
		assert.Nil(t, s.w.Canvas().Overlays().Top())
		assert.True(t, cancelled)
	})
}

// D6: the suggested name uses full-width brackets in Chinese.
func TestCollectionNameFullWidthBrackets(t *testing.T) {
	inLanguages(t, func(t *testing.T, lang string) {
		if lang != "zh" {
			return
		}
		assert.Equal(t, "收藏集 %s（%d 个项目）", i18n.T("Collection %s (%d items)"))
	})
}
