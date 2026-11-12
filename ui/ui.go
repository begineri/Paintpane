package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"github.com/dixieflatline76/Spice/v2/asset"
	"github.com/dixieflatline76/Spice/v2/config"
	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/sysinfo"
	"github.com/dixieflatline76/Spice/v2/pkg/ui"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
	"github.com/dixieflatline76/Spice/v2/util"
	utilLog "github.com/dixieflatline76/Spice/v2/util/log"
)

// SpiceApp represents the application
type SpiceApp struct {
	fyne.App
	assetMgr     *asset.Manager
	trayMenu     *fyne.Menu
	splash       fyne.Window // Splash window for initial setup
	plugins      []ui.Plugin // List of plugins to activate
	os           OS          // Operating system interface
	appConfig    *config.AppConfig
	prefsWindow  fyne.Window      // Singleton preferences window
	prefsNav     *prefsNavigation // Side navigation of the preferences window
	prefsSM      *SettingsManager // Settings manager of the preferences window
	trayMu       sync.Mutex
	trayDebounce *time.Timer

	// The tray menu the app draws itself (Windows); nil uses the native menu.
	popup       trayPopup
	popupFailed bool
	trayIconMu  sync.Mutex
	trayIcons   map[string]fyne.Resource
	update      *trayUpdate // a newer version to offer, if any
	tuneWindow  fyne.Window // the open Tune Image window, if any (T-L)
}

// trayUpdate is the "Update to …" item the update check adds to the tray.
type trayUpdate struct {
	version string
	url     *url.URL
}

// OS interface defines methods for transforming the application state
// to foreground or background. This is useful for managing the application
// behavior on different operating systems, such as macOS where background apps
// do not show a Dock icon.
type OS interface {
	// TransformToForeground changes the application to be a regular app with a Dock icon.
	TransformToForeground()
	// TransformToBackground changes the application to be a background-only app.
	TransformToBackground()
	// SetupLifecycle sets up OS-specific lifecycle hooks.
	// This is where Chrome OS "fake tray" logic resides.
	SetupLifecycle(app fyne.App, sa *SpiceApp)
}

var (
	saInstance *SpiceApp // Singleton instance of the application
	saOnce     sync.Once // Ensures the singleton is created only once
)

// GetPluginManager returns the singleton instance of the application as a UIPluginManager
func GetPluginManager() ui.PluginManager {
	return getInstance()
}

// Register registers a plugin with the application
func (sa *SpiceApp) Register(plugin ui.Plugin) {
	sa.plugins = append(sa.plugins, plugin)
	plugin.Init(sa)
}

// Deregister deregisters a plugin from the application
func (sa *SpiceApp) Deregister(plugin ui.Plugin) {
	// Implementation for deregistering a plugin
	for i, p := range sa.plugins {
		if p == plugin {
			sa.plugins = append(sa.plugins[:i], sa.plugins[i+1:]...)
			break
		}
	}
}

// OpenPreferences opens the preferences window
func (sa *SpiceApp) OpenPreferences(tab string) {
	// Must execute on Fyne's main thread, as this can be called from background goroutines
	// (e.g. the local API HTTP server triggered by the browser extension)
	// macOS explicitly forbids window creation from background threads.
	if sa.App != nil {
		fyne.Do(func() {
			sa.CreatePreferencesWindow(tab)
		})
	} else {
		// Fallback for tests or extreme edge cases before app is fully initialized
		sa.CreatePreferencesWindow(tab)
	}
}

// ... (Lines 91-348 unrelated mostly, but Line 176 calls CreatePreferencesWindow)

// Let's target ONLY `OpenPreferences` first (lines 87-89).
// Then `CreateTrayMenu` call site.
// Then `CreatePreferencesWindow` definition.

// GetPreferences returns the preferences for the application
func (sa *SpiceApp) GetPreferences() fyne.Preferences {
	return sa.Preferences()
}

// GetAssetManager returns the asset manager
func (sa *SpiceApp) GetAssetManager() *asset.Manager {
	return sa.assetMgr
}

// GetApplication returns the singleton instance of the application
func GetApplication() ui.App {
	return getInstance()
}

