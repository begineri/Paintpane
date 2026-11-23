package getty

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/internal/nettest"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
)

// gettyFixtures replays the object record from data.getty.edu recorded on 2026-11-18 (cut down
// to the properties the provider reads), so the tests never touch the real site.
var gettyFixtures = map[string]string{
	"data.getty.edu/museum/collection/object/9a9c0a0c-bcbc-46bc-8a7a-d1dcb06085ed": "testdata/object-9a9c0a0c.json",
}

// The record is read as plain JSON; this is what the JSON-LD framing it replaced (json-gold)
// read from the same record, field for field. The same held for the 46 curated Getty records
// fetched on 2026-11-19 (docs/roadmap.md L3).
func TestFetchObject_SameAsJSONLDFraming(t *testing.T) {
	p := NewProvider(nil, nettest.Replay(t, gettyFixtures))
	img, err := p.fetchObjectByUUID(context.Background(), "9a9c0a0c-bcbc-46bc-8a7a-d1dcb06085ed")
	require.NoError(t, err)
	assert.Equal(t, provider.Image{
		ID:          "9a9c0a0c-bcbc-46bc-8a7a-d1dcb06085ed",
		Path:        "https://media.getty.edu/iiif/image/1a776f13-a3e7-4416-b28e-d702de736159/full/max/0/default.jpg",
		ViewURL:     "https://www.getty.edu/art/collection/object/103RJ0",
		Attribution: "Jacques-Louis David - The Farewell of Telemachus and Eucharis",
		Title:       "The Farewell of Telemachus and Eucharis",
		Artist:      "Jacques-Louis David",
		Provider:    ProviderName,
	}, *img)
}

// Linked Art may write a property as one value or as a list; both read the same, and a record
// that is not an object is refused.
func TestParseObject_OneOrManyAndType(t *testing.T) {
	var single, list linkedArtObject
	require.NoError(t, json.Unmarshal([]byte(`{"type":"HumanMadeObject","_label":"Irises (90.PA.20)",
		"produced_by":{"carried_out_by":{"_label":"Vincent van Gogh"},"timespan":{"_label":"1889"}},
		"representation":{"id":"https://media.getty.edu/iiif/image/x/full/full/0/default.jpg"},
		"subject_of":{"id":"https://www.getty.edu/art/collection/object/X","format":"text/html"}}`), &single))
	require.NoError(t, json.Unmarshal([]byte(`{"type":"HumanMadeObject","_label":"Irises (90.PA.20)",
		"produced_by":[{"carried_out_by":[{"_label":"Vincent van Gogh"},{"_label":"Someone Else"}],"timespan":[{"_label":"1889"}]}],
		"representation":[{"id":"https://media.getty.edu/iiif/image/x/full/full/0/default.jpg"},{"id":"https://other"}],
		"subject_of":[{"id":"https://www.getty.edu/art/collection/object/X.json","format":"application/json"},
			{"id":"https://www.getty.edu/art/collection/object/X","format":"text/html"}]}`), &list))
	for _, obj := range []linkedArtObject{single, list} {
		img, err := parseObject(obj)
		require.NoError(t, err)
		assert.Equal(t, "https://media.getty.edu/iiif/image/x/full/max/0/default.jpg", img.Path)
		assert.Equal(t, "https://www.getty.edu/art/collection/object/X", img.ViewURL)
		assert.Equal(t, "Irises", img.Title)
		assert.Equal(t, "Vincent van Gogh", img.Artist)
		assert.Equal(t, "1889", img.Year)
		assert.Equal(t, "Vincent van Gogh - Irises", img.Attribution)
	}

	_, err := parseObject(linkedArtObject{Type: "Person", Label: "Vincent van Gogh"})
	assert.Error(t, err)
	_, err = parseObject(linkedArtObject{Type: "HumanMadeObject", Label: "No picture"})
	assert.Error(t, err)
}

func TestFetchThumbnails_TDD(t *testing.T) {
	p := NewProvider(nil, nettest.Replay(t, gettyFixtures))

	// Valid ID from Getty (needs to have an image representation)
	ids := []string{"9a9c0a0c-bcbc-46bc-8a7a-d1dcb06085ed"}
	thumbnails, err := p.FetchThumbnails(context.Background(), ids)
	if err != nil {
		t.Fatalf("FetchThumbnails error: %v", err)
	}

	if len(thumbnails) == 0 {
		t.Fatalf("Expected thumbnails, got none")
	}

	if thumbnails[0].ID != "9a9c0a0c-bcbc-46bc-8a7a-d1dcb06085ed" {
		t.Errorf("Expected ID '9a9c0a0c-bcbc-46bc-8a7a-d1dcb06085ed', got %s", thumbnails[0].ID)
	}
	if thumbnails[0].ViewURL == "" {
		t.Errorf("Expected ViewURL to be populated, got empty string")
	}

	// Invalid ID
	_, err = p.FetchThumbnails(context.Background(), []string{"invalid"})
	if err != nil {
		t.Errorf("Expected no error for invalid ID, got %v", err)
	}
}
