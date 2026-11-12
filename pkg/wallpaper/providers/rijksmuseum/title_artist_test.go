package rijksmuseum

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/curation"
)

// The tray shows "Title — Artist", so the title and the artist must stay apart.
func TestCuratedImagesKeepTitleAndArtistApart(t *testing.T) {
	p := &Provider{}
	images, err := p.fetchCurated(&curation.CollectionEntry{Items: []curation.PreResolvedItem{{
		ID: "200108369", AccessionID: "SK-A-2344", Title: "The Milkmaid", Artist: "Johannes Vermeer",
		ImageURL: "https://example.org/milkmaid.jpg",
	}}}, 1)
	require.NoError(t, err)
	require.Len(t, images, 1)
	assert.Equal(t, "The Milkmaid", images[0].Title)
	assert.Equal(t, "Johannes Vermeer", images[0].Artist)
	assert.Equal(t, "Johannes Vermeer - The Milkmaid", images[0].Attribution, "attribution is unchanged")
}

// A2 §12.9 #1: the Rijksmuseum data keeps HTML character references in names
// ("Melchior d&#39; Hondecoeter"); the provider decodes them.
func TestRijksmuseumDecodesCharacterReferences(t *testing.T) {
	artist := ExtractArtist(&Production{ReferredToBy: []LinguisticObject{{
		Content: "Melchior d&#39; Hondecoeter", ClassifiedAs: []ClassifiedAsItem{{ID: aatArtistStatement}},
		Language: []TypedRef{{ID: aatEnglish}},
	}}})
	assert.Equal(t, "Melchior d' Hondecoeter", artist)

	title := ExtractTitle([]byte(`[{"type": "Name", "content": "Birds in a Park &amp; a Fountain"}]`))
	assert.Equal(t, "Birds in a Park & a Fountain", title)

	p := &Provider{}
	images, err := p.fetchCurated(&curation.CollectionEntry{Items: []curation.PreResolvedItem{{
		ID: "1", Title: "A Pelican &amp; Other Birds", Artist: "Melchior d&#39; Hondecoeter",
		ImageURL: "https://example.org/birds.jpg",
	}}}, 1)
	require.NoError(t, err)
	require.Len(t, images, 1)
	assert.Equal(t, "A Pelican & Other Birds", images[0].Title)
	assert.Equal(t, "Melchior d' Hondecoeter", images[0].Artist)
	assert.Equal(t, "Melchior d' Hondecoeter - A Pelican & Other Birds", images[0].Attribution)
}