// GetInstance returns the singleton instance of the application
func getInstance() *SpiceApp {
	// Create a new instance of the application if it doesn't exist
	saOnce.Do(func() {
		fmt.Fprintln(os.Stderr, "[Paintpane] getInstance: starting")

		// Initialize the wallpaper service for right OS
		currentOS := getOS()
		fmt.Fprintln(os.Stderr, "[Paintpane] getInstance: OS interface created")

		a := app.NewWithID(config.AppName)
		fmt.Fprintln(os.Stderr, "[Paintpane] getInstance: Fyne app created")

		if _, ok := a.(desktop.App); ok {
			fmt.Fprintln(os.Stderr, "[Paintpane] getInstance: desktop.App confirmed")

			saInstance = &SpiceApp{
				App:      a,
				assetMgr: asset.NewManager(),
				trayMenu: fyne.NewMenu(""),
				plugins:  []ui.Plugin{},
				os:       currentOS,
			}
			saInstance.appConfig = config.NewAppConfig(saInstance.Preferences())
			fmt.Fprintln(os.Stderr, "[Paintpane] getInstance: app config loaded")

			// Apply saved debug logging preference
			if saInstance.appConfig.GetDebugLoggingEnabled() {
				utilLog.SetDebugEnabled(true)
			}

			// Apply saved language preference
			i18n.SetLanguage(saInstance.appConfig.GetLanguage())

			// Apply saved theme
			applyTheme(saInstance.App, parseThemeMode(saInstance.appConfig.GetTheme()), saInstance.appConfig.GetAccent())
			fmt.Fprintln(os.Stderr, "[Paintpane] getInstance: theme applied")

			saInstance.verifyEULA()
			fmt.Fprintln(os.Stderr, "[Paintpane] getInstance: EULA verified")

			// Setup OS-specific lifecycle hooks (e.g. Chrome OS Pseudo-Tray)
			saInstance.os.SetupLifecycle(saInstance.App, saInstance)

			// Common Lifecycle: Sync Monitors on return to foreground (e.g. Settings opened)
			saInstance.Lifecycle().SetOnEnteredForeground(func() {
				utilLog.Debug("App entered foreground - synchronizing monitors...")
				wp := wallpaper.GetInstance()
				if wp != nil {
					// Low-CPU check
					wp.SyncMonitors(false)
				}
			})
			fmt.Fprintln(os.Stderr, "[Paintpane] getInstance: lifecycle hooks set")
		} else {
			fmt.Fprintln(os.Stderr, "[Paintpane] getInstance: FATAL — not a desktop.App")
			utilLog.Fatal("Paintpane not supported on this platform")
		}
	})
	return saInstance
}

// CreateTrayMenu creates the tray menu for the application
func (sa *SpiceApp) CreateTrayMenu() {
	defer func() {
		if r := recover(); r != nil {
			utilLog.Printf("ERROR: PANIC in CreateTrayMenu: %v", r)
		}
	}()
	utilLog.Debug("CreateTrayMenu: Starting...")
	if _, ok := sa.App.(desktop.App); !ok {
		return
	}

	var items []schema.MenuItemSchema
	for i, plugin := range sa.plugins {
		utilLog.Debugf("CreateTrayMenu: Processing plugin %s...", plugin.Name())
		menuSchema := plugin.CreateTrayMenu()
		if menuSchema == nil {
			continue
		}
		if i == 0 {
			items = append(items, menuSchema.Items...)
		} else {
			items = append(items, schema.MenuItemSchema{
				Label:   plugin.Name(),
				SubMenu: &schema.MenuSchema{Label: plugin.Name(), Items: menuSchema.Items},
			})
		}
	}
	items = append(items, sa.updateMenuItems()...)
	sa.setTrayMenu(items)
}

// updateMenuItems is the tray's update item when a newer version was found (T14, prototype
// v36): its own group just above the Settings / Quit footer, a download icon and bold text
// "Update to v1.0.1", opening the release page. It is the only sign of a new version; the app
// sends no notification (v35).
func (sa *SpiceApp) updateMenuItems() []schema.MenuItemSchema {
	u := sa.update
	if u == nil {
		return nil
	}
	return []schema.MenuItemSchema{{IsSeparator: true}, {
		Label:    i18n.Tf("Update to {{.Version}}", map[string]any{"Version": u.version}),
		IconName: "download.png",
		Glyph:    "download",
		Emphasis: true,
		Action: func() {
			if err := sa.OpenURL(u.url); err != nil {
				utilLog.Printf("Failed to open release URL: %v", err)
			}
		},
	}}
}

