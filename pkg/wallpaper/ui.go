package wallpaper

import (
	"html"
	"regexp"
	"sort"
	"strings"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/setting"
	utilLog "github.com/dixieflatline76/Spice/v2/util/log"
)

// CreateTrayMenu creates the declarative schema for the tray menu.
// All monMu usage is confined to a fast snapshot at the top; the rest is lock-free.
func (wp *Plugin) CreateTrayMenu() *schema.MenuSchema {
	// ── Snapshot phase: read all needed monitor data under monMu ──
	type monSnap struct {
		id          int
		mc          *MonitorController
		image       provider.Image
		initialized bool
		paused      bool
		displayName string
	}

	wp.monMu.RLock()
	snaps := make([]monSnap, 0, len(wp.Monitors))
	for id, mc := range wp.Monitors {
		mc.mu.RLock()
		s := monSnap{
			id:          id,
			mc:          mc,
			image:       mc.State.CurrentImage,
			initialized: mc.State.CurrentID != "" || mc.State.CurrentImage.ID != "",
			paused:      mc.State.Paused,
		}
		mc.mu.RUnlock()

		// Build display name while we have the monitor reference
		s.displayName = i18n.Tf("Display {{.ID}}", map[string]any{"ID": id + 1})
		if mc.Monitor.Name != "" && mc.Monitor.Name != "Primary" && !strings.HasPrefix(mc.Monitor.Name, "Monitor ") {
			s.displayName = i18n.Tf("Display {{.ID}} ({{.Name}})", map[string]any{"ID": id + 1, "Name": mc.Monitor.Name})
		}

		snaps = append(snaps, s)
	}
	wp.monMu.RUnlock()
	// ── monMu released. Everything below is lock-free. ──

	sort.Slice(snaps, func(i, j int) bool { return snaps[i].id < snaps[j].id })

	var items []schema.MenuItemSchema

	// --- HELPER: Create Monitor Section Items ---
	createMonitorItems := func(snap monSnap) []schema.MenuItemSchema {
		mID := snap.id
		currentImage := snap.image
		isInitialized := snap.initialized

		// Header: what is on screen. The app-drawn menu shows the title and
		// the artist on two lines; a native menu has one line for both.
		// Without a current image (an empty library, or last time's wallpaper
		// is no longer in it) the row shows the app's name and does nothing:
		// the start restores the current image at once, so nothing is
		// "initializing" (A2 §14.9 #2).
		header := schema.MenuItemSchema{
			Kind:     schema.MenuItemNowShowing,
			Label:    i18n.T("Paintpane"),
			IconName: "view.png",
		}
		header.Title = header.Label
		if isInitialized {
			header.Label = wp.nowShowingLabel(currentImage)
			header.Title, header.Detail = wp.nowShowingLines(currentImage)
			// The row opens the page of the artwork it shows, even if the
			// wallpaper changes while the menu is open.
			shown := currentImage
			header.Action = func() { go wp.ViewImageOnWeb(shown) }
			if p, exists := wp.providers[currentImage.Provider]; exists {
				if iconStr, ok := p.GetProviderIcon().(string); ok {
					header.IconName = iconStr
				} else if b, ok := p.GetProviderIcon().([]byte); ok {
					header.IconBytes = b
					header.IconName = currentImage.Provider
				}
			}
		}

		// One row of icon buttons (T-S2): previous, pause, next | favourite.
		res := []schema.MenuItemSchema{
			header,
			{
				Kind:             schema.MenuItemIconButton,
				Label:            i18n.T("Previous wallpaper"),
				IconName:         "prev.png",
				Glyph:            "back",
				KeepOpen:         true,
				Action:           func() { go wp.SetPreviousWallpaper(mID, true) },
				ChangesWallpaper: true,
			},
		}
		if wp.cfg.GetWallpaperChangeFrequency() != FrequencyNever {
			pause := schema.MenuItemSchema{
				Kind:     schema.MenuItemIconButton,
				Label:    i18n.T("Pause rotation"),
				IconName: "pause.png",
				Glyph:    "pause",
				KeepOpen: true,
				Action:   func() { wp.TogglePauseMonitorAction(mID) },
			}
			if snap.paused {
				pause.Label = i18n.T("Resume rotation")
				pause.IconName = "play.png"
				pause.Glyph = "play"
			}
			res = append(res, pause)
		}
		res = append(res, schema.MenuItemSchema{
			Kind:             schema.MenuItemIconButton,
			Label:            i18n.T("Next wallpaper"),
			IconName:         "next.png",
			Glyph:            "next",
			KeepOpen:         true,
			Action:           func() { go wp.SetNextWallpaper(mID, true) },
			ChangesWallpaper: true,
		})

		var imageButtons []schema.MenuItemSchema
		if q, exists := wp.cfg.GetQuery(FavoritesQueryID); exists && q.Active {
			fav := schema.MenuItemSchema{
				Kind:     schema.MenuItemIconButton,
				Label:    i18n.T("Add to favorites"),
				IconName: "favorite.png",
				Glyph:    "heartOutline",
				KeepOpen: true,
				Action:   func() { go wp.TriggerFavorite(mID) },
			}
			if isInitialized && (currentImage.IsFavorited || currentImage.Provider == "Favorites") {
				fav.Label = i18n.T("Remove from favorites")
				fav.IconName = "unfavorite.png"
				fav.Glyph = "heart"
			}
			imageButtons = append(imageButtons, fav)
		}
		if len(imageButtons) > 0 {
			res = append(res, schema.MenuItemSchema{IsSeparator: true})
			res = append(res, imageButtons...)
		}

		// Text rows for this image (T7/T8, option B): Tune image first, as a row
		// with icon and text like the others (v45).
		res = append(res, schema.MenuItemSchema{IsSeparator: true})
		if wp.cfg.GetSmartFitMode() != SmartFitOff || wp.cfg.GetArtworkDisplayMode() != ArtworkDisplaySmartCrop {
			res = append(res, schema.MenuItemSchema{
				Label:    i18n.T("Tune image"),
				IconName: "anchor.png",
				Glyph:    "crop",
				Action:   func() { wp.showTuneImagePopup(mID) },
				// One Tune Image window at a time: close the open one first.
				ChangesWallpaper: true,
			})
		}
		res = append(res, schema.MenuItemSchema{
			Label:            i18n.T("Don't show this image again"),
			IconName:         "delete.png",
			Glyph:            "block",
			Action:           func() { go wp.DeleteCurrentImage(mID) },
			ChangesWallpaper: true,
		})
		if isInitialized && !isAutoDetectedNude(currentImage) {
			markLabel := i18n.T("Mark as NSFW")
			if wp.cfg.IsNudeMarked(currentImage.ID) {
				markLabel = i18n.T("Remove NSFW mark")
			}
			res = append(res, schema.MenuItemSchema{
				Label:            markLabel,
				IconName:         "view.png",
				Glyph:            "eyeOff",
				Action:           func() { go wp.ToggleNudeMark(mID) },
				ChangesWallpaper: true,
			})
		}

		return res
	}

	// --- 1. Primary Monitor (Monitor 0) ---
	for _, snap := range snaps {
		if snap.id == 0 {
			items = append(items, createMonitorItems(snap)...)
			break
		}
	}

	// --- 2. Other Monitors (Submenus), then the settings that apply to all ---
	items = append(items, schema.MenuItemSchema{IsSeparator: true})
	// T15 (prototype v48): change only the lock screen. One row, because the lock screen follows the
	// primary monitor; none where the app cannot set the lock screen.
	for _, snap := range snaps {
		if snap.id == 0 && snap.mc.SupportsLockScreen() {
			items = append(items, schema.MenuItemSchema{
				Label:    i18n.T("Next lock screen picture"),
				IconName: "next.png",
				Glyph:    "lock",
				KeepOpen: true,
				Action:   func() { wp.NextLockScreen() },
			})
			break
		}
	}
	for _, snap := range snaps {
		if snap.id == 0 {
			continue // Skip primary
		}
		subMenuItems := createMonitorItems(snap)
		items = append(items, schema.MenuItemSchema{
			Label:    snap.displayName,
			IconName: "display.png",
			Glyph:    "computer",
			SubMenu: &schema.MenuSchema{
				Label: snap.displayName,
				Items: subMenuItems,
			},
		})
	}

	// --- 3. Global Settings ---
	items = append(items,
		wp.artworkDisplayMenuItem(),
		schema.MenuItemSchema{
			Kind:             schema.MenuItemSwitch,
			Label:            i18n.T("Show NSFW images"),
			IsChecked:        wp.cfg.GetShowNudes(),
			Action:           func() { go wp.ToggleShowNudes() },
			ChangesWallpaper: true,
		},
	)

	utilLog.Debugf("Finished Generating Tray Menu Schema for %d monitors.", len(snaps))
	return &schema.MenuSchema{
		Items: items,
	}
}

