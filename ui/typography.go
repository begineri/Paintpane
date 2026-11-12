package ui

import (
	"image/color"
	"reflect"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Type scale and spacing of the settings window and Tune Image
// (docs/ui_refresh.md §12, G2–G7). Values are Fyne logical pixels.
const (
	pageTitleSize    float32 = 18 // G2/G8: page title, bold
	sectionLabelSize float32 = 12 // G2/G8: section and group labels, bold, secondary colour
	backLinkSize     float32 = 13 // the "‹ Sources" link above a source's title

	contentMarginX float32 = 24 // G3: left and right margin of the content column
	cardInsetX     float32 = 16 // G3: text and controls inside a card
	rowPaddingY    float32 = 7  // G4/G8: above and below each card row
	sectionGap     float32 = 20 // G4/G8: between groups
	labelToCard    float32 = 8  // G4: from a section label to its card
	helpGap        float32 = 2  // G4: between a row's label and its help text
	rowControlGap  float32 = 24 // between a row's text and its control

	controlHeight   float32 = 28 // G5/G8: select, segmented, button and entry
	sourceRowHeight float32 = 54 // G4/G8: one source in the Sources list
	sourceRowPadY   float32 = 7  // card padding around a source row

	pageTitleTop    float32 = 18 // G7: level with the first nav item
	pageTitleBottom float32 = 14
)

// ---------------------------------------------------------------------------
// Text without the widget inset
// ---------------------------------------------------------------------------

// noInset cancels the inner padding that Fyne puts around labels, rich text
// and hyperlinks, so text lines up with the edge it is placed at (G3). It
// walks containers and wraps each text widget it finds; buttons, entries and
// selects keep their padding. It returns obj, changed in place when it is a
// container.
func noInset(obj fyne.CanvasObject) fyne.CanvasObject {
	switch o := obj.(type) {
	case *widget.Label, *widget.RichText, *widget.Hyperlink, *accentLink, *wrappedText:
		return container.New(unpadLayout{}, o)
	case *fyne.Container:
		if !walkableLayout(o.Layout) {
			return o
		}
		for i, child := range o.Objects {
			o.Objects[i] = noInset(child)
		}
		return o
	}
	return obj
}

var (
	vboxLayoutType   = reflect.TypeOf(layout.NewVBoxLayout())
	hboxLayoutType   = reflect.TypeOf(layout.NewHBoxLayout())
	centerLayoutType = reflect.TypeOf(layout.NewCenterLayout())
	stackLayoutType  = reflect.TypeOf(layout.NewStackLayout())
)

// walkableLayout reports whether noInset may replace a container's children:
// only layouts that treat every child alike. A Border layout, for one, keeps
// references to its edge objects and would lose a replaced child.
func walkableLayout(l fyne.Layout) bool {
	switch l.(type) {
	case *vstackLayout, *hstackLayout, *emptyCollapseLayout:
		return true
	}
	switch reflect.TypeOf(l) {
	case vboxLayoutType, hboxLayoutType, centerLayoutType, stackLayoutType:
		return true
	}
	return false
}

// unpadLayout lays its child out InnerPadding beyond its own bounds on every
// side, so the child's text starts at the container's edge.
type unpadLayout struct{}

func (unpadLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	p := 2 * theme.InnerPadding()
	var w, h float32
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		m := o.MinSize()
		w, h = fyne.Max(w, m.Width-p), fyne.Max(h, m.Height-p)
	}
	return fyne.NewSize(fyne.Max(w, 0), fyne.Max(h, 0))
}

func (unpadLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	p := theme.InnerPadding()
	for _, o := range objects {
		o.Move(fyne.NewPos(-p, -p))
		o.Resize(size.AddWidthHeight(2*p, 2*p))
	}
}

// ---------------------------------------------------------------------------
// Styled single-line text
// ---------------------------------------------------------------------------

// styledText is one line of text in a given size, weight and theme colour. It
// re-reads the colour on refresh, so it follows theme changes, which a bare
// canvas.Text does not.
type styledText struct {
	widget.BaseWidget
	Text       string
	size       float32
	bold       bool
	colorName  fyne.ThemeColorName
	lineHeight float32 // when set, the height the text takes; otherwise the font's
}

func newStyledText(text string, size float32, bold bool, colorName fyne.ThemeColorName) *styledText {
	t := &styledText{Text: text, size: size, bold: bold, colorName: colorName}
	t.ExtendBaseWidget(t)
	return t
}

