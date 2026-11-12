package schema

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
)

func museumLinks(t *testing.T, cfg MuseumSettingsConfig) []HyperlinkItem {
	t.Helper()
	i18n.SetLanguage("en")
	panel := CreateMuseumSettingsPanel(cfg, func(string) {})
	for _, item := range panel.Sections[0].Items {
		if row, ok := item.(HorizontalRowItem); ok {
			var links []HyperlinkItem
			for _, it := range row.Items {
				links = append(links, it.(HyperlinkItem))
			}
			return links
		}
	}
	require.Fail(t, "no link row")
	return nil
}

func TestMuseumFirstLinkSaysOpenAccessByDefault(t *testing.T) {
	links := museumLinks(t, MuseumSettingsConfig{ID: "X", LicenseURL: "https://example.org/license"})
	assert.Equal(t, "Open Access (CC0)", links[0].Text)
	assert.Equal(t, "https://example.org/license", links[0].URL)
}

func TestMuseumFirstLinkCanNameAnotherLicense(t *testing.T) {
	links := museumLinks(t, MuseumSettingsConfig{ID: "NPM", LicenseURL: "https://example.org/opendata", LicenseText: "Open data (CC BY 4.0)"})
	assert.Equal(t, "Open data (CC BY 4.0)", links[0].Text)
	assert.Equal(t, "https://example.org/opendata", links[0].URL)
}

func TestMuseumPageHasNoAffiliationNote(t *testing.T) {
	panel := CreateMuseumSettingsPanel(MuseumSettingsConfig{ID: "X", Location: "Somewhere"}, func(string) {})
	for _, s := range panel.Sections {
		for _, item := range s.Items {
			if label, ok := item.(LabelItem); ok {
				assert.NotContains(t, label.Text, "affiliated", "the owner dropped the no-affiliation line")
			}
		}
	}
}