// trayFooter is the last row of the tray menu: Settings and Quit (T-S2).
func (sa *SpiceApp) trayFooter() []schema.MenuItemSchema {
	return []schema.MenuItemSchema{
		{
			Label:    i18n.T("Settings…"),
			IconName: "prefs.png",
			Glyph:    "settings",
			Action:   func() { sa.CreatePreferencesWindow("") },
		},
		{
			Label:    i18n.T("Quit"),
			IconName: "quit.png",
			Glyph:    "quit",
			Action:   sa.quit,
		},
	}
}

// quit deactivates the plugins and leaves.
func (sa *SpiceApp) quit() {
	if sa.popup != nil {
		sa.popup.close()
	}
	sa.os.TransformToForeground()     // Ensure the app is in the foreground before quitting
	time.Sleep(50 * time.Millisecond) // Small delay to ensure the OS processes the state change
	sa.deactivateAllPlugins()         // Deactivate all plugins before quitting
	time.Sleep(2 * time.Second)       // Small delay to ensure plugins are deactivate
	utilLog.Println("Quitting Paintpane application")
	sa.Quit()
}

// setTrayMenu shows items (plus the footer) in the tray: in the app-drawn
// menu on Windows, and in the native menu, which other platforms show and the
// debug API reads.
func (sa *SpiceApp) setTrayMenu(items []schema.MenuItemSchema) {
	desk, ok := sa.App.(desktop.App)
	if !ok {
		return
	}
	footer := sa.trayFooter()
	items = sa.closeTuneBefore(items)

	native := sa.renderMenuSchema(&schema.MenuSchema{Items: items})
	settingsItem := sa.CreateMenuItem(footer[0].Label, footer[0].Action, footer[0].IconName)
	quitItem := sa.CreateMenuItem(footer[1].Label, footer[1].Action, footer[1].IconName)
	quitItem.IsQuit = true
	native = append(native, fyne.NewMenuItemSeparator(), settingsItem, fyne.NewMenuItemSeparator(), quitItem)
	sa.trayMenu = fyne.NewMenu("", native...)

	trayIcon, err := sa.assetMgr.GetIcon("tray.png")
	if err != nil {
		utilLog.Printf("ERROR: Failed to load tray icon: %v", err)
	}
	sa.SetIcon(trayIcon)

	utilLog.Debug("Calling desk.SetSystemTrayMenu...")
	desk.SetSystemTrayMenu(sa.trayMenu)
	if trayIcon != nil {
		desk.SetSystemTrayIcon(trayIcon)
	}

	sa.installTrayPopup()
	if sa.popup != nil {
		sa.popup.setMenu(items, footer)
	}
	utilLog.Debug("System Tray Menu and Icon setup process completed.")
}

func (sa *SpiceApp) renderMenuSchema(m *schema.MenuSchema) []*fyne.MenuItem {
	if m == nil {
		return nil
	}
	var items []*fyne.MenuItem
	for _, item := range m.Items {
		if item.IsSeparator {
			items = append(items, fyne.NewMenuItemSeparator())
			continue
		}
		// An item with its own icon bytes (a provider logo) names the resource
		// with IconName; it is not an embedded icon file, so do not look it up.
		embeddedIcon := item.IconName
		if len(item.IconBytes) > 0 {
			embeddedIcon = ""
		}
		var mi *fyne.MenuItem
		if item.SubMenu != nil {
			subItems := sa.renderMenuSchema(item.SubMenu)
			mi = sa.CreateMenuItem(item.Label, item.Action, embeddedIcon)
			if len(item.IconBytes) > 0 {
				mi.Icon = fyne.NewStaticResource(item.IconName, item.IconBytes)
			}
			mi.ChildMenu = fyne.NewMenu(item.Label, subItems...)
		} else {
			mi = sa.CreateMenuItem(item.Label, item.Action, embeddedIcon)
			if len(item.IconBytes) > 0 {
				mi.Icon = fyne.NewStaticResource(item.IconName, item.IconBytes)
			}
			if item.IsChecked {
				mi.Checked = true
			}
			if item.IsDisabled {
				mi.Disabled = true
			}
		}
		items = append(items, mi)
	}
	return items
}

