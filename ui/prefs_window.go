package ui

import (
	"os/exec"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"

	"github.com/dixieflatline76/Spice/v2/config"
	"github.com/dixieflatline76/Spice/v2/pkg/api"
	"github.com/dixieflatline76/Spice/v2/pkg/hotkey"
	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
	utilLog "github.com/dixieflatline76/Spice/v2/util/log"
)

const (
	projectURL = "https://github.com/begineri/Paintpane"
	licenseURL = projectURL + "/blob/main/LICENSE"
)

// RebuildPreferencesContent rebuilds the content of the preferences window.
// initialTab selects a page: "App" opens General, a plugin ID opens the page the
// plugin asks for (Sources when a source should open), and "" keeps the
// current page.
func (sa *SpiceApp) RebuildPreferencesContent(initialTab string) {
	if sa.prefsWindow == nil {
		return
	}

	previous := ""
	if sa.prefsNav != nil {
		previous = sa.prefsNav.Current()
	}
	if sa.prefsSM != nil {
		sa.prefsSM.FlushPending()
	}

	sm := NewSettingsManager(sa.prefsWindow).(*SettingsManager)
	sa.prefsSM = sm
	sm.RegisterOnSettingsSaved(func() {
		sa.RebuildTrayMenu()
	})

	// Plugin pages, keyed by ID. Their "advanced" page is merged with the app's.
	pluginPages := map[string]schema.TabItemSchema{}
	pluginInitial := map[string]string{}
	for _, plugin := range sa.plugins {
		tabs := plugin.CreatePrefsSchema(sm)
		if tabs == nil {
			continue
		}
		for i, item := range tabs.Items {
			pluginPages[item.ID] = item
			if i == tabs.Selected {
				pluginInitial[plugin.ID()] = item.ID
			}
		}
	}

	advanced := schema.TabItemSchema{ID: pageAdvanced, Title: i18n.T("Advanced"), Panel: &schema.PanelSchema{}}
	if p, ok := pluginPages[pageAdvanced]; ok && p.Panel != nil {
		advanced.Panel.Sections = append(advanced.Panel.Sections, p.Panel.Sections...)
	}
	advanced.Panel.Sections = append(advanced.Panel.Sections, sa.appAdvancedSections(sm)...)

	// General shows its page, or the Shortcuts page that its "View all shortcuts" link opens.
	generalView := container.NewStack()
	var generalList fyne.CanvasObject
	showGeneral := func() {
		generalView.Objects = []fyne.CanvasObject{generalList}
		generalView.Refresh()
	}
	showShortcuts := func() {
		generalView.Objects = []fyne.CanvasObject{sa.newShortcutsPage(sm, showGeneral)}
		generalView.Refresh()
	}
	general := schema.TabItemSchema{ID: pageGeneral, Title: i18n.T("General"), Panel: sa.generalPanel(sm, showShortcuts)}

	var pages []navPage
	addPage := func(item schema.TabItemSchema, icon fyne.Resource) {
		pages = append(pages, navPage{id: item.ID, title: item.Title, icon: icon, content: sm.RenderPage(item),
			reselect: func() { sm.subPages.close(item.ID) }})
	}
	if p, ok := pluginPages[pageSources]; ok {
		addPage(p, theme.HomeIcon())
		pages[len(pages)-1].reselect = func() {
			if sm.sourcesShowList != nil {
				sm.sourcesShowList()
			}
		}
	}
	if p, ok := pluginPages[pageWallpaper]; ok {
		addPage(p, theme.MediaPhotoIcon())
	}
	generalList = sm.RenderPage(general)
	generalView.Objects = []fyne.CanvasObject{generalList}
	pages = append(pages, navPage{id: pageGeneral, title: general.Title, icon: theme.SettingsIcon(), content: generalView, reselect: showGeneral})
	addPage(advanced, theme.ComputerIcon())

	initial := previous
	switch {
	case initialTab == "App":
		initial = pageGeneral
	case pluginInitial[initialTab] != "":
		initial = pluginInitial[initialTab]
	}

	nav, root := newPrefsNavigation(pages, initial)
	sa.prefsNav = nav

	// Wrap in minSizeContainer so Fyne doesn't enforce the full content height as
	// the window minimum.
	sa.prefsWindow.SetContent(newMinSizeContainer(root))
	sm.GetCheckAndEnableApplyFunc()() // Trigger initial UI dependency refresh
}

