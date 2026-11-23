package ui

import (
	"bytes"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/go-text/typesetting/font"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
)

// The rewritten font maps every character it kept to the same glyph as the bundled font, and
// none of the dropped ones.
func TestDropFromCmapKeepsEveryOtherCharacter(t *testing.T) {
	for _, res := range []fyne.Resource{fontRegular, fontBold} {
		drop := narrowAlways + narrowOutsideCJK
		data, err := dropFromCmap(res.Content(), []rune(drop))
		require.NoError(t, err, res.Name())

		orig, err := font.ParseTTF(bytes.NewReader(res.Content()))
		require.NoError(t, err)
		patched, err := font.ParseTTF(bytes.NewReader(data))
		require.NoError(t, err, "the rewritten %s loads", res.Name())

		kept := 0
		for it := orig.Cmap.Iter(); it.Next(); {
			r, g := it.Char()
			if g == 0 { // the end marker of a format 4 subtable (U+FFFF), not a character
				continue
			}
			pg, ok := patched.Cmap.Lookup(r)
			if bytes.ContainsRune([]byte(drop), r) {
				assert.False(t, ok, "%s: %q is left out", res.Name(), r)
				continue
			}
			if assert.True(t, ok, "%s: %q is kept", res.Name(), r) {
				assert.Equal(t, g, pg, "%s: %q keeps its glyph", res.Name(), r)
			}
			kept++
		}
		assert.Greater(t, kept, 10000, "%s: the subset's characters are all there", res.Name())
		// The glyphs themselves are untouched.
		g, _ := patched.Cmap.Lookup('汉')
		assert.Equal(t, orig.HorizontalAdvance(mustGlyph(t, orig, '汉')), patched.HorizontalAdvance(g))
	}
}

func mustGlyph(t *testing.T, f *font.Face, r rune) font.GID {
	t.Helper()
	g, ok := f.Cmap.Lookup(r)
	require.True(t, ok, "%q in the font", r)
	return g
}

// What a user sees: in English and German the dot, quotes and ellipsis are narrow; in Chinese
// the quotes and ellipsis keep the full width Chinese typesetting wants and only the dot is
// narrow; Japanese is like Chinese.
func TestPunctuationWidthFollowsTheLanguage(t *testing.T) {
	a := test.NewTempApp(t)
	t.Cleanup(func() { i18n.SetLanguage("en") })

	width := func(s string, bold bool) float32 {
		return fyne.MeasureText(s, 13, fyne.TextStyle{Bold: bold}).Width
	}
	for _, lang := range []string{"en", "de", "ru", "zh", "zh-Hant", "ja"} {
		i18n.SetLanguage(lang)
		a.Settings().SetTheme(newPaintpaneTheme(themeModeLight))
		cjk := lang != "en" && lang != "de" && lang != "ru"
		for _, bold := range []bool{false, true} {
			han := width("汉", bold)
			for _, r := range narrowAlways + narrowOutsideCJK {
				w := width(string(r), bold)
				narrow := !cjk || r == '·'
				if narrow {
					assert.Less(t, w, 0.9*han, "%s bold=%v: %q is narrow (%.1f px, a Chinese character is %.1f)", lang, bold, r, w, han)
				} else {
					assert.InDelta(t, han, w, 0.5, "%s bold=%v: %q stays full width", lang, bold, r)
				}
			}
		}
	}
}

// The English confirm title from the report: the quotes add about a letter's width, not two
// Chinese characters'.
func TestDeleteTitleQuotesAreNarrowInEnglish(t *testing.T) {
	a := test.NewTempApp(t)
	t.Cleanup(func() { i18n.SetLanguage("en") })
	i18n.SetLanguage("en")
	a.Settings().SetTheme(newPaintpaneTheme(themeModeLight))

	style := fyne.TextStyle{Bold: true}
	quoted := fyne.MeasureText("Delete “Wikimedia Featured Astronomy”?", 16, style).Width
	plain := fyne.MeasureText("Delete Wikimedia Featured Astronomy?", 16, style).Width
	han := fyne.MeasureText("汉", 16, style).Width
	assert.Less(t, quoted-plain, 0.9*han, "two quotes are %.1f px together; a Chinese character is %.1f", quoted-plain, han)
}

// Switching the UI language between Chinese and English installs the theme again, so the fonts
// change at once; switching between two Latin languages does not.
func TestApplyThemeFollowsTheLanguage(t *testing.T) {
	a := test.NewTempApp(t)
	t.Cleanup(func() { i18n.SetLanguage("en") })

	i18n.SetLanguage("zh")
	applyTheme(a, themeModeLight, accentDefault)
	zh := currentTheme(t)
	assert.Equal(t, narrowAlways, zh.narrow)

	i18n.SetLanguage("en")
	applyTheme(a, themeModeLight, accentDefault)
	en := currentTheme(t)
	assert.NotSame(t, zh, en, "Chinese to English installs a new theme")
	assert.Equal(t, narrowAlways+narrowOutsideCJK, en.narrow)

	i18n.SetLanguage("de")
	applyTheme(a, themeModeLight, accentDefault)
	assert.Same(t, en, currentTheme(t), "English to German draws the same")
}
