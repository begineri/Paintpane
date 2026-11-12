package ui

import (
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
)

// Blocked images (docs/ui_refresh.md §12, S4-5C, S4-5V, S4-B): the Advanced
// page's row shows how many images are blocked, with View and Reset. View opens
// a page of thumbnails inside the Advanced page; each thumbnail has a small ×
// that unblocks that image.

// ---------------------------------------------------------------------------
// Pages inside a settings page
// ---------------------------------------------------------------------------

// subPages lets a setting open a page inside its settings page, with a link
// back to it ("‹ Advanced"). Choosing the page in the side navigation goes back
// too.
type subPages struct {
	hosts     map[string]*fyne.Container // page ID -> stack showing the page or a page inside it
	pages     map[string]fyne.CanvasObject
	titles    map[string]string
	rendering string // the page RenderPage is building
}

// pageRef names the settings page a setting was rendered on.
type pageRef struct{ id, title string }

func (s *subPages) begin(id, title string) {
	if s.titles == nil {
		s.hosts, s.pages, s.titles = map[string]*fyne.Container{}, map[string]fyne.CanvasObject{}, map[string]string{}
	}
	s.rendering = id
	s.titles[id] = title
}

func (s *subPages) current() pageRef { return pageRef{id: s.rendering, title: s.titles[s.rendering]} }

// host wraps a rendered page so a page inside it can replace it.
func (s *subPages) host(id string, page fyne.CanvasObject) fyne.CanvasObject {
	s.rendering = ""
	h := container.NewStack(page)
	s.hosts[id], s.pages[id] = h, page
	return h
}

// open shows a page inside page: a link back, the title and body.
func (s *subPages) open(page pageRef, title string, body fyne.CanvasObject) {
	h, ok := s.hosts[page.id]
	if !ok {
		return
	}
	back := newBackLink(page.title, func() { s.close(page.id) })
	head := vstack(0,
		inset(hug(back), backLinkTop, 0, 0, contentMarginX-backLinkPadX),
		inset(newPageTitle(title), 2, contentMarginX, pageTitleBottom, contentMarginX))
	h.Objects = []fyne.CanvasObject{withHeader(head, body)}
	h.Refresh()
}

// close goes back from a page inside page to the page itself.
func (s *subPages) close(id string) {
	h, ok := s.hosts[id]
	if !ok || len(h.Objects) == 1 && h.Objects[0] == s.pages[id] {
		return
	}
	h.Objects = []fyne.CanvasObject{s.pages[id]}
	h.Refresh()
}

// ---------------------------------------------------------------------------
// The row on the Advanced page
// ---------------------------------------------------------------------------

// blockedCountText is the row's help line: how many images are blocked.
func blockedCountText(n int) string {
	switch n {
	case 0:
		return i18n.T("None")
	case 1:
		return i18n.T("1 image")
	}
	return i18n.Tf("{{.Count}} images", map[string]any{"Count": n})
}

// renderBlockedImages renders the "Blocked images" row: the count, View and Reset.
func (sm *SettingsManager) renderBlockedImages(item schema.BlockedImagesItem, section *fyne.Container) {
	page := sm.subPages.current()
	count := newSecondaryText("")
	updateCount := func() {
		n := 0
		if item.List != nil {
			n = len(item.List())
		}
		count.Segments = newSecondaryText(blockedCountText(n)).Segments
		count.Refresh()
	}
	updateCount()

	var grid *blockedGrid
	unblock := func(id string) {
		if item.Unblock != nil {
			item.Unblock(id)
		}
		updateCount()
		if grid != nil {
			grid.setItems(item.List())
		}
	}
	view := widget.NewButton(i18n.T("View"), func() {
		grid = newBlockedGrid(item.List(), unblock)
		sm.subPages.open(page, i18n.T("Blocked images"), newBlockedPage(grid))
	})
	// Reset clears every block: red text like every destructive button (v43).
	reset := newDangerButton(item.ResetText, func() {
		showConfirmDialog(sm.prefsWindow, confirmSpec{
			Title:       item.ConfirmTitle,
			Message:     item.ConfirmMessage,
			ConfirmText: item.ConfirmButtonText,
			Danger:      item.ConfirmDanger,
		}, func(ok bool) {
			if !ok {
				return
			}
			if item.Reset != nil {
				item.Reset()
			}
			updateCount()
		})
	})
	sm.allWidgets[item.Name] = reset
	sm.allWidgets[item.Name+".view"] = view

	controls := container.New(&hstackLayout{gap: 8}, sizedControl(view), sizedControl(reset))
	addRow(section, newSettingRow(newRowLabel(item.Label), count, controls, false))
}

