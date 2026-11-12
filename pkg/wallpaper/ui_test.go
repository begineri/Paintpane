//go:build !linux

package wallpaper

import (
	"context"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type managedMockWidget struct {
	widget    fyne.CanvasObject
	enabledIf func() bool
	visibleIf func() bool
}

// MockSettingsManager implements setting.SettingsManager for testing
type MockSettingsManager struct {
	mock.Mock
	// We store the created widgets to inspect them
	allWidgets     map[string]fyne.CanvasObject
	managedWidgets []managedMockWidget
	pendingChanges map[string]bool
	statusLabels   map[string]string
	refreshFuncs   []func()
}

func NewMockSettingsManager() *MockSettingsManager {
	return &MockSettingsManager{
		allWidgets:     make(map[string]fyne.CanvasObject),
		managedWidgets: make([]managedMockWidget, 0),
		pendingChanges: make(map[string]bool),
		statusLabels:   make(map[string]string),
	}
}

func (m *MockSettingsManager) GetApplySettingsButton() *widget.Button {
	return widget.NewButton("Apply", nil)
}

func (m *MockSettingsManager) SetSettingChangedCallback(settingName string, callback func()) {
	// No-op
}

func (m *MockSettingsManager) RemoveSettingChangedCallback(settingName string) {
	// No-op
}

func (m *MockSettingsManager) SetRefreshFlag(settingName string) {
	// No-op
}

func (m *MockSettingsManager) UnsetRefreshFlag(settingName string) {
	// No-op
}

func (m *MockSettingsManager) RegisterRefreshFunc(refreshFunc func()) {
	m.refreshFuncs = append(m.refreshFuncs, refreshFunc)
}

func (m *MockSettingsManager) RegisterOnSettingsSaved(callback func()) {
	// No-op
}

func (m *MockSettingsManager) ShowError(err error) {
	// No-op
}

func (m *MockSettingsManager) ShowConfirm(title, message string, callback func(bool)) {
	// No-op
}

func (m *MockSettingsManager) GetCheckAndEnableApplyFunc() func() {
	return func() {}
}

func (m *MockSettingsManager) SeedBaseline(name string, val interface{}) {
	// No-op
}

func (m *MockSettingsManager) GetBaseline(name string) interface{} {
	return nil
}

func (m *MockSettingsManager) GetValue(name string) interface{} {
	if w, ok := m.allWidgets[name]; ok {
		if c, ok := w.(*widget.Check); ok {
			return c.Checked
		}
		if s, ok := w.(*widget.Select); ok {
			for i, opt := range s.Options {
				if opt == s.Selected {
					return i
				}
			}
			return -1
		}
	}
	return nil
}

func (m *MockSettingsManager) SetValue(name string, val interface{}) {
	if w, ok := m.allWidgets[name]; ok {
		if c, ok := w.(*widget.Check); ok {
			if b, ok := val.(bool); ok {
				c.SetChecked(b)
				m.refreshWidgetStates()
			}
		}
		if s, ok := w.(*widget.Select); ok {
			if str, ok := val.(string); ok {
				s.SetSelected(str)
				m.refreshWidgetStates()
			} else if idx, ok := val.(int); ok {
				if idx >= 0 && idx < len(s.Options) {
					s.SetSelected(s.Options[idx])
					m.refreshWidgetStates()
				}
			}
		}
	}
}

func (m *MockSettingsManager) HasPendingChange(name string) bool {
	return m.pendingChanges[name]
}

func (m *MockSettingsManager) SetPendingChange(name string, pending bool) {
	m.pendingChanges[name] = pending
}

func (m *MockSettingsManager) Refresh() {
	m.refreshWidgetStates()
	for _, f := range m.refreshFuncs {
		f()
	}
}

func (m *MockSettingsManager) RefreshUI() {
	m.refreshWidgetStates()
}

func (m *MockSettingsManager) CommitSetting(name string) {
	m.pendingChanges[name] = false
}

func (m *MockSettingsManager) ResetSettings(resets ...setting.SettingReset) {
	for _, r := range resets {
		m.SetValue(r.Name, r.Value)
		m.pendingChanges[r.Name] = false
	}
}

func (m *MockSettingsManager) SetSettingStatus(name string, message string, importance schema.Importance) {
	m.statusLabels[name] = message
}

func (m *MockSettingsManager) OpenURL(u string) {
	m.Called(u)
}

func (m *MockSettingsManager) ShowAddQueryDialog(cfg schema.AddQueryConfig, initialURL, initialDesc string, onAdded func()) {
	m.Called(cfg, initialURL, initialDesc, onAdded)
}

func (m *MockSettingsManager) RenderSchema(panel schema.PanelSchema) fyne.CanvasObject {
	box := container.NewVBox()
	for _, s := range panel.Sections {
		for _, item := range s.Items {
			switch v := item.(type) {
			case schema.BoolItem:
				check := widget.NewCheck(v.Label, v.OnChanged)
				check.SetChecked(v.InitialValue)
				m.allWidgets[v.Name] = check
				if v.EnabledIf != nil || v.VisibleIf != nil {
					m.managedWidgets = append(m.managedWidgets, managedMockWidget{
						widget:    check,
						enabledIf: v.EnabledIf,
						visibleIf: v.VisibleIf,
					})
				}
				box.Add(check)
			case schema.SelectItem:
				sel := widget.NewSelect(v.Options, nil)
				if idx, ok := v.InitialValue.(int); ok && idx >= 0 && idx < len(v.Options) {
					sel.SetSelected(v.Options[idx])
				}
				m.allWidgets[v.Name] = sel
				if v.EnabledIf != nil || v.VisibleIf != nil {
					m.managedWidgets = append(m.managedWidgets, managedMockWidget{
						widget:    sel,
						enabledIf: v.EnabledIf,
						visibleIf: v.VisibleIf,
					})
				}
				box.Add(sel)
			case schema.ButtonItem:
				btn := widget.NewButton(v.ButtonText, v.OnPressed)
				m.allWidgets[v.Name] = btn
				if v.EnabledIf != nil || v.VisibleIf != nil {
					m.managedWidgets = append(m.managedWidgets, managedMockWidget{
						widget:    btn,
						enabledIf: v.EnabledIf,
						visibleIf: v.VisibleIf,
					})
				}
				box.Add(btn)
			case schema.AsyncButtonItem:
				btn := widget.NewButton(v.ButtonText, nil)
				m.allWidgets[v.Name] = btn
				if v.EnabledIf != nil || v.VisibleIf != nil {
					m.managedWidgets = append(m.managedWidgets, managedMockWidget{
						widget:    btn,
						enabledIf: v.EnabledIf,
						visibleIf: v.VisibleIf,
					})
				}
				box.Add(btn)
			case schema.TextItem:
				entry := widget.NewEntry()
				entry.SetText(v.InitialValue)
				m.allWidgets[v.Name] = entry
				if v.EnabledIf != nil || v.VisibleIf != nil {
					m.managedWidgets = append(m.managedWidgets, managedMockWidget{
						widget:    entry,
						enabledIf: v.EnabledIf,
						visibleIf: v.VisibleIf,
					})
				}
				box.Add(entry)
			case schema.HorizontalRowItem:
				m.RenderSchema(schema.PanelSchema{
					Sections: []schema.SectionSchema{{Items: v.Items}},
				})
			case *schema.HorizontalRowItem:
				m.RenderSchema(schema.PanelSchema{
					Sections: []schema.SectionSchema{{Items: v.Items}},
				})
			}
		}
	}
	return box
}

func (m *MockSettingsManager) RenderTabs(tabs schema.TabsSchema) fyne.CanvasObject {
	box := container.NewVBox()
	for _, item := range tabs.Items {
		if item.Accordion != nil {
			box.Add(m.RenderAccordion(*item.Accordion))
		}
		if item.Panel != nil {
			box.Add(m.RenderSchema(*item.Panel))
		}
		if item.Sources != nil {
			for _, g := range item.Sources.Groups {
				for _, src := range g.Items {
					if src.Detail != nil {
						box.Add(m.RenderSchema(*src.Detail))
					}
				}
			}
		}
	}
	return box
}

func (m *MockSettingsManager) RenderAccordion(acc schema.AccordionSchema) fyne.CanvasObject {
	box := container.NewVBox()
	for _, item := range acc.Items {
		if item.Content != nil {
			box.Add(m.RenderSchema(*item.Content))
		}
	}
	return box
}

func (m *MockSettingsManager) refreshWidgetStates() {
	for _, mw := range m.managedWidgets {
		if mw.visibleIf != nil {
			if mw.visibleIf() {
				mw.widget.Show()
			} else {
				mw.widget.Hide()
			}
		}
		if mw.enabledIf != nil {
			if d, ok := mw.widget.(fyne.Disableable); ok {
				if mw.enabledIf() {
					d.Enable()
				} else {
					d.Disable()
				}
			}
		}
	}
}

// Smart crop has no options left: neither the crop strategy (S2-9, v31) nor Face priority
// (S2-10, v33) is on the page.
func TestSmartCropHasNoOptions(t *testing.T) {
	test.NewApp()

	ResetConfig()
	cfg := GetConfig(NewMockPreferences())
	cfg.SetArtworkDisplayMode(ArtworkDisplaySmartCrop)

	wp := &Plugin{cfg: cfg}
	sm := NewMockSettingsManager()
	sm.RenderTabs(*wp.CreatePrefsSchema(sm))

	assert.NotContains(t, sm.allWidgets, "smartFitMode")
	assert.NotContains(t, sm.allWidgets, "facePriority")
}

// The face setting is fixed at Balanced, Face Crop on and Face Boost off, whatever was saved.
func TestSavedFacePriorityIsReplacedByBalanced(t *testing.T) {
	for _, saved := range [][2]bool{{false, false}, {false, true}, {true, true}} {
		ResetConfig()
		prefs := NewMockPreferences()
		prefs.SetBool(FaceCropPrefKey, saved[0])
		prefs.SetBool(FaceBoostPrefKey, saved[1])

		cfg := GetConfig(prefs)
		assert.True(t, cfg.GetFaceCropEnabled(), "saved %v", saved)
		assert.False(t, cfg.GetFaceBoostEnabled(), "saved %v", saved)
	}
}

func TestFrequencyChoiceIndex(t *testing.T) {
	assert.Equal(t, 0, frequencyIndex(FrequencyNever))
	assert.Equal(t, 2, frequencyIndex(FrequencyHourly))
	assert.Equal(t, len(frequencyChoices), frequencyIndex(Frequency(45)), "Unlisted values map to Custom")
}

func TestSourceSubtitleCountsPendingChanges(t *testing.T) {
	ResetConfig()
	prefs := NewMockPreferences()
	cfg := GetConfig(prefs)

	// Setup: 1 active query for Wallhaven
	cfg.Queries = []ImageQuery{}
	id, _ := cfg.AddImageQuery("Wallhaven Default", "http://wallhaven.cc/1", true)

	wp := &Plugin{cfg: cfg, providers: make(map[string]provider.ImageProvider)}
	p := &MockProvider{IDVal: "Wallhaven", TitleVal: "Wallhaven"}
	wp.providers["Wallhaven"] = p

	sm := NewMockSettingsManager()
	builder := NewPrefsPanelBuilder(wp, sm)
	subtitle := builder.createSubtitleFunc(p)

	assert.Equal(t, "1 on", subtitle())

	// A pending untoggle counts as off until it is reverted.
	sm.SetPendingChange(id, true)
	assert.Equal(t, "None on", subtitle())

	sm.SetPendingChange(id, false)
	assert.Equal(t, "1 on", subtitle())
}

// Minimal MockProvider for testing
type MockProvider struct {
	mock.Mock
	IDVal    string
	TitleVal string
}

func (m *MockProvider) ID() string                                   { return m.IDVal }
func (m *MockProvider) Name() string                                 { return m.IDVal }
func (m *MockProvider) Title() string                                { return m.TitleVal }
func (m *MockProvider) Type() provider.ProviderType                  { return provider.TypeCommunity }
func (m *MockProvider) GetProviderIcon() interface{}                 { return nil }
func (m *MockProvider) GetAttributionType() provider.AttributionType { return provider.AttributionBy }
func (m *MockProvider) ParseURL(url string) (string, error)          { return "", nil }
func (m *MockProvider) CreateSettingsPanel(sm setting.SettingsManager) *schema.PanelSchema {
	return nil
}
func (m *MockProvider) CreateQueryPanel(sm setting.SettingsManager, pendingURL string) *schema.PanelSchema {
	return nil
}
func (m *MockProvider) FetchImages(ctx context.Context, apiURL string, page int) ([]provider.Image, error) {
	return nil, nil
}
func (m *MockProvider) EnrichImage(ctx context.Context, img provider.Image) (provider.Image, error) {
	return img, nil
}
func (m *MockProvider) SupportsUserQueries() bool { return true }
func (m *MockProvider) HomeURL() string           { return "" }
