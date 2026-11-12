package schema

import (
	"context"
	"net/url"
	"time"
)

// ItemSchema is the common interface for all declarative UI elements.
type ItemSchema interface {
	isItemSchema() // Private marker method to ensure only our types implement the interface
}

// ButtonStyle represents the visual priority of a button in a pure Go way.
type ButtonStyle string

const (
	ButtonStyleDefault ButtonStyle = "default"
	ButtonStylePrimary ButtonStyle = "primary"
	ButtonStyleDanger  ButtonStyle = "danger"
	ButtonStyleSuccess ButtonStyle = "success"
)

// Importance defines the visual weight of a UI element.
type Importance string

const (
	ImportanceHigh    Importance = "high"
	ImportanceMedium  Importance = "medium"
	ImportanceLow     Importance = "low"
	ImportanceSuccess Importance = "success"
	ImportanceDanger  Importance = "danger"
)

// PanelSchema is the root of the declarative UI definition.
type PanelSchema struct {
	Sections []SectionSchema
}

// SectionSchema represents a logical grouping of settings.
type SectionSchema struct {
	ID          string // Unique identifier for this section (e.g., "met", "aic")
	Title       string
	Description string
	Compact     bool // If true, rendering will use high-density layouts
	Flat        bool // If true, items are shown as plain text without a card
	// SelectAll, if set, puts a small switch at the right of the section's label
	// that turns the listed boolean settings on or off together.
	SelectAll *SelectAllItem
	Items     []ItemSchema
}

// BoolItem represents a checkbox/toggle.
type BoolItem struct {
	Name         string
	Label        string
	Help         string
	ActionText   string // Optional: Hyperlink text to display next to the checkbox
	ActionFunc   func() // Optional: Function to execute when the hyperlink is clicked
	ActionIcon   string // Optional: show the action as this icon ("view") next to the switch instead of a link
	Note         string // Optional: short muted text after the label, e.g. a count
	InitialValue bool
	OnChanged    func(bool)
	ApplyFunc    func(bool)
	NeedsRefresh bool
	EnabledIf    func() bool
	VisibleIf    func() bool
}

func (BoolItem) isItemSchema() {}

// SelectAllItem is a switch in a section's header that turns a group of boolean
// settings on or off together. It is not a setting itself: it shows on when every
// member is on, and follows the members as they change. The header also shows how
// many members are on.
type SelectAllItem struct {
	Name    string
	Label   string
	Members []string // Names of the BoolItems it controls
}

// TextItem represents a text entry field.
type TextItem struct {
	Name               string
	Label              string
	Help               string
	InitialValue       string
	PlaceHolder        string
	IsPassword         bool
	DisplayStatus      bool
	IsNumeric          bool
	ValidationDebounce time.Duration
	Validator          func(string) error // Pure Go validator
	PostValidateCheck  func(string) error
	OnChanged          func(string)
	ApplyFunc          func(string)
	NeedsRefresh       bool
	SkipApply          bool // When true, changes won't register with the Apply button (e.g. verify-first fields)
	EnabledIf          func() bool
	VisibleIf          func() bool
}

func (TextItem) isItemSchema() {}

// SelectItem represents a dropdown/select field.
type SelectItem struct {
	Name         string
	Label        string
	Help         string
	Options      []string
	Segmented    bool // Render as a segmented control; use for 2-4 short options
	Swatches     bool // Render as a row of colour swatches; Options are their names
	InitialValue interface{}
	OnChanged    func(string, interface{})
	ApplyFunc    func(interface{})
	NeedsRefresh bool
	EnabledIf    func() bool
	VisibleIf    func() bool
}

func (SelectItem) isItemSchema() {}

// SliderItem represents a slider control for numeric values.
type SliderItem struct {
	Name         string
	Label        string
	Help         string
	Min          float64
	Max          float64
	Step         float64
	InitialValue float64
	Format       string // e.g. "%.0f%%"
	OnChanged    func(float64)
	ApplyFunc    func(float64)
	NeedsRefresh bool
	EnabledIf    func() bool
	VisibleIf    func() bool
}

func (SliderItem) isItemSchema() {}

// AsyncButtonItem represents a button that performs a background task.
type AsyncButtonItem struct {
	Name            string
	ButtonText      string
	LoadingText     string
	Style           ButtonStyle
	TargetStatusKey string
	OnPressed       func() error
	OnCompleted     func(error)
	NeedsRefresh    bool
	IconName        string
	EnabledIf       func() bool
	VisibleIf       func() bool
}

func (AsyncButtonItem) isItemSchema() {}

// ConfirmButtonItem represents a button that requires confirmation before execution.
type ConfirmButtonItem struct {
	Name           string
	Label          string
	Help           string
	ButtonText     string
	ConfirmTitle   string
	ConfirmMessage string
	// ConfirmButtonText is what the confirming button of the dialog says, e.g. "Clear cache".
	ConfirmButtonText string
	// ConfirmMessageFunc, when set, builds the dialog text when the button is pressed (it can
	// say how much will be deleted); ConfirmMessage is used when it is nil.
	ConfirmMessageFunc func() string
	// HelpAsync, when set, shows a "Calculating…" help line under the label and calls it with a
	// function that replaces that line once the answer is known (it may be called from any goroutine).
	HelpAsync func(set func(string))
	// ConfirmDanger fills that button in the error colour, for actions that delete.
	ConfirmDanger bool
	Importance    Importance
	OnPressed     func()
	IconName      string
	EnabledIf     func() bool
	VisibleIf     func() bool
}

