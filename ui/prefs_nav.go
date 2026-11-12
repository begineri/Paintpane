package ui

import (
	"bytes"
	"image"
	"image/color"
	_ "image/png" // source logos
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
)

const navWidth float32 = 168

// Page IDs used by the preferences window.
const (
	pageSources   = "sources"
	pageWallpaper = "wallpaper"
	pageGeneral   = "general"
	pageAdvanced  = "advanced"
)

// navPage is one entry of the preferences side navigation.
type navPage struct {
	id      string
	title   string
	icon    fyne.Resource
	content fyne.CanvasObject
	// reselect, if set, runs when the page's nav item is chosen while the page
	// is already showing, e.g. to go back from a source's detail to the list.
	reselect func()
}

// prefsNavigation shows a side navigation and the selected page next to it.
type prefsNavigation struct {
	pages   []navPage
	current string
	items   map[string]*navItem
	content *fyne.Container
}

// Side navigation spacing (G7): the first item is level with the page title,
// items are 2 px apart.
const (
	navPadTop   float32 = 14
	navPadX     float32 = 8
	navItemGap  float32 = 2
	navItemH    float32 = 32
	navItemPadX float32 = 10
)

func newPrefsNavigation(pages []navPage, initial string) (*prefsNavigation, fyne.CanvasObject) {
	n := &prefsNavigation{pages: pages, items: map[string]*navItem{}, content: container.NewStack()}
	list := vstack(navItemGap)
	for _, p := range pages {
		id := p.id
		item := newNavItem(p.title, p.icon, func() { n.Select(id) })
		n.items[id] = item
		list.Add(item)
		n.content.Add(p.content)
	}
	if initial == "" && len(pages) > 0 {
		initial = pages[0].id
	}
	n.Select(initial)

	navBg := canvas.NewRectangle(paintpaneColor(colorNameSurface))
	nav := container.NewStack(navBg, inset(list, navPadTop, navPadX, navPadX, navPadX))
	// The navigation is navWidth wide including its 1 px divider, and the page
	// starts right after it, with no padding (G3: the page's own margin is 24).
	sizedNav := container.New(&fixedWidthLayout{width: navWidth - 1}, nav)
	divider := canvas.NewRectangle(theme.Color(theme.ColorNameSeparator))
	divider.SetMinSize(fyne.NewSize(1, 0))

	root := container.New(fillLastLayout{}, sizedNav, divider, n.content)
	return n, newThemeWatcher(root, func() {
		navBg.FillColor = paintpaneColor(colorNameSurface)
		navBg.Refresh()
		divider.FillColor = theme.Color(theme.ColorNameSeparator)
		divider.Refresh()
	})
}

// Select shows a page by ID. Unknown IDs are ignored.
func (n *prefsNavigation) Select(id string) {
	found := false
	for _, p := range n.pages {
		if p.id == id {
			found = true
		}
	}
	if !found {
		return
	}
	if id == n.current {
		for _, p := range n.pages {
			if p.id == id && p.reselect != nil {
				p.reselect()
			}
		}
		return
	}
	n.current = id
	for i, p := range n.pages {
		obj := n.content.Objects[i]
		if p.id == id {
			obj.Show()
		} else {
			obj.Hide()
		}
		n.items[p.id].setSelected(p.id == id)
	}
	n.content.Refresh()
}

// Current returns the selected page ID.
func (n *prefsNavigation) Current() string { return n.current }

// fixedWidthLayout gives its single child a fixed width and the full height.
type fixedWidthLayout struct{ width float32 }

func (f *fixedWidthLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var h float32
	for _, o := range objects {
		h = fyne.Max(h, o.MinSize().Height)
	}
	return fyne.NewSize(f.width, h)
}

func (f *fixedWidthLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Move(fyne.NewPos(0, 0))
		o.Resize(size)
	}
}

// ---------------------------------------------------------------------------
// Navigation item
// ---------------------------------------------------------------------------

type navItem struct {
	widget.BaseWidget
	title    string
	icon     fyne.Resource
	selected bool
	hovered  bool
	onTap    func()
}