// newBlockedPage is the body of the blocked-images page: the thumbnails on the
// page (no card around them), or a line saying there are none.
func newBlockedPage(grid *blockedGrid) fyne.CanvasObject {
	empty := inset(noInset(newSecondaryText(i18n.T("No blocked images."))), 8, 0, 0, 0)
	grid.empty = empty
	grid.updateEmpty()
	scroll := container.NewVScroll(inset(vstack(0, grid, empty), 4, contentMarginX, sectionGap, contentMarginX))
	grid.onRowsChanged = scroll.Refresh
	return scroll
}

// ---------------------------------------------------------------------------
// Thumbnail grid
// ---------------------------------------------------------------------------

// Sizes from the prototype (v32, v39): 112 px wide 16:10 thumbnails, 10 px apart,
// straight on the page (no card), starting at the page's content edge; a 20 px
// round × 4 px from the corner.
const (
	blockedTileW      float32 = 112
	blockedTileH      float32 = 70
	blockedTileGap    float32 = 10
	blockedTileRadius float32 = 6
	unblockSize       float32 = 20
	unblockInset      float32 = 4
	blockedIDPadX     float32 = 2 // around the ID of an image without a thumbnail

	blockedTipDelay       = 500 * time.Millisecond
	blockedTipPadX        = float32(8)
	blockedTipPadY        = float32(5)
	blockedTipGap         = float32(4) // between a thumbnail and its tooltip
	blockedTipTextSize    = float32(12)
	blockedTipLineSpacing = float32(2)
)

// blockedGrid shows blocked images as thumbnails on the page, several per row.
// Pausing on a thumbnail shows its title, artist and source; pausing on its ×
// shows "Unblock". Fyne has no tooltips, and an overlay would stop the page
// beneath from receiving the pointer, so the grid draws them itself.
type blockedGrid struct {
	widget.BaseWidget
	items     []schema.BlockedImageSchema
	onUnblock func(id string)
	tiles     []*blockedTile

	cols          int    // columns in the last layout
	onRowsChanged func() // the row count changed, so the scroll must re-measure
	empty         fyne.CanvasObject

	tip       int  // tile whose tooltip shows, -1 for none
	tipOnX    bool // the tooltip is the ×'s
	tipCancel func()
	// after runs f on the main thread after d and returns a cancel function.
	after func(d time.Duration, f func()) func()
}

// afterOnMain runs f on Fyne's main thread after d; the returned function cancels it.
func afterOnMain(d time.Duration, f func()) func() {
	t := time.AfterFunc(d, func() { fyne.Do(f) })
	return func() { t.Stop() }
}

func newBlockedGrid(items []schema.BlockedImageSchema, onUnblock func(id string)) *blockedGrid {
	g := &blockedGrid{onUnblock: onUnblock, tip: -1, cols: 1, after: afterOnMain}
	g.ExtendBaseWidget(g)
	g.setItems(items)
	return g
}

func (g *blockedGrid) setItems(items []schema.BlockedImageSchema) {
	g.items = items
	g.tiles = make([]*blockedTile, len(items))
	for i, it := range items {
		g.tiles[i] = newBlockedTile(g, i, it)
	}
	g.hideTip()
	g.updateEmpty()
	g.Refresh()
	if g.onRowsChanged != nil {
		g.onRowsChanged()
	}
}

func (g *blockedGrid) updateEmpty() {
	if g.empty == nil {
		return
	}
	if len(g.items) == 0 {
		g.Hide()
		g.empty.Show()
	} else {
		g.Show()
		g.empty.Hide()
	}
}

// hover is called as the pointer enters or leaves a thumbnail or its ×.
func (g *blockedGrid) hover(i int, onX, in bool) {
	if g.tipCancel != nil {
		g.tipCancel()
		g.tipCancel = nil
	}
	if !in {
		if g.tip == i && g.tipOnX == onX {
			g.hideTip()
		}
		return
	}
	g.hideTip()
	g.tipCancel = g.after(blockedTipDelay, func() { g.showTip(i, onX) })
}

func (g *blockedGrid) showTip(i int, onX bool) {
	if i < 0 || i >= len(g.items) {
		return
	}
	g.tip, g.tipOnX = i, onX
	g.Refresh()
}