// newPageTitle is the 18 px bold title of a page (G2, G8), on a 24 px line.
func newPageTitle(text string) *styledText {
	t := newStyledText(text, pageTitleSize, true, theme.ColorNameForeground)
	t.lineHeight = 24
	return t
}

// newSectionLabel is the 12 px bold secondary label above a card (G2, G8), on a
// 16 px line.
func newSectionLabel(text string) *styledText {
	t := newStyledText(text, sectionLabelSize, true, theme.ColorNamePlaceHolder)
	t.lineHeight = 16
	return t
}

func (t *styledText) SetText(text string) {
	t.Text = text
	t.Refresh()
}

func (t *styledText) CreateRenderer() fyne.WidgetRenderer {
	r := &styledTextRenderer{t: t, text: canvas.NewText(t.Text, color.Black)}
	r.Refresh()
	return r
}

type styledTextRenderer struct {
	t    *styledText
	text *canvas.Text
}

func (r *styledTextRenderer) Layout(size fyne.Size) {
	m := r.text.MinSize()
	r.text.Move(fyne.NewPos(0, (size.Height-m.Height)/2))
	r.text.Resize(fyne.NewSize(size.Width, m.Height))
}

func (r *styledTextRenderer) MinSize() fyne.Size {
	m := r.text.MinSize()
	if r.t.lineHeight > 0 {
		m.Height = r.t.lineHeight
	}
	return m
}

func (r *styledTextRenderer) Refresh() {
	r.text.Text = r.t.Text
	r.text.TextSize = r.t.size
	r.text.TextStyle = fyne.TextStyle{Bold: r.t.bold}
	r.text.Color = paintpaneColor(r.t.colorName)
	r.text.Refresh()
}

func (r *styledTextRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.text} }
func (r *styledTextRenderer) Destroy()                     {}

// ---------------------------------------------------------------------------
// Layout helpers
// ---------------------------------------------------------------------------

// insetLayout places its children inside fixed margins.
type insetLayout struct{ top, right, bottom, left float32 }

func (l *insetLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		m := o.MinSize()
		w, h = fyne.Max(w, m.Width), fyne.Max(h, m.Height)
	}
	return fyne.NewSize(w+l.left+l.right, h+l.top+l.bottom)
}

func (l *insetLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Move(fyne.NewPos(l.left, l.top))
		o.Resize(fyne.NewSize(size.Width-l.left-l.right, size.Height-l.top-l.bottom))
	}
}

func inset(obj fyne.CanvasObject, top, right, bottom, left float32) *fyne.Container {
	return container.New(&insetLayout{top: top, right: right, bottom: bottom, left: left}, obj)
}

// vstackLayout stacks visible children with a fixed gap and no other spacing.
type vstackLayout struct{ gap float32 }

func (l *vstackLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w, h float32
	n := 0
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		m := o.MinSize()
		w = fyne.Max(w, m.Width)
		h += m.Height
		n++
	}
	if n > 1 {
		h += l.gap * float32(n-1)
	}
	return fyne.NewSize(w, h)
}

func (l *vstackLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	var y float32
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		h := o.MinSize().Height
		o.Move(fyne.NewPos(0, y))
		o.Resize(fyne.NewSize(size.Width, h))
		y += h + l.gap
	}
}

func vstack(gap float32, objects ...fyne.CanvasObject) *fyne.Container {
	return container.New(&vstackLayout{gap: gap}, objects...)
}

// fixedHeightLayout gives its child a fixed height and its natural width.
type fixedHeightLayout struct{ height float32 }

func (l *fixedHeightLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w float32
	for _, o := range objects {
		w = fyne.Max(w, o.MinSize().Width)
	}
	return fyne.NewSize(w, l.height)
}

func (l *fixedHeightLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Move(fyne.NewPos(0, (size.Height-l.height)/2))
		o.Resize(fyne.NewSize(size.Width, l.height))
	}
}

// sizedControl gives selects, segmented controls and buttons the one control
// height (G5). Other objects are returned unchanged. Entries keep Fyne's height
// (36 px with this font): their text sits in a scroller that would overflow
// and draw a shadow if the entry were shorter.
func sizedControl(obj fyne.CanvasObject) fyne.CanvasObject {
	switch obj.(type) {
	case *widget.Select, *widget.Button, *segmented, *dangerButton, *borderedSelect:
		return container.New(&fixedHeightLayout{height: controlHeight}, obj)
	}
	return obj
}

// ---------------------------------------------------------------------------
// Select with an input border
// ---------------------------------------------------------------------------

// borderedSelect draws the 1 px input border around a select, as segmented
// controls and entries have (G5). Fyne's select has none.
type borderedSelect struct {
	widget.BaseWidget
	sel *widget.Select
}