var (
	_ fyne.Tappable      = (*navItem)(nil)
	_ desktop.Hoverable  = (*navItem)(nil)
	_ desktop.Cursorable = (*navItem)(nil)
)

func newNavItem(title string, icon fyne.Resource, onTap func()) *navItem {
	n := &navItem{title: title, icon: icon, onTap: onTap}
	n.ExtendBaseWidget(n)
	return n
}

func (n *navItem) setSelected(selected bool) {
	n.selected = selected
	n.Refresh()
}

func (n *navItem) Tapped(_ *fyne.PointEvent) {
	if n.onTap != nil {
		n.onTap()
	}
}
func (n *navItem) MouseIn(_ *desktop.MouseEvent)    { n.hovered = true; n.Refresh() }
func (n *navItem) MouseMoved(_ *desktop.MouseEvent) {}
func (n *navItem) MouseOut()                        { n.hovered = false; n.Refresh() }
func (n *navItem) Cursor() desktop.Cursor           { return desktop.PointerCursor }

func (n *navItem) CreateRenderer() fyne.WidgetRenderer {
	bg := canvas.NewRectangle(color.Transparent)
	bg.CornerRadius = theme.SelectionRadiusSize()
	label := canvas.NewText(n.title, color.Black)
	var icon *widget.Icon
	if n.icon != nil {
		icon = widget.NewIcon(n.icon)
	}
	r := &navItemRenderer{item: n, bg: bg, label: label, icon: icon}
	r.Refresh()
	return r
}

type navItemRenderer struct {
	item  *navItem
	bg    *canvas.Rectangle
	label *canvas.Text
	icon  *widget.Icon
}

func (r *navItemRenderer) MinSize() fyne.Size {
	h := fyne.Max(navItemH, fyne.Max(r.label.MinSize().Height, theme.IconInlineSize()))
	w := r.label.MinSize().Width + 2*navItemPadX
	if r.icon != nil {
		w += theme.IconInlineSize() + navItemPadX
	}
	return fyne.NewSize(w, h)
}

func (r *navItemRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	x := navItemPadX
	if r.icon != nil {
		s := theme.IconInlineSize()
		r.icon.Move(fyne.NewPos(x, (size.Height-s)/2))
		r.icon.Resize(fyne.NewSize(s, s))
		x += s + navItemPadX
	}
	lh := r.label.MinSize().Height
	r.label.Move(fyne.NewPos(x, (size.Height-lh)/2))
	r.label.Resize(fyne.NewSize(size.Width-x-navItemPadX, lh))
}

func (r *navItemRenderer) Refresh() {
	r.label.Text = r.item.title
	r.label.TextSize = theme.TextSize()
	switch {
	case r.item.selected:
		r.bg.FillColor = paintpaneColor(colorNameSelectedTint)
		r.label.Color = paintpaneColor(colorNameSelectedText)
		r.label.TextStyle = fyne.TextStyle{Bold: true}
	case r.item.hovered:
		r.bg.FillColor = theme.Color(theme.ColorNameHover)
		r.label.Color = theme.Color(theme.ColorNameForeground)
		r.label.TextStyle = fyne.TextStyle{}
	default:
		r.bg.FillColor = color.Transparent
		r.label.Color = theme.Color(theme.ColorNameForeground)
		r.label.TextStyle = fyne.TextStyle{}
	}
	r.Layout(r.item.Size())
	canvas.Refresh(r.item)
}

func (r *navItemRenderer) Objects() []fyne.CanvasObject {
	objs := []fyne.CanvasObject{r.bg}
	if r.icon != nil {
		objs = append(objs, r.icon)
	}
	return append(objs, r.label)
}

func (r *navItemRenderer) Destroy() {}

// ---------------------------------------------------------------------------
// Theme watcher
// ---------------------------------------------------------------------------

// themeWatcher re-colours plain canvas objects (which do not follow the theme by
// themselves) whenever Fyne refreshes the tree, for example after a theme change.
type themeWatcher struct {
	widget.BaseWidget
	content fyne.CanvasObject
	recolor func()
}

func newThemeWatcher(content fyne.CanvasObject, recolor func()) *themeWatcher {
	w := &themeWatcher{content: content, recolor: recolor}
	w.ExtendBaseWidget(w)
	return w
}