// closeTuneBefore returns a copy of items in which every item that changes the
// image or the wallpaper first closes Tune Image (T-L, option 2): the tray
// stays usable while Tune Image is open.
func (sa *SpiceApp) closeTuneBefore(items []schema.MenuItemSchema) []schema.MenuItemSchema {
	out := make([]schema.MenuItemSchema, len(items))
	for i, it := range items {
		if it.SubMenu != nil {
			sub := *it.SubMenu
			sub.Items = sa.closeTuneBefore(sub.Items)
			it.SubMenu = &sub
		}
		if action := it.Action; it.ChangesWallpaper && action != nil {
			it.Action = func() {
				sa.closeTuneImage()
				action()
			}
		}
		out[i] = it
	}
	return out
}

// closeTuneImage closes the Tune Image window, if open, as if the user had
// closed it. Call it on the main thread.
func (sa *SpiceApp) closeTuneImage() {
	if w := sa.tuneWindow; w != nil {
		sa.tuneWindow = nil
		w.Close()
	}
}

// DeactivateAllPlugins deactivates all plugins in the application
func (sa *SpiceApp) deactivateAllPlugins() {
	// Deactivate all plugins
	for _, plugin := range sa.plugins {
		plugin.Deactivate()
		utilLog.Printf("Deactivated plugin: %s", plugin.Name())
	}
}

// CreateMenuItem creates a menu item with the given label, action, and icon
func (sa *SpiceApp) CreateMenuItem(label string, action func(), iconName string) *fyne.MenuItem {
	mi := fyne.NewMenuItem(label, func() {
		fyne.Do(action)
	})
	if iconName == "" {
		return mi
	}
	icon, err := sa.assetMgr.GetIcon(iconName)
	if err != nil {
		utilLog.Printf("Failed to load icon: %v", err)
		return mi
	}
	mi.Icon = icon
	return mi
}

// CreateToggleMenuItem creates a toggle menu item with the given label, action, icon, and checked state
func (sa *SpiceApp) CreateToggleMenuItem(label string, action func(bool), iconName string, checked bool) *fyne.MenuItem {

	mi := fyne.NewMenuItem("", nil)

	if checked {
		mi.Label = fmt.Sprintf("%s ✔", label)
	} else {
		mi.Label = label
	}

	if iconName != "" {
		icon, err := sa.assetMgr.GetIcon(iconName)
		if err != nil {
			utilLog.Printf("Failed to load icon: %v", err)
			return mi
		}
		mi.Icon = icon
	}
	mi.Checked = checked
	mi.Action = func() {
		fyne.Do(func() {
			newChecked := !mi.Checked
			if newChecked {
				mi.Label = fmt.Sprintf("%s ✔", label)
			} else {
				mi.Label = label
			}
			mi.Checked = newChecked
			action(newChecked)
			sa.trayMenu.Refresh()
		})
	}

	return mi
}

