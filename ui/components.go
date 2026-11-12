package ui

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Layout constants for the preferences components, in Fyne logical units.
// The type scale and spacing they share with the pages are in typography.go.
const (
	wideControlWidth float32 = 240
	cardRadius       float32 = 8
)

// ---------------------------------------------------------------------------
// Text helpers
// ---------------------------------------------------------------------------

// newSecondaryText returns wrapped, muted text for help lines and group labels.
func newSecondaryText(text string) *widget.RichText {
	rich := widget.NewRichTextWithText(text)
	rich.Wrapping = fyne.TextWrapWord
	for _, seg := range rich.Segments {
		if ts, ok := seg.(*widget.TextSegment); ok {
			ts.Style.ColorName = theme.ColorNamePlaceHolder
			ts.Style.SizeName = theme.SizeNameCaptionText
		}
	}
	return rich
}

// onUIThread runs f on the UI thread when called from a background goroutine. Real Fyne queues f
// for the main thread. The headless test driver runs it on the calling goroutine, so tests replace
// this with a queue that the test goroutine drains (main_test.go), which keeps widget changes and
// layout on one goroutine under -race.
var onUIThread = fyne.Do

// setSecondaryText replaces the text of a line made by newSecondaryText.
func setSecondaryText(rich *widget.RichText, text string) {
	for _, seg := range rich.Segments {
		if ts, ok := seg.(*widget.TextSegment); ok {
			ts.Text = text
			break
		}
	}
	rich.Refresh()
}

// labelWithNote is a row label followed by short muted text on the same line,
// such as a collection's name and its number of pieces. The note is 12 px,
// 8 px after the label, on the label's baseline (G6). A leading ⭐ (starred
// collections) is drawn as a yellow ★ on the same baseline: the colour emoji
// would lift the line.
type labelWithNote struct {
	widget.BaseWidget
	text, note string
	starred    bool
}

const (
	noteGap  float32 = 8
	starGap  float32 = 4
	starSize float32 = 13
)

var starColor = color.NRGBA{R: 0xF4, G: 0xB4, A: 0xFF}

func newLabelWithNote(text, note string) *labelWithNote {
	l := &labelWithNote{text: text, note: note}
	if rest, ok := strings.CutPrefix(text, "⭐"); ok {
		l.text, l.starred = strings.TrimSpace(rest), true
	}
	l.ExtendBaseWidget(l)
	return l
}

func (l *labelWithNote) CreateRenderer() fyne.WidgetRenderer {
	r := &labelWithNoteRenderer{l: l, star: canvas.NewText("★", starColor),
		label: canvas.NewText(l.text, color.Black), note: canvas.NewText(l.note, color.Black)}
	r.star.TextSize = starSize
	if !l.starred {
		r.star.Hide()
	}
	r.Refresh()
	return r
}

type labelWithNoteRenderer struct {
	l                 *labelWithNote
	star, label, note *canvas.Text
}

// texts returns the visible texts from left to right.
func (r *labelWithNoteRenderer) texts() []*canvas.Text {
	if r.l.starred {
		return []*canvas.Text{r.star, r.label, r.note}
	}
	return []*canvas.Text{r.label, r.note}
}

// baseline returns how far below its top a text's baseline sits.
func baseline(t *canvas.Text) float32 {
	_, b := fyne.CurrentApp().Driver().RenderedTextSize(t.Text, t.TextSize, t.TextStyle, nil)
	return b
}

func (r *labelWithNoteRenderer) gapAfter(t *canvas.Text) float32 {
	if t == r.star {
		return starGap
	}
	return noteGap
}

func (r *labelWithNoteRenderer) MinSize() fyne.Size {
	var w, top, bottom float32
	for i, t := range r.texts() {
		m, b := t.MinSize(), baseline(t)
		w += m.Width
		if i > 0 {
			w += r.gapAfter(r.texts()[i-1])
		}
		top, bottom = fyne.Max(top, b), fyne.Max(bottom, m.Height-b)
	}
	return fyne.NewSize(w, top+bottom)
}

func (r *labelWithNoteRenderer) Layout(size fyne.Size) {
	var top float32
	for _, t := range r.texts() {
		top = fyne.Max(top, baseline(t))
	}
	y := (size.Height - r.MinSize().Height) / 2
	var x float32
	for _, t := range r.texts() {
		m := t.MinSize()
		t.Move(fyne.NewPos(x, y+top-baseline(t)))
		t.Resize(m)
		x += m.Width + r.gapAfter(t)
	}
}