func (g *blockedGrid) hideTip() {
	if g.tip < 0 {
		return
	}
	g.tip = -1
	g.Refresh()
}

// tipLines is the tooltip text: "Unblock" on the ×; on a thumbnail the title
// and "artist · source", or the ID for an image blocked before details were kept.
func (g *blockedGrid) tipLines() []string {
	if g.tip < 0 || g.tip >= len(g.items) {
		return nil
	}
	if g.tipOnX {
		return []string{i18n.T("Unblock")}
	}
	it := g.items[g.tip]
	if it.Title == "" && it.Artist == "" && it.Source == "" {
		return []string{it.ID}
	}
	var lines []string
	if it.Title != "" {
		lines = append(lines, it.Title)
	}
	second := ""
	for _, part := range []string{it.Artist, it.Source} {
		if part == "" {
			continue
		}
		if second != "" {
			second += " · "
		}
		second += part
	}
	if second != "" {
		lines = append(lines, second)
	}
	return lines
}

func (g *blockedGrid) unblock(i int) {
	if i < 0 || i >= len(g.items) || g.onUnblock == nil {
		return
	}
	g.onUnblock(g.items[i].ID)
}

// colsFor is how many thumbnails fit in a row of width w.
func colsFor(w float32) int {
	n := int((w + blockedTileGap) / (blockedTileW + blockedTileGap))
	return max(n, 1)
}

func (g *blockedGrid) rows() int {
	if len(g.items) == 0 {
		return 0
	}
	return (len(g.items) + g.cols - 1) / g.cols
}

func (g *blockedGrid) CreateRenderer() fyne.WidgetRenderer {
	tipBg := canvas.NewRectangle(color.Transparent)
	tipBg.CornerRadius = 4
	tipBg.StrokeWidth = 1
	r := &blockedGridRenderer{g: g, tipBg: tipBg}
	r.Refresh()
	return r
}

type blockedGridRenderer struct {
	g        *blockedGrid
	tipBg    *canvas.Rectangle
	tipTexts []*canvas.Text
	objects  []fyne.CanvasObject
}

func (r *blockedGridRenderer) MinSize() fyne.Size {
	var h float32
	if rows := r.g.rows(); rows > 0 {
		h = float32(rows)*blockedTileH + float32(rows-1)*blockedTileGap
	}
	return fyne.NewSize(blockedTileW, h)
}

func (r *blockedGridRenderer) Layout(size fyne.Size) {
	g := r.g
	if cols := colsFor(size.Width); cols != g.cols {
		g.cols = cols
		if g.onRowsChanged != nil {
			defer g.onRowsChanged()
		}
	}
	for i, t := range g.tiles {
		t.Move(r.tilePos(i))
		t.Resize(fyne.NewSize(blockedTileW, blockedTileH))
	}
	r.layoutTip(size)
}

func (r *blockedGridRenderer) tilePos(i int) fyne.Position {
	col, row := i%r.g.cols, i/r.g.cols
	return fyne.NewPos(float32(col)*(blockedTileW+blockedTileGap),
		float32(row)*(blockedTileH+blockedTileGap))
}

// layoutTip puts the tooltip under the hovered thumbnail (or ×), or above it
// when there is no room below, within the grid's width.
func (r *blockedGridRenderer) layoutTip(size fyne.Size) {
	g := r.g
	if g.tip < 0 || len(r.tipTexts) == 0 {
		r.tipBg.Hide()
		return
	}
	maxW := size.Width - 2*blockedTipPadX
	var textW, textH float32
	for _, t := range r.tipTexts {
		t.Text = fitText(t.Text, blockedTipTextSize, t.TextStyle.Bold, maxW)
		m := t.MinSize()
		textW = fyne.Max(textW, m.Width)
		textH += m.Height
	}
	textH += float32(len(r.tipTexts)-1) * blockedTipLineSpacing
	w, h := textW+2*blockedTipPadX, textH+2*blockedTipPadY

	anchor := r.tilePos(g.tip)
	anchorH := blockedTileH
	if g.tipOnX {
		anchor = anchor.Add(fyne.NewPos(blockedTileW-unblockInset-unblockSize, unblockInset))
		anchorH = unblockSize
	}
	x := fyne.Min(anchor.X, size.Width-w)
	x = fyne.Max(x, 0)
	y := anchor.Y + anchorH + blockedTipGap
	if y+h > size.Height && anchor.Y-blockedTipGap-h >= 0 {
		y = anchor.Y - blockedTipGap - h
	}
	r.tipBg.Move(fyne.NewPos(x, y))
	r.tipBg.Resize(fyne.NewSize(w, h))
	r.tipBg.Show()
	ty := y + blockedTipPadY
	for _, t := range r.tipTexts {
		m := t.MinSize()
		t.Move(fyne.NewPos(x+blockedTipPadX, ty))
		t.Resize(m)
		ty += m.Height + blockedTipLineSpacing
	}
}