// CreatePreferencesWindow creates and displays the preferences window.
// If the window is already open, it brings it to the front.
func (sa *SpiceApp) CreatePreferencesWindow(initialTab string) {
	// Guard: skip if OpenGL is unavailable (Fyne GLFW would crash process)
	if !sysinfo.CanCreateWindows() {
		utilLog.Println("OpenGL unavailable — preferences window cannot be created")
		ShowNativeFallbackAlert(i18n.T("Graphics error"), i18n.T("Paintpane requires OpenGL 2.0+ or hardware acceleration to show windows. The application will continue to run in the background. Please try rebooting your machine to restore graphics functionality."))
		return
	}

	sa.os.TransformToForeground()

	// If window already exists, just show it and switch tab if requested
	if sa.prefsWindow != nil {
		sa.RebuildPreferencesContent(initialTab) // Rebuild to pick up any new pendingAddUrls
		sa.prefsWindow.Show()
		sa.prefsWindow.RequestFocus()
		return
	}

	// Create a new window for the preferences
	var prefsWindow fyne.Window
	func() {
		defer func() {
			if r := recover(); r != nil {
				errStr := fmt.Sprintf("%v", r)
				utilLog.Printf("Recovered from preferences window creation panic: %v", errStr)
				if strings.Contains(strings.ToLower(errStr), "apiunavailable") || strings.Contains(strings.ToLower(errStr), "wgl") || strings.Contains(strings.ToLower(errStr), "opengl") {
					ShowNativeFallbackAlert(i18n.T("Graphics error"), i18n.T("Paintpane requires OpenGL 2.0+ or hardware acceleration to show windows. The application will continue to run in the background. Please try rebooting your machine to restore graphics functionality."))
				}
			}
		}()
		prefsWindow = sa.NewWindow(i18n.T("Paintpane settings"))
	}()

	if prefsWindow == nil {
		utilLog.Println("Preferences window creation failed (returned nil or recovered)")
		ShowNativeFallbackAlert(i18n.T("Graphics error"), i18n.T("Paintpane requires OpenGL 2.0+ or hardware acceleration to show windows. The application will continue to run in the background. Please try rebooting your machine to restore graphics functionality."))
		return
	}

	// Wrap the entire window setup in recovery — Fyne can return a non-nil but broken
	// window when OpenGL is unavailable (e.g. Remote Desktop). The actual crash often
	// happens at Show() or Content().MinSize(), not at NewWindow().
	func() {
		defer func() {
			if r := recover(); r != nil {
				utilLog.Printf("Recovered from preferences window setup panic: %v", r)
				ShowNativeFallbackAlert(i18n.T("Graphics error"), i18n.T("Paintpane requires OpenGL 2.0+ or hardware acceleration to show windows. The application will continue to run in the background. Please try rebooting your machine to restore graphics functionality."))
				// Clean up the broken window reference
				sa.prefsWindow = nil
				sa.prefsNav = nil
			}
		}()

		sa.prefsWindow = prefsWindow // Store reference

		// Build and bind the UI contents first, so the layout engine knows existing elements.
		sa.RebuildPreferencesContent(initialTab)

		// Size: the size the user last left the window at, or a fixed logical
		// default, never smaller than the content needs or larger than 90% of the screen.
		targetWidth, targetHeight := sa.savedPrefsWindowSize()
		if minW := prefsWindow.Content().MinSize().Width; targetWidth < minW {
			targetWidth = minW
		}
		osScale := sysinfo.GetOSDisplayScale()
		if osScale <= 0 {
			osScale = 1.0
		}
		if physWidth, physHeight, err := sysinfo.GetScreenDimensions(); err == nil && physWidth > 0 && physHeight > 0 {
			maxW := float32(physWidth) / osScale * 0.9
			maxH := float32(physHeight) / osScale * 0.9
			targetWidth = fyne.Min(targetWidth, maxW)
			targetHeight = fyne.Min(targetHeight, maxH)
		}

		prefsWindow.SetOnClosed(func() {
			// Apply any change made just before closing.
			if sa.prefsSM != nil {
				sa.prefsSM.FlushPending()
				sa.prefsSM = nil
			}
			sa.savePrefsWindowSize(prefsWindow.Canvas().Size())
			sa.os.TransformToBackground()
			sa.prefsWindow = nil // Clear reference on close
			sa.prefsNav = nil    // Clear navigation reference
		})

		// Resize before Show so macOS and Wayland allocate the right framebuffer
		// size before the first paint.
		prefsWindow.Resize(fyne.NewSize(targetWidth, targetHeight))
		prefsWindow.CenterOnScreen()
		prefsWindow.Show()
		styleTitleBar(prefsWindow)
	}()
}

// minSizeContainer wraps a CanvasObject and overrides its MinSize to prevent
// Fyne from enforcing the child's full content height as the window minimum.
// The child is still laid out at the full available size.
type minSizeContainer struct {
	widget.BaseWidget
	content fyne.CanvasObject
}

func newMinSizeContainer(content fyne.CanvasObject) *minSizeContainer {
	c := &minSizeContainer{content: content}
	c.ExtendBaseWidget(c)
	return c
}

func (m *minSizeContainer) CreateRenderer() fyne.WidgetRenderer {
	return &minSizeRenderer{content: m.content}
}

type minSizeRenderer struct {
	content fyne.CanvasObject
}

func (r *minSizeRenderer) Layout(size fyne.Size) {
	r.content.Resize(size)
	r.content.Move(fyne.NewPos(0, 0))
}

func (r *minSizeRenderer) MinSize() fyne.Size {
	// Report the content's width but a small height, so the window
	// can be resized shorter than the content's natural height.
	return fyne.NewSize(r.content.MinSize().Width, 100)
}

func (r *minSizeRenderer) Refresh() {
	r.content.Refresh()
}

func (r *minSizeRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.content}
}

func (r *minSizeRenderer) Destroy() {}