func (r *labelWithNoteRenderer) Refresh() {
	r.label.Text, r.note.Text = r.l.text, r.l.note
	r.label.TextSize = theme.TextSize()
	r.note.TextSize = theme.CaptionTextSize()
	r.label.Color = theme.Color(theme.ColorNameForeground)
	r.note.Color = theme.Color(theme.ColorNamePlaceHolder)
	r.Layout(r.l.Size())
	r.star.Refresh()
	r.label.Refresh()
	r.note.Refresh()
}

func (r *labelWithNoteRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.star, r.label, r.note}
}
func (r *labelWithNoteRenderer) Destroy() {}

// actionIconResource maps a schema action icon name to a theme icon.
func actionIconResource(name string) fyne.Resource {
	switch name {
	case "view":
		return theme.NewColoredResource(theme.VisibilityIcon(), theme.ColorNamePlaceHolder)
	default:
		return theme.MoreHorizontalIcon()
	}
}

// newRowLabel returns the primary label of a setting row.
func newRowLabel(text string) *widget.Label {
	l := widget.NewLabel(text)
	l.Wrapping = fyne.TextWrapWord
	return l
}

// hug keeps a control at its natural width instead of stretching it, and at
// the one control height.
func hug(obj fyne.CanvasObject) fyne.CanvasObject {
	return container.NewHBox(sizedControl(obj))
}

// collapseWhenEmpty wraps a label so it takes no space while its text is empty,
// such as a validation status line that is usually blank.
func collapseWhenEmpty(l *widget.Label) fyne.CanvasObject {
	return container.New(&emptyCollapseLayout{label: l}, l)
}

type emptyCollapseLayout struct {
	label *widget.Label
}

// The child is the label, or a wrapper around it (see noInset).
func (e *emptyCollapseLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if e.label.Text == "" || len(objects) == 0 {
		return fyne.NewSize(0, 0)
	}
	return objects[0].MinSize()
}

func (e *emptyCollapseLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Move(fyne.NewPos(0, 0))
		o.Resize(size)
	}
}

// ---------------------------------------------------------------------------
// Setting row
// ---------------------------------------------------------------------------

// settingRowLayout places a label block on the left and a control on the right.
// Wide controls (selects, entries, sliders) get a fixed width; others keep their
// natural width. The control is vertically centred on the label block.
type settingRowLayout struct {
	left    fyne.CanvasObject
	control fyne.CanvasObject
	wide    bool
}

func (l *settingRowLayout) controlWidth(total float32) float32 {
	if l.control == nil {
		return 0
	}
	w := l.control.MinSize().Width
	if l.wide && w < wideControlWidth {
		w = wideControlWidth
	}
	if limit := total * 0.55; w > limit && limit > 0 {
		w = fyne.Max(limit, l.control.MinSize().Width)
	}
	return w
}

func (l *settingRowLayout) MinSize(_ []fyne.CanvasObject) fyne.Size {
	leftMin := l.left.MinSize()
	if l.control == nil {
		return leftMin
	}
	ctrl := l.control.MinSize()
	cw := ctrl.Width
	if l.wide && cw < wideControlWidth {
		cw = wideControlWidth
	}
	return fyne.NewSize(leftMin.Width+rowControlGap+cw, fyne.Max(leftMin.Height, ctrl.Height))
}

func (l *settingRowLayout) Layout(_ []fyne.CanvasObject, size fyne.Size) {
	if l.control == nil {
		l.left.Resize(size)
		l.left.Move(fyne.NewPos(0, 0))
		return
	}
	cw := l.controlWidth(size.Width)
	lw := size.Width - cw - rowControlGap
	if lw < 0 {
		lw = 0
	}
	l.left.Resize(fyne.NewSize(lw, l.left.MinSize().Height))
	leftH := l.left.MinSize().Height
	ch := l.control.MinSize().Height
	rowH := fyne.Max(fyne.Max(leftH, ch), size.Height)

	l.left.Move(fyne.NewPos(0, (rowH-leftH)/2))
	l.left.Resize(fyne.NewSize(lw, leftH))
	l.control.Move(fyne.NewPos(size.Width-cw, (rowH-ch)/2))
	l.control.Resize(fyne.NewSize(cw, ch))
}

