package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// toggleButton is a button that stays pressed (docs/ui_refresh.md §12, J4). It
// shows its state by colour only: pressed uses the primary colour, not pressed
// the plain button with the input border. A disabled button keeps its state.
type toggleButton struct {
	widget.DisableableWidget
	Text     string
	Pressed  bool
	OnTapped func()
	hovered  bool
}

var (
	_ fyne.Tappable      = (*toggleButton)(nil)
	_ desktop.Hoverable  = (*toggleButton)(nil)
	_ desktop.Cursorable = (*toggleButton)(nil)
)

func newToggleButton(text string, pressed bool, onTapped func()) *toggleButton {
	b := &toggleButton{Text: text, Pressed: pressed, OnTapped: onTapped}
	b.ExtendBaseWidget(b)
	return b
}

// SetPressed changes the state without calling OnTapped.
func (b *toggleButton) SetPressed(pressed bool) {
	if b.Pressed == pressed {
		return
	}
	b.Pressed = pressed
	b.Refresh()
}

func (b *toggleButton) Tapped(_ *fyne.PointEvent) {
	if b.Disabled() || b.OnTapped == nil {
		return
	}
	b.OnTapped()
}

func (b *toggleButton) MouseIn(_ *desktop.MouseEvent)    { b.hovered = true; b.Refresh() }
func (b *toggleButton) MouseMoved(_ *desktop.MouseEvent) {}
func (b *toggleButton) MouseOut()                        { b.hovered = false; b.Refresh() }

func (b *toggleButton) Cursor() desktop.Cursor {
	if b.Disabled() {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}

func (b *toggleButton) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.Transparent)
	bg.CornerRadius = theme.InputRadiusSize()
	bg.StrokeWidth = 1
	hover := canvas.NewRectangle(color.Transparent)
	hover.CornerRadius = theme.InputRadiusSize()
	text := canvas.NewText(b.Text, color.Black)
	text.TextStyle = fyne.TextStyle{Bold: true}
	r := &toggleButtonRenderer{b: b, bg: bg, hover: hover, text: text}
	r.Refresh()
	return r
}

type toggleButtonRenderer struct {
	b     *toggleButton
	bg    *canvas.Rectangle
	hover *canvas.Rectangle
	text  *canvas.Text
}

func (r *toggleButtonRenderer) MinSize() fyne.Size {
	m := r.text.MinSize()
	return fyne.NewSize(m.Width+2*buttonPadX, controlHeight)
}

func (r *toggleButtonRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.hover.Resize(size)
	m := r.text.MinSize()
	r.text.Move(fyne.NewPos((size.Width-m.Width)/2, (size.Height-m.Height)/2))
	r.text.Resize(m)
}

func (r *toggleButtonRenderer) Refresh() {
	r.text.Text = r.b.Text
	r.text.TextSize = theme.TextSize()
	if r.b.Pressed {
		r.bg.FillColor = theme.Color(theme.ColorNamePrimary)
		r.bg.StrokeColor = theme.Color(theme.ColorNamePrimary)
		r.text.Color = theme.Color(theme.ColorNameForegroundOnPrimary)
	} else {
		r.bg.FillColor = theme.Color(theme.ColorNameBackground)
		r.bg.StrokeColor = theme.Color(theme.ColorNameInputBorder)
		r.text.Color = theme.Color(theme.ColorNameForeground)
	}
	r.hover.FillColor = color.Transparent
	if r.b.hovered && !r.b.Disabled() {
		r.hover.FillColor = theme.Color(theme.ColorNameHover)
	}
	r.Layout(r.b.Size())
	canvas.Refresh(r.b)
}

func (r *toggleButtonRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.hover, r.text}
}

func (r *toggleButtonRenderer) Destroy() {}
