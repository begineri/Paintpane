package ui

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/dixieflatline76/Spice/v2/asset"
	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/setting"
	utilLog "github.com/dixieflatline76/Spice/v2/util/log"
)

// SettingsManager handles UI elements for settings.
type SettingsManager struct {
	chgPrefsCallbacks   map[string]func()
	refreshFlags        map[string]bool
	refreshFuncs        []func()
	registry            map[string]interface{}
	valueGetters        map[string]func() interface{}
	checkAndEnableApply func()
	applyButton         *widget.Button
	prefsWindow         fyne.Window
	onSettingsSaved     []func()
	managedWidgets      []managedWidget // Track widgets with EnabledIf conditions
	allWidgets          map[string]fyne.CanvasObject
	statusLabels        map[string]*widget.Label     // NEW: Maps setting names to their status labels
	applyFuncs          map[string]func(interface{}) // Type-safe wrappers for native apply functions
	isRenderingCompact  bool                         // Tracks if the current section being rendered is compact
	assetMgr            *asset.Manager

	// Changes apply on their own after applyDelay without further edits. The
	// Apply button is no longer shown; it remains for tests and callers that
	// want to apply at once.
	applyDelay time.Duration
	applyTimer *time.Timer

	// valueWatchers run when a boolean setting's switch changes, e.g. to keep a
	// Select all switch in step with its members.
	valueWatchers map[string][]func()
	// afterRender runs once a panel is fully rendered, for items that depend on
	// items rendered after them.
	afterRender []func()

	// sourcesShowList returns the Sources page from a source's detail to the list.
	sourcesShowList func()
	// subPages shows pages inside a settings page, such as the blocked images.
	subPages subPages
}

// defaultApplyDelay is how long settings wait after the last change before they
// apply, so typing in a field or flicking through options applies once.
const defaultApplyDelay = 800 * time.Millisecond

type managedWidget struct {
	widget    fyne.CanvasObject
	enabledIf func() bool
	visibleIf func() bool
}

type selectConfig struct {
	Name         string
	Options      []string
	Segmented    bool
	Swatches     bool
	InitialValue interface{}
	Label        fyne.CanvasObject
	HelpContent  fyne.CanvasObject
	OnChanged    func(string, interface{})
	ApplyFunc    func(interface{})
	NeedsRefresh bool
	EnabledIf    func() bool
	VisibleIf    func() bool
}

type boolConfig struct {
	Name         string
	InitialValue bool
	LabelText    string
	Note         string // muted text after the label
	ActionIcon   string // icon of an action button left of the switch
	ActionFunc   func()
	Label        fyne.CanvasObject
	HelpContent  fyne.CanvasObject
	OnChanged    func(bool)
	ApplyFunc    func(bool)
	NeedsRefresh bool
	EnabledIf    func() bool
	VisibleIf    func() bool
}

type textEntrySettingConfig struct {
	Name               string
	InitialValue       string
	PlaceHolder        string
	Label              fyne.CanvasObject
	HelpContent        fyne.CanvasObject
	Validator          fyne.StringValidator
	OnChanged          func(string)
	PostValidateCheck  func(string) error
	ApplyFunc          func(string)
	NeedsRefresh       bool
	SkipApply          bool
	DisplayStatus      bool
	IsPassword         bool
	IsNumeric          bool
	EnabledIf          func() bool
	VisibleIf          func() bool
	ValidationDebounce time.Duration
}

type buttonWithConfirmationConfig struct {
	Name           string
	Label          fyne.CanvasObject
	HelpContent    fyne.CanvasObject
	ButtonText     string
	ConfirmTitle   string
	ConfirmMessage string
	// ConfirmButtonText is what the dialog's confirming button says; ConfirmDanger fills it red.
	ConfirmButtonText  string
	ConfirmDanger      bool
	ConfirmMessageFunc func() string
	HelpAsync          func(set func(string))
	Importance         widget.Importance
	OnPressed          func()
	IconName           string
	EnabledIf          func() bool
	VisibleIf          func() bool
}

type asyncButtonConfig struct {
	Name            string
	ButtonText      string
	LoadingText     string
	Importance      widget.Importance
	OnPressed       func() error
	OnCompleted     func(error)
	TargetStatusKey string
	NeedsRefresh    bool
	IconName        string
	EnabledIf       func() bool
	VisibleIf       func() bool
}

type buttonConfig struct {
	Name        string
	Label       fyne.CanvasObject
	HelpContent fyne.CanvasObject
	ButtonText  string
	Importance  widget.Importance
	OnPressed   func()
	IconName    string
	EnabledIf   func() bool
	VisibleIf   func() bool
}

// NewSettingsManager creates a new SettingsManager.
func NewSettingsManager(window fyne.Window) setting.SettingsManager {
	cpcs := make(map[string]func())
	rns := make(map[string]bool)
	rfs := make([]func(), 0)

	sm := &SettingsManager{
		chgPrefsCallbacks:   cpcs,
		refreshFlags:        rns,
		refreshFuncs:        rfs,
		registry:            make(map[string]interface{}),
		valueGetters:        make(map[string]func() interface{}),
		checkAndEnableApply: nil,
		applyButton:         nil,
		prefsWindow:         window,
		onSettingsSaved:     make([]func(), 0),
		managedWidgets:      make([]managedWidget, 0),
		allWidgets:          make(map[string]fyne.CanvasObject),
		statusLabels:        make(map[string]*widget.Label),
		applyFuncs:          make(map[string]func(interface{})),
		assetMgr:            asset.NewManager(),
		applyDelay:          defaultApplyDelay,
		valueWatchers:       make(map[string][]func()),
	}

	sm.applyButton = createApplyButton(sm)
	sm.checkAndEnableApply = func() {
		// Evaluate UI dependencies first
		sm.refreshWidgetStates()

		if sm.hasPending() {
			sm.applyButton.Enable() // Enable if changes or refresh needed
			sm.scheduleApply()
		} else {
			sm.applyButton.Disable() // Otherwise, disable
		}
		sm.applyButton.Refresh()
	}

	return sm
}

// SeedBaseline seeds the initial state for a setting to track changes.
func (sm *SettingsManager) SeedBaseline(name string, val interface{}) {
	sm.registry[name] = val
}

// OpenURL opens the specified URL in the system's default browser.
func (sm *SettingsManager) OpenURL(u string) {
	if parsed, err := url.Parse(u); err == nil {
		_ = fyne.CurrentApp().OpenURL(parsed)
	}
}

