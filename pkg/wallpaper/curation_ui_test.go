//go:build !linux

package wallpaper

import (
	"testing"

	"github.com/dixieflatline76/Spice/v2/pkg/curation"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
	"github.com/stretchr/testify/assert"
)

func TestBuildCuratedUIItem_WithPreview(t *testing.T) {
	// Setup mocks
	p := new(MockImageProvider)
	p.On("ID").Return("testprov")
	cfg := &Config{} // Get default config

	// Test entry with preview items
	entry := curation.CollectionEntry{
		Key:  "test_collection",
		Name: "Test Collection",
		Type: "curated",
		IDs:  []string{"img1", "img2"},
	}

	// Build the item
	item := buildCuratedUIItem(p, nil, cfg, entry)

	// Collections with pieces are a switch row with a Preview link.
	boolItem, ok := item.(schema.BoolItem)
	assert.True(t, ok, "Expected buildCuratedUIItem to return a schema.BoolItem")
	if ok {
		assert.Equal(t, "Preview", boolItem.ActionText)
		assert.NotNil(t, boolItem.ActionFunc)
		assert.Equal(t, "Test Collection", boolItem.Label)
		assert.Equal(t, "2 pieces", boolItem.Note)
		assert.Equal(t, "view", boolItem.ActionIcon)
	}
}