// newSettingRow builds one row: label and optional help text on the left, the
// control on the right. A nil control makes the left block span the row.
// The text has no inset, so it starts at the card's text edge (G3), and the
// control gets the one control height (G5).
func newSettingRow(label, help, control fyne.CanvasObject, wide bool) *fyne.Container {
	var leftParts []fyne.CanvasObject
	if label != nil {
		leftParts = append(leftParts, label)
	}
	if help != nil {
		leftParts = append(leftParts, help)
	}
	left := noInset(vstack(helpGap, leftParts...))
	objects := []fyne.CanvasObject{left}
	if control != nil {
		control = sizedControl(control)
		objects = append(objects, control)
	}
	return container.New(&settingRowLayout{left: left, control: control, wide: wide}, objects...)
}

// tightVBox stacks objects with no gap so a label and its help text read as one block.
type tightVBox struct{}

func (t *tightVBox) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		m := o.MinSize()
		w = fyne.Max(w, m.Width)
		h += m.Height
	}
	return fyne.NewSize(w, h)
}

func (t *tightVBox) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	var y float32
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		o.Resize(fyne.NewSize(size.Width, o.MinSize().Height))
		o.Move(fyne.NewPos(0, y))
		h := o.MinSize().Height
		o.Resize(fyne.NewSize(size.Width, h))
		y += h
	}
}

// ---------------------------------------------------------------------------
// Card
// ---------------------------------------------------------------------------

// card groups rows inside a rounded, bordered box with dividers between the
// visible rows. Hidden rows (VisibleIf) take no space and get no divider.
// Rows are inset cardInsetX on both sides and padY above and below.
type card struct {
	widget.BaseWidget
	rows []fyne.CanvasObject
	padY float32
}

func newCard(rows ...fyne.CanvasObject) *card {
	c := &card{rows: rows, padY: rowPaddingY}
	c.ExtendBaseWidget(c)
	return c
}

func (c *card) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.Transparent)
	bg.CornerRadius = cardRadius
	bg.StrokeWidth = 1
	r := &cardRenderer{card: c, bg: bg}
	for range c.rows {
		r.dividers = append(r.dividers, canvas.NewRectangle(color.Transparent))
	}
	r.applyColors()
	return r
}

type cardRenderer struct {
	card     *card
	bg       *canvas.Rectangle
	dividers []*canvas.Rectangle
}

func (r *cardRenderer) applyColors() {
	r.bg.FillColor = theme.Color(theme.ColorNameBackground)
	r.bg.StrokeColor = theme.Color(theme.ColorNameSeparator)
	for _, d := range r.dividers {
		d.FillColor = theme.Color(theme.ColorNameSeparator)
	}
}

func (r *cardRenderer) MinSize() fyne.Size {
	var w, h float32
	visible := 0
	for _, row := range r.card.rows {
		if !row.Visible() {
			continue
		}
		m := row.MinSize()
		w = fyne.Max(w, m.Width)
		h += rowSlot(m.Height) + 2*r.card.padY
		visible++
	}
	if visible > 1 {
		h += float32(visible - 1)
	}
	return fyne.NewSize(w+2*cardInsetX, h)
}

func (r *cardRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.bg.Move(fyne.NewPos(0, 0))
	innerW := size.Width - 2*cardInsetX
	padY := r.card.padY
	var y float32
	first := true
	for i, row := range r.card.rows {
		div := r.dividers[i]
		if !row.Visible() {
			div.Hide()
			continue
		}
		if first {
			div.Hide()
			first = false
		} else {
			div.Show()
			div.Move(fyne.NewPos(cardInsetX, y))
			div.Resize(fyne.NewSize(size.Width-2*cardInsetX, 1))
			y++
		}
		row.Resize(fyne.NewSize(innerW, row.MinSize().Height))
		h := row.MinSize().Height
		slot := rowSlot(h)
		row.Move(fyne.NewPos(cardInsetX, y+padY+(slot-h)/2))
		row.Resize(fyne.NewSize(innerW, h))
		y += slot + 2*padY
	}
}

// rowSlot is the height a card row takes: at least one control high, so rows
// with a switch or a line of text are as tall as rows with a select (G4).
func rowSlot(h float32) float32 { return fyne.Max(h, controlHeight+2) }

func (r *cardRenderer) Refresh() {
	r.applyColors()
	r.bg.Refresh()
	for _, d := range r.dividers {
		d.Refresh()
	}
	for _, row := range r.card.rows {
		row.Refresh()
	}
	r.Layout(r.card.Size())
}

func (r *cardRenderer) Objects() []fyne.CanvasObject {
	objs := []fyne.CanvasObject{r.bg}
	for _, d := range r.dividers {
		objs = append(objs, d)
	}
	return append(objs, r.card.rows...)
}