func (ConfirmButtonItem) isItemSchema() {}

// BlockedImagesItem is the "Blocked images" row (S4-5): how many images are
// blocked, View, which opens a page of their thumbnails where each can be
// unblocked (S4-B), and Reset, which unblocks all after a confirmation.
type BlockedImagesItem struct {
	Name           string
	Label          string
	ResetText      string
	ConfirmTitle   string
	ConfirmMessage string
	// ConfirmButtonText and ConfirmDanger are as on ConfirmButtonItem.
	ConfirmButtonText string
	ConfirmDanger     bool
	// List returns the blocked images, newest first.
	List    func() []BlockedImageSchema
	Unblock func(id string)
	Reset   func()
}

func (BlockedImagesItem) isItemSchema() {}

// BlockedImageSchema is one blocked image. Images blocked before details were
// kept have only an ID.
type BlockedImageSchema struct {
	ID     string
	Title  string
	Artist string
	Source string // the source's name, in the UI language
	// ThumbPath is a small copy of the image saved when it was blocked; empty
	// when there is none.
	ThumbPath string
}

// ButtonItem represents a standard action button.
type ButtonItem struct {
	Name       string
	Label      string
	Help       string
	ButtonText string
	Importance Importance
	OnPressed  func()
	IconName   string
	EnabledIf  func() bool
	VisibleIf  func() bool
}

func (ButtonItem) isItemSchema() {}

// HyperlinkItem represents a clickable URL.
type HyperlinkItem struct {
	ID   string
	Text string
	URL  string
	// OnTapped, when set, runs instead of opening URL: a link to another page of the window.
	OnTapped func()
}

func (HyperlinkItem) isItemSchema() {}

// LabelItem represents static text or a sub-title.
type LabelItem struct {
	ID         string
	Text       string
	IsTitle    bool
	Importance Importance // Low importance maps to muted description style
}

func (LabelItem) isItemSchema() {}

// OAuthPickerItem defines the contract for complex OAuth sessions and picker workflows.
type OAuthPickerItem struct {
	Name  string
	Label string
	Help  string

	// Pure Domain Core Logic
	CheckAuthStatus func() (isAuth bool, isExpired bool)
	OnAuthorize     func() error
	OnDisconnect    func() error

	// Async Polling & Download (Takes a pure string callback for UI stream updates)
	OnLaunchPicker func(ctx context.Context, updateStatus func(string)) (itemCount int, guid string, err error)

	// Collection Persistence Hooks
	OnSaveCollection   func(guid string, description string, active bool) error
	OnCancelCollection func(guid string)
}

func (OAuthPickerItem) isItemSchema() {}

// FolderPickerItem represents a button that launches a directory selection dialog.
type FolderPickerItem struct {
	Name       string
	Label      string
	Help       string
	ButtonText string
	// OnFolderSelected is called when a folder is picked.
	// It should add the query (inactive) and return its ID.
	OnFolderSelected func(path string) (string, error)
	// EnableQuery activates a query by its ID when Apply is clicked.
	EnableQuery func(id string) error
	EnabledIf   func() bool
	VisibleIf   func() bool
}

func (FolderPickerItem) isItemSchema() {}

// Query represents a generic abstract query data record to break dependency chains.
type Query struct {
	ID          string
	URL         string
	Description string
	Active      bool
	Managed     bool // Whether this query is managed by sync (cannot be deleted)
}

// QueryListItem represents the abstraction for the list of queries/collections.
type QueryListItem struct {
	ID                   string
	GetQueries           func() []Query
	EnableQuery          func(id string) error
	DisableQuery         func(id string) error
	RemoveQuery          func(id string) error
	GetDisplayText       func(Query) string // Optional: converts a query to its display label
	GetDisplayURL        func(Query) *url.URL
	DeleteLabel          string // Text for the action button. Default is "Delete".
	ForceActionEnabled   bool   // Enable the action button even for managed queries.
	DeleteConfirmMessage string // Custom message for the confirmation dialog.
}

func (QueryListItem) isItemSchema() {}

// RowLayout defines how items are arranged inside a HorizontalRowItem.
type RowLayout string

const (
	RowLayoutAdaptiveGrid RowLayout = ""            // Default: Divides space equally among items
	RowLayoutHBoxSpacer   RowLayout = "hbox_spacer" // Places a spacer between the first and second item, pushing them to opposite sides
	RowLayoutInline       RowLayout = "inline"      // Places items one after another at their natural width, e.g. a line of links
)

// HorizontalRowItem groups multiple items horizontally.
type HorizontalRowItem struct {
	ID            string
	Items         []ItemSchema
	Layout        RowLayout
	ShowSeparator bool
}

func (HorizontalRowItem) isItemSchema() {}