func newBorderedSelect(sel *widget.Select) *borderedSelect {
	b := &borderedSelect{sel: sel}
	b.ExtendBaseWidget(b)
	return b
}

func (b *borderedSelect) CreateRenderer() fyne.WidgetRenderer {
	border := canvas.NewRectangle(color.Transparent)
	border.StrokeWidth = 1
	r := &borderedSelectRenderer{b: b, border: border}
	r.Refresh()
	return r
}

type borderedSelectRenderer struct {
	b      *borderedSelect
	border *canvas.Rectangle
}

func (r *borderedSelectRenderer) Layout(size fyne.Size) {
	r.b.sel.Move(fyne.NewPos(0, 0))
	r.b.sel.Resize(size)
	r.border.Move(fyne.NewPos(0, 0))
	r.border.Resize(size)
}

func (r *borderedSelectRenderer) MinSize() fyne.Size { return r.b.sel.MinSize() }

func (r *borderedSelectRenderer) Refresh() {
	r.border.StrokeColor = theme.Color(theme.ColorNameInputBorder)
	r.border.CornerRadius = theme.InputRadiusSize()
	r.border.Refresh()
	r.b.sel.Refresh()
}

func (r *borderedSelectRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.b.sel, r.border}
}
func (r *borderedSelectRenderer) Destroy() {}

// inlineGap separates the items of a line of links (G6).
const inlineGap float32 = 20

// hstackLayout places visible children side by side at their natural width,
// a fixed gap apart, all as tall as the row.
type hstackLayout struct{ gap float32 }

func (l *hstackLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w, h float32
	n := 0
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		m := o.MinSize()
		w += m.Width
		h = fyne.Max(h, m.Height)
		n++
	}
	if n > 1 {
		w += l.gap * float32(n-1)
	}
	return fyne.NewSize(w, h)
}

func (l *hstackLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	var x float32
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		w := o.MinSize().Width
		o.Move(fyne.NewPos(x, 0))
		o.Resize(fyne.NewSize(w, size.Height))
		x += w + l.gap
	}
}

// vcenterLayout gives its child the full width and its natural height,
// centred vertically.
type vcenterLayout struct{}

func (vcenterLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, o := range objects {
		m := o.MinSize()
		w, h = fyne.Max(w, m.Width), fyne.Max(h, m.Height)
	}
	return fyne.NewSize(w, h)
}

func (vcenterLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		h := o.MinSize().Height
		o.Move(fyne.NewPos(0, (size.Height-h)/2))
		o.Resize(fyne.NewSize(size.Width, h))
	}
}

func vcenter(obj fyne.CanvasObject) *fyne.Container {
	return container.New(vcenterLayout{}, obj)
}

// ---------------------------------------------------------------------------
// Thin chevrons (G6)
// ---------------------------------------------------------------------------

// Filled outlines 1.5 px wide (in a 24 px box): Fyne recolours an SVG's fill,
// not its stroke.
var (
	chevronRight = fyne.NewStaticResource("chevron-right.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M9.5 6.5L8.44 7.56L12.88 12L8.44 16.44L9.5 17.5L15 12Z" fill="#000000"/></svg>`))
	chevronDown  = fyne.NewStaticResource("chevron-down.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M17.5 9.5L16.44 8.44L12 12.88L7.56 8.44L6.5 9.5L12 15Z" fill="#000000"/></svg>`))
	chevronLeft  = fyne.NewStaticResource("chevron-left.svg", []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M14.5 6.5L15.56 7.56L11.12 12L15.56 16.44L14.5 17.5L9 12Z" fill="#000000"/></svg>`))
)

// newChevron is a thin chevron in the secondary text colour, size px square.
func newChevron(shape fyne.Resource, size float32) fyne.CanvasObject {
	icon := widget.NewIcon(theme.NewColoredResource(shape, theme.ColorNamePlaceHolder))
	return container.New(&fixedSizeLayout{size: fyne.NewSquareSize(size)}, icon)
}

// ---------------------------------------------------------------------------
// Tap area and back link
// ---------------------------------------------------------------------------

// tapArea makes its content tappable without adding a button's size or
// background, for example a group label that collapses its group.
type tapArea struct {
	widget.BaseWidget
	content fyne.CanvasObject
	onTap   func()
}

var (
	_ fyne.Tappable      = (*tapArea)(nil)
	_ desktop.Cursorable = (*tapArea)(nil)
)

