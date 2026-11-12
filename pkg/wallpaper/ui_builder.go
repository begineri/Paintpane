package wallpaper

import (
	"errors"
	"sort"
	"strconv"

	"github.com/dixieflatline76/Spice/v2/pkg/curation"
	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/setting"
)

// PrefsPanelBuilder constructs the preferences UI.
type PrefsPanelBuilder struct {
	plugin *Plugin
	sm     setting.SettingsManager
}

func NewPrefsPanelBuilder(p *Plugin, sm setting.SettingsManager) *PrefsPanelBuilder {
	sm.RegisterOnSettingsSaved(func() {
		p.cfg.save()
	})
	return &PrefsPanelBuilder{
		plugin: p,
		sm:     sm,
	}
}

// isSmartCropSelected reports whether the pending artwork display mode is Smart Crop. The
// settings that only affect cropping are disabled otherwise.
func (b *PrefsPanelBuilder) isSmartCropSelected() bool {
	val := b.sm.GetValue("artworkDisplayMode")
	if val == nil {
		return b.plugin.cfg.GetArtworkDisplayMode() == ArtworkDisplaySmartCrop
	}
	return artworkDisplayModeAt(val.(int)) == ArtworkDisplaySmartCrop
}

// frequencyChoices are the rotation intervals offered in the select, in display order.
// Any other saved value shows as Custom.
var frequencyChoices = []Frequency{
	FrequencyNever, Frequency30Minutes, FrequencyHourly, Frequency3Hours,
	Frequency6Hours, Frequency(720), FrequencyDaily,
}

func frequencyLabel(f Frequency) string {
	switch f {
	case FrequencyNever:
		return i18n.T("Never")
	case Frequency30Minutes:
		return i18n.T("Every 30 minutes")
	case FrequencyHourly:
		return i18n.T("Every hour")
	case Frequency3Hours:
		return i18n.T("Every 3 hours")
	case Frequency6Hours:
		return i18n.T("Every 6 hours")
	case Frequency(720):
		return i18n.T("Every 12 hours")
	case FrequencyDaily:
		return i18n.T("Daily")
	}
	return ""
}

// frequencyIndex returns the select index of a frequency; values not in the list map to Custom.
func frequencyIndex(f Frequency) int {
	for i, c := range frequencyChoices {
		if c == f {
			return i
		}
	}
	return len(frequencyChoices)
}

func (b *PrefsPanelBuilder) pendingDisplayMode() ArtworkDisplayMode {
	if val, ok := b.sm.GetValue("artworkDisplayMode").(int); ok {
		return artworkDisplayModeAt(val)
	}
	return b.plugin.cfg.GetArtworkDisplayMode()
}

// framesVisible reports whether frame settings apply: only in Museum Frame. Smart crop fills
// the screen and never frames.
func (b *PrefsPanelBuilder) framesVisible() bool {
	return b.pendingDisplayMode() == ArtworkDisplayFramed
}

// wallVisible reports whether the wall colour shows anywhere.
func (b *PrefsPanelBuilder) wallVisible() bool {
	return b.pendingDisplayMode() != ArtworkDisplaySmartCrop
}

