package getty

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// A2 §12.9 #7: the object number at the end of a Getty title is removed; other
// parentheses stay.
func TestStripAccessionNumber(t *testing.T) {
	cases := map[string]string{
		"A Walk at Dusk (93.PA.14)":                         "A Walk at Dusk",
		"Irises (90.PA.20)":                                 "Irises",
		"Wheatstacks, Snow Effect, Morning (95.PA.63)":      "Wheatstacks, Snow Effect, Morning",
		"Still Life (84.XM.1004.1)":                         "Still Life",
		"Vase of Flowers (85.GA.27a)":                       "Vase of Flowers",
		"Portrait of a Woman (2001.43)":                     "Portrait of a Woman",
		"Landscape (2011.17.1)":                             "Landscape",
		"Christ on the Cross (Crucifixion)":                 "Christ on the Cross (Crucifixion)",
		"The Abduction of Europa (c. 1632)":                 "The Abduction of Europa (c. 1632)",
		"Madonna (1500)":                                    "Madonna (1500)",
		"Study (recto) (93.GB.51)":                          "Study (recto)",
		"(93.PA.14)":                                        "(93.PA.14)",
		"Untitled":                                          "Untitled",
		"Bifolium from the Pink Qur’an (Ms. 122 (2021.44))": "Bifolium from the Pink Qur’an (Ms. 122)",
		"Leaf (Ms. Ludwig IX 7 (83.ML.103))":                "Leaf (Ms. Ludwig IX 7)",
		"Saint (Study (c. 1632))":                           "Saint (Study (c. 1632))",
		"Portrait (after Titian (1500))":                    "Portrait (after Titian (1500))",
		"The Annunciation (93.PA.14) and Other Studies":     "The Annunciation (93.PA.14) and Other Studies",
	}
	for in, want := range cases {
		assert.Equal(t, want, stripAccessionNumber(in), in)
	}
}

// A record without a title or an artist leaves that name empty instead of
// "Unknown Title" / "Unknown Artist", which the tray would show.
func TestObjectNames(t *testing.T) {
	full := map[string]interface{}{
		"_label": "Irises (90.PA.20)",
		"produced_by": map[string]interface{}{
			"carried_out_by": []interface{}{
				map[string]interface{}{"_label": " Vincent van Gogh "},
				map[string]interface{}{"_label": "Someone Else"},
			},
		},
	}
	title, artist := objectNames(full)
	assert.Equal(t, "Irises", title)
	assert.Equal(t, "Vincent van Gogh", artist)
	assert.Equal(t, "Vincent van Gogh - Irises", attribution(artist, title))

	title, artist = objectNames(map[string]interface{}{"_label": "Irises (90.PA.20)"})
	assert.Equal(t, "Irises", title)
	assert.Empty(t, artist)
	assert.Equal(t, "Irises", attribution(artist, title))

	title, artist = objectNames(map[string]interface{}{
		"produced_by": map[string]interface{}{
			"carried_out_by": []interface{}{map[string]interface{}{"_label": "Vincent van Gogh"}},
		},
	})
	assert.Empty(t, title)
	assert.Equal(t, "Vincent van Gogh", artist)
	assert.Equal(t, "Vincent van Gogh", attribution(artist, title))

	title, artist = objectNames(map[string]interface{}{"produced_by": map[string]interface{}{"carried_out_by": []interface{}{}}})
	assert.Empty(t, title)
	assert.Empty(t, artist)
	assert.Empty(t, attribution(artist, title))
}
