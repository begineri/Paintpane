package curation

import "strings"

// gettyMixedNudeCollectionKey is treated as a nude collection as a whole because
// it mixes artistic nudes with other photography and has no per-image labels.
const gettyMixedNudeCollectionKey = "getty_photography"

func isNudeEntry(e CollectionEntry) bool {
	if e.Key == gettyMixedNudeCollectionKey {
		return true
	}
	return strings.Contains(strings.ToLower(e.Key+" "+e.Name), "nude")
}

// IsNudeID reports whether a namespaced image ID ("<ProviderID>_<ID>") belongs to a
// nude collection. Matching on the image rather than its source query also catches
// artworks that appear in other collections.
func (m *Manager) IsNudeID(id string) bool {
	m.nudeMu.Lock()
	defer m.nudeMu.Unlock()
	if m.nudeIDs == nil {
		m.nudeIDs = m.buildNudeIDs()
	}
	return m.nudeIDs[id]
}

func (m *Manager) buildNudeIDs() map[string]bool {
	ids := make(map[string]bool)
	for providerID := range ProviderIDToFilename {
		col := m.GetCollection(providerID)
		if col == nil {
			continue
		}
		prefix := providerID + "_"
		for _, entry := range col.Entries {
			if !isNudeEntry(entry) {
				continue
			}
			for _, id := range entry.IDs {
				ids[prefix+id] = true
			}
			for _, item := range entry.Items {
				ids[prefix+item.ID] = true
			}
		}
	}
	return ids
}

// invalidateNudeIDs must not be called while holding m.mu: IsNudeID acquires nudeMu
// before m.mu.
func (m *Manager) invalidateNudeIDs() {
	m.nudeMu.Lock()
	defer m.nudeMu.Unlock()
	m.nudeIDs = nil
}