// BuildWallpaperPage creates the Wallpaper page: rotation, display and content.
func (b *PrefsPanelBuilder) BuildWallpaperPage() *schema.PanelSchema {
	cfg := b.plugin.cfg
	customIndex := len(frequencyChoices)
	freqOptions := make([]string, 0, customIndex+1)
	for _, f := range frequencyChoices {
		freqOptions = append(freqOptions, frequencyLabel(f))
	}
	freqOptions = append(freqOptions, i18n.T("Custom…"))

	isCustomFrequency := func() bool {
		val, ok := b.sm.GetValue("changeFrequencyChoice").(int)
		if !ok {
			return frequencyIndex(cfg.GetWallpaperChangeFrequency()) == customIndex
		}
		return val == customIndex
	}

	return &schema.PanelSchema{
		Sections: []schema.SectionSchema{
			{
				Title: i18n.T("Rotation"),
				Items: []schema.ItemSchema{
					schema.SelectItem{
						Name:         "changeFrequencyChoice",
						Label:        i18n.T("Change wallpaper"),
						Options:      freqOptions,
						InitialValue: frequencyIndex(cfg.GetWallpaperChangeFrequency()),
						ApplyFunc: func(val interface{}) {
							if i, ok := val.(int); ok && i >= 0 && i < customIndex {
								b.plugin.ChangeWallpaperFrequency(frequencyChoices[i], true)
							}
						},
						NeedsRefresh: true,
					},
					schema.TextItem{
						Name:         "changeFrequency",
						Label:        i18n.T("Custom interval (minutes)"),
						InitialValue: strconv.Itoa(int(cfg.GetWallpaperChangeFrequency())),
						IsNumeric:    true,
						Validator: func(s string) error {
							val, err := strconv.Atoi(s)
							if err != nil || val < 1 {
								return errors.New(i18n.T("Enter a number of minutes, 1 or more"))
							}
							return nil
						},
						ApplyFunc: func(val string) {
							if !isCustomFrequency() {
								return
							}
							if minutes, err := strconv.Atoi(val); err == nil && minutes > 0 {
								b.plugin.ChangeWallpaperFrequency(Frequency(minutes), true)
							}
						},
						VisibleIf:    isCustomFrequency,
						NeedsRefresh: true,
					},
					schema.BoolItem{
						Name:         "nightlyRefresh",
						Label:        i18n.T("Refresh wallpapers nightly"),
						InitialValue: cfg.GetNightlyRefresh(),
						ApplyFunc:    cfg.SetNightlyRefresh,
					},
				},
			},
			{
				Title: i18n.T("Picture"),
				Items: []schema.ItemSchema{
					schema.SelectItem{
						Name:      "artworkDisplayMode",
						Label:     i18n.T("Display mode"),
						Options:   GetArtworkDisplayModes(),
						Segmented: true,
						// The value is a position in the list, not the saved mode.
						InitialValue: artworkDisplayModeIndex(cfg.GetArtworkDisplayMode()),
						ApplyFunc: func(val interface{}) {
							cfg.SetArtworkDisplayMode(artworkDisplayModeAt(val.(int)))
						},
						NeedsRefresh: true,
					},
					schema.SliderItem{
						Name:         "virtualFrameSize",
						Label:        i18n.T("Frame size"),
						Min:          50,
						Max:          100,
						Step:         1,
						Format:       "%.0f%%",
						InitialValue: cfg.VirtualFrameSize * 100.0,
						ApplyFunc: func(val float64) {
							cfg.VirtualFrameSize = val / 100.0
						},
						VisibleIf: b.framesVisible,
					},
					schema.BoolItem{
						Name:         "virtualPaperMatting",
						Label:        i18n.T("Paper mat"),
						InitialValue: cfg.VirtualPaperMatting,
						ApplyFunc: func(val bool) {
							cfg.VirtualPaperMatting = val
						},
						VisibleIf: b.framesVisible,
					},
					schema.SelectItem{
						Name:         "virtualWallColor",
						Label:        i18n.T("Wall color"),
						Options:      []string{i18n.T("Matches the artwork"), i18n.T("Neutral gray")},
						InitialValue: int(cfg.VirtualWallColor),
						ApplyFunc: func(val interface{}) {
							cfg.VirtualWallColor = VirtualWallMode(val.(int))
						},
						VisibleIf:    b.wallVisible,
						NeedsRefresh: true,
					},
				},
			},
			{
				Title: i18n.T("Content"),
				Items: []schema.ItemSchema{
					schema.BoolItem{
						Name:         "showNudes",
						Label:        i18n.T("Show NSFW images"),
						InitialValue: cfg.GetShowNudes(),
						ApplyFunc:    b.plugin.SetShowNudes,
					},
				},
			},
		},
	}
}

