package ui

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	spiceui "github.com/dixieflatline76/Spice/v2/pkg/ui"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/setting"
)

// blockedPlugin supplies an Advanced page with the "Blocked images" row over
// an in-memory list, like the wallpaper plugin's (S4-5, S4-B).
type blockedPlugin struct {
	items     []schema.BlockedImageSchema
	unblocked []string
	resets    int
}

func (p *blockedPlugin) ID() string                         { return "blocked" }
func (p *blockedPlugin) Name() string                       { return "Blocked" }
func (p *blockedPlugin) Activate()                          {}
func (p *blockedPlugin) Deactivate()                        {}
func (p *blockedPlugin) CreateTrayMenu() *schema.MenuSchema { return nil }
func (p *blockedPlugin) Init(spiceui.PluginManager)         {}
func (p *blockedPlugin) CreatePrefsSchema(setting.SettingsManager) *schema.TabsSchema {
	return &schema.TabsSchema{Items: []schema.TabItemSchema{{ID: pageAdvanced, Title: "Advanced",
		Panel: &schema.PanelSchema{Sections: []schema.SectionSchema{{
			Title: i18n.T("Storage"),
			Items: []schema.ItemSchema{schema.BlockedImagesItem{
				Name:           "resetAvoidSet",
				Label:          i18n.T("Blocked images"),
				ResetText:      i18n.T("Reset"),
				ConfirmTitle:   i18n.T("Please Confirm"),
				ConfirmMessage: i18n.T("This cannot be undone. Are you sure?"),
				List: func() []schema.BlockedImageSchema {
					return append([]schema.BlockedImageSchema(nil), p.items...)
				},
				Unblock: func(id string) {
					p.unblocked = append(p.unblocked, id)
					for i, it := range p.items {
						if it.ID == id {
							p.items = append(p.items[:i], p.items[i+1:]...)
							break
						}
					}
				},
				Reset: func() { p.resets++; p.items = nil },
			}},
		}}},
	}}}
}

// sampleThumb writes a small JPEG to stand in for a saved thumbnail.
func sampleThumb(t *testing.T, w, h int, c color.RGBA) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: c.R, G: uint8(int(c.G) * y / h), B: uint8(int(c.B) * x / w), A: 0xFF})
		}
	}
	path := filepath.Join(t.TempDir(), "thumb.jpg")
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	require.NoError(t, jpeg.Encode(f, img, nil))
	return path
}

// sampleBlocked is the prototype's list: four images with details, then one
// blocked before details were kept.
func sampleBlocked(t *testing.T) []schema.BlockedImageSchema {
	return []schema.BlockedImageSchema{
		{ID: "Rijksmuseum_1", Title: "The Night Watch", Artist: "Rembrandt van Rijn", Source: i18n.T("Rijksmuseum"),
			ThumbPath: sampleThumb(t, 360, 300, color.RGBA{R: 0x70, G: 0x50, B: 0x30})},
		{ID: "Getty_2", Title: "Irises", Artist: "Vincent van Gogh", Source: i18n.T("The Getty"),
			ThumbPath: sampleThumb(t, 360, 280, color.RGBA{R: 0x30, G: 0x60, B: 0xA0})},
		{ID: "CMA_3", Title: "Mount Starr King, Yosemite", Artist: "Albert Bierstadt", Source: i18n.T("Cleveland Museum of Art"),
			ThumbPath: sampleThumb(t, 360, 240, color.RGBA{R: 0x90, G: 0x80, B: 0x60})},
		{ID: "Wallhaven_9dkeqx", Title: "wallhaven-9dkeqx", Source: "Wallhaven",
			ThumbPath: sampleThumb(t, 240, 360, color.RGBA{R: 0x20, G: 0x40, B: 0x50})},
		{ID: "MetMuseum_436535"},
	}
}

func (s *scenario) blockedGrid() *blockedGrid {
	s.t.Helper()
	return s.findShown(func(o fyne.CanvasObject) bool { _, ok := o.(*blockedGrid); return ok }).(*blockedGrid)
}

// confirm answers the confirmation dialog on top of the window.
func (s *scenario) confirm(yes bool) {
	s.t.Helper()
	buttons := dialogButtons(s)
	require.Len(s.t, buttons, 2, "a dialog with Cancel and the confirming button is open")
	if yes {
		test.Tap(buttons[1])
	} else {
		test.Tap(buttons[0])
	}
}

