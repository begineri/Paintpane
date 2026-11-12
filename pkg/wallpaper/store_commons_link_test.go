package wallpaper

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Wikimedia images kept from before 2026-11-12 linked to the file itself; loading the library
// points them at the file's page on Commons and leaves every other link alone.
func TestLoadCacheLinksWikimediaImagesToTheirCommonsPage(t *testing.T) {
	tmpDir := t.TempDir()
	cache := filepath.Join(tmpDir, "image_cache_map.json")
	require.NoError(t, os.WriteFile(cache, []byte(`[
		{"ID":"Wikimedia_7914551","Provider":"Wikimedia",
		 "ViewURL":"https://upload.wikimedia.org/wikipedia/commons/1/16/Lagoon_Nebula_%28ESO%29.jpg?utm_source=commons.wikimedia.org"},
		{"ID":"Wikimedia_42","Provider":"Wikimedia","ViewURL":"https://commons.wikimedia.org/wiki/File:Already.jpg"},
		{"ID":"ClevelandMuseum_675579","Provider":"ClevelandMuseum","ViewURL":"https://clevelandart.org/art/2024.27"}
	]`), 0o600))

	store := NewImageStore()
	store.SetFileManager(NewFileManager(tmpDir), cache)
	require.NoError(t, store.LoadCache())

	want := map[string]string{
		"Wikimedia_7914551":      "https://commons.wikimedia.org/w/index.php?curid=7914551",
		"Wikimedia_42":           "https://commons.wikimedia.org/wiki/File:Already.jpg",
		"ClevelandMuseum_675579": "https://clevelandart.org/art/2024.27",
	}
	for id, url := range want {
		img, ok := store.GetByID(id)
		require.True(t, ok, id)
		assert.Equal(t, url, img.ViewURL, id)
	}
}