// BuildAdvancedPage creates the plugin's part of the Advanced page: storage and behavior.
func (b *PrefsPanelBuilder) BuildAdvancedPage() *schema.PanelSchema {
	cfg := b.plugin.cfg
	var usage cacheUsageCache
	var setCacheHelp func(string)
	return &schema.PanelSchema{
		Sections: []schema.SectionSchema{
			{
				Title: i18n.T("Storage"),
				Items: []schema.ItemSchema{
					schema.SelectItem{
						Name:         "cacheSize",
						Label:        i18n.T("Cache size"),
						Options:      cacheSizeOptions(),
						InitialValue: cacheSizeIndex(cfg.GetCacheSize()),
						ApplyFunc: func(val interface{}) {
							cfg.SetCacheSize(cacheSizeAt(val.(int)))
						},
					},
					// S4-3H (prototype v37/v40): off caps originals at about 4K (master_cap.go).
					schema.BoolItem{
						Name:         "highResOriginals",
						Label:        i18n.T("High-resolution originals"),
						Help:         i18n.T("Turn on if images look soft on a 5K or 8K screen. Uses more disk space."),
						InitialValue: cfg.GetHighResOriginals(),
						ApplyFunc:    cfg.SetHighResOriginals,
					},
					schema.ConfirmButtonItem{
						Name:              "clearCache",
						Label:             i18n.T("Downloaded wallpapers"),
						ButtonText:        i18n.T("Clear cache"),
						ConfirmTitle:      i18n.T("Clear cache?"),
						ConfirmButtonText: i18n.T("Clear cache"),
						ConfirmDanger:     true,
						Importance:        schema.ImportanceDanger,
						// What the cache holds is measured in the background when the page opens,
						// and the dialog says how much will be deleted.
						HelpAsync: func(set func(string)) {
							setCacheHelp = set
							go func() {
								u := b.plugin.CacheUsage()
								usage.set(u)
								set(u.Summary())
							}()
						},
						ConfirmMessageFunc: func() string {
							u, ok := usage.get()
							if !ok {
								u = b.plugin.CacheUsage()
							}
							return u.ClearMessage()
						},
						OnPressed: func() {
							b.plugin.ClearCache()
							usage.set(CacheUsage{})
							if setCacheHelp != nil {
								setCacheHelp(CacheUsage{}.Summary())
							}
						},
					},
					// S4-5: the count, View (the thumbnails, S4-B) and Reset.
					schema.BlockedImagesItem{
						Name:              "resetAvoidSet",
						Label:             i18n.T("Blocked images"),
						ResetText:         i18n.T("Reset"),
						ConfirmTitle:      i18n.T("Please confirm"),
						ConfirmMessage:    i18n.T("This cannot be undone. Are you sure?"),
						ConfirmButtonText: i18n.T("Reset"),
						ConfirmDanger:     true,
						List:              b.plugin.BlockedImageList,
						Unblock:           b.plugin.UnblockImage,
						Reset:             b.plugin.ResetBlockedImages,
					},
				},
			},
			{
				Title: i18n.T("Behavior"),
				Items: []schema.ItemSchema{
					schema.BoolItem{
						Name:         "staggerChanges",
						Label:        i18n.T("Stagger display changes"),
						InitialValue: cfg.GetStaggerMonitorChanges(),
						ApplyFunc:    cfg.SetStaggerMonitorChanges,
					},
					schema.BoolItem{
						Name:         "museumCollectionOTA",
						Label:        i18n.T("Update curated collections automatically"),
						InitialValue: cfg.GetMuseumCollectionOTA(),
						ApplyFunc:    cfg.SetMuseumCollectionOTA,
					},
				},
			},
		},
	}
}

// BuildPrefsTabsSchema returns the plugin's preferences pages: Sources, Wallpaper
// and its part of Advanced. Selected points at Sources when a source should open.
func (b *PrefsPanelBuilder) BuildPrefsTabsSchema() *schema.TabsSchema {
	sources := b.BuildSources()
	selected := 1
	if sources.OpenSourceID != "" {
		selected = 0
	}
	return &schema.TabsSchema{
		Selected: selected,
		Items: []schema.TabItemSchema{
			{ID: "sources", Title: i18n.T("Sources"), IconName: "color_palette", Sources: &sources},
			{ID: "wallpaper", Title: i18n.T("Wallpaper"), IconName: "settings", Panel: b.BuildWallpaperPage()},
			{ID: "advanced", Title: i18n.T("Advanced"), IconName: "computer", Panel: b.BuildAdvancedPage()},
		},
	}
}

// hiddenSources are registered providers that the Sources page does not list,
// so they can be neither found nor opened there. Google Photos needs OAuth
// credentials that release builds do not have, so its page could never work
// (ui_refresh.md §12, P10). The provider itself stays registered.
var hiddenSources = map[string]bool{
	"GooglePhotos": true,
}