func (r *cardRenderer) Destroy() {}

// ---------------------------------------------------------------------------
// Switch
// ---------------------------------------------------------------------------

const (
	switchWidth  float32 = 32 // G8
	switchHeight float32 = 18
	switchKnob   float32 = 14
)

// toggleSwitch is an on/off switch for boolean settings.
type toggleSwitch struct {
	widget.DisableableWidget
	Checked   bool
	OnChanged func(bool)
	focused   bool
}

var (
	_ fyne.Tappable      = (*toggleSwitch)(nil)
	_ fyne.Focusable     = (*toggleSwitch)(nil)
	_ desktop.Cursorable = (*toggleSwitch)(nil)
)

func newToggleSwitch(checked bool, onChanged func(bool)) *toggleSwitch {
	s := &toggleSwitch{Checked: checked, OnChanged: onChanged}
	s.ExtendBaseWidget(s)
	return s
}

// SetChecked updates the state and fires OnChanged when it changes.
func (s *toggleSwitch) SetChecked(checked bool) {
	if s.Checked == checked {
		return
	}
	s.Checked = checked
	if s.OnChanged != nil {
		s.OnChanged(checked)
	}
	s.Refresh()
}

func (s *toggleSwitch) Tapped(_ *fyne.PointEvent) {
	if s.Disabled() {
		return
	}
	s.SetChecked(!s.Checked)
}

func (s *toggleSwitch) Cursor() desktop.Cursor {
	if s.Disabled() {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}

func (s *toggleSwitch) FocusGained()     { s.focused = true; s.Refresh() }
func (s *toggleSwitch) FocusLost()       { s.focused = false; s.Refresh() }
func (s *toggleSwitch) TypedRune(r rune) {}
func (s *toggleSwitch) TypedKey(ev *fyne.KeyEvent) {
	if ev.Name == fyne.KeySpace || ev.Name == fyne.KeyReturn || ev.Name == fyne.KeyEnter {
		s.Tapped(nil)
	}
}

func (s *toggleSwitch) CreateRenderer() fyne.WidgetRenderer {
	track := canvas.NewRectangle(color.Transparent)
	track.CornerRadius = switchHeight / 2
	ring := canvas.NewRectangle(color.Transparent)
	ring.CornerRadius = switchHeight/2 + 2
	ring.StrokeWidth = 2
	knob := canvas.NewCircle(color.White)
	r := &switchRenderer{sw: s, track: track, ring: ring, knob: knob}
	r.Refresh()
	return r
}

type switchRenderer struct {
	sw    *toggleSwitch
	track *canvas.Rectangle
	ring  *canvas.Rectangle
	knob  *canvas.Circle
}

// dims returns the track width, track height and knob size. Every switch,
// including Select all, has the same size (G5).
func (r *switchRenderer) dims() (float32, float32, float32) {
	return switchWidth, switchHeight, switchKnob
}

func (r *switchRenderer) MinSize() fyne.Size {
	w, h, _ := r.dims()
	return fyne.NewSize(w+4, h+4)
}

func (r *switchRenderer) Layout(size fyne.Size) {
	w, h, knob := r.dims()
	x := (size.Width - w) / 2
	y := (size.Height - h) / 2
	r.track.CornerRadius = h / 2
	r.ring.CornerRadius = h/2 + 2
	r.ring.Move(fyne.NewPos(x-2, y-2))
	r.ring.Resize(fyne.NewSize(w+4, h+4))
	r.track.Move(fyne.NewPos(x, y))
	r.track.Resize(fyne.NewSize(w, h))
	inset := (h - knob) / 2
	kx := x + inset
	if r.sw.Checked {
		kx = x + w - knob - inset
	}
	r.knob.Move(fyne.NewPos(kx, y+inset))
	r.knob.Resize(fyne.NewSize(knob, knob))
}

func (r *switchRenderer) Refresh() {
	trackColor := paintpaneColor(colorNameSwitchOff)
	if r.sw.Checked {
		trackColor = theme.Color(theme.ColorNamePrimary)
	}
	knobColor := paintpaneColor(colorNameSwitchKnob)
	if r.sw.Disabled() {
		trackColor = withAlpha(trackColor, 0x66)
		knobColor = withAlpha(knobColor, 0xB0)
	}
	r.track.FillColor = trackColor
	r.knob.FillColor = knobColor
	r.ring.StrokeColor = color.Transparent
	if r.sw.focused {
		r.ring.StrokeColor = theme.Color(theme.ColorNameFocus)
	}
	r.Layout(r.sw.Size())
	r.track.Refresh()
	r.knob.Refresh()
	r.ring.Refresh()
}

func (r *switchRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.ring, r.track, r.knob}
}