// chooseThemeMode saves and applies the Appearance choice at index val (an int).
func (sa *SpiceApp) chooseThemeMode(val interface{}) {
	i, _ := val.(int)
	if i < 0 || i >= len(themeModes) {
		return
	}
	mode := themeModes[i]
	sa.appConfig.SetTheme(string(mode))
	applyTheme(sa.App, mode, sa.appConfig.GetAccent())
}

// chooseAccent saves and applies the Accent color choice at index val (an int).
func (sa *SpiceApp) chooseAccent(val interface{}) {
	i, _ := val.(int)
	if i < 0 || i >= len(accents) {
		return
	}
	sa.appConfig.SetAccent(accents[i].key)
	// Links follow the accent on the theme's refresh (accentLink), so leaving or
	// choosing graphite needs no page rebuild.
	applyTheme(sa.App, parseThemeMode(sa.appConfig.GetTheme()), accents[i].key)
}

// generalPanel builds the General page: appearance, running and shortcuts.
func (sa *SpiceApp) generalPanel(sm *SettingsManager, openShortcuts func()) *schema.PanelSchema {
	themeOptions := make([]string, len(themeModes))
	for i, mode := range themeModes {
		themeOptions[i] = mode.label()
	}

	accentOptions := make([]string, len(accents))
	for i, a := range accents {
		accentOptions[i] = accentLabel(a.key)
	}

	langOptions := i18n.GetLanguageNames()
	langDisplayOptions := i18n.GetLanguageNames()
	langDisplayOptions[0] = i18n.T("System default")
	initialLangIndex := 0
	for idx, l := range langOptions {
		if l == sa.appConfig.GetLanguage() {
			initialLangIndex = idx
			break
		}
	}

	return &schema.PanelSchema{
		Sections: []schema.SectionSchema{
			{
				Title: i18n.T("Personalization"),
				Items: []schema.ItemSchema{
					schema.SelectItem{
						Name:         "theme",
						Label:        i18n.T("Appearance"),
						Options:      themeOptions,
						Segmented:    true,
						InitialValue: themeModeIndex(parseThemeMode(sa.appConfig.GetTheme())),
						// Appearance and Accent color apply on the click, not after the page's
						// auto-apply delay; ApplyFunc then finds nothing left to change.
						OnChanged: func(_ string, val interface{}) { sa.chooseThemeMode(val) },
						ApplyFunc: sa.chooseThemeMode,
					},
					schema.SelectItem{
						Name:         "accent",
						Label:        i18n.T("Accent color"),
						Options:      accentOptions,
						Swatches:     true,
						InitialValue: accentIndex(sa.appConfig.GetAccent()),
						OnChanged:    func(_ string, val interface{}) { sa.chooseAccent(val) },
						ApplyFunc:    sa.chooseAccent,
					},
					schema.SelectItem{
						Name:         "language",
						Label:        i18n.T("Language"),
						Options:      langDisplayOptions,
						InitialValue: initialLangIndex,
						ApplyFunc: func(val interface{}) {
							i, _ := val.(int)
							if i < 0 || i >= len(langOptions) {
								return
							}
							selectedLang := langOptions[i]
							sa.appConfig.SetLanguage(selectedLang)
							i18n.SetLanguage(selectedLang)

							if srv := api.GetServer(); srv != nil {
								if err := srv.BroadcastLanguage(i18n.GetLanguage()); err != nil {
									utilLog.Printf("Failed to broadcast language change to extensions: %v", err)
								}
							}

							// Rebuild after the current apply finishes, keeping the current page.
							fyne.Do(func() {
								sa.RebuildPreferencesContent("")
								sa.RebuildTrayMenu()
							})
						},
					},
				},
			},
			{
				Title: i18n.T("Running"),
				Items: []schema.ItemSchema{
					// S3-6: the program's own switch, on when it has registered itself to start with
					// Windows. The installer ticks it by default.
					schema.BoolItem{
						Name:         "startWithWindows",
						Label:        i18n.T("Start with Windows"),
						InitialValue: autostartOn(),
						ApplyFunc:    sa.setAutostart,
						VisibleIf:    autostartAvailable,
					},
					schema.ButtonItem{
						Name:       "manageMacStartup",
						Label:      i18n.T("Start at login"),
						ButtonText: i18n.T("Manage in macOS Settings"),
						OnPressed: func() {
							_ = exec.Command("open", "x-apple.systempreferences:com.apple.LoginItems-Settings.extension").Start()
						},
						VisibleIf: func() bool { return runtime.GOOS == "darwin" },
					},
					schema.BoolItem{
						Name:         "enableUpdateCheck",
						Label:        i18n.T("Check for new versions"),
						InitialValue: sa.appConfig.GetUpdateCheckEnabled(),
						ApplyFunc:    sa.appConfig.SetUpdateCheckEnabled,
						VisibleIf:    func() bool { return !config.IsStoreDistribution() },
					},
				},
			},
			{
				Title: i18n.T("Shortcuts"),
				Items: []schema.ItemSchema{
					schema.BoolItem{
						Name:         "enableShortcuts",
						Label:        i18n.T("Global shortcuts"),
						InitialValue: !wallpaper.GetInstance().GetShortcutsDisabled(),
						ApplyFunc: func(val bool) {
							wallpaper.GetInstance().SetShortcutsDisabled(!val)
							hotkey.StartListeners(GetPluginManager())
						},
					},
					schema.BoolItem{
						Name:         "enableTargetedShortcuts",
						Label:        i18n.T("Single-display shortcuts (hold Alt + display number)"),
						InitialValue: !wallpaper.GetInstance().GetTargetedShortcutsDisabled(),
						ApplyFunc: func(val bool) {
							wallpaper.GetInstance().SetTargetedShortcutsDisabled(!val)
							hotkey.StartListeners(GetPluginManager())
						},
						EnabledIf: func() bool {
							val, ok := sm.GetValue("enableShortcuts").(bool)
							return !ok || val
						},
					},
					schema.HyperlinkItem{
						ID:       "shortcutsLink",
						Text:     i18n.T("View all shortcuts →"),
						OnTapped: openShortcuts,
					},
				},
			},
		},
	}
}

