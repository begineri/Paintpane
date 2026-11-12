package ui

// Scenario tests drive the real preferences window (RebuildPreferencesContent,
// side navigation, Sources list and detail, immediate apply) with Fyne's test
// driver: no screen, no mouse. A fake plugin supplies the pages so the tests do
// not start the wallpaper plugin, which would write to the user's app folder.
//
// Helpers find widgets by the text a user sees, not by layout, so they keep
// working when the layout changes.

import (
	"net/url"
	"sync"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/asset"
	"github.com/dixieflatline76/Spice/v2/config"
	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	spiceui "github.com/dixieflatline76/Spice/v2/pkg/ui"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/setting"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"

	// The "Yours" sources, so the Sources page lists what users see. Their
	// constructors only read the app folder (Favorites reads its metadata).
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/favorites"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/googlephotos"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/localfolder"
)

// scenarioPlugin provides a Sources page with one museum and a Wallpaper page.
type scenarioPlugin struct {
	applied map[string]bool
}

func (p *scenarioPlugin) ID() string                         { return "scenario" }
func (p *scenarioPlugin) Name() string                       { return "Scenario" }
func (p *scenarioPlugin) Activate()                          {}
func (p *scenarioPlugin) Deactivate()                        {}
func (p *scenarioPlugin) CreateTrayMenu() *schema.MenuSchema { return nil }
func (p *scenarioPlugin) Init(spiceui.PluginManager)         {}
func (p *scenarioPlugin) apply(name string) func(bool)       { return func(b bool) { p.applied[name] = b } }
func (p *scenarioPlugin) CreatePrefsSchema(setting.SettingsManager) *schema.TabsSchema {
	museum := &schema.PanelSchema{Sections: []schema.SectionSchema{{
		Title:     i18n.T("Curated collections"),
		SelectAll: &schema.SelectAllItem{Name: "getty_all", Label: i18n.T("Select all"), Members: []string{"getty_best", "getty_paintings"}},
		Items: []schema.ItemSchema{
			schema.BoolItem{Name: "getty_best", Label: "Best of Getty", ApplyFunc: p.apply("getty_best")},
			schema.BoolItem{Name: "getty_paintings", Label: "European Paintings", InitialValue: true, ApplyFunc: p.apply("getty_paintings")},
		},
	}}}
	wallpaperPage := &schema.PanelSchema{Sections: []schema.SectionSchema{{
		Title: "Rotation",
		Items: []schema.ItemSchema{
			schema.BoolItem{Name: "change_on_start", Label: "Change wallpaper on start", InitialValue: true, ApplyFunc: p.apply("change_on_start")},
		},
	}}}
	return &schema.TabsSchema{Items: []schema.TabItemSchema{
		{ID: pageSources, Title: "Sources", Sources: &schema.SourcesSchema{Groups: []schema.SourceGroupSchema{{
			ID: "museums", Title: "Museums",
			Items: []schema.SourceSchema{{ID: "getty", Title: i18n.T("The Getty"), Detail: museum}},
		}}}},
		{ID: pageWallpaper, Title: "Wallpaper", Panel: wallpaperPage},
	}}
}

// scenarioManager is the plugin manager handed to the wallpaper plugin, whose
// settings the General page reads. TestMain points its cache at a temp folder.
type scenarioManager struct {
	prefs fyne.Preferences
	am    *asset.Manager
}

func (m *scenarioManager) Register(spiceui.Plugin)   {}
func (m *scenarioManager) Deregister(spiceui.Plugin) {}
func (m *scenarioManager) CreateMenuItem(string, func(), string) *fyne.MenuItem {
	return nil
}
func (m *scenarioManager) CreateToggleMenuItem(string, func(bool), string, bool) *fyne.MenuItem {
	return nil
}
func (m *scenarioManager) OpenURL(*url.URL) error           { return nil }
func (m *scenarioManager) OpenPreferences(string)           {}
func (m *scenarioManager) GetPreferences() fyne.Preferences { return m.prefs }
func (m *scenarioManager) GetAssetManager() *asset.Manager  { return m.am }
func (m *scenarioManager) RefreshTrayMenu()                 {}
func (m *scenarioManager) RebuildTrayMenu()                 {}
func (m *scenarioManager) ShowTuneImagePopup(int, string, provider.TuningOptions, provider.TuningOptions, [9]string, [9]provider.CropAnchor, spiceui.TunePreview, bool, func(provider.TuningOptions, func()), func()) {
}