// refreshWidgetStates evaluates all EnabledIf and VisibleIf conditions and updates widget states.
func (sm *SettingsManager) refreshWidgetStates() {
	for _, mw := range sm.managedWidgets {
		// Handle Visibility
		if mw.visibleIf != nil {
			if mw.visibleIf() {
				mw.widget.Show()
			} else {
				mw.widget.Hide()
			}
		}

		// Handle Enabling (requires fyne.Disableable)
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

// GetValue returns the live/current value for a setting from its valueGetter.
func (sm *SettingsManager) GetValue(name string) interface{} {
	val, ok := sm.valueGetters[name]
	if !ok || val == nil {
		return nil
	}
	return val()
}

// SetValue programmatically updates the live value of a setting.
func (sm *SettingsManager) SetValue(name string, val interface{}) {
	w, ok := sm.allWidgets[name]
	if !ok {
		return
	}

	fyne.Do(func() {
		switch v := w.(type) {
		case *widget.Check:
			if b, ok := val.(bool); ok {
				v.SetChecked(b)
				v.Refresh()
			}
		case *toggleSwitch:
			if b, ok := val.(bool); ok {
				v.SetChecked(b)
			}
		case *segmented:
			if i, ok := val.(int); ok {
				v.SetSelectedIndex(i)
			}
		case *widget.Entry:
			if s, ok := val.(string); ok {
				v.SetText(s)
				v.Refresh()
			}
		case *widget.Select:
			if i, ok := val.(int); ok {
				v.SetSelectedIndex(i)
				v.Refresh()
			} else if s, ok := val.(string); ok {
				v.SetSelected(s)
				v.Refresh()
			}
		}

		sm.refreshWidgetStates()
	})
}

// GetBaseline returns the initial state for a setting.
func (sm *SettingsManager) GetBaseline(name string) interface{} {
	return sm.registry[name]
}

// HasPendingChange returns true if the user has toggled a setting but not yet applied.
func (sm *SettingsManager) HasPendingChange(name string) bool {
	_, exists := sm.chgPrefsCallbacks[name]
	return exists
}

// createApplyButton creates the (hidden) Apply button, which applies pending changes at once.
func createApplyButton(sm *SettingsManager) *widget.Button {
	applyButton := widget.NewButton(i18n.T("Apply changes"), func() {
		sm.applyPending()
	})
	applyButton.Disable()
	return applyButton
}

func (sm *SettingsManager) hasPending() bool {
	return len(sm.refreshFlags) > 0 || len(sm.chgPrefsCallbacks) > 0
}

// scheduleApply (re)starts the timer that applies pending changes. Each new
// change restarts it, so a burst of edits applies once.
func (sm *SettingsManager) scheduleApply() {
	if sm.applyTimer != nil {
		sm.applyTimer.Stop()
	}
	sm.applyTimer = time.AfterFunc(sm.applyDelay, func() {
		fyne.Do(sm.applyPending)
	})
}

// FlushPending applies pending changes now. Call it before the window closes so
// a change made in the last moment is not lost.
func (sm *SettingsManager) FlushPending() {
	if sm.applyTimer != nil {
		sm.applyTimer.Stop()
		sm.applyTimer = nil
	}
	if sm.hasPending() {
		sm.applyPending()
	}
}

// applyPending runs the apply callbacks of every changed setting, moves their
// baselines, then refreshes once if any of them asked for it. It must run on
// the Fyne thread.
func (sm *SettingsManager) applyPending() {
	if !sm.hasPending() {
		return
	}
	if len(sm.chgPrefsCallbacks) > 0 {
		for name, callback := range sm.chgPrefsCallbacks {
			callback()
			// Update registry baseline so subsequent changes are compared to NEW state
			if getter, ok := sm.valueGetters[name]; ok {
				sm.registry[name] = getter()
			}
		}
		sm.chgPrefsCallbacks = make(map[string]func())
	}

	// Conditional refresh based on dirty flags (evaluated during ApplyFuncs above)
	if len(sm.refreshFlags) > 0 {
		sm.fullRefresh()
		sm.refreshFlags = make(map[string]bool)
	}

	// Notify listeners that settings have been saved (e.g. refresh counters)
	for _, cb := range sm.onSettingsSaved {
		cb()
	}

	if sm.applyButton != nil {
		sm.applyButton.Disable()
		sm.applyButton.Refresh()
	}
}

// GetApplySettingsButton returns the Apply Changes button from the SettingsManager to be used in the UI.
func (sm *SettingsManager) GetApplySettingsButton() *widget.Button {
	return sm.applyButton
}

// renderSelectSetting creates a reusable select widget.
func (sm *SettingsManager) renderSliderSetting(cfg *schema.SliderItem, header *fyne.Container) {
	slider := widget.NewSlider(cfg.Min, cfg.Max)
	slider.Step = cfg.Step
	slider.SetValue(cfg.InitialValue)

	valLabel := widget.NewLabel(fmt.Sprintf(cfg.Format, cfg.InitialValue))

	slider.OnChanged = func(f float64) {
		valLabel.SetText(fmt.Sprintf(cfg.Format, f))
	}

	sm.registry[cfg.Name] = cfg.InitialValue

	if cfg.ApplyFunc != nil {
		sm.applyFuncs[cfg.Name] = func(val interface{}) {
			cfg.ApplyFunc(val.(float64))
		}
	}

	slider.OnChangeEnded = func(f float64) {
		if f != sm.registry[cfg.Name].(float64) {
			applyFunc := cfg.ApplyFunc
			needsRefresh := cfg.NeedsRefresh
			settingName := cfg.Name
			sm.chgPrefsCallbacks[settingName] = func() {
				if applyFunc != nil {
					applyFunc(f)
				}
				if needsRefresh {
					sm.refreshFlags[settingName] = true
				}
			}
		} else {
			delete(sm.chgPrefsCallbacks, cfg.Name)
		}
		if cfg.OnChanged != nil {
			cfg.OnChanged(f)
		}
		sm.checkAndEnableApply()
	}

	sm.valueGetters[cfg.Name] = func() interface{} {
		return slider.Value
	}

	control := container.NewBorder(nil, nil, nil, valLabel, slider)

	var help fyne.CanvasObject
	if cfg.Help != "" {
		help = sm.CreateSettingDescriptionLabel(cfg.Help)
	}
	var row fyne.CanvasObject
	if cfg.Label != "" {
		row = addRow(header, newSettingRow(sm.CreateSettingTitleLabel(cfg.Label), help, control, true))
	} else {
		row = addRow(header, control, help)
	}

	if cfg.EnabledIf != nil {
		sm.managedWidgets = append(sm.managedWidgets, managedWidget{widget: slider, enabledIf: cfg.EnabledIf})
	}
	if cfg.VisibleIf != nil {
		sm.managedWidgets = append(sm.managedWidgets, managedWidget{widget: row, visibleIf: cfg.VisibleIf})
		sm.managedWidgets = append(sm.managedWidgets, managedWidget{widget: slider, visibleIf: cfg.VisibleIf})
	}
	sm.allWidgets[cfg.Name] = slider
}

func (sm *SettingsManager) renderSelectSetting(cfg *selectConfig, header *fyne.Container) {
	if cfg.Swatches {
		sm.renderSwatchSetting(cfg, header)
		return
	}
	if cfg.Segmented {
		sm.renderSegmentedSetting(cfg, header)
		return
	}
	selectWidget := widget.NewSelect(cfg.Options, func(selected string) {})
	selectWidget.SetSelectedIndex(cfg.InitialValue.(int))
	sm.registry[cfg.Name] = cfg.InitialValue.(int)
	sm.applyFuncs[cfg.Name] = func(val interface{}) {
		switch v := val.(type) {
		case int:
			cfg.ApplyFunc(v)
		case string:
			for i, opt := range cfg.Options {
				if opt == v {
					cfg.ApplyFunc(i)
					return
				}
			}
		}
	}
	sm.valueGetters[cfg.Name] = func() interface{} {
		return selectWidget.SelectedIndex()
	}

	var row fyne.CanvasObject
	if hasLabel(cfg.Label) {
		row = addRow(header, newSettingRow(cfg.Label, cfg.HelpContent, newBorderedSelect(selectWidget), true))
	} else {
		row = addRow(header, newBorderedSelect(selectWidget), cfg.HelpContent)
	}

	selectWidget.OnChanged = func(s string) {
		options := cfg.Options
		selectedIndex := -1
		for i, opt := range options {
			if opt == s { // Assuming cfg.Options are strings
				selectedIndex = i
				break
			}
		}

		if selectedIndex != sm.registry[cfg.Name].(int) {
			sm.chgPrefsCallbacks[cfg.Name] = func() {
				cfg.ApplyFunc(selectedIndex)
				if cfg.NeedsRefresh {
					sm.refreshFlags[cfg.Name] = true
				}
			}
		} else {
			delete(sm.chgPrefsCallbacks, cfg.Name)
		}

		if cfg.OnChanged != nil {
			cfg.OnChanged(s, selectedIndex)
		}

		sm.checkAndEnableApply()
	}

	// Register value getter for dependency tracking
	sm.valueGetters[cfg.Name] = func() interface{} {
		s := selectWidget.Selected
		options := cfg.Options
		for i, opt := range options {
			if opt == s { // Assuming cfg.Options are strings
				return i
			}
		}
		return -1
	}

	if cfg.EnabledIf != nil {
		sm.managedWidgets = append(sm.managedWidgets, managedWidget{widget: selectWidget, enabledIf: cfg.EnabledIf})
	}
	if cfg.VisibleIf != nil {
		sm.managedWidgets = append(sm.managedWidgets, managedWidget{widget: row, visibleIf: cfg.VisibleIf})
		sm.managedWidgets = append(sm.managedWidgets, managedWidget{widget: selectWidget, visibleIf: cfg.VisibleIf})
	}

	sm.allWidgets[cfg.Name] = selectWidget
}

// renderSegmentedSetting renders a select setting as a segmented control.
// renderSwatchSetting renders a choice among colours as a row of round swatches.
func (sm *SettingsManager) renderSwatchSetting(cfg *selectConfig, header *fyne.Container) {
	initial, _ := cfg.InitialValue.(int)
	picker := newSwatchPicker(cfg.Options, initial, nil)
	sm.registry[cfg.Name] = initial
	sm.applyFuncs[cfg.Name] = func(val interface{}) {
		if i, ok := val.(int); ok {
			cfg.ApplyFunc(i)
		}
	}
	sm.valueGetters[cfg.Name] = func() interface{} { return picker.Selected }
	row := addRow(header, newSettingRow(cfg.Label, cfg.HelpContent, picker, false))
	picker.OnChanged = func(i int) {
		if i != sm.registry[cfg.Name].(int) {
			sm.chgPrefsCallbacks[cfg.Name] = func() { cfg.ApplyFunc(i) }
		} else {
			delete(sm.chgPrefsCallbacks, cfg.Name)
		}
		if cfg.OnChanged != nil && i >= 0 && i < len(cfg.Options) {
			cfg.OnChanged(cfg.Options[i], i)
		}
		sm.checkAndEnableApply()
	}
	if cfg.VisibleIf != nil {
		sm.managedWidgets = append(sm.managedWidgets, managedWidget{widget: row, visibleIf: cfg.VisibleIf})
	}
	sm.allWidgets[cfg.Name] = picker
}

func (sm *SettingsManager) renderSegmentedSetting(cfg *selectConfig, header *fyne.Container) {
	initial, _ := cfg.InitialValue.(int)
	seg := newSegmented(cfg.Options, initial, nil)
	sm.registry[cfg.Name] = initial
	sm.applyFuncs[cfg.Name] = func(val interface{}) {
		if i, ok := val.(int); ok {
			cfg.ApplyFunc(i)
		}
	}
	sm.valueGetters[cfg.Name] = func() interface{} {
		return seg.Selected
	}

	var row fyne.CanvasObject
	if hasLabel(cfg.Label) {
		row = addRow(header, newSettingRow(cfg.Label, cfg.HelpContent, seg, false))
	} else {
		row = addRow(header, hug(seg), cfg.HelpContent)
	}

	seg.OnChanged = func(i int) {
		if i != sm.registry[cfg.Name].(int) {
			sm.chgPrefsCallbacks[cfg.Name] = func() {
				cfg.ApplyFunc(i)
				if cfg.NeedsRefresh {
					sm.refreshFlags[cfg.Name] = true
				}
			}
		} else {
			delete(sm.chgPrefsCallbacks, cfg.Name)
		}
		if cfg.OnChanged != nil && i >= 0 && i < len(cfg.Options) {
			cfg.OnChanged(cfg.Options[i], i)
		}
		sm.checkAndEnableApply()
	}

	if cfg.EnabledIf != nil {
		sm.managedWidgets = append(sm.managedWidgets, managedWidget{widget: seg, enabledIf: cfg.EnabledIf})
	}
	if cfg.VisibleIf != nil {
		sm.managedWidgets = append(sm.managedWidgets, managedWidget{widget: row, visibleIf: cfg.VisibleIf})
		sm.managedWidgets = append(sm.managedWidgets, managedWidget{widget: seg, visibleIf: cfg.VisibleIf})
	}
	sm.allWidgets[cfg.Name] = seg
}

// renderSelectAllHeader renders a section label with a count of the members that
// are on ("Curated Collections · 2/7 on") and, at its right, a Select all switch
// that turns every member on or off. The switch shows on only while every member
// is on. It has the row switches' size and lines up with them (G5).
func (sm *SettingsManager) renderSelectAllHeader(title string, v schema.SelectAllItem) fyne.CanvasObject {
	countOn := func() int {
		n := 0
		for _, m := range v.Members {
			if on, ok := sm.GetValue(m).(bool); ok && on {
				n++
			}
		}
		return n
	}

	title = strings.TrimSpace(title)
	titleText := newSectionLabel(title)
	setTitle := func(on int) {
		text := i18n.Tf("{{.On}}/{{.Total}} on", map[string]any{"On": on, "Total": len(v.Members)})
		if title != "" {
			text = title + " · " + text
		}
		titleText.SetText(text)
	}

	sw := newToggleSwitch(false, nil)
	syncing := false // set while the switch follows its members, so it does not drive them back
	sync := func() {
		on := countOn()
		setTitle(on)
		syncing = true
		sw.SetChecked(len(v.Members) > 0 && on == len(v.Members))
		syncing = false
	}
	sw.OnChanged = func(on bool) {
		if syncing {
			return
		}
		for _, m := range v.Members {
			sm.SetValue(m, on)
		}
		sync()
	}
	for _, m := range v.Members {
		sm.watchValue(m, sync)
	}
	// The members are rendered after the header, so read them once the panel is done.
	sm.afterRender = append(sm.afterRender, sync)
	sm.allWidgets[v.Name] = sw

	allLabel := newStyledText(v.Label, sectionLabelSize, false, theme.ColorNamePlaceHolder)
	// The right inset is the card's, so the switch sits above the row switches.
	right := inset(container.NewHBox(container.NewCenter(allLabel), container.NewCenter(sw)), 0, cardInsetX, 0, 0)
	return container.NewBorder(nil, nil, nil, right, vcenter(titleText))
}

// watchValue runs fn whenever the named boolean setting changes in the UI.
func (sm *SettingsManager) watchValue(name string, fn func()) {
	sm.valueWatchers[name] = append(sm.valueWatchers[name], fn)
}

func (sm *SettingsManager) notifyValueChanged(name string) {
	for _, fn := range sm.valueWatchers[name] {
		fn()
	}
}

// renderBoolSetting renders a boolean setting as a row with a switch.
func (sm *SettingsManager) renderBoolSetting(cfg *boolConfig, header *fyne.Container) *toggleSwitch {
	label := cfg.Label
	if !hasLabel(label) {
		label = nil
		if cfg.LabelText != "" {
			label = sm.CreateSettingTitleLabel(cfg.LabelText)
		}
	}
	if cfg.Note != "" && cfg.LabelText != "" {
		label = newLabelWithNote(cfg.LabelText, cfg.Note)
	}

	sw := newToggleSwitch(cfg.InitialValue, nil)
	var control fyne.CanvasObject = sw
	if cfg.ActionIcon != "" && cfg.ActionFunc != nil {
		actionBtn := widget.NewButtonWithIcon("", actionIconResource(cfg.ActionIcon), cfg.ActionFunc)
		actionBtn.Importance = widget.LowImportance
		control = container.NewHBox(sizedControl(actionBtn), sw)
	}
	sm.registry[cfg.Name] = cfg.InitialValue
	if cfg.ApplyFunc != nil {
		sm.applyFuncs[cfg.Name] = func(val interface{}) {
			cfg.ApplyFunc(val.(bool))
		}
	}

	row := addRow(header, newSettingRow(label, cfg.HelpContent, control, false))

	sw.OnChanged = func(b bool) {
		if b != sm.registry[cfg.Name].(bool) {
			sm.chgPrefsCallbacks[cfg.Name] = func() {
				if cfg.ApplyFunc != nil {
					cfg.ApplyFunc(b)
				}
				if cfg.NeedsRefresh {
					sm.refreshFlags[cfg.Name] = true
				}
			}
		} else {
			delete(sm.chgPrefsCallbacks, cfg.Name)
		}

		if cfg.OnChanged != nil {
			cfg.OnChanged(b)
		}

		sm.checkAndEnableApply()
		sm.notifyValueChanged(cfg.Name)
	}

	sm.valueGetters[cfg.Name] = func() interface{} {
		return sw.Checked
	}

	if cfg.EnabledIf != nil {
		sm.managedWidgets = append(sm.managedWidgets, managedWidget{widget: sw, enabledIf: cfg.EnabledIf})
	}
	if cfg.VisibleIf != nil {
		sm.managedWidgets = append(sm.managedWidgets, managedWidget{widget: row, visibleIf: cfg.VisibleIf})
		sm.managedWidgets = append(sm.managedWidgets, managedWidget{widget: sw, visibleIf: cfg.VisibleIf})
	}

	sm.allWidgets[cfg.Name] = sw
	return sw
}

// renderTextEntrySetting creates a reusable text entry setting.
func (sm *SettingsManager) renderTextEntrySetting(cfg *textEntrySettingConfig, header *fyne.Container) *widget.Entry {
	entry := widget.NewEntry()
	entry.SetPlaceHolder(cfg.PlaceHolder)
	entry.SetText(cfg.InitialValue)
	sm.registry[cfg.Name] = cfg.InitialValue
	sm.applyFuncs[cfg.Name] = func(val interface{}) {
		cfg.ApplyFunc(val.(string))
	}
	sm.valueGetters[cfg.Name] = func() interface{} {
		return entry.Text
	}
	sm.allWidgets[cfg.Name] = entry

	if cfg.IsPassword {
		entry.Password = true
	}

	if cfg.Validator != nil {
		entry.Validator = cfg.Validator
	}

	statusLabel := widget.NewLabel("")
	statusLabel.Wrapping = fyne.TextWrapWord
	statusLabel.SizeName = theme.SizeNameCaptionText
	sm.statusLabels[cfg.Name] = statusLabel

	// The status line sits under the help text and only takes space when it has text.
	status := collapseWhenEmpty(statusLabel)
	helpBlock := status
	if cfg.HelpContent != nil {
		helpBlock = container.New(&tightVBox{}, cfg.HelpContent, status)
	}

	var row fyne.CanvasObject
	if hasLabel(cfg.Label) {
		row = addRow(header, newSettingRow(cfg.Label, helpBlock, entry, true))
	} else {
		row = addRow(header, entry, helpBlock)
	}

	var debounceTimer *time.Timer
	entry.OnChanged = func(s string) {
		if cfg.IsNumeric {
			var clean []rune
			for _, r := range s {
				if r >= '0' && r <= '9' {
					clean = append(clean, r)
				}
			}
			for len(clean) > 1 && clean[0] == '0' {
				clean = clean[1:]
			}
			cleanStr := string(clean)
			if s != cleanStr {
				entry.SetText(cleanStr)
				return
			}
		}

		sm.handleTextEntryChanged(s, cfg, entry, statusLabel, &debounceTimer)
	}

	if cfg.EnabledIf != nil {
		sm.managedWidgets = append(sm.managedWidgets, managedWidget{widget: entry, enabledIf: cfg.EnabledIf})
	}
	if cfg.VisibleIf != nil {
		sm.managedWidgets = append(sm.managedWidgets, managedWidget{widget: row, visibleIf: cfg.VisibleIf})
		sm.managedWidgets = append(sm.managedWidgets, managedWidget{widget: entry, visibleIf: cfg.VisibleIf})
	}

	return entry
}

func (sm *SettingsManager) handleTextEntryChanged(s string, cfg *textEntrySettingConfig, entry *widget.Entry, statusLabel *widget.Label, debounceTimer **time.Timer) {
	if *debounceTimer != nil {
		(*debounceTimer).Stop()
	}

	var entryErr error
	if cfg.Validator != nil {
		entryErr = entry.Validate()
	}

	if entryErr != nil {
		statusLabel.SetText(entryErr.Error())
		statusLabel.Importance = widget.DangerImportance
		sm.RemoveSettingChangedCallback(cfg.Name)
		if cfg.NeedsRefresh {
			sm.UnsetRefreshFlag(cfg.Name)
		}
		sm.GetCheckAndEnableApplyFunc()()
		statusLabel.Refresh()
		if cfg.OnChanged != nil {
			cfg.OnChanged(s)
		}
		return
	}

	// Validation passed (or none), clear status and importance while waiting/running post-check
	statusLabel.SetText("")
	statusLabel.Importance = widget.LowImportance
	statusLabel.Refresh()

	runPostCheck := func(val string) {
		sm.runTextEntryPostCheck(val, cfg, entry, statusLabel)
	}

	if cfg.ValidationDebounce > 0 {
		*debounceTimer = time.AfterFunc(cfg.ValidationDebounce, func() {
			runPostCheck(s)
		})
	} else {
		runPostCheck(s)
	}

	if cfg.OnChanged != nil {
		cfg.OnChanged(s)
	}
}

func (sm *SettingsManager) runTextEntryPostCheck(val string, cfg *textEntrySettingConfig, entry *widget.Entry, statusLabel *widget.Label) {
	var postErr error
	if cfg.PostValidateCheck != nil {
		postErr = cfg.PostValidateCheck(val)
	}

	fyne.Do(func() {
		// Ensure the value hasn't changed since we started this check
		if entry.Text != val {
			return
		}

		if postErr != nil {
			statusLabel.SetText(postErr.Error())
			statusLabel.Importance = widget.DangerImportance
			sm.RemoveSettingChangedCallback(cfg.Name)
			if cfg.NeedsRefresh {
				sm.UnsetRefreshFlag(cfg.Name)
			}
		} else {
			if cfg.DisplayStatus && val != "" {
				statusLabel.SetText(fmt.Sprintf("%s OK", cfg.Name))
				statusLabel.Importance = widget.SuccessImportance
			} else {
				statusLabel.SetText("")
				statusLabel.Importance = widget.LowImportance
			}

			baseline, _ := sm.registry[cfg.Name].(string)
			if val != baseline && !cfg.SkipApply {
				sm.SetSettingChangedCallback(cfg.Name, func() {
					enteredTxt := entry.Text
					if enteredTxt != sm.registry[cfg.Name].(string) {
						cfg.ApplyFunc(enteredTxt)
					}
				})
				if cfg.NeedsRefresh {
					sm.SetRefreshFlag(cfg.Name)
				}
			} else {
				sm.RemoveSettingChangedCallback(cfg.Name)
				if cfg.NeedsRefresh {
					sm.UnsetRefreshFlag(cfg.Name)
				}
			}
		}
		statusLabel.Refresh()
		sm.GetCheckAndEnableApplyFunc()()
	})
}

// renderButtonWithConfirmationSetting renders a button that asks for confirmation first.
func (sm *SettingsManager) renderButtonWithConfirmationSetting(cfg *buttonWithConfirmationConfig, header *fyne.Container) {
	onTap := func() {
		message := cfg.ConfirmMessage
		if cfg.ConfirmMessageFunc != nil {
			message = cfg.ConfirmMessageFunc()
		}
		if cfg.ConfirmTitle != "" && message != "" {
			showConfirmDialog(sm.prefsWindow, confirmSpec{
				Title:       cfg.ConfirmTitle,
				Message:     message,
				ConfirmText: cfg.ConfirmButtonText,
				Danger:      cfg.ConfirmDanger,
			}, func(b bool) {
				if b {
					cfg.OnPressed()
				}
			})
		} else {
			cfg.OnPressed()
		}
	}

	// Destructive actions are red text buttons without a fill.
	var button fyne.CanvasObject
	if cfg.Importance == widget.DangerImportance {
		button = newDangerButton(cfg.ButtonText, onTap)
	} else {
		b := widget.NewButtonWithIcon(cfg.ButtonText, sm.getIconResource(cfg.IconName), onTap)
		b.Importance = cfg.Importance
		button = b
	}
	sm.allWidgets[cfg.Name] = button

	help := cfg.HelpContent
	if cfg.HelpAsync != nil && help == nil {
		line := newSecondaryText(i18n.T("Calculating…"))
		help = line
		cfg.HelpAsync(func(text string) {
			onUIThread(func() { setSecondaryText(line, text) })
		})
	}
	row := sm.addButtonRow(header, cfg.Label, help, button)
	sm.manageButton(button, row, cfg.EnabledIf, cfg.VisibleIf)
}

// renderButtonSetting renders a standard button setting.
func (sm *SettingsManager) renderButtonSetting(cfg *buttonConfig, header *fyne.Container) {
	var button fyne.CanvasObject
	if cfg.Importance == widget.DangerImportance { // red text, not Fyne's red fill (v43)
		button = newDangerButton(cfg.ButtonText, cfg.OnPressed)
	} else {
		b := widget.NewButtonWithIcon(cfg.ButtonText, sm.getIconResource(cfg.IconName), cfg.OnPressed)
		b.Importance = cfg.Importance
		button = b
	}

	row := sm.addButtonRow(header, cfg.Label, cfg.HelpContent, button)
	sm.manageButton(button, row, cfg.EnabledIf, cfg.VisibleIf)
}

// addButtonRow puts a labelled button at the right of a setting row, or an
// unlabelled one at its natural width on the left.
func (sm *SettingsManager) addButtonRow(header *fyne.Container, label, help fyne.CanvasObject, button fyne.CanvasObject) fyne.CanvasObject {
	if hasLabel(label) {
		return addRow(header, newSettingRow(label, help, button, false))
	}
	return addRow(header, hug(button), help)
}

func (sm *SettingsManager) manageButton(button fyne.CanvasObject, row fyne.CanvasObject, enabledIf, visibleIf func() bool) {
	if enabledIf != nil {
		sm.managedWidgets = append(sm.managedWidgets, managedWidget{widget: button, enabledIf: enabledIf})
	}
	if visibleIf != nil {
		sm.managedWidgets = append(sm.managedWidgets, managedWidget{widget: row, visibleIf: visibleIf})
		sm.managedWidgets = append(sm.managedWidgets, managedWidget{widget: button, visibleIf: visibleIf})
	}
}

// SetSettingChangedCallback sets a callback function to be called when a setting changes.
func (sm *SettingsManager) SetSettingChangedCallback(settingName string, callback func()) {
	sm.chgPrefsCallbacks[settingName] = callback
}

// RemoveSettingChangedCallback removes a callback function associated with a specific setting.
func (sm *SettingsManager) RemoveSettingChangedCallback(settingName string) {
	delete(sm.chgPrefsCallbacks, settingName)
}

// SetRefreshFlag sets a flag to indicate that a specific setting needs a refresh.
func (sm *SettingsManager) SetRefreshFlag(settingName string) {
	sm.refreshFlags[settingName] = true
}

// UnsetRefreshFlag removes the refresh flag for a specific setting.
func (sm *SettingsManager) UnsetRefreshFlag(settingName string) {
	delete(sm.refreshFlags, settingName)
}

// RegisterOnSettingsSaved registers a function to be called after settings are applied.
func (sm *SettingsManager) RegisterOnSettingsSaved(callback func()) {
	sm.onSettingsSaved = append(sm.onSettingsSaved, callback)
}

// RegisterRefreshFunc registers a function to be called when the settings need to be refreshed.
// This should be done by each plugin that needs to perform some action after settings changes.
// Like refreshing all wallpaper images for example.
func (sm *SettingsManager) RegisterRefreshFunc(refreshFunc func()) {
	sm.refreshFuncs = append(sm.refreshFuncs, refreshFunc)
}

// GetSettingsWindow returns the window associated with the SettingsManager.
func (sm *SettingsManager) GetSettingsWindow() fyne.Window {
	return sm.prefsWindow
}

// GetCheckAndEnableApplyFunc returns the check and enable apply function for the SettingsManager.
func (sm *SettingsManager) GetCheckAndEnableApplyFunc() func() {
	return sm.checkAndEnableApply
}

// Refresh performs a FULL refresh cycle: UI state evaluation, registered callbacks, and widget repaints.
//
// Steps:
//  1. Evaluates all EnabledIf/VisibleIf predicates on managed widgets.
//  2. Runs ALL registered refresh functions (via RegisterRefreshFunc). These may include
//     wallpaper rotation triggers, collection sync operations, accordion title recalculations,
//     and other engine-level side effects.
//  3. Calls .Refresh() on every widget in the allWidgets registry.
//
// Use Refresh() when a state change has been COMMITTED (e.g., after Apply, after sync completes,
// after a username verification resets state). Do NOT use it for pending/uncommitted user interactions
// like checkbox toggles — use RefreshUI() instead to avoid triggering wallpaper changes.
func (sm *SettingsManager) fullRefresh() {
	// 1. Evaluate all UI dependencies (EnabledIf/VisibleIf)
	sm.refreshWidgetStates()

	// 2. Run registered refresh callbacks (may have engine-level side effects)
	for _, rf := range sm.refreshFuncs {
		rf()
	}

	// 3. Repaint all managed widgets
	for _, w := range sm.allWidgets {
		w.Refresh()
	}
}

// RefreshUI performs a UI-ONLY refresh: state evaluation and widget repaints,
// WITHOUT running registered refresh callbacks.
//
// Steps:
//  1. Evaluates all EnabledIf/VisibleIf predicates on managed widgets.
//  2. Calls .Refresh() on every widget in the allWidgets registry.
//
// This is safe to call from interactive handlers (checkbox toggles, text edits)
// because it will NOT trigger engine-level side effects like wallpaper rotation or
// collection syncs. It only updates the visual state of the UI (e.g., enabling/disabling
// dependent fields, updating accordion titles).
func (sm *SettingsManager) RefreshUI() {
	// 1. Evaluate all UI dependencies (EnabledIf/VisibleIf)
	sm.refreshWidgetStates()

	// 2. Repaint all managed widgets (no registered callbacks)
	for _, w := range sm.allWidgets {
		w.Refresh()
	}
}

// renderAsyncButton creates a button that handles background tasks and UI thread transitions.
func (sm *SettingsManager) renderAsyncButton(cfg *asyncButtonConfig, header *fyne.Container) *widget.Button {
	icon := sm.getIconResource(cfg.IconName)
	btn := widget.NewButtonWithIcon(cfg.ButtonText, icon, nil)
	btn.Importance = cfg.Importance

	// Define the handler to capture 'btn' properly
	btn.OnTapped = func() {
		originalText := btn.Text
		btn.Disable()
		sm.applyButton.Disable()
		btn.SetText(cfg.LoadingText)
		btn.Refresh()

		go func() {
			err := cfg.OnPressed()
			fyne.Do(func() {
				btn.Enable()
				btn.SetText(originalText)
				sm.applyButton.Enable()

				// NEW: Automatic framework-managed status update
				if cfg.TargetStatusKey != "" {
					if err != nil {
						sm.SetSettingStatus(cfg.TargetStatusKey, err.Error(), schema.ImportanceDanger)
					} else {
						sm.SetSettingStatus(cfg.TargetStatusKey, i18n.T("Success"), schema.ImportanceSuccess)
					}
				}

				cfg.OnCompleted(err)
				if cfg.NeedsRefresh {
					sm.fullRefresh()
				} else {
					sm.refreshWidgetStates()
				}
				sm.checkAndEnableApply()
			})
		}()
	}

	row := addRow(header, hug(btn))
	sm.allWidgets[cfg.Name] = btn
	sm.manageButton(btn, row, cfg.EnabledIf, cfg.VisibleIf)

	return btn
}

// CommitSetting atomically reads the current UI value, applies it to the native setter, and updates the baseline.
func (sm *SettingsManager) CommitSetting(name string) {
	val := sm.GetValue(name)
	if val == nil {
		return
	}

	if apply, ok := sm.applyFuncs[name]; ok {
		apply(val)
	}

	sm.SeedBaseline(name, val)
	sm.RemoveSettingChangedCallback(name)

	// Since Commit is usually triggered by a successful verification/on-press,
	// we always refresh to catch the visibility state change.
	sm.fullRefresh()
	sm.checkAndEnableApply()
}

// ResetSettings atomically clears multiple settings, updates native getters, and resyncs baselines.
func (sm *SettingsManager) ResetSettings(resets ...setting.SettingReset) {
	for _, r := range resets {
		// 1. Update native Fyne preference
		if apply, ok := sm.applyFuncs[r.Name]; ok {
			apply(r.Value)
		}
		// 2. Update Baseline
		sm.SeedBaseline(r.Name, r.Value)
		// 3. Update UI
		sm.SetValue(r.Name, r.Value)
		// 4. Remove pending
		sm.RemoveSettingChangedCallback(r.Name)
	}

	sm.fullRefresh()
	sm.checkAndEnableApply()
}

// SetSettingStatus programmatically updates a setting's status label (thread-safe).
func (sm *SettingsManager) SetSettingStatus(name string, message string, importance schema.Importance) {
	label, ok := sm.statusLabels[name]
	if !ok || label == nil {
		return
	}

	fyneImportance := widget.LowImportance
	switch importance {
	case schema.ImportanceHigh:
		fyneImportance = widget.HighImportance
	case schema.ImportanceMedium:
		fyneImportance = widget.MediumImportance
	case schema.ImportanceLow:
		fyneImportance = widget.LowImportance
	case schema.ImportanceSuccess:
		fyneImportance = widget.SuccessImportance
	case schema.ImportanceDanger:
		fyneImportance = widget.DangerImportance
	}

	fyne.Do(func() {
		label.SetText(message)
		label.Importance = fyneImportance
		label.Refresh()
	})
}

// RenderSchema takes a pure Go UI definition and renders it to a Fyne container.
func (sm *SettingsManager) RenderSchema(p schema.PanelSchema) fyne.CanvasObject {
	var topItems []fyne.CanvasObject
	var expandingList fyne.CanvasObject

	createLabel := func(text string) fyne.CanvasObject {
		if text == "" || sm.isRenderingCompact {
			return nil
		}
		return sm.CreateSettingTitleLabel(text)
	}
	createHelp := func(text string) fyne.CanvasObject {
		if text == "" {
			return nil
		}
		return sm.CreateSettingDescriptionLabel(text)
	}

	for _, section := range p.Sections {
		sm.isRenderingCompact = section.Compact
		// Rows collect here and are wrapped in a card once the section is complete.
		sectionContainer := container.NewVBox()
		sectionHasItems := false

		var sectionHeader []fyne.CanvasObject
		if section.SelectAll != nil {
			sectionHeader = append(sectionHeader, sm.renderSelectAllHeader(section.Title, *section.SelectAll))
			sectionHasItems = true
		} else if section.Title != "" {
			sectionHeader = append(sectionHeader, sm.CreateSectionTitleLabel(section.Title))
			sectionHasItems = true
		}
		if section.Description != "" {
			sectionHeader = append(sectionHeader, sm.CreateSettingDescriptionLabel(section.Description))
			sectionHasItems = true
		}

		for _, item := range section.Items {
			switch v := item.(type) {
			case *schema.OAuthPickerItem:
				val := *v
				if lbl := createLabel(val.Label); lbl != nil {
					sectionContainer.Add(lbl)
				}
				if hlp := createHelp(val.Help); hlp != nil {
					sectionContainer.Add(hlp)
				}
				sectionContainer.Add(sm.renderOAuthPickerInline(val))
				sectionHasItems = true

			case schema.OAuthPickerItem:
				if lbl := createLabel(v.Label); lbl != nil {
					sectionContainer.Add(lbl)
				}
				if hlp := createHelp(v.Help); hlp != nil {
					sectionContainer.Add(hlp)
				}
				sectionContainer.Add(sm.renderOAuthPickerInline(v))
				sectionHasItems = true

			case schema.BoolItem:
				help := createHelp(v.Help)
				if v.ActionText != "" && v.ActionFunc != nil && v.ActionIcon == "" {
					link := newHyperlink(v.ActionText, nil)
					link.OnTapped = v.ActionFunc
					if help != nil {
						help = container.New(&tightVBox{}, help, hug(link))
					} else {
						help = hug(link)
					}
				}
				sm.renderBoolSetting(&boolConfig{
					Name:         v.Name,
					InitialValue: v.InitialValue,
					LabelText:    v.Label,
					Label:        createLabel(v.Label),
					Note:         v.Note,
					ActionIcon:   v.ActionIcon,
					ActionFunc:   v.ActionFunc,
					HelpContent:  help,
					OnChanged:    v.OnChanged,
					ApplyFunc:    v.ApplyFunc,
					NeedsRefresh: v.NeedsRefresh,
					EnabledIf:    v.EnabledIf,
					VisibleIf:    v.VisibleIf,
				}, sectionContainer)
				sectionHasItems = true

			case schema.TextItem:
				var fyneValidator fyne.StringValidator
				if v.Validator != nil {
					fyneValidator = v.Validator
				}

				sm.renderTextEntrySetting(&textEntrySettingConfig{
					Name:               v.Name,
					InitialValue:       v.InitialValue,
					PlaceHolder:        v.PlaceHolder,
					Label:              createLabel(v.Label),
					HelpContent:        createHelp(v.Help),
					Validator:          fyneValidator,
					OnChanged:          v.OnChanged,
					PostValidateCheck:  v.PostValidateCheck,
					ApplyFunc:          v.ApplyFunc,
					NeedsRefresh:       v.NeedsRefresh,
					SkipApply:          v.SkipApply,
					DisplayStatus:      v.DisplayStatus,
					IsPassword:         v.IsPassword,
					IsNumeric:          v.IsNumeric,
					EnabledIf:          v.EnabledIf,
					VisibleIf:          v.VisibleIf,
					ValidationDebounce: v.ValidationDebounce,
				}, sectionContainer)
				sectionHasItems = true

			case schema.SelectItem:
				sm.renderSelectSetting(&selectConfig{
					Name:         v.Name,
					Options:      v.Options,
					Segmented:    v.Segmented,
					Swatches:     v.Swatches,
					InitialValue: v.InitialValue,
					Label:        createLabel(v.Label),
					HelpContent:  createHelp(v.Help),
					OnChanged:    v.OnChanged,
					ApplyFunc:    v.ApplyFunc,
					NeedsRefresh: v.NeedsRefresh,
					EnabledIf:    v.EnabledIf,
					VisibleIf:    v.VisibleIf,
				}, sectionContainer)
				sectionHasItems = true

			case schema.SliderItem:
				sm.renderSliderSetting(&v, sectionContainer)
				sectionHasItems = true

			case schema.AsyncButtonItem:
				importance := widget.LowImportance
				switch v.Style {
				case schema.ButtonStylePrimary:
					importance = widget.HighImportance
				case schema.ButtonStyleDanger:
					importance = widget.DangerImportance
				case schema.ButtonStyleSuccess:
					importance = widget.SuccessImportance
				}

				sm.renderAsyncButton(&asyncButtonConfig{
					Name:            v.Name,
					ButtonText:      v.ButtonText,
					LoadingText:     v.LoadingText,
					Importance:      importance,
					OnPressed:       v.OnPressed,
					OnCompleted:     v.OnCompleted,
					TargetStatusKey: v.TargetStatusKey,
					NeedsRefresh:    v.NeedsRefresh,
					IconName:        v.IconName,
					EnabledIf:       v.EnabledIf,
					VisibleIf:       v.VisibleIf,
				}, sectionContainer)
				sectionHasItems = true

			case schema.BlockedImagesItem:
				sm.renderBlockedImages(v, sectionContainer)
				sectionHasItems = true

			case schema.ConfirmButtonItem:
				fyneImportance := widget.LowImportance
				switch v.Importance {
				case schema.ImportanceHigh:
					fyneImportance = widget.HighImportance
				case schema.ImportanceMedium:
					fyneImportance = widget.MediumImportance
				case schema.ImportanceLow:
					fyneImportance = widget.LowImportance
				case schema.ImportanceSuccess:
					fyneImportance = widget.SuccessImportance
				case schema.ImportanceDanger:
					fyneImportance = widget.DangerImportance
				}

				sm.renderButtonWithConfirmationSetting(&buttonWithConfirmationConfig{
					Name:               v.Name,
					ButtonText:         v.ButtonText,
					ConfirmTitle:       v.ConfirmTitle,
					ConfirmMessage:     v.ConfirmMessage,
					ConfirmButtonText:  v.ConfirmButtonText,
					ConfirmDanger:      v.ConfirmDanger,
					ConfirmMessageFunc: v.ConfirmMessageFunc,
					HelpAsync:          v.HelpAsync,
					Importance:         fyneImportance,
					OnPressed:          v.OnPressed,
					IconName:           v.IconName,
					EnabledIf:          v.EnabledIf,
					VisibleIf:          v.VisibleIf,
					Label:              createLabel(v.Label),
					HelpContent:        createHelp(v.Help),
				}, sectionContainer)
				sectionHasItems = true

			case schema.ButtonItem:
				fyneImportance := widget.MediumImportance
				switch v.Importance {
				case schema.ImportanceHigh:
					fyneImportance = widget.HighImportance
				case schema.ImportanceMedium:
					fyneImportance = widget.MediumImportance
				case schema.ImportanceLow:
					fyneImportance = widget.LowImportance
				case schema.ImportanceSuccess:
					fyneImportance = widget.SuccessImportance
				case schema.ImportanceDanger:
					fyneImportance = widget.DangerImportance
				}

				sm.renderButtonSetting(&buttonConfig{
					Name:        v.Name,
					ButtonText:  v.ButtonText,
					Importance:  fyneImportance,
					OnPressed:   v.OnPressed,
					IconName:    v.IconName,
					EnabledIf:   v.EnabledIf,
					VisibleIf:   v.VisibleIf,
					Label:       createLabel(v.Label),
					HelpContent: createHelp(v.Help),
				}, sectionContainer)
				sectionHasItems = true

			case schema.HyperlinkItem:
				u, err := url.Parse(v.URL)
				var hl fyne.CanvasObject
				if v.OnTapped != nil {
					link := newHyperlink(v.Text, nil)
					link.OnTapped = v.OnTapped
					hl = link
				} else if err == nil {
					hl = newHyperlink(v.Text, u)
				} else {
					hl = widget.NewLabel(v.Text + " (" + v.URL + ")")
				}
				if v.ID != "" {
					sm.allWidgets[v.ID] = hl
				}
				sectionContainer.Add(noInset(hug(hl)))
				sectionHasItems = true

			case schema.LabelItem:
				var labelObj fyne.CanvasObject
				switch {
				case v.IsTitle:
					labelObj = sm.CreateSectionTitleLabel(v.Text)
				case section.Flat && v.Importance == schema.ImportanceLow:
					// A line under the page title, such as a museum's location (G6).
					loc := newStyledText(v.Text, backLinkSize, false, theme.ColorNamePlaceHolder)
					loc.lineHeight = 20
					labelObj = loc
				case v.Importance == schema.ImportanceLow:
					rich := widget.NewRichTextWithText(v.Text)
					rich.Segments[0].(*widget.TextSegment).Style.ColorName = theme.ColorNamePlaceHolder
					rich.Wrapping = fyne.TextWrapWord
					labelObj = noInset(rich)
				default:
					lbl := widget.NewLabel(v.Text)
					lbl.Wrapping = fyne.TextWrapWord
					labelObj = noInset(lbl)
				}
				if v.ID != "" {
					sm.allWidgets[v.ID] = labelObj
				}
				sectionContainer.Add(labelObj)
				sectionHasItems = true

			case schema.QueryListItem:
				list := sm.renderQueryList(v)
				sm.RegisterRefreshFunc(func() { list.Refresh() })
				expandingList = list
				// Always register in allWidgets so RefreshUI() can repaint the list
				// (e.g., after an async sync operation completes in a provider goroutine).
				widgetID := v.ID
				if widgetID == "" {
					widgetID = fmt.Sprintf("__querylist_%d", len(sm.allWidgets))
				}
				sm.allWidgets[widgetID] = list

			case schema.FolderPickerItem:
				btn := widget.NewButtonWithIcon(v.ButtonText, theme.FolderOpenIcon(), func() {
					showOSFolderPicker(sm.GetSettingsWindow(), func(path string, err error) {
						if err != nil || path == "" {
							return
						}
						queryID, err := v.OnFolderSelected(path)
						if err != nil || queryID == "" {
							return
						}
						// Seed baseline as inactive so the pending change shows as checked.
						sm.SeedBaseline(queryID, false)
						// Register a pending callback to enable the query when Apply is clicked.
						enableQuery := v.EnableQuery
						sm.SetSettingChangedCallback(queryID, func() {
							if enableQuery != nil {
								_ = enableQuery(queryID)
							}
							sm.SeedBaseline(queryID, true)
						})
						sm.SetRefreshFlag(queryID)
						sm.fullRefresh()
						sm.checkAndEnableApply()
					})
				})
				sectionContainer.Add(hug(btn))
				sm.allWidgets[v.Name] = btn
				sectionHasItems = true

			case *schema.HorizontalRowItem:
				val := *v
				row := sm.renderHorizontalRow(val)
				if val.ID != "" {
					sm.allWidgets[val.ID] = row
				}
				sectionContainer.Add(noInset(row)) // links and text start at the card's text edge (G3)
				sectionHasItems = true

			case schema.HorizontalRowItem:
				row := sm.renderHorizontalRow(v)
				if v.ID != "" {
					sm.allWidgets[v.ID] = row
				}
				sectionContainer.Add(noInset(row)) // links and text start at the card's text edge (G3)
				sectionHasItems = true

			case schema.SecretItem:
				sectionContainer.Add(sm.renderSecretItem(v))
				sectionHasItems = true
			}
		}

		if sectionHasItems {
			topItems = append(topItems, buildSection(sectionHeader, sectionContainer.Objects, section.Flat))
		}
	}

	topContainer := pageColumn(topItems...)
	if len(p.Sections) > 0 && p.Sections[0].Flat {
		// Lines of text under a source's title (location, links) sit 6 px lower.
		topContainer = inset(topContainer, flatRowGap, 0, 0, 0)
	}

	for _, fn := range sm.afterRender {
		fn()
	}
	sm.afterRender = nil

	if expandingList != nil {
		// ONLY use Border + VScroll for providers with editable query tables (Pexels, Wallhaven, etc.)
		list := inset(expandingList, 0, contentMarginX, sectionGap, contentMarginX)
		content := container.NewBorder(topContainer, nil, nil, nil, list)
		return container.NewVScroll(content)
	}

	// Wrap non-list panels (App tab, General accordion sections, Museums) in VScroll
	// so content clips cleanly to its viewport and shows a vertical scrollbar when the
	// window is shorter than the section's natural height (e.g. at 200% HiDPI scaling).
	return container.NewVScroll(topContainer)
}

// renderSecretItem renders a high-density API key/credential item.
func (sm *SettingsManager) renderSecretItem(v schema.SecretItem) fyne.CanvasObject {
	if sm.GetBaseline(v.Name) == nil {
		sm.SeedBaseline(v.Name, v.InitialValue)
	}

	// Register a value getter immediately so cross-item dependencies
	// (e.g. Wallhaven username verification needing the API key) can
	// read the current value via sm.GetValue(). The getter returns the
	// live registry value which is kept in sync by CommitSetting/SeedBaseline.
	sm.valueGetters[v.Name] = func() interface{} {
		return sm.registry[v.Name]
	}

	if v.ApplyFunc != nil {
		sm.applyFuncs[v.Name] = func(val interface{}) {
			v.ApplyFunc(val.(string))
		}
	}

	// --- Empty State: Label + Entry + Save Button ---
	entry := widget.NewEntry()
	entry.Password = true
	entry.SetPlaceHolder(v.Placeholder)

	saveBtn := widget.NewButtonWithIcon(i18n.T("Verify & save"), theme.ConfirmIcon(), nil)
	saveBtn.Importance = widget.HighImportance
	saveBtn.OnTapped = func() {
		saveBtn.Disable()
		err := v.OnVerify(entry.Text)
		if err != nil {
			sm.ShowError(err)
			saveBtn.Enable()
		} else {
			// Temporarily override the value getter so CommitSetting picks up the
			// fresh entry text. After commit, restore the registry-based getter
			// so cross-item reads (e.g. Wallhaven username needing API key) work.
			sm.valueGetters[v.Name] = func() interface{} {
				return entry.Text
			}
			sm.CommitSetting(v.Name) // Writes entry.Text → applyFunc → registry
			sm.valueGetters[v.Name] = func() interface{} {
				return sm.registry[v.Name]
			}
		}
	}

	// Build empty state: SplitRow with label + entry on top, save button full-width below
	var emptyRows []fyne.CanvasObject
	if v.Label != "" {
		label := sm.CreateSettingTitleLabel(v.Label)
		emptyRows = append(emptyRows, newSettingRow(label, nil, entry, true))
	} else {
		emptyRows = append(emptyRows, entry)
	}
	emptyRows = append(emptyRows, hug(saveBtn))
	emptyState := container.NewVBox(emptyRows...)

	// --- Saved State: Masked display + full-width Clear button ---
	display := widget.NewEntry()
	display.SetText("********************************")
	display.Disable()

	clearBtn := newDangerButton(i18n.T("Clear API key"), func() {
		v.OnClear() // OnClear implementations call ResetSettings which already calls Refresh()
	})

	var savedRows []fyne.CanvasObject
	if v.Label != "" {
		label := sm.CreateSettingTitleLabel(v.Label)
		savedRows = append(savedRows, newSettingRow(label, nil, display, true))
	} else {
		savedRows = append(savedRows, display)
	}
	savedRows = append(savedRows, hug(clearBtn))
	savedState := container.NewVBox(savedRows...)

	// Stack them so we can toggle visibility easily
	stack := container.NewStack(emptyState, savedState)

	refresh := func() {
		baseline, _ := sm.registry[v.Name].(string)
		if baseline == "" {
			emptyState.Show()
			savedState.Hide()
			entry.SetText("") // Clear sensitive input when returning to empty state
			saveBtn.Enable()
		} else {
			emptyState.Hide()
			savedState.Show()
		}
		stack.Refresh()
	}

	sm.RegisterRefreshFunc(refresh)
	refresh() // Initial state trigger

	sm.allWidgets[v.Name] = stack
	return stack
}

// renderOAuthPickerInline renders the full OAuth authorize/disconnect + picker UI for a provider.
func (sm *SettingsManager) renderOAuthPickerInline(v schema.OAuthPickerItem) fyne.CanvasObject {
	statusLabel := widget.NewLabel(i18n.T("Status: checking..."))
	var connectBtn *widget.Button
	var safeFullRefresh func()

	updateUI := func() {
		isAuth, _ := v.CheckAuthStatus()
		if isAuth {
			statusLabel.SetText(i18n.T("Status: authorized (ready to select)"))
			connectBtn.SetText(i18n.T("Disconnect authorisation"))
			connectBtn.Importance = widget.DangerImportance
			connectBtn.OnTapped = func() {
				err := v.OnDisconnect()
				if err != nil {
					sm.ShowError(err)
				}
				if safeFullRefresh != nil {
					safeFullRefresh()
				}
			}
		} else {
			statusLabel.SetText(i18n.T("Status: not authorized"))
			connectBtn.SetText(i18n.T("Authorize application"))
			connectBtn.Importance = widget.MediumImportance
			connectBtn.OnTapped = func() {
				err := v.OnAuthorize()
				if err != nil {
					sm.ShowError(err)
				} else if safeFullRefresh != nil {
					// The status line now says it is authorized; no extra dialog.
					safeFullRefresh()
				}
			}
		}
		connectBtn.Refresh()
	}

	connectBtn = widget.NewButton("", nil)

	authContainer := newSettingRow(statusLabel, nil, connectBtn, false)

	pickerContainer := container.NewVBox()

	progressBar := widget.NewProgressBarInfinite()
	progressBar.Hide()
	pickerStatus := widget.NewLabel("")

	addBtn := widget.NewButton(i18n.T("Select photos via web picker"), nil)
	cancelBtn := widget.NewButton(i18n.T("Cancel"), nil)
	cancelBtn.Importance = widget.LowImportance
	cancelBtn.Hide()

	var cancelFunc context.CancelFunc

	updateAddBtn := func() {
		isAuth, _ := v.CheckAuthStatus()
		if isAuth {
			addBtn.Enable()
			pickerStatus.SetText("")
		} else {
			addBtn.Disable()
			pickerStatus.SetText(i18n.T("Please authorize above first."))
		}
	}

	safeFullRefresh = func() {
		updateUI()
		updateAddBtn()
	}
	safeFullRefresh()

	cancelBtn.OnTapped = func() {
		if cancelFunc != nil {
			cancelFunc()
			pickerStatus.SetText(i18n.T("Operation cancelled."))
		}
		cancelBtn.Hide()
		progressBar.Hide()
		addBtn.Show()
		addBtn.Enable()
	}

	addBtn.OnTapped = func() {
		isAuth, _ := v.CheckAuthStatus()
		if !isAuth {
			sm.ShowError(fmt.Errorf("please authorize first"))
			return
		}

		addBtn.Disable()
		addBtn.Hide()
		cancelBtn.Show()
		progressBar.Show()

		updateStatus := func(msg string) {
			fyne.Do(func() {
				pickerStatus.SetText(msg)
			})
		}
		updateStatus(i18n.T("Creating web session..."))

		ctx, cancel := context.WithCancel(context.Background())
		cancelFunc = cancel

		go func() {
			count, guid, err := v.OnLaunchPicker(ctx, updateStatus)

			fyne.Do(func() {
				cancelBtn.Hide()
				addBtn.Show()
				addBtn.Enable()
				progressBar.Hide()
				pickerStatus.SetText("")
				cancelFunc = nil

				if err != nil {
					if ctx.Err() != context.Canceled {
						sm.ShowError(err)
						pickerStatus.SetText(i18n.T("Error: ") + err.Error())
					}
					return
				}

				if count == 0 {
					sm.ShowError(fmt.Errorf("no photos selected"))
					return
				}

				defaultName := fmt.Sprintf(i18n.T("Collection %s (%d items)"), time.Now().Format("Jan 02 15:04"), count)
				sm.showSaveCollectionDialog(defaultName, func(name string, active bool) error {
					if err := v.OnSaveCollection(guid, name, active); err != nil {
						return err
					}
					sm.SetRefreshFlag("queries")
					sm.GetCheckAndEnableApplyFunc()()
					sm.fullRefresh()
					return nil
				}, func() {
					if v.OnCancelCollection != nil {
						v.OnCancelCollection(guid)
					}
				})
			})
		}()
	}

	pickerContainer.Add(hug(container.NewStack(addBtn, cancelBtn)))
	pickerContainer.Add(progressBar)
	pickerContainer.Add(pickerStatus)

	mainStack := container.NewVBox(authContainer, widget.NewSeparator(), pickerContainer)
	sm.allWidgets[v.Name] = mainStack
	return mainStack
}

// renderHorizontalRow groups multiple items horizontally.
func (sm *SettingsManager) renderHorizontalRow(v schema.HorizontalRowItem) fyne.CanvasObject {
	if len(v.Items) == 0 {
		return nil
	}

	temps := make([]*fyne.Container, len(v.Items))
	for i, item := range v.Items {
		temp := container.NewVBox()
		switch item := item.(type) {
		case schema.TextItem:
			sm.renderTextEntrySetting(&textEntrySettingConfig{
				Name:               item.Name,
				InitialValue:       item.InitialValue,
				PlaceHolder:        item.PlaceHolder,
				IsPassword:         item.IsPassword,
				NeedsRefresh:       item.NeedsRefresh,
				DisplayStatus:      item.DisplayStatus,
				ValidationDebounce: item.ValidationDebounce,
				OnChanged:          item.OnChanged,
				PostValidateCheck:  item.PostValidateCheck,
				ApplyFunc:          item.ApplyFunc,
				Validator:          item.Validator,
				EnabledIf:          item.EnabledIf,
				VisibleIf:          item.VisibleIf,
			}, temp)
		case schema.ButtonItem:
			sm.renderButtonSetting(&buttonConfig{
				Name:       item.Name,
				ButtonText: item.ButtonText,
				Importance: sm.mapImportance(item.Importance),
				OnPressed:  item.OnPressed,
				IconName:   item.IconName,
				EnabledIf:  item.EnabledIf,
				VisibleIf:  item.VisibleIf,
			}, temp)
		case schema.ConfirmButtonItem:
			sm.renderButtonWithConfirmationSetting(&buttonWithConfirmationConfig{
				Name:               item.Name,
				ButtonText:         item.ButtonText,
				ConfirmTitle:       item.ConfirmTitle,
				ConfirmMessage:     item.ConfirmMessage,
				ConfirmButtonText:  item.ConfirmButtonText,
				ConfirmDanger:      item.ConfirmDanger,
				ConfirmMessageFunc: item.ConfirmMessageFunc,
				HelpAsync:          item.HelpAsync,
				Importance:         sm.mapImportance(item.Importance),
				OnPressed:          item.OnPressed,
				IconName:           item.IconName,
				EnabledIf:          item.EnabledIf,
				VisibleIf:          item.VisibleIf,
			}, temp)
		case schema.BoolItem:
			var hlp fyne.CanvasObject
			if item.Help != "" {
				hlp = sm.CreateSettingDescriptionLabel(item.Help)
			}
			sm.renderBoolSetting(&boolConfig{
				Name:         item.Name,
				InitialValue: item.InitialValue,
				LabelText:    item.Label,
				Label:        nil, // Inline label for row items to prevent NewSplitRow squishing
				HelpContent:  hlp,
				OnChanged:    item.OnChanged,
				ApplyFunc:    item.ApplyFunc,
				NeedsRefresh: item.NeedsRefresh,
				EnabledIf:    item.EnabledIf,
				VisibleIf:    item.VisibleIf,
			}, temp)
		case schema.LabelItem:
			switch {
			case item.IsTitle:
				temp.Add(sm.CreateSettingTitleLabel(item.Text))
			case v.Layout == schema.RowLayoutInline:
				// In a line of items a wrapping label would collapse to one character per line.
				text := newSecondaryText(item.Text)
				text.Wrapping = fyne.TextWrapOff
				temp.Add(text)
			default:
				temp.Add(sm.CreateSettingDescriptionLabel(item.Text))
			}
		case schema.HyperlinkItem:
			if u, err := url.Parse(item.URL); err == nil {
				temp.Add(newHyperlink(item.Text, u))
			}
		}
		temps[i] = temp
	}

	var res fyne.CanvasObject
	if v.Layout == schema.RowLayoutHBoxSpacer && len(temps) == 2 {
		res = container.NewHBox(temps[0], layout.NewSpacer(), temps[1])
	} else if v.Layout == schema.RowLayoutInline {
		inline := container.New(&hstackLayout{gap: inlineGap})
		for _, t := range temps {
			// Centre each item so text and links of different heights share one line.
			inline.Add(container.NewCenter(t))
		}
		res = inline
	} else {
		row := container.NewAdaptiveGrid(len(v.Items))
		for _, t := range temps {
			row.Add(t)
		}
		res = row
	}

	if v.ShowSeparator {
		return container.NewVBox(res, widget.NewSeparator())
	}
	return res
}

func (sm *SettingsManager) mapImportance(i schema.Importance) widget.Importance {
	switch i {
	case schema.ImportanceHigh:
		return widget.HighImportance
	case schema.ImportanceMedium:
		return widget.MediumImportance
	case schema.ImportanceLow:
		return widget.LowImportance
	case schema.ImportanceSuccess:
		return widget.SuccessImportance
	case schema.ImportanceDanger:
		return widget.DangerImportance
	default:
		return widget.MediumImportance
	}
}

func (sm *SettingsManager) getIconResource(name string) fyne.Resource {
	switch strings.ToLower(name) {
	case "help", "question":
		return theme.HelpIcon()
	case "map", "mappin":
		return theme.InfoIcon()
	case "home", "browser":
		return theme.HomeIcon()
	case "heart", "favorite":
		return theme.ConfirmIcon()
	case "info":
		return theme.InfoIcon()
	case "add", "plus":
		return theme.ContentAddIcon()
	case "settings", "prefs":
		return theme.SettingsIcon()
	case "search":
		return theme.SearchIcon()
	default:
		return nil
	}
}

// ShowConfirm displays a modal confirmation dialog to the user.
func (sm *SettingsManager) ShowConfirm(title, message string, callback func(bool)) {
	showConfirmDialog(sm.prefsWindow, confirmSpec{Title: title, Message: message}, callback)
}

// renderQueryList builds a scroll-safe widget.List for query management.
// It handles baseline seeding, pending state preservation across Fyne cell recycling,
// and wires up the enable/disable/delete interactions with the SettingsManager.
func (sm *SettingsManager) renderQueryList(v schema.QueryListItem) *widget.List {
	var queryList *widget.List
	queryList = widget.NewList(
		// Length
		func() int {
			return len(v.GetQueries())
		},
		// CreateItem — builds the cell template (no data binding here)
		func() fyne.CanvasObject {
			urlLink := widget.NewHyperlink(i18n.T("Placeholder"), nil)
			activeCheck := widget.NewCheck(i18n.T("Active"), nil)
			deleteButton := newDangerButton(i18n.T("Delete"), nil) // red text, as every destructive button (v43)
			return container.NewHBox(urlLink, layout.NewSpacer(), activeCheck, deleteButton)
		},
		// UpdateItem — binds data to a recycled cell (scroll-safe)
		func(i int, o fyne.CanvasObject) {
			queries := v.GetQueries()
			if i >= len(queries) {
				return
			}
			query := queries[i]
			queryKey := query.ID

			c := o.(*fyne.Container)
			urlLink := c.Objects[0].(*widget.Hyperlink)
			activeCheck := c.Objects[2].(*widget.Check)
			deleteButton := c.Objects[3].(*dangerButton)

			// Set display text
			if v.GetDisplayText != nil {
				urlLink.SetText(v.GetDisplayText(query))
			} else {
				urlLink.SetText(queryDisplayName(query.Description))
			}

			// Set clickable URL
			if v.GetDisplayURL != nil {
				if u := v.GetDisplayURL(query); u != nil {
					urlLink.SetURL(u)
				}
			} else {
				if u, err := url.Parse(query.URL); err == nil {
					urlLink.SetURL(u)
				}
			}

			// --- Scroll-Safe State Management ---
			if sm.GetBaseline(queryKey) == nil {
				sm.SeedBaseline(queryKey, query.Active)
			}

			// MUST clear OnChanged before SetChecked to prevent recycling bugs
			activeCheck.OnChanged = nil

			if sm.HasPendingChange(queryKey) {
				activeCheck.SetChecked(!sm.GetBaseline(queryKey).(bool))
			} else {
				activeCheck.SetChecked(query.Active)
			}

			// Wire checkbox toggle
			activeCheck.OnChanged = func(b bool) {
				if b != sm.GetBaseline(queryKey).(bool) {
					sm.SetSettingChangedCallback(queryKey, func() {
						var err error
						if b {
							err = v.EnableQuery(query.ID)
						} else {
							err = v.DisableQuery(query.ID)
						}
						if err != nil {
							utilLog.Printf("Failed to update query status: %v", err)
						} else {
							sm.SeedBaseline(queryKey, b)
							query.Active = b
						}
					})
					sm.SetRefreshFlag(queryKey)
				} else {
					sm.RemoveSettingChangedCallback(queryKey)
					sm.UnsetRefreshFlag(queryKey)
				}
				sm.checkAndEnableApply()
				sm.RefreshUI() // Update UI states (accordion titles) without triggering wallpaper changes
			}

			// Wire action button
			label := i18n.T("Delete")
			if v.DeleteLabel != "" {
				label = v.DeleteLabel
			}
			deleteButton.SetText(label)

			deleteButton.OnTapped = func() {
				// DQ: the question carries the name and there is no text under it (Favorites has no
				// Clear button, so there is no clear dialog).
				confirmTitle := i18n.Tf("Delete “{{.Name}}”?", map[string]any{"Name": queryDisplayName(query.Description)})
				confirmMsg := v.DeleteConfirmMessage

				showConfirmDialog(sm.prefsWindow, confirmSpec{Title: confirmTitle, Message: confirmMsg, ConfirmText: label, Danger: true}, func(b bool) {
					if b {
						if query.Active {
							sm.SetRefreshFlag(queryKey)
							sm.checkAndEnableApply()
						}
						if err := v.RemoveQuery(query.ID); err != nil {
							utilLog.Printf("Failed to remove query: %v", err)
						}
						queryList.Refresh()
					}
				})
			}

			// Managed/synced queries: show a disabled Delete button for visual consistency.
			// Non-managed queries: enable the Delete button.
			if query.Managed && !v.ForceActionEnabled {
				deleteButton.Disable()
			} else {
				deleteButton.Enable()
			}
			deleteButton.Refresh()
		},
	)
	return queryList
}

// RenderTabs takes a declarative tabs schema and renders it into a Fyne container.
func (sm *SettingsManager) RenderTabs(t schema.TabsSchema) fyne.CanvasObject {
	var tabItems []*container.TabItem
	for _, item := range t.Items {
		var content fyne.CanvasObject
		if item.Accordion != nil {
			content = sm.RenderAccordion(*item.Accordion)
		} else if item.Panel != nil {
			content = sm.RenderSchema(*item.Panel)
		}
		if content == nil {
			content = container.NewVBox()
		}

		var icon fyne.Resource
		if len(item.IconBytes) > 0 {
			icon = fyne.NewStaticResource(item.IconName, item.IconBytes)
		} else if item.IconName != "" {
			icon = sm.resolveIcon(item.IconName)
		}
		if icon != nil {
			tabItems = append(tabItems, container.NewTabItemWithIcon(item.Title, icon, content))
		} else {
			tabItems = append(tabItems, container.NewTabItem(item.Title, content))
		}
	}

	tabs := container.NewAppTabs(tabItems...)
	switch t.Location {
	case schema.TabLocationLeading:
		tabs.SetTabLocation(container.TabLocationLeading)
	case schema.TabLocationBottom:
		tabs.SetTabLocation(container.TabLocationBottom)
	case schema.TabLocationTrailing:
		tabs.SetTabLocation(container.TabLocationTrailing)
	default:
		tabs.SetTabLocation(container.TabLocationTop)
	}

	if t.Selected >= 0 && t.Selected < len(tabs.Items) {
		tabs.SelectIndex(t.Selected)
	}

	return container.NewStack(tabs)
}

// RenderAccordion takes a declarative accordion schema and renders an interactive collapsing container.
func (sm *SettingsManager) RenderAccordion(a schema.AccordionSchema) fyne.CanvasObject {
	accordionContainer := container.NewStack()

	items := a.Items

	// Pre-render content objects once to preserve widget state and bindings across header refreshes
	renderedContents := make([]fyne.CanvasObject, len(items))
	for i, item := range items {
		if item.Content != nil {
			renderedContents[i] = sm.RenderSchema(*item.Content)
		}
	}

	var refreshAccordion func()
	refreshAccordion = func() {
		fyne.Do(func() {
			if len(items) == 0 {
				msg := a.EmptyMessage
				if msg == "" {
					msg = i18n.T("No items available.")
				}
				accordionContainer.Objects = []fyne.CanvasObject{widget.NewLabel(msg)}
				accordionContainer.Refresh()
				return
			}

			topHeaders := container.NewVBox()
			bottomHeaders := container.NewVBox()
			var centerContent fyne.CanvasObject
			foundOpen := false

			for i := range items {
				index := i
				item := &items[index]

				var arrowIcon fyne.Resource
				if item.Open {
					arrowIcon = theme.MoveDownIcon()
				} else {
					arrowIcon = theme.NavigateNextIcon()
				}

				onTapped := func() {
					if item.Open {
						item.Open = false
						nextIndex := (index + 1) % len(items)
						items[nextIndex].Open = true
					} else {
						for j := range items {
							items[j].Open = (j == index)
						}
					}
					refreshAccordion()
				}

				bgBtn := widget.NewButton("", onTapped)
				bgBtn.Alignment = widget.ButtonAlignLeading

				title := item.Title
				if item.TitleFunc != nil {
					title = item.TitleFunc()
				}

				titleLabel := widget.NewLabel(title)
				titleLabel.TextStyle = fyne.TextStyle{Bold: item.Open}

				headerContent := container.NewHBox(
					widget.NewIcon(arrowIcon),
				)
				var headerIcon fyne.Resource
				if len(item.IconBytes) > 0 {
					headerIcon = fyne.NewStaticResource(item.IconName, item.IconBytes)
				} else if item.IconName != "" {
					headerIcon = sm.resolveIcon(item.IconName)
				}
				if headerIcon != nil {
					headerContent.Add(widget.NewIcon(headerIcon))
				}
				headerContent.Add(titleLabel)

				headerStack := container.NewStack(bgBtn, container.NewPadded(headerContent))

				if item.Open {
					topHeaders.Add(headerStack)
					if index < len(renderedContents) {
						centerContent = renderedContents[index]
					}
					foundOpen = true
				} else {
					if foundOpen {
						bottomHeaders.Add(headerStack)
					} else {
						topHeaders.Add(headerStack)
					}
				}
			}

			content := container.NewBorder(topHeaders, bottomHeaders, nil, nil, centerContent)
			accordionContainer.Objects = []fyne.CanvasObject{content}
			accordionContainer.Refresh()
		})
	}

	sm.RegisterOnSettingsSaved(func() {
		refreshAccordion()
	})
	sm.RegisterRefreshFunc(func() {
		refreshAccordion()
	})

	refreshAccordion()
	return accordionContainer
}

func (sm *SettingsManager) resolveIcon(name string) fyne.Resource {
	if name == "" {
		return nil
	}
	switch name {
	case "settings":
		return theme.SettingsIcon()
	case "color_palette":
		return theme.ColorPaletteIcon()
	case "grid":
		return theme.GridIcon()
	case "folder":
		return theme.FolderIcon()
	case "history":
		return theme.HistoryIcon()
	case "fullscreen":
		return theme.ViewFullScreenIcon()
	case "image":
		return theme.FileImageIcon()
	case "check":
		return theme.CheckButtonCheckedIcon()
	case "computer":
		return theme.ComputerIcon()
	case "refresh":
		return theme.ViewRefreshIcon()
	}
	if sm.assetMgr != nil {
		if res, err := sm.assetMgr.GetIcon(name); err == nil && res != nil {
			return res
		}
	}
	return nil
}
