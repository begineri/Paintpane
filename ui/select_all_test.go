package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"

	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
)

func TestSelectAllHeaderFollowsAndDrivesMembers(t *testing.T) {
	app := test.NewApp()
	w := app.NewWindow("Select all")
	sm := NewSettingsManager(w).(*SettingsManager)

	applied := map[string]bool{}
	member := func(name string, on bool) schema.BoolItem {
		return schema.BoolItem{Name: name, Label: name, InitialValue: on, ApplyFunc: func(b bool) { applied[name] = b }}
	}
	sm.RenderSchema(schema.PanelSchema{Sections: []schema.SectionSchema{{
		Title:     "Curated Collections",
		SelectAll: &schema.SelectAllItem{Name: "all", Label: "Select all", Members: []string{"a", "b", "c"}},
		Items:     []schema.ItemSchema{member("a", true), member("b", false), member("c", true)},
	}}})

	all := sm.allWidgets["all"].(*toggleSwitch)
	sw := func(name string) *toggleSwitch { return sm.allWidgets[name].(*toggleSwitch) }
	assert.Equal(t, sw("a").MinSize(), all.MinSize(), "Select all has the row switches' size (G5)")

	assert.False(t, all.Checked, "off while one member is off")

	// Turning it on turns every member on.
	test.Tap(all)
	assert.True(t, all.Checked)
	assert.True(t, sw("a").Checked && sw("b").Checked && sw("c").Checked)

	// Turning one member off turns Select all off, and leaves the others on.
	test.Tap(sw("a"))
	assert.False(t, all.Checked)
	assert.True(t, sw("b").Checked && sw("c").Checked)

	// Turning that member back on turns Select all on again.
	test.Tap(sw("a"))
	assert.True(t, all.Checked)

	// Turning it off turns every member off; the changes apply together.
	test.Tap(all)
	assert.False(t, sw("a").Checked || sw("b").Checked || sw("c").Checked)
	sm.FlushPending()
	assert.Equal(t, map[string]bool{"a": false, "c": false}, applied, "only members that changed from their saved value apply")
}

func TestSelectAllHeaderShowsCount(t *testing.T) {
	app := test.NewApp()
	w := app.NewWindow("Select all count")
	sm := NewSettingsManager(w).(*SettingsManager)
	// Apply at the end so the auto-apply timer does not fire during later tests.
	defer sm.FlushPending()
	content := sm.RenderSchema(schema.PanelSchema{Sections: []schema.SectionSchema{{
		Title:     "Curated Collections",
		SelectAll: &schema.SelectAllItem{Name: "all", Label: "Select all", Members: []string{"a", "b"}},
		Items: []schema.ItemSchema{
			schema.BoolItem{Name: "a", Label: "a", InitialValue: true},
			schema.BoolItem{Name: "b", Label: "b", InitialValue: false},
		},
	}}})
	w.SetContent(content)
	w.Resize(fyne.NewSize(600, 400))

	header := func() string {
		for _, o := range test.LaidOutObjects(content) {
			if r, ok := o.(*styledText); ok && strings.HasPrefix(r.Text, "Curated Collections") {
				return r.Text
			}
		}
		return ""
	}
	assert.Equal(t, "Curated Collections · 1/2 on", header())
	test.Tap(sm.allWidgets["b"].(*toggleSwitch))
	assert.Equal(t, "Curated Collections · 2/2 on", header())

	// Select all lines up with the row switches below it (G5).
	right := func(o fyne.CanvasObject) float32 {
		return fyne.CurrentApp().Driver().AbsolutePositionForObject(o).X + o.Size().Width
	}
	all := sm.allWidgets["all"].(*toggleSwitch)
	assert.InDelta(t, right(sm.allWidgets["a"].(*toggleSwitch)), right(all), 0.5)
}