// appAdvancedSections are the app's sections on the Advanced page.
func (sa *SpiceApp) appAdvancedSections(_ *SettingsManager) []schema.SectionSchema {
	return []schema.SectionSchema{
		{
			Title: i18n.T("Troubleshooting"),
			Items: []schema.ItemSchema{
				schema.BoolItem{
					Name:         "enableDebugLogging",
					Label:        i18n.T("Debug logging"),
					InitialValue: sa.appConfig.GetDebugLoggingEnabled(),
					ApplyFunc: func(b bool) {
						sa.appConfig.SetDebugLoggingEnabled(b)
						utilLog.SetDebugEnabled(b)
					},
				},
			},
		},
		{
			Title: i18n.T("About"),
			Items: []schema.ItemSchema{
				schema.ButtonItem{
					Name:       "aboutPaintpane",
					Label:      i18n.T("Paintpane") + " " + config.AppVersion,
					ButtonText: i18n.T("About Paintpane"),
					OnPressed:  sa.CreateAboutSplash,
				},
				schema.HorizontalRowItem{
					ID:     "aboutLinks",
					Layout: schema.RowLayoutInline,
					Items: []schema.ItemSchema{
						schema.HyperlinkItem{Text: i18n.T("Help"), URL: projectURL + "/issues"},
						schema.HyperlinkItem{Text: i18n.T("Source code"), URL: projectURL},
						schema.HyperlinkItem{Text: i18n.T("License"), URL: licenseURL},
					},
				},
			},
		},
	}
}