var initWallpaperOnce sync.Once

type scenario struct {
	t      *testing.T
	sa     *SpiceApp
	w      fyne.Window
	plugin *scenarioPlugin
}

func newScenario(t *testing.T) *scenario {
	t.Helper()
	p := &scenarioPlugin{applied: map[string]bool{}}
	s := newScenarioWith(t, p)
	s.plugin = p
	return s
}

// newScenarioWith opens the preferences window with the given plugins, e.g. the
// real wallpaper plugin for pages the fake does not provide.
func newScenarioWith(t *testing.T, plugins ...spiceui.Plugin) *scenario {
	t.Helper()
	a := test.NewTempApp(t)
	initWallpaperOnce.Do(func() {
		wallpaper.GetInstance().Init(&scenarioManager{prefs: a.Preferences(), am: asset.NewManager()})
	})
	w := test.NewWindow(nil)
	t.Cleanup(w.Close)
	w.SetPadded(false)
	w.Resize(fyne.NewSize(defaultPrefsWindowWidth, defaultPrefsWindowHeight))
	sa := &SpiceApp{App: a, appConfig: config.NewAppConfig(a.Preferences()), assetMgr: asset.NewManager(),
		plugins: plugins, prefsWindow: w}
	sa.RebuildPreferencesContent("")
	w.Resize(fyne.NewSize(defaultPrefsWindowWidth, defaultPrefsWindowHeight))
	// Apply what is left at the end so the auto-apply timer does not fire during later tests.
	t.Cleanup(func() { sa.prefsSM.FlushPending() })
	return &scenario{t: t, sa: sa, w: w}
}

// shownObjects lists every object that is visible, i.e. it and all its parents are shown.
func shownObjects(o fyne.CanvasObject) []fyne.CanvasObject {
	flushUI()
	if o == nil || !o.Visible() {
		return nil
	}
	out := []fyne.CanvasObject{o}
	switch x := o.(type) {
	case *fyne.Container:
		for _, c := range x.Objects {
			out = append(out, shownObjects(c)...)
		}
	case fyne.Widget:
		for _, c := range test.WidgetRenderer(x).Objects() {
			out = append(out, shownObjects(c)...)
		}
	}
	return out
}

func labelText(o fyne.CanvasObject) (string, bool) {
	switch l := o.(type) {
	case *wrappedText: // dialog text: the sentence, not where it is wrapped
		return l.source, true
	case *widget.Label:
		return l.Text, true
	case *widget.Button:
		return l.Text, true
	case *canvas.Text:
		return l.Text, true
	case *widget.RichText:
		return l.String(), true
	}
	return "", false
}

// shows reports whether some visible label or button reads text.
func (s *scenario) shows(text string) bool {
	for _, o := range shownObjects(s.w.Canvas().Content()) {
		if got, ok := labelText(o); ok && got == text {
			return true
		}
	}
	return false
}

// tapText taps the visible label or button reading text, the way a user would
// click it: at its position, so whatever tappable lies under it receives the tap.
// With several matches it taps the last one drawn, which is the one on top.
func (s *scenario) tapText(text string) {
	s.t.Helper()
	var target fyne.CanvasObject
	for _, o := range shownObjects(s.w.Canvas().Content()) {
		if got, ok := labelText(o); ok && got == text {
			// A label's own widget is centred on its row; the text drawn inside it can sit
			// off-centre in a row with a single line, so prefer the widget.
			if _, isLabel := target.(*widget.Label); !isLabel || isLabelObj(o) {
				target = o
			}
		}
	}
	require.NotNil(s.t, target, "nothing visible reads %q", text)
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(target)
	test.TapCanvas(s.w.Canvas(), pos.Add(fyne.NewPos(target.Size().Width/2, target.Size().Height/2)))
}

func isLabelObj(o fyne.CanvasObject) bool { _, ok := o.(*widget.Label); return ok }

