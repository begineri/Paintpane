package asset

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	npmCreditZh = "國立故宮博物院，臺北，CC BY 4.0 @ www.npm.gov.tw"
	npmCreditEn = "The National Palace Museum, Taipei, CC BY 4.0 @ www.npm.gov.tw"
)

// The EULA carries the credit the National Palace Museum requires for its CC BY 4.0
// images (section 2.6), and states the Met's CC0 terms without a non-commercial limit.
func TestEULAMuseumTerms(t *testing.T) {
	text, err := NewManager().GetText("eula.txt")
	require.NoError(t, err)

	assert.Contains(t, text, "### 2.6 National Palace Museum, Taipei")
	assert.Contains(t, text, npmCreditZh)
	assert.Contains(t, text, npmCreditEn)
	assert.Contains(t, text, "Creative Commons Zero (CC0)")
	assert.NotContains(t, text, "CC0) for non-commercial use", "CC0 has no non-commercial limit")
}

// Google Photos is hidden (P10), so the EULA no longer has a section for it (E7).
func TestEULAHasNoGooglePhotos(t *testing.T) {
	for _, name := range []string{"eula.txt", "eula.zh.txt"} {
		text, err := NewManager().GetText(name)
		require.NoError(t, err, name)
		assert.NotContains(t, text, "Google", name)
		assert.NotContains(t, text, "谷歌", name)
		assert.NotContains(t, text, "Photos API", name)
	}
}

// The EULA writes "wallhaven" in lower case, as the site and the app's interface do (D5).
func TestEULAWallhavenLowerCase(t *testing.T) {
	for _, name := range []string{"eula.txt", "eula.zh.txt"} {
		text, err := NewManager().GetText(name)
		require.NoError(t, err, name)
		assert.Contains(t, text, "### 2.1 wallhaven", name)
		assert.NotContains(t, text, "Wallhaven", name)
	}
}

// The Chinese text has the same sections as the English one and the same credit, and says that
// the English text prevails (E3).
func TestEULAChineseMatchesEnglish(t *testing.T) {
	en, err := NewManager().GetText("eula.txt")
	require.NoError(t, err)
	zh, err := NewManager().GetText("eula.zh.txt")
	require.NoError(t, err)

	numbered := regexp.MustCompile(`(?m)^(#+) (\d+(?:\.\d+)?)\.? `)
	headings := func(s string) (out []string) {
		for _, m := range numbered.FindAllStringSubmatch(s, -1) {
			out = append(out, m[1]+" "+m[2])
		}
		return out
	}
	assert.Equal(t, headings(en), headings(zh), "same headings in the same order (numbers)")
	assert.Contains(t, zh, npmCreditZh)
	assert.Contains(t, zh, "以英文版")
}