func (w *themeWatcher) CreateRenderer() fyne.WidgetRenderer {
	return &themeWatcherRenderer{w: w}
}

type themeWatcherRenderer struct{ w *themeWatcher }

func (r *themeWatcherRenderer) Layout(size fyne.Size)        { r.w.content.Resize(size) }
func (r *themeWatcherRenderer) MinSize() fyne.Size           { return r.w.content.MinSize() }
func (r *themeWatcherRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.w.content} }
func (r *themeWatcherRenderer) Destroy()                     {}
func (r *themeWatcherRenderer) Refresh() {
	if r.w.recolor != nil {
		r.w.recolor()
	}
	r.w.content.Refresh()
}

// ---------------------------------------------------------------------------
// Pages
// ---------------------------------------------------------------------------

// newPageHeading returns a page title: 20 px bold (G2), on the content's left
// edge (G3) and level with the first nav item (G7).
func newPageHeading(text string) fyne.CanvasObject {
	return inset(newPageTitle(text), pageTitleTop, contentMarginX, pageTitleBottom, contentMarginX)
}

// RenderPage renders one preferences page: its heading and its content.
func (sm *SettingsManager) RenderPage(item schema.TabItemSchema) fyne.CanvasObject {
	sm.subPages.begin(item.ID, item.Title)
	var body fyne.CanvasObject
	switch {
	case item.Sources != nil:
		body = sm.renderSources(*item.Sources)
	case item.Panel != nil:
		body = sm.RenderSchema(*item.Panel)
	case item.Accordion != nil:
		body = sm.RenderAccordion(*item.Accordion)
	default:
		body = container.NewVBox()
	}
	if item.Sources != nil {
		// The Sources page draws its own heading: the page title on the list, the
		// source's name in a detail view.
		return body
	}
	return sm.subPages.host(item.ID, withHeader(newPageHeading(item.Title), body))
}

// renderSources renders a searchable, grouped list of sources. Selecting one
// replaces the list with its detail panel and a link back.
func (sm *SettingsManager) renderSources(src schema.SourcesSchema) fyne.CanvasObject {
	details := map[string]fyne.CanvasObject{}
	titles := map[string]string{}
	for _, g := range src.Groups {
		for _, item := range g.Items {
			if item.Detail != nil {
				details[item.ID] = sm.RenderSchema(*item.Detail)
			} else {
				details[item.ID] = container.NewVBox()
			}
			titles[item.ID] = item.Title
		}
	}

	collapsed := map[string]bool{}
	for _, g := range src.Groups {
		collapsed[g.ID] = g.Collapsed
	}

	view := container.NewStack()
	search := widget.NewEntry()
	search.SetPlaceHolder(i18n.T("Search museums and sources"))
	// Groups: label 8 px above its card, groups 24 px apart (G4).
	listBox := vstack(sectionGap)

	var showList func()
	var showDetail func(id string)
	var rebuildList func()

	rebuildList = func() {
		query := strings.ToLower(strings.TrimSpace(search.Text))
		listBox.Objects = nil
		for _, g := range src.Groups {
			var rows []fyne.CanvasObject
			for _, item := range g.Items {
				if query != "" && !strings.Contains(strings.ToLower(item.Title), query) {
					continue
				}
				id := item.ID
				rows = append(rows, sm.newSourceRow(item, func() { showDetail(id) }))
			}
			if len(rows) == 0 {
				continue
			}
			groupID := g.ID
			open := query != "" || !collapsed[groupID]
			header := newGroupHeader(g.Title, len(rows), open, func() {
				collapsed[groupID] = !collapsed[groupID]
				rebuildList()
			})
			group := vstack(labelToCard, header)
			if open {
				c := newCard(rows...)
				c.padY = sourceRowPadY
				group.Add(c)
			}
			listBox.Add(group)
		}
		listBox.Refresh()
	}
	search.OnChanged = func(string) { rebuildList() }

	searchBox := inset(search, 0, contentMarginX, 0, contentMarginX) // 36 px, Fyne's entry height
	listView := withHeader(vstack(0, newPageHeading(i18n.T("Sources")), searchBox),
		container.NewVScroll(inset(listBox, sourcesListTop, contentMarginX, sectionGap, contentMarginX)))

	// In a detail view a small "‹ Sources" link goes back and the source's name is the page title.
	backBtn := newBackLink(i18n.T("Sources"), nil)
	detailTitle := newPageTitle("")
	detailHead := vstack(0,
		inset(hug(backBtn), backLinkTop, 0, 0, contentMarginX-backLinkPadX),
		inset(detailTitle, 2, contentMarginX, pageTitleBottom, contentMarginX))
	detailBody := container.NewStack()
	detailView := withHeader(detailHead, detailBody)

	showList = func() {
		rebuildList()
		view.Objects = []fyne.CanvasObject{listView}
		view.Refresh()
	}
	showDetail = func(id string) {
		body, ok := details[id]
		if !ok {
			return
		}
		detailTitle.SetText(titles[id])
		detailBody.Objects = []fyne.CanvasObject{body}
		detailBody.Refresh()
		view.Objects = []fyne.CanvasObject{detailView}
		view.Refresh()
	}
	backBtn.onTap = showList
	sm.sourcesShowList = showList

	// Counts in the list change when collections are switched on or off.
	sm.RegisterOnSettingsSaved(func() { rebuildList() })

	showList()
	if src.OpenSourceID != "" {
		showDetail(src.OpenSourceID)
	}
	return view
}