// toggle taps the switch of the named boolean setting.
func (s *scenario) toggle(name string) {
	s.t.Helper()
	sw, ok := s.sa.prefsSM.allWidgets[name].(*toggleSwitch)
	require.True(s.t, ok, "no switch for %q", name)
	test.Tap(sw)
}

func (s *scenario) switchOn(name string) bool {
	return s.sa.prefsSM.allWidgets[name].(*toggleSwitch).Checked
}

func TestScenarioOpenSourceToggleCollectionAndGoBack(t *testing.T) {
	s := newScenario(t)
	require.True(t, s.shows("The Getty"), "Sources starts on the list")
	require.False(t, s.shows("Best of Getty"))

	// Open the museum from the list.
	s.tapText("The Getty")
	assert.True(t, s.shows("Best of Getty"), "the detail shows the museum's collections")
	assert.False(t, s.shows("Museums"), "the list is replaced by the detail")

	// Turn a collection on: it applies once pending changes flush.
	assert.False(t, s.switchOn("getty_best"))
	s.toggle("getty_best")
	assert.True(t, s.switchOn("getty_best"))
	s.sa.prefsSM.FlushPending()
	assert.Equal(t, map[string]bool{"getty_best": true}, s.plugin.applied, "only the changed setting applies")

	// Choosing Sources in the side navigation goes back to the list.
	s.sa.prefsNav.Select(pageSources)
	assert.True(t, s.shows("The Getty"))
	assert.False(t, s.shows("Best of Getty"))
}

func TestScenarioSwitchPagesAndApplyImmediately(t *testing.T) {
	s := newScenario(t)

	s.tapText("Wallpaper")
	assert.Equal(t, pageWallpaper, s.sa.prefsNav.Current())
	assert.True(t, s.shows("Change wallpaper on start"))
	assert.False(t, s.shows("The Getty"), "only the selected page is shown")

	s.toggle("change_on_start")
	s.toggle("change_on_start")
	s.sa.prefsSM.FlushPending()
	assert.Empty(t, s.plugin.applied, "switching back to the saved value applies nothing")

	s.toggle("change_on_start")
	s.sa.prefsSM.FlushPending()
	assert.Equal(t, map[string]bool{"change_on_start": false}, s.plugin.applied)

	s.tapText("General")
	assert.Equal(t, pageGeneral, s.sa.prefsNav.Current())
}

// Prototype v35: the app sends no Windows notifications, so the General page has no System
// notifications switch (S3-7).
func TestScenarioGeneralHasNoNotificationsSwitch(t *testing.T) {
	s := newScenario(t)
	s.sa.prefsNav.Select(pageGeneral)
	assert.True(t, s.shows("Check for new versions"), "the General page is shown")
	assert.False(t, s.shows("System notifications"))
	assert.NotContains(t, s.sa.prefsSM.allWidgets, "enableNotifications")
}

// inLanguages runs a scenario once per UI language, English and Chinese.
func inLanguages(t *testing.T, run func(t *testing.T, lang string)) {
	for _, lang := range []string{"en", "zh"} {
		t.Run(lang, func(t *testing.T) {
			i18n.SetLanguage(lang)
			t.Cleanup(func() { i18n.SetLanguage("en") })
			run(t, lang)
		})
	}
}

func TestScenarioMuseumSelectAll(t *testing.T) {
	inLanguages(t, func(t *testing.T, lang string) {
		want := map[string]struct{ museum, oneOn, allOn, noneOn string }{
			"en": {"The Getty", "Curated collections · 1/2 on", "Curated collections · 2/2 on", "Curated collections · 0/2 on"},
			"zh": {"盖蒂博物馆", "精选收藏 · 1/2 已启用", "精选收藏 · 2/2 已启用", "精选收藏 · 0/2 已启用"},
		}[lang]
		s := newScenario(t)
		s.tapText(want.museum)
		require.True(t, s.shows(want.oneOn), "the header counts the collections that are on")
		assert.False(t, s.switchOn("getty_all"), "Select all is off while one collection is off")

		// Select all turns every collection on, and the count follows.
		s.toggle("getty_all")
		assert.True(t, s.switchOn("getty_best") && s.switchOn("getty_paintings"))
		assert.True(t, s.shows(want.allOn))

		// Turning it off turns every collection off.
		s.toggle("getty_all")
		assert.False(t, s.switchOn("getty_best") || s.switchOn("getty_paintings"))
		assert.True(t, s.shows(want.noneOn))
		s.sa.prefsSM.FlushPending()
		assert.Equal(t, map[string]bool{"getty_paintings": false}, s.plugin.applied,
			"only the collection that changed from its saved value applies")
	})
}

