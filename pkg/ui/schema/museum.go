package schema

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
)

// MuseumSettingsConfig holds the configuration for a generic Museum settings panel.
type MuseumSettingsConfig struct {
	ID          string // Unique identifier (e.g., "AIC", "Met") — lowercased for control name prefixes
	Title       string
	Location    string
	LicenseURL  string
	LicenseText string // text of the first link; "Open Access (CC0)" when empty
	Description string // Not shown in the panel; kept for providers and galleries
	MapQuery    string
	WebsiteURL  string
	DonateURL   string

	// Optional: API Key/Token support (leave empty for zero-auth providers)
	APIKeyLabel     string        // e.g. "API Key:" or "API Token:" — empty means no key needed
	APIKeyHelp      string        // Help text for the key input
	RegistrationURL string        // URL where users can register for a key
	APIKeyGetFunc   func() string // Read current key value
	APIKeySetFunc   func(string)  // Save new key value

}

// CreateMuseumSettingsPanel generates a standard PanelSchema for museum providers.
func CreateMuseumSettingsPanel(cfg MuseumSettingsConfig, openURL func(string)) *PanelSchema {
	prefix := strings.ToLower(cfg.ID)

	// Header: the location on its own line, then one line of links (G6). The
	// museum's name is the page title.
	mapURL := fmt.Sprintf("https://www.google.com/maps/search/?api=1&query=%s", url.QueryEscape(cfg.MapQuery))
	licenseText := cfg.LicenseText
	if licenseText == "" {
		licenseText = i18n.T("Open Access (CC0)")
	}
	links := []ItemSchema{
		HyperlinkItem{Text: licenseText, URL: cfg.LicenseURL},
		HyperlinkItem{Text: i18n.T("Visit website"), URL: cfg.WebsiteURL},
		HyperlinkItem{Text: i18n.T("Plan a visit"), URL: mapURL},
		HyperlinkItem{Text: i18n.T("Donate"), URL: cfg.DonateURL},
	}
	sections := []SectionSchema{
		{
			ID:      cfg.ID,
			Compact: true,
			Flat:    true, // lines of text under the title, not a card
			Items: []ItemSchema{
				LabelItem{Text: cfg.Location, Importance: ImportanceLow},
				HorizontalRowItem{ID: prefix + "_links", Layout: RowLayoutInline, Items: links},
			},
		},
	}

	// Optional: API Key section (only if APIKeyLabel is set)
	if cfg.APIKeyLabel != "" {
		apiItems := []ItemSchema{
			TextItem{
				Name:         prefix + "_apikey",
				Label:        cfg.APIKeyLabel,
				Help:         cfg.APIKeyHelp,
				InitialValue: cfg.APIKeyGetFunc(),
				IsPassword:   true,
				ApplyFunc: func(val string) {
					cfg.APIKeySetFunc(val)
				},
			},
		}
		if cfg.RegistrationURL != "" {
			apiItems = append(apiItems, HyperlinkItem{
				Text: i18n.T("Get a free API key"),
				URL:  cfg.RegistrationURL,
			})
		}
		sections = append(sections, SectionSchema{
			Title:   i18n.T("Authentication"),
			Compact: true,
			Items:   apiItems,
		})
	}

	return &PanelSchema{
		Sections: sections,
	}
}