// artworkDisplayMenuItem builds the tray submenu for switching the display mode.
// The tray has its own sentence-case labels (T-R2); the settings window keeps
// ArtworkDisplayMode.String.
func (wp *Plugin) artworkDisplayMenuItem() schema.MenuItemSchema {
	current := wp.cfg.GetArtworkDisplayMode()
	modes := artworkDisplayModes()
	subItems := make([]schema.MenuItemSchema, 0, len(modes))
	for _, mode := range modes {
		subItems = append(subItems, schema.MenuItemSchema{
			Label:     trayDisplayModeLabel(mode),
			IsChecked: mode == current,
			Action: func() {
				go wp.SetArtworkDisplayMode(mode)
			},
			ChangesWallpaper: true,
		})
	}

	label := i18n.T("Display mode")
	return schema.MenuItemSchema{
		Label:    label,
		Detail:   trayDisplayModeLabel(current),
		IconName: "display.png",
		Glyph:    "photo",
		SubMenu:  &schema.MenuSchema{Label: label, Items: subItems},
	}
}

// trayDisplayModeLabel names a display mode in the tray.
func trayDisplayModeLabel(m ArtworkDisplayMode) string {
	switch m {
	case ArtworkDisplaySmartCrop:
		return i18n.T("Smart crop")
	case ArtworkDisplayFramed:
		return i18n.T("Museum frame")
	default:
		return i18n.T("Full artwork")
	}
}