func TestScenarioAdvancedStorage(t *testing.T) {
	inLanguages(t, func(t *testing.T, lang string) {
		want := map[string]struct{ page, clear, size string }{
			"en": {"Advanced", "Clear cache", "150 images"},
			"zh": {"高级", "清除缓存", "150 张"},
		}[lang]
		s := newScenarioWith(t, wallpaper.GetInstance())
		s.tapText(want.page)
		require.Equal(t, pageAdvanced, s.sa.prefsNav.Current())

		// Every cache size names its unit.
		size, ok := s.sa.prefsSM.allWidgets["cacheSize"].(*widget.Select)
		require.True(t, ok, "cache size is a drop-down")
		assert.Contains(t, size.Options, want.size)
		assert.Equal(t, want.size, size.Selected, "the default size reads with its unit")

		// Clear cache is a red text button, not a filled one.
		clear, ok := s.sa.prefsSM.allWidgets["clearCache"].(*dangerButton)
		require.True(t, ok, "Clear cache is a danger text button")
		assert.Equal(t, want.clear, clear.Text)
		assert.True(t, s.shows(want.clear))
	})
}

// S4-3H (prototype v37/v40): High-resolution originals sits under Cache size, is off by default,
// says what it is for, and turning it on keeps originals at full size.
func TestScenarioHighResolutionOriginals(t *testing.T) {
	inLanguages(t, func(t *testing.T, lang string) {
		want := map[string]struct{ page, label, help string }{
			"en": {"Advanced", "High-resolution originals", "Turn on if images look soft on a 5K or 8K screen. Uses more disk space."},
			"zh": {"高级", "高分辨率原图", "5K、8K 屏幕觉得模糊时打开，会多占磁盘空间。"},
		}[lang]
		s := newScenarioWith(t, wallpaper.GetInstance())
		cfg := wallpaper.GetConfigInstance()
		cfg.SetHighResOriginals(false)
		t.Cleanup(func() { cfg.SetHighResOriginals(false) })
		s.tapText(want.page)
		require.Equal(t, pageAdvanced, s.sa.prefsNav.Current())
		assert.True(t, s.shows(want.label))
		assert.True(t, s.shows(want.help))
		assert.False(t, s.switchOn("highResOriginals"), "off by default: originals are capped at about 4K")

		// It sits right under Cache size, in Storage.
		sizePos, _ := s.absRect(s.sa.prefsSM.allWidgets["cacheSize"])
		switchPos, _ := s.absRect(s.sa.prefsSM.allWidgets["highResOriginals"])
		clearPos, _ := s.absRect(s.sa.prefsSM.allWidgets["clearCache"])
		assert.Less(t, sizePos.Y, switchPos.Y, "below Cache size")
		assert.Less(t, switchPos.Y, clearPos.Y, "above Clear cache")

		s.toggle("highResOriginals")
		s.sa.prefsSM.FlushPending()
		assert.True(t, cfg.GetHighResOriginals(), "on keeps originals at full size")
		s.toggle("highResOriginals")
		s.sa.prefsSM.FlushPending()
		assert.False(t, cfg.GetHighResOriginals())
	})
}

func TestScenarioAboutShowsAppName(t *testing.T) {
	inLanguages(t, func(t *testing.T, lang string) {
		want := map[string]struct{ page, name, button, title string }{
			"en": {"Advanced", "Paintpane", "About Paintpane", "Paintpane settings"},
			"zh": {"高级", "观画壁纸", "关于观画壁纸", "观画壁纸设置"},
		}[lang]
		s := newScenario(t)
		s.tapText(want.page)
		require.Equal(t, pageAdvanced, s.sa.prefsNav.Current())

		assert.True(t, s.shows(want.name+" "+config.AppVersion), "the About row names the app in the UI language")
		assert.True(t, s.shows(want.button))
		assert.Equal(t, want.title, i18n.T("Paintpane settings"), "the settings window title")
	})
}