func (r *switchRenderer) Destroy() {}

func withAlpha(c color.Color, a uint8) color.Color {
	r, g, b, _ := c.RGBA()
	return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: a}
}

// ---------------------------------------------------------------------------
// Segmented control
// ---------------------------------------------------------------------------

// segmented is a row of 2-4 mutually exclusive options.
type segmented struct {
	widget.DisableableWidget
	Options   []string
	Selected  int
	OnChanged func(int)
}

var (
	_ fyne.Tappable      = (*segmented)(nil)
	_ desktop.Cursorable = (*segmented)(nil)
)

func newSegmented(options []string, selected int, onChanged func(int)) *segmented {
	s := &segmented{Options: options, Selected: selected, OnChanged: onChanged}
	s.ExtendBaseWidget(s)
	return s
}

// SetSelectedIndex selects an option and fires OnChanged when it changes.
func (s *segmented) SetSelectedIndex(i int) {
	if i < 0 || i >= len(s.Options) || i == s.Selected {
		return
	}
	s.Selected = i
	if s.OnChanged != nil {
		s.OnChanged(i)
	}
	s.Refresh()
}

func (s *segmented) Tapped(ev *fyne.PointEvent) {
	if s.Disabled() || len(s.Options) == 0 {
		return
	}
	w := s.Size().Width / float32(len(s.Options))
	i := int(ev.Position.X / w)
	if i >= len(s.Options) {
		i = len(s.Options) - 1
	}
	s.SetSelectedIndex(i)
}

