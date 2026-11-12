package curation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newNudeTestManager(t *testing.T) *Manager {
	t.Helper()
	m := NewManager()
	m.CacheDir = t.TempDir() // Only the data embedded in this repo
	return m
}

func TestIsNudeID_CoversEveryNudeCollection(t *testing.T) {
	m := newNudeTestManager(t)

	expected := []struct{ providerID, key string }{
		{"ArtInstituteChicago", "artic_nudes"},
		{"ClevelandMuseum", "Artistic Nudes"},
		{"Getty", "getty_nudes"},
		{"Getty", "getty_photography"},
		{"MetMuseum", "metmuseum_artistic_nudes"},
		{"Rijksmuseum", "rijks_nudes_photography"},
		{"Rijksmuseum", "rijks_nudes_mixed"},
		{"StatensMuseumForKunst", "smk_artistic_nudes"},
	}
	for _, c := range expected {
		entry := m.GetEntry(c.providerID, c.key)
		require.NotNil(t, entry, c.key)
		ids := append([]string{}, entry.IDs...)
		for _, item := range entry.Items {
			ids = append(ids, item.ID)
		}
		require.NotEmpty(t, ids, c.key)
		for _, id := range ids {
			assert.True(t, m.IsNudeID(c.providerID+"_"+id), "%s %s", c.key, id)
		}
	}
}

func TestIsNudeID_IgnoresOtherCollections(t *testing.T) {
	m := newNudeTestManager(t)

	col := m.GetCollection("NationalPalaceMuseum")
	require.NotNil(t, col)
	for _, entry := range col.Entries {
		for _, id := range entry.IDs {
			assert.False(t, m.IsNudeID("NationalPalaceMuseum_"+id), id)
		}
	}
	assert.False(t, m.IsNudeID("Wallhaven_abc123"))

	// The same bare ID under another provider is a different image.
	nudes := m.GetEntry("MetMuseum", "metmuseum_artistic_nudes")
	require.NotNil(t, nudes)
	assert.False(t, m.IsNudeID("ClevelandMuseum_"+nudes.IDs[0]))
}

func TestIsNudeID_RebuildsAfterCollectionChange(t *testing.T) {
	m := newNudeTestManager(t)
	m.embeddedData["Getty"] = []byte(`{"version":"v1.0.0","collections":[{"key":"x_nudes","name":"Nudes","type":"curated","ids":["a"]}]}`)
	assert.True(t, m.IsNudeID("Getty_a"))

	m.embeddedData["Getty"] = []byte(`{"version":"v1.0.1","collections":[{"key":"x_other","name":"Other","type":"curated","ids":["a"]}]}`)
	m.ClearCache("Getty")
	assert.False(t, m.IsNudeID("Getty_a"))
}
