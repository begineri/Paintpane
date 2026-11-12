//go:build screenshots

// Renders the preferences window with Fyne's software painter so UI changes
// can be reviewed without launching the app. Run with:
//
//	SCREENSHOT_DIR=<dir> go test -tags screenshots ./ui/ -run TestPreferencesScreenshots
//
// Point LOCALAPPDATA (Windows) at a temporary directory first: the wallpaper
// plugin's Init creates its working directories under the user cache dir.
package ui

import (
	"image/color"
	"image/png"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	fynecanvas "fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/dixieflatline76/Spice/v2/asset"
	"github.com/dixieflatline76/Spice/v2/config"
	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	spiceui "github.com/dixieflatline76/Spice/v2/pkg/ui"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/artic"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/cleveland"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/favorites"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/getty"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/googlephotos"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/localfolder"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/metmuseum"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/npm"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/pexels"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/rijksmuseum"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/smk"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/wallhaven"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/wikimedia"
)

type screenshotManager struct {
	prefs fyne.Preferences
	am    *asset.Manager
}

func (m *screenshotManager) Register(spiceui.Plugin)   {}
func (m *screenshotManager) Deregister(spiceui.Plugin) {}
func (m *screenshotManager) CreateMenuItem(string, func(), string) *fyne.MenuItem {
	return nil
}
func (m *screenshotManager) CreateToggleMenuItem(string, func(bool), string, bool) *fyne.MenuItem {
	return nil
}
func (m *screenshotManager) OpenURL(*url.URL) error           { return nil }
func (m *screenshotManager) OpenPreferences(string)           {}
func (m *screenshotManager) GetPreferences() fyne.Preferences { return m.prefs }
func (m *screenshotManager) GetAssetManager() *asset.Manager  { return m.am }
func (m *screenshotManager) RefreshTrayMenu()                 {}
func (m *screenshotManager) RebuildTrayMenu()                 {}
func (m *screenshotManager) ShowTuneImagePopup(int, string, provider.TuningOptions, provider.TuningOptions, [9]string, [9]provider.CropAnchor, spiceui.TunePreview, bool, func(provider.TuningOptions, func()), func()) {
}

// screenshotScreen selects a preferences page and, optionally, a source to open.
type screenshotScreen struct {
	name   string
	page   string
	source string // translation key of a source name to open on the Sources page
	link   string // translation key of a link to follow on the page
}

func TestPreferencesScreenshots(t *testing.T) {
	out := os.Getenv("SCREENSHOT_DIR")
	if out == "" {
		t.Skip("SCREENSHOT_DIR not set")
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}

	a := test.NewTempApp(t)
	wp := wallpaper.GetInstance()
	wp.Init(&screenshotManager{prefs: a.Preferences(), am: asset.NewManager()})

	screens := []screenshotScreen{
		{name: "sources", page: pageSources},
		{name: "museum", page: pageSources, source: "The Getty"},
		{name: "local-folders", page: pageSources, source: "Local folders"},
		{name: "wallpaper", page: pageWallpaper},
		{name: "general", page: pageGeneral},
		{name: "advanced", page: pageAdvanced},
		{name: "shortcuts", page: pageGeneral, link: "View all shortcuts →"},
	}
	variants := map[string]themeMode{"light": themeModeLight, "dark": themeModeDark}

	for _, lang := range []string{"zh", "en", "de"} {
		i18n.SetLanguage(lang)
		for vName, mode := range variants {
			a.Settings().SetTheme(newPaintpaneTheme(mode))
			for _, s := range screens {
				name := lang + "-" + vName + "-" + s.name + ".png"
				capturePreferences(t, a, wp, s, filepath.Join(out, name))
			}
			for _, tc := range tuneScreens() {
				captureTune(t, tc.req, filepath.Join(out, lang+"-"+vName+"-tune-"+tc.name+".png"))
			}
		}
	}
	i18n.SetLanguage("en")
}