func TestScenarioDisplayModeOrder(t *testing.T) {
	inLanguages(t, func(t *testing.T, lang string) {
		want := map[string]struct {
			page  string
			modes []string
		}{
			"en": {"Wallpaper", []string{"Full artwork", "Museum frame", "Smart crop"}},
			"zh": {"壁纸", []string{"完整显示", "博物馆画框", "智能裁切"}},
		}[lang]
		cfg := wallpaper.GetConfigInstance()
		cfg.SetArtworkDisplayMode(wallpaper.ArtworkDisplayFull)
		t.Cleanup(func() { cfg.SetArtworkDisplayMode(wallpaper.ArtworkDisplayFull) })

		s := newScenarioWith(t, wallpaper.GetInstance())
		s.tapText(want.page)
		modes, ok := s.sa.prefsSM.allWidgets["artworkDisplayMode"].(*segmented)
		require.True(t, ok, "display mode is a segmented control")

		// The two modes that never crop come first; the default is selected.
		assert.Equal(t, want.modes, modes.Options)
		assert.Equal(t, 0, modes.Selected, "Full artwork is selected")

		// Choosing Smart crop, last in the list, saves Smart crop.
		s.tapText(want.modes[2])
		assert.Equal(t, 2, modes.Selected)
		s.sa.prefsSM.FlushPending()
		assert.Equal(t, wallpaper.ArtworkDisplaySmartCrop, cfg.GetArtworkDisplayMode())

		// And Museum frame, in the middle, saves Museum frame.
		s.tapText(want.modes[1])
		s.sa.prefsSM.FlushPending()
		assert.Equal(t, wallpaper.ArtworkDisplayFramed, cfg.GetArtworkDisplayMode())
	})
}

// searchEntry returns the visible Sources search box.
func (s *scenario) searchEntry() *widget.Entry {
	s.t.Helper()
	for _, o := range shownObjects(s.w.Canvas().Content()) {
		if e, ok := o.(*widget.Entry); ok && e.PlaceHolder == i18n.T("Search museums and sources") {
			return e
		}
	}
	require.FailNow(s.t, "no search box on the Sources page")
	return nil
}

// listShows is shows without the search box, which draws the typed text.
func (s *scenario) listShows(text string) bool {
	var walk func(o fyne.CanvasObject) bool
	walk = func(o fyne.CanvasObject) bool {
		if o == nil || !o.Visible() {
			return false
		}
		if _, ok := o.(*widget.Entry); ok {
			return false
		}
		if got, ok := labelText(o); ok && got == text {
			return true
		}
		var children []fyne.CanvasObject
		switch x := o.(type) {
		case *fyne.Container:
			children = x.Objects
		case fyne.Widget:
			children = test.WidgetRenderer(x).Objects()
		}
		for _, c := range children {
			if walk(c) {
				return true
			}
		}
		return false
	}
	return walk(s.w.Canvas().Content())
}

// Google Photos needs OAuth credentials that release builds do not have, so
// the Sources page neither lists it nor finds it (ui_refresh.md §12, P10).
func TestScenarioSourcesHideGooglePhotos(t *testing.T) {
	inLanguages(t, func(t *testing.T, lang string) {
		want := map[string]struct{ yours, favorites, local, google string }{
			"en": {"Yours · 2", "Favorites", "Local folders", "Google Photos"},
			"zh": {"我的 · 2", "收藏夹", "本地文件夹", "Google 相册"},
		}[lang]
		s := newScenarioWith(t, wallpaper.GetInstance())
		require.Equal(t, pageSources, s.sa.prefsNav.Current())

		assert.True(t, s.shows(want.yours), "Yours counts Favorites and Local folders only")
		assert.True(t, s.shows(want.favorites))
		assert.True(t, s.shows(want.local))
		assert.False(t, s.shows(want.google), "Google Photos is not listed")

		// The search box draws what is typed, so look at the list only.
		search := s.searchEntry()
		for _, q := range []string{"Google", want.google} {
			search.SetText(q)
			assert.False(t, s.listShows(want.google), "searching %q does not find Google Photos", q)
		}
		search.SetText(want.local)
		assert.True(t, s.listShows(want.local), "search still finds the other sources")
		search.SetText("")
	})
}

