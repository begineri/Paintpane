package curation

import (
	"encoding/json"
	"os"
	"testing"
)

// A fresh install gets its museum queries from asset/text/default_config.json. Each one must name a
// collection that the app ships (docs/collections/*.json), or that museum returns no images at all.
//
// A2 acceptance (docs/test-report-v1.0.md, problem 3) found that the Met default was the retired key
// "spice_melange", so a fresh install got no images from the Met. This test guards against that.
func TestDefaultMuseumQueriesNameShippedCollections(t *testing.T) {
	data, err := os.ReadFile("../../asset/text/default_config.json")
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Queries []struct {
			Desc     string `json:"desc"`
			URL      string `json:"url"`
			Provider string `json:"provider"`
		} `json:"queries"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}

	m := NewManager()
	m.CacheDir = t.TempDir() // never read the user's cached collections
	checked := 0
	for _, q := range cfg.Queries {
		if _, isMuseum := ProviderIDToFilename[q.Provider]; !isMuseum {
			continue
		}
		checked++
		if m.GetEntry(q.Provider, q.URL) == nil {
			t.Errorf("default query %q (%s) names collection %q, which is not shipped in docs/collections", q.Desc, q.Provider, q.URL)
		}
	}
	if checked == 0 {
		t.Fatal("no museum queries found in default_config.json")
	}
}