func (r *blockedGridRenderer) Refresh() {
	g := r.g
	r.tipBg.FillColor = theme.Color(theme.ColorNameOverlayBackground)
	r.tipBg.StrokeColor = theme.Color(theme.ColorNameSeparator)

	r.tipTexts = nil
	for i, line := range g.tipLines() {
		c := theme.Color(theme.ColorNameForeground)
		if i > 0 {
			c = theme.Color(theme.ColorNamePlaceHolder)
		}
		t := canvas.NewText(line, c)
		t.TextSize = blockedTipTextSize
		r.tipTexts = append(r.tipTexts, t)
	}

	r.objects = nil
	for _, t := range g.tiles {
		r.objects = append(r.objects, t)
		t.Refresh()
	}
	r.objects = append(r.objects, r.tipBg)
	for _, t := range r.tipTexts {
		r.objects = append(r.objects, t)
	}
	r.tipBg.Refresh()
	r.Layout(g.Size())
	canvas.Refresh(g)
}

func (r *blockedGridRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *blockedGridRenderer) Destroy()                     {}

// ---------------------------------------------------------------------------
// Thumbnail
// ---------------------------------------------------------------------------

// blockedTile is one thumbnail with its × in the top-right corner. An image
// blocked before thumbnails were kept shows the block icon and its ID.
type blockedTile struct {
	widget.BaseWidget
	grid  *blockedGrid
	index int
	item  schema.BlockedImageSchema
	x     *unblockButton
}

var _ desktop.Hoverable = (*blockedTile)(nil)

func newBlockedTile(g *blockedGrid, i int, item schema.BlockedImageSchema) *blockedTile {
	t := &blockedTile{grid: g, index: i, item: item}
	t.x = newUnblockButton(func() { g.unblock(i) }, func(in bool) { g.hover(i, true, in) })
	t.ExtendBaseWidget(t)
	return t
}

func (t *blockedTile) MouseIn(*desktop.MouseEvent)    { t.grid.hover(t.index, false, true) }
func (t *blockedTile) MouseMoved(*desktop.MouseEvent) {}
func (t *blockedTile) MouseOut()                      { t.grid.hover(t.index, false, false) }

func (t *blockedTile) CreateRenderer() fyne.WidgetRenderer {
	r := &blockedTileRenderer{t: t}
	if t.item.ThumbPath != "" {
		img := canvas.NewImageFromFile(t.item.ThumbPath)
		img.FillMode = canvas.ImageFillCover
		img.ScaleMode = canvas.ImageScaleSmooth
		img.CornerRadius = blockedTileRadius
		r.pic = img
	} else {
		r.ph = canvas.NewRectangle(color.Transparent)
		r.ph.CornerRadius = blockedTileRadius
		r.ph.StrokeWidth = 1
		r.icon = canvas.NewImageFromResource(nil)
		r.icon.FillMode = canvas.ImageFillContain
		r.id = canvas.NewText("", color.Transparent)
		r.id.TextSize = 11
		r.id.Alignment = fyne.TextAlignCenter
	}
	r.Refresh()
	return r
}

type blockedTileRenderer struct {
	t    *blockedTile
	pic  *canvas.Image
	ph   *canvas.Rectangle
	icon *canvas.Image
	id   *canvas.Text
}

func (r *blockedTileRenderer) MinSize() fyne.Size { return fyne.NewSize(blockedTileW, blockedTileH) }

func (r *blockedTileRenderer) Layout(size fyne.Size) {
	if r.pic != nil {
		r.pic.Move(fyne.NewPos(0, 0))
		r.pic.Resize(size)
	} else {
		r.ph.Move(fyne.NewPos(0, 0))
		r.ph.Resize(size)
		// A long ID is cut; the tooltip shows it whole.
		r.id.Text = fitText(r.t.item.ID, r.id.TextSize, false, size.Width-2*blockedIDPadX)
		th := r.id.MinSize().Height
		top := (size.Height - 20 - 4 - th) / 2
		r.icon.Move(fyne.NewPos((size.Width-20)/2, top))
		r.icon.Resize(fyne.NewSquareSize(20))
		r.id.Move(fyne.NewPos(blockedIDPadX, top+20+4))
		r.id.Resize(fyne.NewSize(size.Width-2*blockedIDPadX, th))
	}
	r.t.x.Move(fyne.NewPos(size.Width-unblockInset-unblockSize, unblockInset))
	r.t.x.Resize(fyne.NewSquareSize(unblockSize))
}