// absRect returns an object's position in the window and its size.
func (s *scenario) absRect(o fyne.CanvasObject) (fyne.Position, fyne.Size) {
	return fyne.CurrentApp().Driver().AbsolutePositionForObject(o), o.Size()
}

// findShown returns the first visible object that matches.
func (s *scenario) findShown(match func(fyne.CanvasObject) bool) fyne.CanvasObject {
	s.t.Helper()
	for _, o := range shownObjects(s.w.Canvas().Content()) {
		if match(o) {
			return o
		}
	}
	require.FailNow(s.t, "no visible object matches")
	return nil
}

// The typography and layout pass (ui_refresh.md §12, G3–G7), measured on the
// real pages in both languages.
func TestScenarioLayoutMetrics(t *testing.T) {
	inLanguages(t, func(t *testing.T, lang string) {
		want := map[string]struct{ sources, favorites, local, wallpaper, advanced string }{
			"en": {"Sources", "Favorites", "Local folders", "Wallpaper", "Advanced"},
			"zh": {"图源", "收藏夹", "本地文件夹", "壁纸", "高级"},
		}[lang]
		s := newScenarioWith(t, wallpaper.GetInstance())
		// Measure with the app's theme and fonts, not Fyne's test theme.
		s.sa.App.Settings().SetTheme(newPaintpaneTheme(themeModeLight))
		s.sa.RebuildPreferencesContent("")
		s.w.Resize(fyne.NewSize(defaultPrefsWindowWidth, defaultPrefsWindowHeight))

		// G3: the page title, the search box and the cards share one left edge,
		// 24 px from the navigation.
		title := s.findShown(func(o fyne.CanvasObject) bool {
			st, ok := o.(*styledText)
			return ok && st.Text == want.sources
		})
		titlePos, titleSize := s.absRect(title)
		searchPos, searchSize := s.absRect(s.searchEntry())
		cardPos, _ := s.absRect(s.findShown(func(o fyne.CanvasObject) bool { _, ok := o.(*card); return ok }))
		navPos, navSize := s.absRect(s.sa.prefsNav.items[pageSources])
		// Positions are relative to the first nav item, which sits navPadX from
		// the window edge (the test canvas adds its own offset).
		assert.Equal(t, navWidth+contentMarginX, titlePos.X-navPos.X+navPadX, "title 24 px from the navigation")
		assert.Equal(t, titlePos.X, searchPos.X, "search box on the title's edge")
		assert.Equal(t, titlePos.X, cardPos.X, "cards on the title's edge")
		assert.InDelta(t, 35, searchSize.Height, 1, "search box: Fyne's entry height at 13 px text")

		// G7: the first navigation item is level with the page title.
		assert.InDelta(t, navPos.Y+navSize.Height/2, titlePos.Y+titleSize.Height/2, 1, "nav item and title centred on one line")
		assert.Equal(t, navItemH, navSize.Height)

		// G4, G8: source rows are 54 px high: the row and the card's padding.
		for _, o := range shownObjects(s.w.Canvas().Content()) {
			if c, ok := o.(*card); ok {
				assert.Equal(t, sourceRowPadY, c.padY)
				for _, row := range c.rows {
					assert.Equal(t, sourceRowHeight, row.Size().Height+2*c.padY, "a source row is 54 px high")
				}
			}
		}

		// G5, G8: selects, segmented controls and buttons are 28 px high.
		s.tapText(want.wallpaper)
		for _, name := range []string{"artworkDisplayMode", "changeFrequencyChoice"} {
			w, ok := s.sa.prefsSM.allWidgets[name]
			require.True(t, ok, name)
			assert.Equal(t, controlHeight, w.Size().Height, "%s is one control high", name)
		}
		s.tapText(want.advanced)
		for _, name := range []string{"cacheSize", "clearCache"} {
			w, ok := s.sa.prefsSM.allWidgets[name]
			require.True(t, ok, name)
			assert.Equal(t, controlHeight, w.Size().Height, "%s is one control high", name)
		}
	})
}