// Start activates all plugins and runs the Fyne application
func (sa *SpiceApp) Start() {

	// Create the tray menu
	saInstance.CreateTrayMenu()
	if !config.IsStoreDistribution() {
		go sa.StartStartupUpdateCheck()
	}
	go refreshAutostart() // keep an existing "start with Windows" entry pointing at this exe

	// Activate all plugins
	go func() {
		time.Sleep(500 * time.Millisecond) // Wait for the tray menu to be created and the ui to be ready
		for _, plugin := range sa.plugins {
			plugin.Activate()
			utilLog.Printf("Activated plugin: %s", plugin.Name())
		}
	}()

	// Run the Fyne application
	sa.Run()
}

// StartStartupUpdateCheck performs a one-time check for updates on startup,
// and registers the callback for the nightly background check.
func (sa *SpiceApp) StartStartupUpdateCheck() {
	// Register a self-contained callback that the nightly scheduler invokes.
	// It owns the full lifecycle: preference check → network call → tray update.
	if plugin := wallpaper.GetInstance(); plugin != nil {
		plugin.SetUpdateCallback(func() {
			if !sa.appConfig.GetUpdateCheckEnabled() {
				utilLog.Print("Nightly update check skipped: disabled by user.")
				return
			}
			utilLog.Print("Nightly Maintenance: Checking for application updates...")
			updateInfo, err := util.CheckForUpdates(nil)
			if err != nil {
				utilLog.Printf("Nightly Maintenance: Update check failed: %v", err)
				return
			}
			sa.updateTrayMenu(updateInfo)
		})
	}

	// Perform the initial check after a short delay
	time.Sleep(1 * time.Minute)
	if !sa.appConfig.GetUpdateCheckEnabled() {
		utilLog.Print("Startup update check disabled by user.")
		return
	}
	utilLog.Print("Checking for application updates on startup...")
	updateInfo, err := util.CheckForUpdates(nil)
	if err != nil {
		utilLog.Printf("Update check failed: %v", err)
		return
	}
	sa.updateTrayMenu(updateInfo)
}

// updateTrayMenu adds or removes the "Update to …" tray item based on check results.
func (sa *SpiceApp) updateTrayMenu(info *util.CheckForUpdatesResult) {
	fyne.Do(func() {
		var next *trayUpdate
		if info.UpdateAvailable {
			releaseURL, err := url.Parse(info.ReleaseURL)
			if err != nil {
				utilLog.Printf("Could not parse release URL for update menu item: %v", err)
				return
			}
			utilLog.Printf("New version available: %s", info.LatestVersion)
			next = &trayUpdate{version: info.LatestVersion, url: releaseURL}
		}
		sa.update = next
		// The tray item is the only sign of a new version; no notification (prototype v35).
		sa.CreateTrayMenu()
	})
}

// RefreshTrayMenu triggers a full rebuild of the tray menu structure.
// We use RebuildTrayMenu (debounced) instead of a simple Refresh() to ensure
// the system tray driver cleanly replaces the menu items, preventing duplicates
// on platforms like Windows when labels change.
func (sa *SpiceApp) RefreshTrayMenu() {
	sa.RebuildTrayMenu()
}

// RebuildTrayMenu rebuilds the tray menu list from scratch with debouncing.
func (sa *SpiceApp) RebuildTrayMenu() {
	sa.trayMu.Lock()
	defer sa.trayMu.Unlock()

	if sa.trayDebounce != nil {
		sa.trayDebounce.Stop()
	}

	sa.trayDebounce = time.AfterFunc(300*time.Millisecond, func() {
		fyne.Do(func() {
			sa.CreateTrayMenu()
		})
	})
}

// decodeGifToFrames decodes a GIF and pre-renders each frame onto a canvas
// handling disposal methods to prevent artifacts.
func decodeGifToFrames(data []byte) ([]image.Image, []time.Duration, error) {
	g, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return nil, nil, err
	}

	width, height := g.Config.Width, g.Config.Height
	numFrames := len(g.Image)
	frames := make([]image.Image, numFrames)
	delays := make([]time.Duration, numFrames)

	// Canvas to draw on (accumulates frames)
	canvasImg := image.NewRGBA(image.Rect(0, 0, width, height))

	for i, frame := range g.Image {
		// Handle disposal of previous frame
		// Disposal limits:
		// 1: Do not dispose (Keep) -> Default behavior of drawing over
		// 2: Restore to background -> Clear the area of the previous frame
		// 3: Restore to previous -> Not supported by standard image/draw efficiently, but rare.
		//    We approximated by handling 2.
		// Note: We process disposal BEFORE drawing current frame (Wait, spec says disposal happens AFTER display)
		// Actually, in a loop:
		// 1. Compose current frame onto canvas
		// 2. Save canvas as "Display Frame"
		// 3. Apply disposal to canvas for NEXT iteration

		// Standard "draw over"
		draw.Draw(canvasImg, frame.Bounds(), frame, frame.Bounds().Min, draw.Over)

		// Save a copy for display
		// We must copy because canvasImg will change
		displayFrame := image.NewRGBA(canvasImg.Bounds())
		draw.Draw(displayFrame, canvasImg.Bounds(), canvasImg, image.Point{}, draw.Src)
		frames[i] = displayFrame

		// Calculate delay
		delay := time.Duration(g.Delay[i]) * 10 * time.Millisecond
		if delay == 0 {
			delay = 100 * time.Millisecond
		}
		delays[i] = delay

		// Handle Disposal for NEXT frame
		disposal := g.Disposal[i]
		if disposal == 2 { // Restore to background
			// Clear the *current frame's* bounds from the canvas
			draw.Draw(canvasImg, frame.Bounds(), image.Transparent, image.Point{}, draw.Src)
		}
	}

	return frames, delays, nil
}