func capturePreferences(t *testing.T, a fyne.App, wp *wallpaper.Plugin, s screenshotScreen, path string) {
	t.Helper()
	w := test.NewWindow(nil)
	defer w.Close()
	w.SetPadded(false)
	w.Resize(fyne.NewSize(defaultPrefsWindowWidth, defaultPrefsWindowHeight))

	sa := &SpiceApp{App: a, appConfig: config.NewAppConfig(a.Preferences()), plugins: []spiceui.Plugin{wp}, prefsWindow: w}
	sa.RebuildPreferencesContent("")
	defer sa.prefsSM.FlushPending() // no auto-apply timer firing during later captures
	sa.prefsNav.Select(s.page)
	w.Resize(fyne.NewSize(defaultPrefsWindowWidth, defaultPrefsWindowHeight))
	if s.source != "" {
		if !tapSourceRow(w.Canvas().Content(), i18n.T(s.source)) {
			t.Errorf("source %q not found", s.source)
		}
	}

	if s.link != "" {
		followed := false
		for _, o := range test.LaidOutObjects(w.Canvas().Content()) {
			var h *widget.Hyperlink
			switch x := o.(type) {
			case *widget.Hyperlink:
				h = x
			case *accentLink:
				h = &x.Hyperlink
			}
			if h != nil && h.Text == i18n.T(s.link) {
				pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(o)
				test.TapCanvas(w.Canvas(), pos.Add(fyne.NewPos(o.Size().Width/2, o.Size().Height/2)))
				followed = true
			}
		}
		if !followed {
			t.Errorf("link %q not found", s.link)
		}
	}

	// The software painter cannot draw rectangles with a nil fill; the GL painter skips them.
	for _, o := range test.LaidOutObjects(w.Canvas().Content()) {
		if r, ok := o.(*fynecanvas.Rectangle); ok && r.FillColor == nil {
			r.FillColor = color.Transparent
		}
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, w.Canvas().Capture()); err != nil {
		t.Fatal(err)
	}
}

// tapSourceRow taps the source row whose title label reads title.
func tapSourceRow(root fyne.CanvasObject, title string) bool {
	for _, o := range test.LaidOutObjects(root) {
		stack, ok := o.(*fyne.Container)
		if !ok || len(stack.Objects) != 2 {
			continue
		}
		btn, ok := stack.Objects[0].(*widget.Button)
		if !ok || btn.Text != "" {
			continue
		}
		for _, inner := range test.LaidOutObjects(stack.Objects[1]) {
			if l, ok := inner.(*widget.Label); ok && l.Text == title {
				test.Tap(btn)
				return true
			}
		}
	}
	return false
}

type tuneScreen struct {
	name string
	req  tuneRequest
}

func tuneScreens() []tuneScreen {
	preview := filepath.Join("..", "pkg", "wallpaper", "testdata", "tuning_images", "4ld7or.jpg")
	base := provider.TuningOptions{
		CropOffset:    30,
		HasCropOffset: true,
		FrameOverride: provider.FrameOverrideForceOff,
		FrameSize:     0.85,
		Matting:       provider.MattingOverrideOn,
		WallColor:     provider.WallColorOverrideAlgorithmic,
	}
	framed := base
	framed.FrameOverride = provider.FrameOverrideForceOn
	return []tuneScreen{
		{"crop", tuneRequest{previewPath: preview, opts: base, global: provider.DisplayCrop}},
		{"frame", tuneRequest{previewPath: preview, opts: framed, global: provider.DisplayCrop}},
		{"whole", tuneRequest{previewPath: preview, opts: base, global: provider.DisplayFull}},
	}
}

func captureTune(t *testing.T, req tuneRequest, path string) {
	t.Helper()
	w := test.NewWindow(nil)
	defer w.Close()
	w.SetPadded(false)
	content := newTunePanel(req)
	w.SetContent(content)
	w.Resize(content.MinSize())

	for _, o := range test.LaidOutObjects(w.Canvas().Content()) {
		if r, ok := o.(*fynecanvas.Rectangle); ok && r.FillColor == nil {
			r.FillColor = color.Transparent
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, w.Canvas().Capture()); err != nil {
		t.Fatal(err)
	}
}
