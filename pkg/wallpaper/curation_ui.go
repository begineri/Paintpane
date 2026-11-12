package wallpaper

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/dixieflatline76/Spice/v2/config"
	"github.com/dixieflatline76/Spice/v2/pkg/curation"
	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/setting"
)

// CreateCuratedQueryPanel generates a standard panel schema for any museum provider
// that uses the central curation.Manager.
func CreateCuratedQueryPanel(p provider.ImageProvider, sm setting.SettingsManager, cfg *Config) *schema.PanelSchema {
	mgr := curation.GetManager()

	// Get the collection from the manager using the provider's ID.
	col := mgr.GetCollection(p.ID())
	if col == nil {
		return &schema.PanelSchema{}
	}

	var items []schema.ItemSchema
	for _, entry := range col.Entries {
		items = append(items, buildCuratedUIItem(p, sm, cfg, entry))
	}

	return &schema.PanelSchema{
		Sections: []schema.SectionSchema{
			{
				Title:     i18n.T("Curated collections"),
				SelectAll: curatedSelectAll(p, col.Entries),
				Items:     items,
			},
		},
	}
}

// curatedSelectAll is the Select all switch in the curated collections' header,
// which turns them all on or off; it shows on while every collection is on.
func curatedSelectAll(p provider.ImageProvider, entries []curation.CollectionEntry) *schema.SelectAllItem {
	members := make([]string, 0, len(entries))
	for _, entry := range entries {
		members = append(members, curatedSettingName(p, entry))
	}
	return &schema.SelectAllItem{
		Name:    p.ID() + "_curated_all",
		Label:   i18n.T("Select all"),
		Members: members,
	}
}

// curatedSettingName is the setting name of a curated collection's switch.
func curatedSettingName(p provider.ImageProvider, entry curation.CollectionEntry) string {
	return p.ID() + "_" + entry.Key
}

func buildCuratedUIItem(p provider.ImageProvider, sm setting.SettingsManager, cfg *Config, entry curation.CollectionEntry) schema.ItemSchema {
	// Helper to find existing query state
	getQuery := func(key string) (bool, string) {
		for _, q := range cfg.GetQueries() {
			if q.Provider == p.ID() && q.URL == key {
				return q.Active, q.ID
			}
		}
		return false, ""
	}

	active, _ := getQuery(entry.Key)

	actionText := ""
	var actionFunc func()
	label := strings.TrimSpace(i18n.TMap(entry.Name, entry.NameTranslations))
	note := ""

	if entry.Type == "curated" && len(entry.IDs) > 0 {
		actionText = i18n.T("Preview")
		note = curatedPieces(len(entry.IDs))

		actionFunc = func() {
			safeName := strings.ReplaceAll(strings.ToLower(entry.Key), " ", "_")
			fileName := fmt.Sprintf("%s.html", safeName)
			providerCacheDir := filepath.Join(config.GetCacheDir(), strings.ToLower(p.ID()))
			outPath := filepath.Join(providerCacheDir, fileName)

			langParam := i18n.GetLanguage()
			localeJSPath := filepath.Join(providerCacheDir, "current_locale.js")
			_ = os.WriteFile(localeJSPath, fmt.Appendf(nil, "window.spiceAppLocale = '%s';", langParam), 0600)

			// Stage files outside the MSIX sandbox so the browser can access them.
			outPath = resolveMSIXDocumentPath(outPath)
			_ = resolveMSIXDocumentPath(localeJSPath)

			fileURL := fmt.Sprintf("file:///%s", filepath.ToSlash(outPath))
			if u, err := url.Parse(fileURL); err == nil {
				_ = u
				sm.OpenURL(fileURL)
			}
		}
	} else if entry.Type == "curated" && len(entry.Items) > 0 {
		// Pre-resolved items (e.g. Rijksmuseum)
		actionText = i18n.T("Preview")
		note = curatedPieces(len(entry.Items))

		actionFunc = func() {
			safeName := strings.ReplaceAll(strings.ToLower(entry.Key), " ", "_")
			fileName := fmt.Sprintf("%s.html", safeName)
			providerCacheDir := filepath.Join(config.GetCacheDir(), strings.ToLower(p.ID()))
			outPath := filepath.Join(providerCacheDir, fileName)

			langParam := i18n.GetLanguage()
			localeJSPath := filepath.Join(providerCacheDir, "current_locale.js")
			_ = os.WriteFile(localeJSPath, fmt.Appendf(nil, "window.spiceAppLocale = '%s';", langParam), 0600)

			// Stage files outside the MSIX sandbox so the browser can access them.
			outPath = resolveMSIXDocumentPath(outPath)
			_ = resolveMSIXDocumentPath(localeJSPath)

			fileURL := fmt.Sprintf("file:///%s", filepath.ToSlash(outPath))
			if u, err := url.Parse(fileURL); err == nil {
				_ = u
				sm.OpenURL(fileURL)
			}
		}
	}

	boolItem := schema.BoolItem{
		Name:         curatedSettingName(p, entry),
		Label:        label,
		Note:         note,
		ActionText:   actionText,
		ActionIcon:   "view",
		ActionFunc:   actionFunc,
		InitialValue: active,
		NeedsRefresh: true,
		ApplyFunc: func(b bool) {
			found := false
			for _, q := range cfg.GetQueries() {
				if q.Provider == p.ID() && q.URL == entry.Key {
					found = true
					if b {
						_ = cfg.EnableImageQuery(q.ID)
					} else {
						_ = cfg.DisableImageQuery(q.ID)
					}
				}
			}
			if !found && b {
				_, _ = cfg.AddProviderQuery(entry.Name, entry.Key, p.ID(), true, false)
			}
		},
	}

	return boolItem
}

// curatedPieces is the number of pieces shown after a collection's name.
func curatedPieces(count int) string {
	return i18n.Tf("{{.Count}} pieces", map[string]any{"Count": count})
}
