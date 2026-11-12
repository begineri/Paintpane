package ui

import (
	"testing"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/stretchr/testify/assert"
)

func TestQueryDisplayName(t *testing.T) {
	t.Cleanup(func() { i18n.SetLanguage("en") })

	defaults := []string{
		"Random General",
		"Toplist last 3 months General",
		"Wikimedia Featured",
		"Wikimedia Featured Astronomy",
		"Favorite Images",
	}

	i18n.SetLanguage("en")
	for _, d := range defaults {
		assert.Equal(t, d, queryDisplayName(d), "English shows the stored name")
	}

	for _, lang := range []string{"zh", "zh-Hant"} {
		i18n.SetLanguage(lang)
		for _, d := range defaults {
			assert.NotEqual(t, d, queryDisplayName(d), "%s: default query %q must be translated", lang, d)
		}
	}

	i18n.SetLanguage("zh")
	assert.Equal(t, "维基共享资源精选图片", queryDisplayName("Wikimedia Featured"))
	assert.Equal(t, "收藏的图片", queryDisplayName("Favorite Images"))
	assert.Equal(t, "排行榜 · 近 3 个月 · 综合", queryDisplayName("Toplist last 3 months General"))
	assert.Equal(t, "My own query", queryDisplayName("My own query"), "names the user typed are left alone")
	assert.Equal(t, "General", queryDisplayName("General"), "only the known default names are translated")
}