// BuildSources lists every provider in three groups: museums first, then the
// user's own sources, then community sources (collapsed).
func (b *PrefsPanelBuilder) BuildSources() schema.SourcesSchema {
	var museums, yours, community []schema.SourceSchema
	openID := ""

	for _, name := range b.getSortedProviderIDs() {
		p := b.plugin.providers[name]
		if p == nil || hiddenSources[p.ID()] {
			continue
		}

		pendingURL := ""
		if b.plugin.pendingAddUrl != "" {
			if _, err := p.ParseURL(b.plugin.pendingAddUrl); err == nil {
				pendingURL = b.plugin.pendingAddUrl
				b.plugin.pendingAddUrl = ""
				openID = p.ID()
			}
		}
		if b.plugin.focusProviderName == p.ID() {
			b.plugin.focusProviderName = ""
			openID = p.ID()
		}

		content := b.buildProviderContent(p, pendingURL)
		if content == nil {
			continue
		}

		item := schema.SourceSchema{
			ID:           p.ID(),
			Title:        p.Name(),
			SubtitleFunc: b.createSubtitleFunc(p),
			IconName:     p.ID(),
			Detail:       content,
		}
		if iconStr, ok := p.GetProviderIcon().(string); ok {
			item.IconName = iconStr
		} else if bytes, ok := p.GetProviderIcon().([]byte); ok {
			item.IconBytes = bytes
		}

		switch p.Type() {
		case provider.TypeMuseum:
			museums = append(museums, item)
		case provider.TypePersonal:
			yours = append(yours, item)
		default:
			community = append(community, item)
		}
	}

	sortByTitle := func(items []schema.SourceSchema) {
		sort.SliceStable(items, func(i, j int) bool { return items[i].Title < items[j].Title })
	}
	sortByTitle(museums)
	sortByTitle(yours)
	sortByTitle(community)

	return schema.SourcesSchema{
		OpenSourceID: openID,
		Groups: []schema.SourceGroupSchema{
			{ID: "museums", Title: i18n.T("Museums"), Items: museums},
			{ID: "yours", Title: i18n.T("Yours"), Items: yours},
			{ID: "community", Title: i18n.T("Community"), Items: community, Collapsed: true},
		},
	}
}

// createSubtitleFunc describes how many of a source's collections are on.
func (b *PrefsPanelBuilder) createSubtitleFunc(p provider.ImageProvider) func() string {
	return func() string {
		n := b.activeCount(p)
		if n == 0 {
			return i18n.T("None on")
		}
		return i18n.Tf("{{.Count}} on", map[string]any{"Count": n})
	}
}

func (b *PrefsPanelBuilder) getSortedProviderIDs() []string {
	var names []string
	for n := range b.plugin.providers {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func (b *PrefsPanelBuilder) buildProviderContent(p provider.ImageProvider, pendingURL string) *schema.PanelSchema {
	settingsPanel := p.CreateSettingsPanel(b.sm)
	queryPanel := p.CreateQueryPanel(b.sm, pendingURL)

	if settingsPanel != nil && queryPanel != nil {
		return &schema.PanelSchema{
			Sections: append(append([]schema.SectionSchema{}, settingsPanel.Sections...), queryPanel.Sections...),
		}
	} else if settingsPanel != nil {
		return settingsPanel
	} else if queryPanel != nil {
		return queryPanel
	}
	return nil
}

// activeCount counts a source's collections that are on, counting changes that
// have not been applied yet.
func (b *PrefsPanelBuilder) activeCount(p provider.ImageProvider) int {
	count := 0
	queries := b.plugin.cfg.GetQueries()
	if col := curation.GetManager().GetCollection(p.ID()); col != nil {
		// Curated museum provider: count active curated collection entries
		for _, entry := range col.Entries {
			isActive := false
			for _, q := range queries {
				if q.Provider == p.ID() && q.URL == entry.Key && q.Active {
					isActive = true
					break
				}
			}
			if b.sm.HasPendingChange(p.ID() + "_" + entry.Key) {
				isActive = !isActive
			}
			if isActive {
				count++
			}
		}
		return count
	}
	// Query list provider (Wallhaven, Pexels, etc.)
	for _, q := range queries {
		if q.Provider != p.ID() {
			continue
		}
		isActive := q.Active
		if b.sm.HasPendingChange(q.ID) {
			isActive = !q.Active
		}
		if isActive {
			count++
		}
	}
	return count
}

// The drop-down works with the position of a size in GetCacheSizes, not with the saved CacheSize
// number (which has gaps: 0 and 2 are retired). Using the saved number as the position showed the
// default 300 as "1000 images" and saved the wrong size for most choices.

// cacheSizeIndex is the drop-down position of a cache size.
func cacheSizeIndex(size CacheSize) int {
	for i, s := range GetCacheSizes() {
		if s.(CacheSize) == size {
			return i
		}
	}
	return cacheSizeIndex(DefaultCacheSize)
}

// cacheSizeAt is the cache size at a drop-down position.
func cacheSizeAt(index int) CacheSize {
	sizes := GetCacheSizes()
	if index < 0 || index >= len(sizes) {
		return DefaultCacheSize
	}
	return sizes[index].(CacheSize)
}

// cacheSizeOptions are the cache sizes with a unit, e.g. "1000 images".
func cacheSizeOptions() []string {
	sizes := GetCacheSizes()
	options := make([]string, len(sizes))
	for i, size := range sizes {
		options[i] = i18n.Tf("{{.Count}} images", map[string]any{"Count": size.String()})
	}
	return options
}