func (r *blockedTileRenderer) Refresh() {
	if r.ph != nil {
		r.ph.FillColor = theme.Color(theme.ColorNameInputBackground)
		r.ph.StrokeColor = theme.Color(theme.ColorNameInputBorder)
		r.ph.Refresh()
		r.icon.Resource = trayGlyph("block", theme.Color(theme.ColorNamePlaceHolder))
		r.icon.Refresh()
		r.id.Color = theme.Color(theme.ColorNamePlaceHolder)
		r.id.Refresh()
	}
	r.t.x.Refresh()
	r.Layout(r.t.Size())
}

func (r *blockedTileRenderer) Objects() []fyne.CanvasObject {
	if r.pic != nil {
		return []fyne.CanvasObject{r.pic, r.t.x}
	}
	return []fyne.CanvasObject{r.ph, r.icon, r.id, r.t.x}
}

func (r *blockedTileRenderer) Destroy() {}

// ---------------------------------------------------------------------------
// The × on a thumbnail
// ---------------------------------------------------------------------------

// unblockButton is a 20 px dark translucent circle with a white ×.
type unblockButton struct {
	widget.BaseWidget
	onTap   func()
	onHover func(in bool)
	hovered bool
}

var (
	_ fyne.Tappable      = (*unblockButton)(nil)
	_ desktop.Hoverable  = (*unblockButton)(nil)
	_ desktop.Cursorable = (*unblockButton)(nil)
)

func newUnblockButton(onTap func(), onHover func(bool)) *unblockButton {
	b := &unblockButton{onTap: onTap, onHover: onHover}
	b.ExtendBaseWidget(b)
	return b
}

func (b *unblockButton) Tapped(*fyne.PointEvent) {
	if b.onTap != nil {
		b.onTap()
	}
}

func (b *unblockButton) MouseIn(*desktop.MouseEvent) {
	b.hovered = true
	b.Refresh()
	if b.onHover != nil {
		b.onHover(true)
	}
}

func (b *unblockButton) MouseMoved(*desktop.MouseEvent) {}

func (b *unblockButton) MouseOut() {
	b.hovered = false
	b.Refresh()
	if b.onHover != nil {
		b.onHover(false)
	}
}

func (b *unblockButton) Cursor() desktop.Cursor { return desktop.PointerCursor }

func (b *unblockButton) CreateRenderer() fyne.WidgetRenderer {
	white := color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}
	r := &unblockRenderer{b: b, circle: canvas.NewCircle(color.Transparent),
		l1: canvas.NewLine(white), l2: canvas.NewLine(white)}
	r.l1.StrokeWidth, r.l2.StrokeWidth = 1.25, 1.25
	r.Refresh()
	return r
}

type unblockRenderer struct {
	b      *unblockButton
	circle *canvas.Circle
	l1, l2 *canvas.Line
}

func (r *unblockRenderer) MinSize() fyne.Size { return fyne.NewSquareSize(unblockSize) }

func (r *unblockRenderer) Layout(size fyne.Size) {
	r.circle.Move(fyne.NewPos(0, 0))
	r.circle.Resize(size)
	const arm = 3.5 // half the × width: a 7 px ×
	cx, cy := size.Width/2, size.Height/2
	r.l1.Position1, r.l1.Position2 = fyne.NewPos(cx-arm, cy-arm), fyne.NewPos(cx+arm, cy+arm)
	r.l2.Position1, r.l2.Position2 = fyne.NewPos(cx+arm, cy-arm), fyne.NewPos(cx-arm, cy+arm)
}

func (r *unblockRenderer) Refresh() {
	alpha := uint8(0x8C) // 55 %
	if r.b.hovered {
		alpha = 0xBF // 75 %
	}
	r.circle.FillColor = color.NRGBA{A: alpha}
	r.circle.Refresh()
	r.Layout(r.b.Size())
	r.l1.Refresh()
	r.l2.Refresh()
}

func (r *unblockRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.circle, r.l1, r.l2}
}

func (r *unblockRenderer) Destroy() {}