// CreateAboutSplash creates an animated splash screen for the "About" dialog.
func (sa *SpiceApp) CreateAboutSplash() {
	defer func() {
		if r := recover(); r != nil {
			errStr := fmt.Sprintf("%v", r)
			utilLog.Printf("ERROR: PANIC in CreateAboutSplash (likely stale GLFW context): %v", errStr)
			if strings.Contains(strings.ToLower(errStr), "apiunavailable") || strings.Contains(strings.ToLower(errStr), "wgl") || strings.Contains(strings.ToLower(errStr), "opengl") {
				ShowNativeFallbackAlert(i18n.T("Graphics error"), i18n.T("Paintpane requires OpenGL 2.0+ or hardware acceleration to show windows. The application will continue to run in the background. Please try rebooting your machine to restore graphics functionality."))
			}
		}
	}()
	if sa.splash != nil {
		return // Already showing
	}

	// Use the driver's native splash window creation
	drv, ok := sa.Driver().(desktop.Driver)
	if !ok {
		return // Not supported on this platform
	}
	splashWindow := drv.CreateSplashWindow()

	// Load the raw GIF data
	gifData, err := sa.assetMgr.GetRawImage("about_splash.gif")
	if err != nil {
		utilLog.Printf("Failed to load about_splash.gif: %v", err)
		return
	}

	// Pre-render frames to fix artifacts
	frames, delays, err := decodeGifToFrames(gifData)
	if err != nil {
		utilLog.Printf("Failed to decode GIF: %v", err)
		return
	}

	// Create initial image canvas
	img := canvas.NewImageFromImage(frames[0])
	img.FillMode = canvas.ImageFillContain
	img.ScaleMode = canvas.ImageScaleSmooth

	// Create Version Overlay
	versionLabel := canvas.NewText(config.AppVersion, color.RGBA{100, 50, 0, 200})
	versionLabel.TextSize = 18
	versionLabel.TextStyle = fyne.TextStyle{Bold: true}
	versionLabel.Alignment = fyne.TextAlignTrailing

	// Layout: Layer the version text over the image
	textContainer := container.NewPadded(container.NewBorder(nil, versionLabel, nil, nil))
	// A click on the animation, or Esc, stops it and closes the splash (A).
	closeSplash := func() {
		if sa.splash == nil {
			return
		}
		sa.splash.Hide()
		sa.splash = nil
		sa.os.TransformToBackground()
	}
	content := container.NewStack(img, textContainer, newTapCatcher(closeSplash))
	splashWindow.Canvas().SetOnTypedKey(func(ev *fyne.KeyEvent) {
		if ev.Name == fyne.KeyEscape {
			closeSplash()
		}
	})

	splashWindow.SetContent(content)
	splashWindow.Resize(fyne.NewSize(300, 300)) // Scaled to 300px (Original)
	splashWindow.CenterOnScreen()
	sa.splash = splashWindow
	sa.splash.Show()

	// Start manual animation loop with pre-rendered frames
	go func() {
		defer fyne.Do(closeSplash)

		// One full cycle
		for i, frame := range frames {
			// Check if closed externally
			if sa.splash == nil {
				return
			}

			// Update frame
			currentFrame := frame
			fyne.Do(func() {
				img.Image = currentFrame
				img.Refresh()
			})

			time.Sleep(delays[i])
		}
	}()
}
