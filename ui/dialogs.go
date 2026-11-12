package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
)

// Confirmation dialogs in the settings window's style (docs/ui_refresh.md §12,
// A-1): title and text left-aligned, buttons right-aligned and named for what
// they do ("Cancel" / "Clear cache"), the destructive one filled in the error
// colour.

const (
	dialogWidth   float32 = 360
	dialogPadding float32 = 20
	dialogRadius  float32 = 10
	dialogTitle   float32 = 16
)

// confirmSpec describes one confirmation dialog.
type confirmSpec struct {
	Title       string
	Message     string
	ConfirmText string // what the confirming button does, e.g. "Clear cache"
	Danger      bool   // fill the confirming button in the error colour
}

// showConfirmDialog shows a modal confirmation over the window and calls
// result(true) when the confirming button is pressed and result(false) when
// the person cancels.
func showConfirmDialog(parent fyne.Window, spec confirmSpec, result func(bool)) {
	if parent == nil {
		return
	}
	if spec.ConfirmText == "" {
		spec.ConfirmText = i18n.T("OK")
	}
	var popup *widget.PopUp
	finish := func(ok bool) {
		popup.Hide()
		if result != nil {
			result(ok)
		}
	}

	title := newDialogTitle(spec.Title)

	kind := buttonNeutral
	if spec.Danger {
		kind = buttonDanger
	}
	cancel := newDialogButton(i18n.T("Cancel"), buttonNeutral, func() { finish(false) })
	confirm := newDialogButton(spec.ConfirmText, kind, func() { finish(true) })
	buttons := container.New(&rightRowLayout{gap: 8}, cancel, confirm)

	// A dialog that is only a question (delete "Random General"?) has no text under its title.
	parts := []fyne.CanvasObject{title}
	if spec.Message != "" {
		parts = append(parts, noInset(newDialogText(spec.Message, dialogWidth)))
	}
	parts = append(parts, inset(buttons, 12, 0, 0, 0))

	popup = newDialogPopup(parent, dialogWidth, vstack(8, parts...))
}

// newDialogTitle is the bold, left-aligned title of a dialog.
func newDialogTitle(text string) fyne.CanvasObject {
	title := newStyledText(text, dialogTitle, true, theme.ColorNameForeground)
	title.lineHeight = 24
	return title
}

// newDialogText is the muted, wrapped sentence under the title of a dialog dialogW wide. It
// wraps between Chinese characters and keeps a number with its unit (wrapText).
func newDialogText(text string, dialogW float32) *wrappedText {
	return newWrappedText(text, dialogW-2*dialogPadding)
}

// newDialogPopup puts body on the rounded dialog panel, width wide, and shows it modally over
// parent.
func newDialogPopup(parent fyne.Window, width float32, body fyne.CanvasObject) *widget.PopUp {
	panel := container.NewStack(newDialogBackground(), inset(body, dialogPadding, dialogPadding, dialogPadding, dialogPadding))
	sized := container.New(&fixedWidthLayout{width: width}, panel)
	popup := widget.NewModalPopUp(sized, parent.Canvas())
	// The pop-up draws its own square panel, an inner padding larger than the content, under the
	// rounded one (A2 §15.3b #8: white corners in light, a lighter square in dark). The pop-up
	// itself gets no panel colour and no padding, so its shadow hugs the rounded panel; the
	// content keeps the app's theme.
	container.NewThemeOverride(popup, popUpFrameTheme{})
	container.NewThemeOverride(sized, appTheme{})
	popup.Show()
	return popup
}

// appTheme is the app's current theme, looked up on every call so that a dialog that is open when
// the theme changes follows it.
type appTheme struct{}

func (appTheme) current() fyne.Theme { return fyne.CurrentApp().Settings().Theme() }

func (a appTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	return a.current().Color(n, v)
}

func (a appTheme) Font(s fyne.TextStyle) fyne.Resource     { return a.current().Font(s) }
func (a appTheme) Icon(n fyne.ThemeIconName) fyne.Resource { return a.current().Icon(n) }
func (a appTheme) Size(n fyne.ThemeSizeName) float32       { return a.current().Size(n) }

// popUpFrameTheme is appTheme for a dialog's pop-up frame: no panel colour, no padding.
type popUpFrameTheme struct{ appTheme }

func (p popUpFrameTheme) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	if n == theme.ColorNameOverlayBackground {
		return color.Transparent
	}
	return p.appTheme.Color(n, v)
}

func (p popUpFrameTheme) Size(n fyne.ThemeSizeName) float32 {
	if n == theme.SizeNameInnerPadding {
		return 0
	}
	return p.appTheme.Size(n)
}

// rightRowLayout lines its children up at the right edge, gap apart.
type rightRowLayout struct{ gap float32 }

func (l *rightRowLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for i, o := range objects {
		m := o.MinSize()
		w += m.Width
		if i > 0 {
			w += l.gap
		}
		h = fyne.Max(h, m.Height)
	}
	return fyne.NewSize(w, h)
}

func (l *rightRowLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	x := size.Width
	for i := len(objects) - 1; i >= 0; i-- {
		m := objects[i].MinSize()
		x -= m.Width
		objects[i].Move(fyne.NewPos(x, (size.Height-m.Height)/2))
		objects[i].Resize(m)
		x -= l.gap
	}
}