func (s *segmented) Cursor() desktop.Cursor {
	if s.Disabled() {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}

func (s *segmented) CreateRenderer() fyne.WidgetRenderer {
	frame := canvas.NewRectangle(color.Transparent)
	frame.StrokeWidth = 1
	frame.CornerRadius = theme.InputRadiusSize()
	r := &segmentedRenderer{seg: s, frame: frame}
	for _, opt := range s.Options {
		bg := canvas.NewRectangle(color.Transparent)
		bg.CornerRadius = theme.InputRadiusSize() - 1
		txt := canvas.NewText(opt, color.Black)
		txt.Alignment = fyne.TextAlignCenter
		r.cells = append(r.cells, bg)
		r.texts = append(r.texts, txt)
	}
	for i := 1; i < len(s.Options); i++ {
		r.dividers = append(r.dividers, canvas.NewRectangle(color.Transparent))
	}
	r.Refresh()
	return r
}

type segmentedRenderer struct {
	seg      *segmented
	frame    *canvas.Rectangle
	cells    []*canvas.Rectangle
	texts    []*canvas.Text
	dividers []*canvas.Rectangle
}

const segmentPadX float32 = 14

func (r *segmentedRenderer) MinSize() fyne.Size {
	var cellW float32
	for _, t := range r.texts {
		cellW = fyne.Max(cellW, t.MinSize().Width+2*segmentPadX)
	}
	return fyne.NewSize(cellW*float32(len(r.texts)), controlHeight)
}

func (r *segmentedRenderer) Layout(size fyne.Size) {
	r.frame.Resize(size)
	if len(r.cells) == 0 {
		return
	}
	w := size.Width / float32(len(r.cells))
	for i := range r.cells {
		x := float32(i) * w
		r.cells[i].Move(fyne.NewPos(x+1, 1))
		r.cells[i].Resize(fyne.NewSize(w-2, size.Height-2))
		th := r.texts[i].MinSize().Height
		r.texts[i].Move(fyne.NewPos(x, (size.Height-th)/2))
		r.texts[i].Resize(fyne.NewSize(w, th))
	}
	for i, d := range r.dividers {
		d.Move(fyne.NewPos(float32(i+1)*w, 4))
		d.Resize(fyne.NewSize(1, size.Height-8))
	}
}

func (r *segmentedRenderer) Refresh() {
	sep := theme.Color(theme.ColorNameInputBorder)
	r.frame.StrokeColor = sep
	r.frame.FillColor = theme.Color(theme.ColorNameInputBackground)
	for i := range r.cells {
		selected := i == r.seg.Selected
		r.cells[i].FillColor = color.Transparent
		r.texts[i].Color = theme.Color(theme.ColorNamePlaceHolder)
		r.texts[i].TextSize = theme.TextSize()
		if selected {
			r.cells[i].FillColor = paintpaneColor(colorNameSelectedTint)
			r.texts[i].Color = paintpaneColor(colorNameSelectedText)
		}
		if r.seg.Disabled() {
			r.texts[i].Color = theme.Color(theme.ColorNameDisabled)
		}
		r.texts[i].Text = r.seg.Options[i]
	}
	for i, d := range r.dividers {
		// Hide the divider next to the selected cell so the tint reads as one shape.
		if i == r.seg.Selected || i+1 == r.seg.Selected {
			d.FillColor = color.Transparent
		} else {
			d.FillColor = sep
		}
	}
	r.Layout(r.seg.Size())
	canvas.Refresh(r.seg)
}

func (r *segmentedRenderer) Objects() []fyne.CanvasObject {
	objs := []fyne.CanvasObject{r.frame}
	for _, c := range r.cells {
		objs = append(objs, c)
	}
	for _, d := range r.dividers {
		objs = append(objs, d)
	}
	for _, t := range r.texts {
		objs = append(objs, t)
	}
	return objs
}

func (r *segmentedRenderer) Destroy() {}

// ---------------------------------------------------------------------------
// Danger text button
// ---------------------------------------------------------------------------

// dangerButton is a button for destructive actions such as clearing the cache:
// the same shape and fill as other buttons, with bold red text (G5).
type dangerButton struct {
	widget.DisableableWidget
	Text     string
	OnTapped func()
	hovered  bool
}

var (
	_ fyne.Tappable      = (*dangerButton)(nil)
	_ desktop.Hoverable  = (*dangerButton)(nil)
	_ desktop.Cursorable = (*dangerButton)(nil)
)

func newDangerButton(text string, onTapped func()) *dangerButton {
	b := &dangerButton{Text: text, OnTapped: onTapped}
	b.ExtendBaseWidget(b)
	return b
}

// SetText changes the label (a recycled list row reuses the button).
func (b *dangerButton) SetText(text string) {
	b.Text = text
	b.Refresh()
}

func (b *dangerButton) Tapped(_ *fyne.PointEvent) {
	if b.Disabled() || b.OnTapped == nil {
		return
	}
	b.OnTapped()
}

func (b *dangerButton) MouseIn(_ *desktop.MouseEvent)    { b.hovered = true; b.Refresh() }
func (b *dangerButton) MouseMoved(_ *desktop.MouseEvent) {}
func (b *dangerButton) MouseOut()                        { b.hovered = false; b.Refresh() }

func (b *dangerButton) Cursor() desktop.Cursor {
	if b.Disabled() {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}

func (b *dangerButton) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.Transparent)
	bg.CornerRadius = theme.InputRadiusSize()
	hover := canvas.NewRectangle(color.Transparent)
	hover.CornerRadius = theme.InputRadiusSize()
	text := canvas.NewText(b.Text, color.Black)
	text.TextStyle = fyne.TextStyle{Bold: true}
	r := &dangerButtonRenderer{button: b, bg: bg, hover: hover, text: text}
	r.Refresh()
	return r
}

type dangerButtonRenderer struct {
	button *dangerButton
	bg     *canvas.Rectangle
	hover  *canvas.Rectangle
	text   *canvas.Text
}

const buttonPadX float32 = 14

func (r *dangerButtonRenderer) MinSize() fyne.Size {
	m := r.text.MinSize()
	return fyne.NewSize(m.Width+2*buttonPadX, controlHeight)
}

func (r *dangerButtonRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.hover.Resize(size)
	m := r.text.MinSize()
	r.text.Move(fyne.NewPos((size.Width-m.Width)/2, (size.Height-m.Height)/2))
	r.text.Resize(m)
}

func (r *dangerButtonRenderer) Refresh() {
	r.text.Text = r.button.Text
	r.text.TextSize = theme.TextSize()
	r.text.Color = theme.Color(theme.ColorNameError)
	r.bg.FillColor = theme.Color(theme.ColorNameButton)
	if r.button.Disabled() {
		r.text.Color = theme.Color(theme.ColorNameDisabled)
		r.bg.FillColor = theme.Color(theme.ColorNameDisabledButton)
	}
	r.hover.FillColor = color.Transparent
	if r.button.hovered && !r.button.Disabled() {
		r.hover.FillColor = theme.Color(theme.ColorNameHover)
	}
	r.Layout(r.button.Size())
	canvas.Refresh(r.button)
}

func (r *dangerButtonRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.hover, r.text}
}

func (r *dangerButtonRenderer) Destroy() {}
