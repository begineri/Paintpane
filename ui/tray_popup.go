package ui

import (
	"image"
	"sync"

	"fyne.io/fyne/v2"

	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
	utilLog "github.com/dixieflatline76/Spice/v2/util/log"
)

// trayPopup is the tray menu the app draws itself (T-S1). Only Windows has one;
// elsewhere newTrayPopup returns nil and the native menu shows.
type trayPopup interface {
	// setMenu replaces the menu's content; an open menu updates in place.
	setMenu(items, footer []schema.MenuItemSchema)
	// toggle opens the menu at the mouse pointer, or closes it.
	toggle()
	// refreshTheme redraws an open menu after Appearance changes.
	refreshTheme()
	close()
}

// trayPopupDebug is what the dev-build debug API uses to check the menu
// without clicking the tray.
type trayPopupDebug interface {
	debugOpen(x, y int, at bool) error
	debugState() map[string]any
	debugCapture() image.Image
	debugUse(label string, sub bool) error
	debugHover(label string) error
	debugKey(name fyne.KeyName, sub bool)
}

// trayIconResource returns the logo or icon an item shows.
func (sa *SpiceApp) trayIconResource(it schema.MenuItemSchema) fyne.Resource {
	if len(it.IconBytes) > 0 {
		return fyne.NewStaticResource(it.IconName, it.IconBytes)
	}
	if it.IconName == "" || sa.assetMgr == nil {
		return nil
	}
	sa.trayIconMu.Lock()
	defer sa.trayIconMu.Unlock()
	if res, ok := sa.trayIcons[it.IconName]; ok {
		return res
	}
	res, err := sa.assetMgr.GetIcon(it.IconName)
	if err != nil {
		utilLog.Printf("Failed to load tray icon %s: %v", it.IconName, err)
		res = nil
	}
	if sa.trayIcons == nil {
		sa.trayIcons = map[string]fyne.Resource{}
	}
	sa.trayIcons[it.IconName] = res
	return res
}

var trayThemeListener sync.Once

// listenTrayTheme keeps an open app-drawn menu in step with Appearance.
func (sa *SpiceApp) listenTrayTheme() {
	trayThemeListener.Do(func() {
		sa.Settings().AddListener(func(fyne.Settings) {
			fyne.Do(func() {
				if sa.popup != nil {
					sa.popup.refreshTheme()
				}
			})
		})
	})
}

// subMenuWidth is the minimum width of a submenu: a list of choices is narrow
// (150), a display's submenu is as wide as the menu.
func subMenuWidth(items []schema.MenuItemSchema) float32 {
	for _, it := range items {
		if it.Kind != schema.MenuItemRow || it.Glyph != "" {
			return trayMinWidth
		}
	}
	return traySubMinMode
}
