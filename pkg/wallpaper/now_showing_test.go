package wallpaper

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
)

func TestNowShowingLabelKeepsArtistWhole(t *testing.T) {
	wp := &Plugin{providers: map[string]provider.ImageProvider{}}

	long := provider.Image{
		Title:    "Two Sisters (On the Terrace), a Painting with a Very Long Descriptive Title",
		Artist:   "Pierre-Auguste Renoir",
		Provider: "ArtInstituteChicago",
	}
	label := wp.nowShowingLabel(long)
	assert.LessOrEqual(t, len([]rune(label)), 56)
	assert.True(t, strings.HasSuffix(label, " — Pierre-Auguste Renoir"), label)
	assert.Contains(t, label, "…")
	assert.NotContains(t, label, "·", "the source is not repeated in the text")

	short := provider.Image{Title: "Irises", Artist: "Vincent van Gogh", Provider: "Getty"}
	assert.Equal(t, "Irises — Vincent van Gogh", wp.nowShowingLabel(short))

	inTitle := provider.Image{Title: "Rembrandt van Rijn - The Night Watch", Artist: "Rembrandt van Rijn"}
	assert.Equal(t, "Rembrandt van Rijn - The Night Watch", wp.nowShowingLabel(inTitle), "an artist already in the title is not repeated")

	assert.Equal(t, "Ansel Adams", wp.nowShowingLabel(provider.Image{Attribution: "Ansel Adams"}))
}

func TestNowShowingLabelRijksmuseum(t *testing.T) {
	wp := &Plugin{}
	img := provider.Image{Provider: "Rijksmuseum", Title: "The Milkmaid", Artist: "Johannes Vermeer",
		Attribution: "Johannes Vermeer - The Milkmaid"}
	assert.Equal(t, "The Milkmaid — Johannes Vermeer", wp.nowShowingLabel(img))
}

// A2 §12.9 #2: a Wikimedia file without an author has only the licence in its
// attribution; the tray shows no artist for it.
func TestNowShowingLicenceOnlyAttribution(t *testing.T) {
	title, artist := titleAndArtist(provider.Image{Attribution: "(Public domain)"})
	assert.Equal(t, "", title)
	assert.Equal(t, "", artist)
}

// T-R3: the artist is reduced to the name, capped at 24 characters; the title gets the rest of 56.
func TestCleanArtistName(t *testing.T) {
	cases := map[string]string{
		"Albert Bierstadt (American, born Prussia [now Germany], 1830–1902)": "Albert Bierstadt",
		"Vincent van Gogh":                      "Vincent van Gogh",
		"Rembrandt van Rijn, 1606-1669":         "Rembrandt van Rijn",
		"Utagawa Hiroshige; Utagawa Kunisada":   "Utagawa Hiroshige",
		"Claude Monet 1840–1926":                "Claude Monet",
		"Katsushika Hokusai, c. 1760":           "Katsushika Hokusai",
		"Harald Sohlberg\nNorwegian, 1869-1935": "Harald Sohlberg",
		"郭熙（宋）":                                 "郭熙",
		"Workshop of Rogier van der Weyden [?]": "Workshop of Rogier van der Weyden",
		// A2 §12.9 #1: a character reference is decoded, not cut at its semicolon.
		"Melchior d&#39; Hondecoeter": "Melchior d' Hondecoeter",
		"":                            "",
	}
	for in, want := range cases {
		assert.Equal(t, want, cleanArtistName(in), in)
	}
}

// The Met, Getty and the Rijksmuseum attribute "Artist - Title"; without an
// artist the tray shows the title alone, not "- Title".
func TestNowShowingAttributionWithoutArtist(t *testing.T) {
	wp := &Plugin{}
	title := "Buddha Attended by Bodhisattvas Avalokiteshvara and Maitreya"
	img := provider.Image{Title: title, Attribution: " - " + title}
	assert.Equal(t, "Buddha Attended by Bodhisattvas Avalokiteshvara and Mai…", wp.nowShowingLabel(img), "the title alone, cut at 56")
	gotTitle, gotArtist := wp.nowShowingLines(img)
	assert.Equal(t, title, gotTitle)
	assert.Empty(t, gotArtist)

	img = provider.Image{Title: "The Milkmaid", Attribution: "Johannes Vermeer - The Milkmaid"}
	_, gotArtist = wp.nowShowingLines(img)
	assert.Equal(t, "Johannes Vermeer", gotArtist, "the artist from an attribution, without the title")

	img = provider.Image{Title: "Untitled", Attribution: "Photo by Ana Ruiz"}
	_, gotArtist = wp.nowShowingLines(img)
	assert.Equal(t, "Photo by Ana Ruiz", gotArtist, "other attributions are kept")
}

func TestNowShowingLabelArtistRule(t *testing.T) {
	wp := &Plugin{}
	img := provider.Image{Title: "Mount Starr King, Yosemite",
		Artist: "Albert Bierstadt (American, born Prussia [now Germany], 1830–1902)"}
	assert.Equal(t, "Mount Starr King, Yosemite — Albert Bierstadt", wp.nowShowingLabel(img))

	long := provider.Image{Title: "A View of the Grand Canal with the Church of Santa Maria della Salute at Dusk",
		Artist: "Giovanni Antonio Canal called Canaletto"}
	label := wp.nowShowingLabel(long)
	assert.LessOrEqual(t, len([]rune(label)), 56, label)
	assert.True(t, strings.HasSuffix(label, " — Giovanni Antonio Canal…"), "artist capped at 24: %s", label)

	title, artist := wp.nowShowingLines(img)
	assert.Equal(t, "Mount Starr King, Yosemite", title)
	assert.Equal(t, "Albert Bierstadt", artist, "the app-drawn menu shows the cleaned name on its own line")
}