// dialogBackground is the rounded, bordered panel of a dialog.
type dialogBackground struct{ widget.BaseWidget }

func newDialogBackground() *dialogBackground {
	b := &dialogBackground{}
	b.ExtendBaseWidget(b)
	return b
}

func (b *dialogBackground) CreateRenderer() fyne.WidgetRenderer {
	r := &dialogBackgroundRenderer{rect: canvas.NewRectangle(color.Transparent)}
	r.rect.CornerRadius = dialogRadius
	r.rect.StrokeWidth = 1
	r.Refresh()
	return r
}

type dialogBackgroundRenderer struct{ rect *canvas.Rectangle }

func (r *dialogBackgroundRenderer) Layout(size fyne.Size)        { r.rect.Resize(size) }
func (r *dialogBackgroundRenderer) MinSize() fyne.Size           { return fyne.NewSize(0, 0) }
func (r *dialogBackgroundRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.rect} }
func (r *dialogBackgroundRenderer) Destroy()                     {}
func (r *dialogBackgroundRenderer) Refresh() {
	r.rect.FillColor = theme.Color(theme.ColorNameOverlayBackground)
	r.rect.StrokeColor = theme.Color(theme.ColorNameSeparator)
	r.rect.Refresh()
}

// ---------------------------------------------------------------------------
// Dialog button
// ---------------------------------------------------------------------------

type buttonKind int

const (
	buttonNeutral buttonKind = iota // the usual button fill
	buttonDanger                    // grey with red text, like every destructive button (v43)
	buttonPrimary                   // filled in the accent colour (Accept, Add, OK)
)

// dialogButton is a 28 px high button with bold text; a danger button has red text (v43); was filled
// in the error colour with white text.
type dialogButton struct {
	widget.DisableableWidget
	Text     string
	kind     buttonKind
	OnTapped func()
	hovered  bool
}

var (
	_ fyne.Tappable      = (*dialogButton)(nil)
	_ desktop.Hoverable  = (*dialogButton)(nil)
	_ desktop.Cursorable = (*dialogButton)(nil)
)

func newDialogButton(text string, kind buttonKind, onTapped func()) *dialogButton {
	b := &dialogButton{Text: text, kind: kind, OnTapped: onTapped}
	b.ExtendBaseWidget(b)
	return b
}

func (b *dialogButton) Tapped(_ *fyne.PointEvent) {
	if b.Disabled() || b.OnTapped == nil {
		return
	}
	b.OnTapped()
}

func (b *dialogButton) MouseIn(_ *desktop.MouseEvent)    { b.hovered = true; b.Refresh() }
func (b *dialogButton) MouseMoved(_ *desktop.MouseEvent) {}
func (b *dialogButton) MouseOut()                        { b.hovered = false; b.Refresh() }

func (b *dialogButton) Cursor() desktop.Cursor {
	if b.Disabled() {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}

func (b *dialogButton) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.Transparent)
	bg.CornerRadius = theme.InputRadiusSize()
	hover := canvas.NewRectangle(color.Transparent)
	hover.CornerRadius = theme.InputRadiusSize()
	text := canvas.NewText(b.Text, color.Black)
	text.TextStyle = fyne.TextStyle{Bold: true}
	r := &dialogButtonRenderer{b: b, bg: bg, hover: hover, text: text}
	r.Refresh()
	return r
}

type dialogButtonRenderer struct {
	b     *dialogButton
	bg    *canvas.Rectangle
	hover *canvas.Rectangle
	text  *canvas.Text
}

func (r *dialogButtonRenderer) MinSize() fyne.Size {
	m := r.text.MinSize()
	return fyne.NewSize(m.Width+2*buttonPadX, controlHeight)
}

func (r *dialogButtonRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.hover.Resize(size)
	m := r.text.MinSize()
	r.text.Move(fyne.NewPos((size.Width-m.Width)/2, (size.Height-m.Height)/2))
	r.text.Resize(m)
}

func (r *dialogButtonRenderer) Refresh() {
	r.text.Text = r.b.Text
	r.text.TextSize = theme.TextSize()
	r.text.Color = theme.Color(theme.ColorNameForeground)
	r.bg.FillColor = theme.Color(theme.ColorNameButton)
	switch r.b.kind {
	case buttonDanger:
		r.text.Color = theme.Color(theme.ColorNameError)
	case buttonPrimary:
		r.text.Color = theme.Color(theme.ColorNameForegroundOnPrimary)
		r.bg.FillColor = theme.Color(theme.ColorNamePrimary)
	}
	if r.b.Disabled() { // a button that cannot be pressed yet is grey, whatever it will become
		r.text.Color = theme.Color(theme.ColorNameDisabled)
		r.bg.FillColor = theme.Color(theme.ColorNameDisabledButton)
	}
	r.hover.FillColor = color.Transparent
	if r.b.hovered && !r.b.Disabled() {
		r.hover.FillColor = theme.Color(theme.ColorNameHover)
	}
	r.Layout(r.b.Size())
	canvas.Refresh(r.b)
}

func (r *dialogButtonRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.hover, r.text}
}

func (r *dialogButtonRenderer) Destroy() {}