func newTapArea(content fyne.CanvasObject, onTap func()) *tapArea {
	t := &tapArea{content: content, onTap: onTap}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tapArea) Tapped(_ *fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}

func (t *tapArea) Cursor() desktop.Cursor { return desktop.PointerCursor }

func (t *tapArea) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(t.content)
}

// Back link: "‹ Sources" above a source's title, 13 px secondary text.
const (
	backLinkTop    float32 = 14
	backLinkHeight float32 = 28
	backLinkPadX   float32 = 8
	sourcesListTop float32 = 20 // from the search box to the first group label
)

type backLink struct {
	widget.BaseWidget
	text    string
	onTap   func()
	hovered bool
}

var (
	_ fyne.Tappable      = (*backLink)(nil)
	_ desktop.Hoverable  = (*backLink)(nil)
	_ desktop.Cursorable = (*backLink)(nil)
)

func newBackLink(text string, onTap func()) *backLink {
	b := &backLink{text: text, onTap: onTap}
	b.ExtendBaseWidget(b)
	return b
}

func (b *backLink) Tapped(_ *fyne.PointEvent) {
	if b.onTap != nil {
		b.onTap()
	}
}
func (b *backLink) MouseIn(_ *desktop.MouseEvent)    { b.hovered = true; b.Refresh() }
func (b *backLink) MouseMoved(_ *desktop.MouseEvent) {}
func (b *backLink) MouseOut()                        { b.hovered = false; b.Refresh() }
func (b *backLink) Cursor() desktop.Cursor           { return desktop.PointerCursor }

func (b *backLink) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.Transparent)
	bg.CornerRadius = theme.SelectionRadiusSize()
	r := &backLinkRenderer{b: b, bg: bg, icon: newChevron(chevronLeft, 16),
		label: newStyledText(b.text, backLinkSize, false, theme.ColorNamePlaceHolder)}
	r.Refresh()
	return r
}

type backLinkRenderer struct {
	b     *backLink
	bg    *canvas.Rectangle
	icon  fyne.CanvasObject
	label *styledText
}

func (r *backLinkRenderer) MinSize() fyne.Size {
	return fyne.NewSize(backLinkPadX+16+2+r.label.MinSize().Width+backLinkPadX, backLinkHeight)
}

func (r *backLinkRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.icon.Move(fyne.NewPos(backLinkPadX, (size.Height-16)/2))
	r.icon.Resize(fyne.NewSquareSize(16))
	m := r.label.MinSize()
	r.label.Move(fyne.NewPos(backLinkPadX+16+2, (size.Height-m.Height)/2))
	r.label.Resize(m)
}

func (r *backLinkRenderer) Refresh() {
	r.bg.FillColor = color.Transparent
	if r.b.hovered {
		r.bg.FillColor = theme.Color(theme.ColorNameHover)
	}
	r.bg.Refresh()
	r.label.SetText(r.b.text)
	r.icon.Refresh()
	r.Layout(r.b.Size())
}

func (r *backLinkRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.icon, r.label}
}
func (r *backLinkRenderer) Destroy() {}

// headerBodyLayout puts the first child on top at its natural height and gives
// the second the rest, with no padding between them (Fyne's Border adds 4 px).
type headerBodyLayout struct{}

func (headerBodyLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	h, b := objects[0].MinSize(), objects[1].MinSize()
	return fyne.NewSize(fyne.Max(h.Width, b.Width), h.Height+b.Height)
}

func (headerBodyLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	hh := objects[0].MinSize().Height
	objects[0].Move(fyne.NewPos(0, 0))
	objects[0].Resize(fyne.NewSize(size.Width, hh))
	objects[1].Move(fyne.NewPos(0, hh))
	objects[1].Resize(fyne.NewSize(size.Width, size.Height-hh))
}

// withHeader stacks a page's fixed header (title, search box) above its
// scrolling body.
func withHeader(header, body fyne.CanvasObject) *fyne.Container {
	return container.New(headerBodyLayout{}, header, body)
}

// fillLastLayout places children side by side with no gap: each at its natural
// width except the last, which takes the remaining width. All are full height.
type fillLastLayout struct{}

func (fillLastLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var w, h float32
	for _, o := range objects {
		m := o.MinSize()
		w += m.Width
		h = fyne.Max(h, m.Height)
	}
	return fyne.NewSize(w, h)
}

func (fillLastLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	var x float32
	for i, o := range objects {
		w := o.MinSize().Width
		if i == len(objects)-1 {
			w = size.Width - x
		}
		o.Move(fyne.NewPos(x, 0))
		o.Resize(fyne.NewSize(w, size.Height))
		x += w
	}
}