// CreatePrefsSchema creates a declarative preferences tabs schema for wallpaper settings.
func (wp *Plugin) CreatePrefsSchema(sm setting.SettingsManager) *schema.TabsSchema {
	builder := NewPrefsPanelBuilder(wp, sm)

	// Register the wallpaper refresh function
	sm.RegisterRefreshFunc(wp.RefreshImagesAndPulse)

	return builder.BuildPrefsTabsSchema()
}

// nowShowingLabel describes an image in one line, for a native menu:
// "Title — Artist". The source is not repeated: the menu item shows the
// provider's logo. The artist is reduced to the name (T-R3) and capped at 24
// characters; the line stays at 56 and the title gets the rest. An artist
// already contained in the title is not repeated.
func (wp *Plugin) nowShowingLabel(img provider.Image) string {
	const maxRunes, maxArtist = 56, 24
	const sep = " — "
	title, artist := wp.nowShowingParts(img)
	switch {
	case title != "" && artist != "":
		artist = truncateRunes(artist, maxArtist)
		room := maxRunes - len([]rune(artist)) - len([]rune(sep))
		return truncateRunes(title, room) + sep + artist
	case title != "":
		return truncateRunes(title, maxRunes)
	case artist != "":
		return truncateRunes(artist, maxRunes)
	}
	if p, ok := wp.providers[img.Provider]; ok {
		return p.Name()
	}
	return wp.GetProviderTitle(img.Provider)
}

// nowShowingLines returns the two lines of the app-drawn tray's artwork block:
// the title and the artist's name. The menu cuts each line at its width.
func (wp *Plugin) nowShowingLines(img provider.Image) (string, string) {
	title, artist := wp.nowShowingParts(img)
	switch {
	case title != "":
		return title, artist
	case artist != "":
		return artist, ""
	}
	if p, ok := wp.providers[img.Provider]; ok {
		return p.Name(), ""
	}
	return wp.GetProviderTitle(img.Provider), ""
}

// nowShowingParts returns the title and the cleaned artist name; the artist is
// empty when the title already contains it.
func (wp *Plugin) nowShowingParts(img provider.Image) (title, artist string) {
	return titleAndArtist(img)
}

// titleAndArtist returns an image's title and the cleaned artist name, as the
// tray shows them; the artist is empty when the title already contains it.
func titleAndArtist(img provider.Image) (title, artist string) {
	title = SanitizeMenuString(img.Title)
	// Clean before collapsing white space: a line break separates artists
	// and notes ("Harald Sohlberg\nNorwegian, 1869-1935").
	artist = SanitizeMenuString(cleanArtistName(img.Artist))
	if artist == "" {
		artist = SanitizeMenuString(cleanArtistName(attributionArtist(img.Attribution, img.Title)))
	}
	if title != "" && artist != "" && strings.Contains(title, artist) {
		artist = ""
	}
	return title, artist
}

var (
	artistNoteRe  = regexp.MustCompile(`\s*[(\[（【].*$`)
	artistYearsRe = regexp.MustCompile(`,\s*(c\.\s*)?\d{3,4}.*$`)
	artistRangeRe = regexp.MustCompile(`\s+(c\.\s*)?\d{3,4}\s*[-–]\s*\d{0,4}\s*$`)
)

// cleanArtistName reduces an artist credit to the name (T-R3): the first
// artist only, without text in parentheses or brackets and without trailing
// years. "Albert Bierstadt (American, born Prussia [now Germany], 1830–1902)"
// becomes "Albert Bierstadt".
func cleanArtistName(s string) string {
	// Decode character references first, so "d&#39; Hondecoeter" is not cut
	// at its semicolon.
	s = html.UnescapeString(s)
	if i := strings.IndexAny(s, ";\n"); i >= 0 {
		s = s[:i]
	}
	s = artistNoteRe.ReplaceAllString(s, "")
	s = artistYearsRe.ReplaceAllString(s, "")
	s = artistRangeRe.ReplaceAllString(s, "")
	return strings.TrimSpace(strings.TrimRight(strings.TrimSpace(s), ","))
}

// attributionArtist returns the artist part of an attribution. Several
// museums write "Artist - Title"; without an artist that is " - Title", which
// leaves nothing.
func attributionArtist(attribution, title string) string {
	a := strings.TrimSpace(attribution)
	if t := strings.TrimSpace(title); t != "" {
		a = strings.TrimSuffix(a, t)
	}
	return strings.Trim(a, " -–—")
}

// truncateRunes shortens s to at most n runes, ending with an ellipsis when cut.
func truncateRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return strings.TrimRight(string(runes[:n-1]), " ") + "…"
}
