package ui

import (
	"strings"
	"testing"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
)

// monoWidth measures Latin characters as 1 and Chinese ones as 2, like a monospace terminal.
func monoWidth(s string) float32 {
	var w float32
	for _, r := range s {
		if isCJK(r) {
			w += 2
		} else {
			w++
		}
	}
	return w
}

func TestWrapTextKeepsNumberWithUnit(t *testing.T) {
	// The A2 case: the line is full right after "12"; Fyne broke "（12 / MB）".
	got := wrapText("将删除 2 张已下载的图片（12 MB）。之后需要联网才能看到新壁纸。", 30, monoWidth)
	assert.NotContains(t, got, "12\n", got)
	assert.Contains(t, got, "12 MB", got)
	for _, line := range strings.Split(got, "\n") {
		assert.LessOrEqual(t, monoWidth(line), float32(30), line)
		assert.False(t, strings.HasPrefix(line, "）") || strings.HasPrefix(line, "。"), "closing punctuation does not start a line: %q", line)
		assert.False(t, strings.HasSuffix(line, "（"), "opening punctuation does not end a line: %q", line)
	}
	assert.Equal(t, "将删除 2 张已下载的图片（12 MB）。之后需要联网才能看到新壁纸。",
		strings.ReplaceAll(got, "\n", ""), "only line breaks are added")
}

func TestWrapTextChineseBreaksBetweenCharacters(t *testing.T) {
	// Fyne would break at the space after "2", leaving "将删除 2" alone on the first line.
	got := wrapText("将删除 2 张已下载的图片之后需要联网才能看到新壁纸", 24, monoWidth)
	first := strings.Split(got, "\n")[0]
	assert.Greater(t, monoWidth(first), float32(20), "the first line is filled: %q", first)
	assert.NotContains(t, got, "2\n", "the number stays with 张")
}

func TestWrapTextEnglishBreaksAtSpaces(t *testing.T) {
	got := wrapText("This deletes 312 downloaded images (1.2 GB). You will need internet to see new wallpapers.", 30, monoWidth)
	for _, line := range strings.Split(got, "\n") {
		assert.LessOrEqual(t, monoWidth(line), float32(30), line)
		assert.False(t, strings.HasPrefix(line, " ") || strings.HasSuffix(line, " "), "no spaces at line ends: %q", line)
	}
	assert.NotContains(t, got, "1.2\n")
	assert.NotContains(t, got, "312\n")
	assert.Equal(t, "This deletes 312 downloaded images (1.2 GB). You will need internet to see new wallpapers.",
		strings.ReplaceAll(got, "\n", " "))
}

func TestWrapTextEdgeCases(t *testing.T) {
	assert.Equal(t, "", wrapText("", 10, monoWidth))
	assert.Equal(t, "short", wrapText("short", 10, monoWidth))
	assert.Equal(t, "a\nb", wrapText("a\nb", 10, monoWidth), "existing line breaks are kept")
	got := wrapText("abcdefghijklmnop", 5, monoWidth)
	assert.Equal(t, "abcde\nfghij\nklmno\np", got, "a word wider than the line is broken between characters")
	assert.Equal(t, "a\nb", wrapText("ab", 0.5, monoWidth), "one character per line when nothing fits; never loops")
}

// The real dialog: at the dialog's width, with the app's font, "（12 MB）" is on one line in every
// language that has it.
func TestClearCacheDialogTextKeepsSizeTogether(t *testing.T) {
	newScenarioWith(t, wallpaper.GetInstance()) // a test app with the Paintpane theme's fonts
	width := dialogWidth - 2*dialogPadding
	for _, lang := range []string{"zh", "zh-Hant", "en", "de", "ja"} {
		i18n.SetLanguage(lang)
		for _, bytes := range []int64{12e6, 3_800_000_000, 950e3} {
			msg := wallpaper.CacheUsage{Images: 2, Bytes: bytes}.ClearMessage()
			size := wallpaper.FormatBytes(bytes)
			txt := newDialogText(msg, dialogWidth)
			require.True(t, utf8.ValidString(txt.Text))
			assert.Contains(t, txt.Text, size, "%s: %q is on one line in %q", lang, size, txt.Text)
			for _, line := range strings.Split(txt.Text, "\n") {
				w := fyne.MeasureText(line, theme.TextSize(), fyne.TextStyle{}).Width
				assert.LessOrEqual(t, w, width, "%s: %q fits", lang, line)
			}
		}
	}
	i18n.SetLanguage("en")
}
