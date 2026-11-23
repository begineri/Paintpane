package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
)

// A collection's name in the list is a link over the whole name: a click on its last letters opens
// it, as one on its first. The link used to keep the width of the cell template's "Placeholder"
// text, so only the start of a longer name was clickable and the rest was drawn past the link. A
// name too long for the row ends in "…" before the Active switch (approved 2026-11-18).
func TestCollectionLinkSpansItsName(t *testing.T) {
	s := newScenario(t)
	long := "A collection whose name is far too long to fit in one row of the list beside its switch and button"
	queries := []schema.Query{
		{ID: "short", URL: "https://example.invalid/short", Description: "Random General", Active: true},
		{ID: "long", URL: "https://example.invalid/long", Description: long},
	}
	list := s.sa.prefsSM.renderQueryList(schema.QueryListItem{
		GetQueries:   func() []schema.Query { return queries },
		EnableQuery:  func(string) error { return nil },
		DisableQuery: func(string) error { return nil },
		RemoveQuery:  func(string) error { return nil },
	})
	w := test.NewWindow(list)
	defer w.Close()
	w.SetPadded(false)
	w.Resize(fyne.NewSize(560, 200))

	links := map[string]*widget.Hyperlink{}
	for _, o := range shownObjects(list) {
		if h, ok := o.(*widget.Hyperlink); ok {
			links[h.Text] = h
		}
	}
	require.Contains(t, links, "Random General")
	require.Contains(t, links, long)

	// The short name: drawn whole, inside its link, and a click on its last letter opens it.
	short := links["Random General"]
	opened := 0
	short.OnTapped = func() { opened++ }
	drawn := drawnTexts(short)
	require.Equal(t, []string{"Random General"}, textsOf(drawn))
	pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(short)
	textEnd := drawn[0].pos.X + drawn[0].width
	assert.LessOrEqual(t, textEnd, pos.X+short.Size().Width, "the name is drawn inside its link")
	test.TapCanvas(w.Canvas(), fyne.NewPos(textEnd-2, pos.Y+short.Size().Height/2))
	assert.Equal(t, 1, opened, "a click on the name's last letter opens it")

	// The long name: cut with "…" before the Active switch, which stays whole in the row.
	longLink := links[long]
	drawn = drawnTexts(longLink)
	require.Len(t, drawn, 1)
	assert.NotEqual(t, long, drawn[0].text)
	assert.Equal(t, "…", string([]rune(drawn[0].text)[len([]rune(drawn[0].text))-1:]), "a name too long for the row ends in …")
	var check *widget.Check
	for _, o := range shownObjects(list) {
		if c, ok := o.(*widget.Check); ok {
			check = c
		}
	}
	require.NotNil(t, check)
	checkX := fyne.CurrentApp().Driver().AbsolutePositionForObject(check).X
	assert.LessOrEqual(t, drawn[0].pos.X+drawn[0].width, checkX, "the name stops before the Active switch")
	assert.LessOrEqual(t, checkX+check.Size().Width, float32(560), "the switch stays in the row")
}

type linkText struct {
	text  string
	pos   fyne.Position
	width float32
}

// drawnTexts lists the texts drawn inside o, with their position in the window and natural width.
func drawnTexts(o fyne.CanvasObject) []linkText {
	var out []linkText
	for _, c := range shownObjects(o) {
		if txt, ok := c.(*canvas.Text); ok && txt.Text != "" {
			out = append(out, linkText{txt.Text, fyne.CurrentApp().Driver().AbsolutePositionForObject(txt), txt.MinSize().Width})
		}
	}
	return out
}

func textsOf(in []linkText) []string {
	var out []string
	for _, d := range in {
		out = append(out, d.text)
	}
	return out
}