// AddQueryConfig defines the configuration for a standardized Add Query modal.
type AddQueryConfig struct {
	Title           string
	Description     string // Optional description for the modal
	URLLabel        string // label of the address field: "Pexels URL", "Category or search term"
	URLPlaceholder  string
	URLValidator    string
	URLErrorMsg     string
	DescPlaceholder string
	DescValidator   string
	DescErrorMsg    string

	// ValidateFunc is an optional custom validation logic.
	ValidateFunc func(url, desc string) error

	// AddHandler performs the actual addition of the query.
	// It returns the new query ID and an error if it fails.
	AddHandler func(desc, url string, active bool) (string, error)
}

// SecretItem represents a sensitive credential (like an API key).
type SecretItem struct {
	Name         string
	Label        string
	Help         string
	InitialValue string
	Placeholder  string
	OnVerify     func(key string) error
	OnClear      func()
	ApplyFunc    func(string)
}

func (SecretItem) isItemSchema() {}

// TabLocation defines where tabs are positioned.
type TabLocation string

const (
	TabLocationTop      TabLocation = "top"
	TabLocationLeading  TabLocation = "leading"
	TabLocationBottom   TabLocation = "bottom"
	TabLocationTrailing TabLocation = "trailing"
)

// TabsSchema represents a tabbed navigation container (e.g. Preferences window tabs).
type TabsSchema struct {
	Items    []TabItemSchema
	Selected int
	Location TabLocation
}

// TabItemSchema represents an individual tab.
type TabItemSchema struct {
	ID        string // Stable identifier, e.g. "sources", "wallpaper", "advanced"
	Title     string
	IconName  string
	IconBytes []byte
	Panel     *PanelSchema
	Accordion *AccordionSchema
	Sources   *SourcesSchema
}

// AccordionSchema represents a set of collapsible accordion items.
type AccordionSchema struct {
	Items        []AccordionItemSchema
	EmptyMessage string
}

// AccordionItemSchema represents a collapsible section with dynamic headers.
type AccordionItemSchema struct {
	Title     string
	TitleFunc func() string // Optional dynamic title evaluator (e.g. for reactive item counts)
	IconName  string
	IconBytes []byte
	Open      bool
	Content   *PanelSchema
}

// MenuSchema represents a context or tray menu.
type MenuSchema struct {
	Label string
	Items []MenuItemSchema
}

// MenuItemSchema represents an individual entry in a menu.
type MenuItemSchema struct {
	Label       string
	IconName    string
	IconBytes   []byte
	Action      func()
	IsSeparator bool
	IsChecked   bool
	IsDisabled  bool
	SubMenu     *MenuSchema

	// The fields below are for the tray menu the app draws itself
	// (docs/ui_refresh.md §12, T-S1/T-S2). A native menu ignores them and shows
	// Label and IconName.

	// Kind says how the app-drawn menu shows the item.
	Kind MenuItemKind
	// Title and Detail are the two lines of a MenuItemNowShowing item (the
	// artwork's title and the artist's name). Detail on a row with a submenu
	// is the current choice, shown on the right.
	Title  string
	Detail string
	// Glyph names the monochrome icon the app-drawn menu shows (for example
	// "next", "pause", "heart"); empty means no icon.
	Glyph string
	// KeepOpen leaves the app-drawn menu open after the item is used, for
	// items whose result shows in the menu itself (next, pause, favourite).
	KeepOpen bool
	// Emphasis draws a row's label in bold and its icon in the accent colour, for
	// the item that offers a new version (T14).
	Emphasis bool
	// ChangesWallpaper marks an item that changes the image or the wallpaper
	// (next, display mode, ...). The app closes Tune Image before running it,
	// as if the user had closed the window (T-L).
	ChangesWallpaper bool
}

// MenuItemKind is how the app-drawn tray menu shows an item.
type MenuItemKind int

const (
	// MenuItemRow is a text row with an optional icon on the left.
	MenuItemRow MenuItemKind = iota
	// MenuItemNowShowing is the block for the current artwork: source logo,
	// title and artist.
	MenuItemNowShowing
	// MenuItemIconButton is an icon-only button. Consecutive icon buttons share
	// one row; a separator between two of them is a thin vertical divider.
	// Label is the button's name, shown on hover.
	MenuItemIconButton
	// MenuItemSwitch is a row with a switch on the right; IsChecked is its state.
	MenuItemSwitch
)

// SourcesSchema is a searchable list of image sources in groups. Selecting a
// source shows its detail panel in place of the list.
type SourcesSchema struct {
	Groups []SourceGroupSchema
	// OpenSourceID, when set, opens that source's detail view first.
	OpenSourceID string
}

// SourceGroupSchema is one group of sources, such as Museums.
type SourceGroupSchema struct {
	ID        string
	Title     string
	Collapsed bool // Starts collapsed; the user can expand it
	Items     []SourceSchema
}

// SourceSchema is one source row and its detail panel.
type SourceSchema struct {
	ID           string
	Title        string
	SubtitleFunc func() string // Re-evaluated after changes, e.g. "Chicago · 2 collections on"
	IconName     string
	IconBytes    []byte
	Detail       *PanelSchema
}