func TestScenarioBlockedImages(t *testing.T) {
	inLanguages(t, func(t *testing.T, lang string) {
		want := map[string]struct{ advanced, view, title, five, four, none, empty, unblock string }{
			"en": {"Advanced", "View", "Blocked images", "5 images", "4 images", "None", "No blocked images.", "Unblock"},
			"zh": {"高级", "查看", "已屏蔽的图片", "5 张", "4 张", "没有", "没有已屏蔽的图片。", "取消屏蔽"},
		}[lang]
		p := &blockedPlugin{items: sampleBlocked(t)}
		s := newScenarioWith(t, p)
		s.tapText(want.advanced)
		require.Equal(t, pageAdvanced, s.sa.prefsNav.Current())

		// S4-5C: the row says how many images are blocked.
		assert.True(t, s.shows(want.title))
		assert.True(t, s.shows(want.five))

		// S4-5V: View opens the thumbnails inside the Advanced page, with a link back.
		s.tapText(want.view)
		assert.False(t, s.shows(want.five), "the Advanced page is replaced")
		assert.True(t, s.shows(want.title), "the page title")
		g := s.blockedGrid()
		require.Len(t, g.tiles, 5)

		// S4-B (v39): 112 × 70 thumbnails, 10 px apart, straight on the page with no
		// card around them, starting on the page title's left edge.
		pos0, size0 := s.absRect(g.tiles[0])
		pos1, _ := s.absRect(g.tiles[1])
		gpos, _ := s.absRect(g)
		assert.Equal(t, fyne.NewSize(112, 70), size0)
		assert.Equal(t, float32(122), pos1.X-pos0.X)
		assert.Equal(t, fyne.NewPos(0, 0), fyne.NewPos(pos0.X-gpos.X, pos0.Y-gpos.Y), "no padding inside the grid")
		title := s.findShown(func(o fyne.CanvasObject) bool {
			st, ok := o.(*styledText)
			return ok && st.Text == want.title
		})
		titlePos, _ := s.absRect(title)
		assert.Equal(t, titlePos.X, pos0.X, "thumbnails on the page title's left edge")
		for _, o := range test.WidgetRenderer(g).Objects() {
			if r, ok := o.(*canvas.Rectangle); ok {
				assert.NotEqual(t, g.Size(), r.Size(), "no card behind the thumbnails")
			}
		}
		assert.Equal(t, fyne.NewSize(112, float32(g.rows())*70+float32(g.rows()-1)*10).Height, g.MinSize().Height, "no padding around the rows")
		assert.Greater(t, g.cols, 2, "several per row")
		xpos, xsize := s.absRect(g.tiles[0].x)
		assert.Equal(t, fyne.NewSquareSize(20), xsize, "a 20 px ×")
		assert.Equal(t, pos0.X+112-4-20, xpos.X, "in the top-right corner")
		assert.Equal(t, pos0.Y+4, xpos.Y)

		// S4-B5: an image blocked before details were kept shows its ID.
		assert.True(t, s.shows(fitText("MetMuseum_436535", 11, false, 112-4)), "cut to the tile if too long")

		// Pausing on a thumbnail shows its title, artist and source; on the ×, Unblock.
		var delay time.Duration
		g.after = func(d time.Duration, f func()) func() { delay = d; f(); return func() {} }
		g.tiles[1].MouseIn(nil)
		assert.Equal(t, 500*time.Millisecond, delay, "after a pause")
		assert.Equal(t, 1, g.tip)
		assert.Equal(t, []string{"Irises", "Vincent van Gogh · " + i18n.T("The Getty")}, g.tipLines())
		g.tiles[1].MouseOut()
		assert.Equal(t, -1, g.tip, "the tooltip goes when the pointer leaves")
		g.showTip(4, false)
		assert.Equal(t, []string{"MetMuseum_436535"}, g.tipLines())
		g.showTip(0, true)
		assert.Equal(t, []string{want.unblock}, g.tipLines())
		g.hideTip()

		// The × unblocks that image only; the count on Advanced follows.
		test.Tap(g.tiles[1].x)
		assert.Equal(t, []string{"Getty_2"}, p.unblocked)
		assert.Len(t, g.tiles, 4)
		assert.Equal(t, "CMA_3", g.tiles[1].item.ID)
		back := s.findShown(func(o fyne.CanvasObject) bool { _, ok := o.(*backLink); return ok }).(*backLink)
		assert.Equal(t, want.advanced, back.text)
		test.Tap(back)
		assert.True(t, s.shows(want.four))

		// Choosing Advanced in the side navigation also goes back.
		s.tapText(want.view)
		s.sa.prefsNav.Select(pageAdvanced)
		assert.True(t, s.shows(want.four))

		// Reset still clears the whole list, after a confirmation.
		s.tapText(i18n.T("Reset"))
		s.confirm(true)
		assert.Equal(t, 1, p.resets)
		assert.True(t, s.shows(want.none))
		s.tapText(want.view)
		assert.True(t, s.shows(want.empty))
	})
}

func TestBlockedGridColumns(t *testing.T) {
	assert.Equal(t, 1, colsFor(100))
	assert.Equal(t, 1, colsFor(112))
	assert.Equal(t, 1, colsFor(2*112+10-1))
	assert.Equal(t, 2, colsFor(2*112+10))
	assert.Equal(t, 4, colsFor(500))
	assert.Equal(t, 4, colsFor(4*112+3*10))
}