// newSourceRow is one tappable row in the sources list.
func (sm *SettingsManager) newSourceRow(item schema.SourceSchema, onTap func()) fyne.CanvasObject {
	var icon fyne.Resource
	var decoded image.Image
	if len(item.IconBytes) > 0 {
		// Decode a PNG logo once: an image made from a resource decodes it again on every
		// refresh, and changing Appearance or Accent color refreshes every row of the list.
		if img, _, err := image.Decode(bytes.NewReader(item.IconBytes)); err == nil {
			decoded = img
		} else {
			icon = fyne.NewStaticResource(item.ID+"-icon", item.IconBytes) // e.g. SVG
		}
	} else if item.IconName != "" {
		icon = sm.resolveIcon(item.IconName)
	}

	title := newRowLabel(item.Title)
	var subtitle fyne.CanvasObject
	if item.SubtitleFunc != nil {
		if text := item.SubtitleFunc(); text != "" {
			subtitle = newSecondaryText(text)
		}
	}
	var left fyne.CanvasObject
	if icon != nil || decoded != nil {
		img := canvas.NewImageFromResource(icon)
		if decoded != nil {
			img = canvas.NewImageFromImage(decoded)
		}
		img.FillMode = canvas.ImageFillContain
		img.SetMinSize(fyne.NewSize(24, 24))
		// 12 px from the logo to the text, with the Border's own padding.
		left = inset(container.NewCenter(img), 0, 12-theme.Padding(), 0, 0)
	}
	text := vcenter(noInset(vstack(1, nonNilObjects(title, subtitle)...)))
	chevron := container.NewCenter(newChevron(chevronRight, 20))

	content := container.NewBorder(nil, nil, left, chevron, text)
	tapTarget := widget.NewButton("", onTap)
	tapTarget.Importance = widget.LowImportance
	row := container.NewStack(tapTarget, content)
	return container.New(&fixedHeightLayout{height: sourceRowHeight - 2*sourceRowPadY}, row)
}

// newGroupHeader is a group label that collapses or expands its group: a thin
// grey chevron and a 13 px bold label (G2, G6).
func newGroupHeader(title string, count int, open bool, onTap func()) fyne.CanvasObject {
	shape := chevronRight
	if open {
		shape = chevronDown
	}
	label := newSectionLabel(title + " · " + strconv.Itoa(count))
	content := container.New(&hstackLayout{gap: 4}, container.NewCenter(newChevron(shape, 16)), container.NewCenter(label))
	return newTapArea(container.NewHBox(content, layout.NewSpacer()), onTap)
}

func nonNilObjects(objs ...fyne.CanvasObject) []fyne.CanvasObject {
	var out []fyne.CanvasObject
	for _, o := range objs {
		if o != nil {
			out = append(out, o)
		}
	}
	return out
}
