package ui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
)

// Switching Appearance or Accent color (docs: theme lag, 2026-11-08). Installing a theme makes
// Fyne redraw every window, so the switch happens on the click and only once.

func currentTheme(t *testing.T) *paintpaneTheme {
	t.Helper()
	th, ok := fyne.CurrentApp().Settings().Theme().(*paintpaneTheme)
	require.True(t, ok, "the Paintpane theme is installed")
	return th
}

func TestApplyThemeSkipsTheSameLook(t *testing.T) {
	a := test.NewTempApp(t)
	applyTheme(a, themeModeDark, "sage")
	first := currentTheme(t)
	assert.Equal(t, themeModeDark, first.mode)

	applyTheme(a, themeModeDark, "sage")
	assert.Same(t, first, currentTheme(t), "the same mode and accent install nothing")
	applyTheme(a, themeModeDark, "")
	assert.NotSame(t, first, currentTheme(t), "another accent installs a new theme")
	assert.Equal(t, "graphite", currentTheme(t).accent.key)
	second := currentTheme(t)
	applyTheme(a, themeModeDark, "no-such-colour")
	assert.Same(t, second, currentTheme(t), "an unknown key is graphite, which is already shown")
	applyTheme(a, themeModeLight, "graphite")
	assert.Equal(t, themeModeLight, currentTheme(t).mode)
	assert.Equal(t, theme.VariantLight, currentTheme(t).variant)
}

func TestScenarioAppearanceAppliesOnClick(t *testing.T) {
	s := newScenario(t)
	s.tapText("General")
	seg, ok := s.sa.prefsSM.allWidgets["theme"].(*segmented)
	require.True(t, ok, "Appearance is a segmented control")

	for _, mode := range []themeMode{themeModeDark, themeModeLight, themeModeSystem} {
		seg.SetSelectedIndex(themeModeIndex(mode))
		// No FlushPending: the new look is there before the page's auto-apply delay.
		th := currentTheme(t)
		assert.Equal(t, mode, th.mode, "the click applies %s at once", mode)
		assert.Equal(t, string(mode), s.sa.appConfig.GetTheme(), "and saves it")
		assert.Equal(t, mode != themeModeSystem, th.forced)

		s.sa.prefsSM.FlushPending()
		assert.Same(t, th, currentTheme(t), "the delayed apply does not install it a second time")
	}
}

func TestScenarioAccentAppliesOnClick(t *testing.T) {
	s := newScenario(t)
	s.tapText("General")
	picker, ok := s.sa.prefsSM.allWidgets["accent"].(*swatchPicker)
	require.True(t, ok)
	content := s.w.Content()

	for i, a := range accents {
		if i == picker.Selected {
			continue
		}
		picker.SetSelectedIndex(i)
		th := currentTheme(t)
		assert.Equal(t, a.key, th.accent.key, "the click applies %s at once", a.key)
		assert.Equal(t, a.key, s.sa.appConfig.GetAccent())
		flushUI()
		s.sa.prefsSM.FlushPending()
		assert.Same(t, th, currentTheme(t), "the delayed apply does not install %s again", a.key)
	}

	// Choosing or leaving graphite keeps the page (docs: graphite switch, 2026-11-09).
	picker.SetSelectedIndex(0)
	flushUI()
	assert.True(t, graphiteAccent())
	picker.SetSelectedIndex(3)
	flushUI()
	assert.False(t, graphiteAccent())
	assert.Same(t, picker, s.sa.prefsSM.allWidgets["accent"], "the page is not rebuilt")
	assert.Same(t, content, s.w.Content())
	assert.Equal(t, 3, picker.Selected)
}

// accentLinks returns the links on every settings page, shown or not.
func accentLinks(o fyne.CanvasObject) []*accentLink {
	var out []*accentLink
	for _, x := range test.LaidOutObjects(o) {
		if l, ok := x.(*accentLink); ok {
			out = append(out, l)
		}
	}
	return out
}

// linkDrawnUnderlined reports whether the text a link draws is underlined.
func linkDrawnUnderlined(t *testing.T, l *accentLink) bool {
	t.Helper()
	var texts []*canvas.Text
	for _, x := range test.LaidOutObjects(l) {
		if c, ok := x.(*canvas.Text); ok && c.Text != "" {
			texts = append(texts, c)
		}
	}
	require.NotEmpty(t, texts, "link %q draws its text", l.Text)
	for _, c := range texts {
		if !c.TextStyle.Underline {
			return false
		}
	}
	return true
}

// Links are underlined only in graphite. Switching to or from graphite restyles
// the links already shown, through the theme's own refresh, in light and dark.
func TestScenarioLinksFollowGraphiteInPlace(t *testing.T) {
	s := newScenario(t)
	s.tapText("Advanced")
	links := accentLinks(s.w.Content())
	require.NotEmpty(t, links, "the Advanced page has links (Help, Source code, License)")
	content := s.w.Content()

	for _, mode := range []themeMode{themeModeLight, themeModeDark} {
		var keys []string // every accent, each entered from graphite and left for it
		for _, a := range accents[1:] {
			keys = append(keys, accentDefault, a.key)
		}
		for _, key := range append(keys, accentDefault) {
			applyTheme(s.sa.App, mode, key)
			flushUI()
			want := key == accentDefault
			for _, l := range links {
				assert.Equal(t, want, l.TextStyle.Underline, "%s %s: link %q", mode, key, l.Text)
				assert.Equal(t, want, linkDrawnUnderlined(t, l), "%s %s: link %q as drawn", mode, key, l.Text)
			}
		}
	}
	assert.Same(t, content, s.w.Content(), "no page rebuild")
	assert.ElementsMatch(t, links, accentLinks(s.w.Content()), "the same links, restyled")
}

// A page switched in place draws exactly what the same page built fresh in the
// new accent draws, for every accent entered from graphite and graphite entered
// from it, in light and dark. Positions are left out: the test driver's layout of
// a card can differ by a few pixels between two builds even without a switch.
func TestScenarioGraphiteSwitchLooksLikeAFreshPage(t *testing.T) {
	s := newScenario(t)
	look := func() []string {
		s.tapText("Advanced") // links, a switch and the accent-coloured selection
		flushUI()
		return drawnLook(s.w.Content())
	}
	fresh := func() []string {
		s.sa.RebuildPreferencesContent("")
		s.w.Resize(fyne.NewSize(defaultPrefsWindowWidth, defaultPrefsWindowHeight))
		return look()
	}

	for _, mode := range []themeMode{themeModeLight, themeModeDark} {
		for _, a := range accents[1:] {
			for _, step := range [][2]string{{accentDefault, a.key}, {a.key, accentDefault}} {
				applyTheme(s.sa.App, mode, step[0])
				before := fresh()
				applyTheme(s.sa.App, mode, step[1])
				switched := look()
				want := fresh()
				require.NotEqual(t, before, want, "%s: %s and %s look different", mode, step[0], step[1])
				assert.Equal(t, want, switched, "%s: %s to %s in place looks like a fresh page", mode, step[0], step[1])
			}
		}
	}
}

// drawnLook lists what every visible text, shape and image under o is drawn
// with, in tree order.
func drawnLook(o fyne.CanvasObject) []string {
	var out []string
	for _, x := range test.LaidOutObjects(o) {
		if !x.Visible() {
			continue
		}
		switch c := x.(type) {
		case *canvas.Text:
			out = append(out, fmt.Sprintf("text %q %v %v %v", c.Text, c.Color, c.TextStyle, c.TextSize))
		case *canvas.Rectangle:
			out = append(out, fmt.Sprintf("rect %v %v %v %v", c.FillColor, c.StrokeColor, c.StrokeWidth, c.CornerRadius))
		case *canvas.Circle:
			out = append(out, fmt.Sprintf("circle %v %v %v", c.FillColor, c.StrokeColor, c.StrokeWidth))
		case *canvas.Line:
			out = append(out, fmt.Sprintf("line %v %v", c.StrokeColor, c.StrokeWidth))
		case *canvas.Image:
			name := ""
			if c.Resource != nil {
				name = c.Resource.Name()
			}
			out = append(out, fmt.Sprintf("image %q %p", name, c.Image))
		}
	}
	return out
}

func TestFyneThreadChecksAreOff(t *testing.T) {
	m := paintpaneMetadata()
	assert.True(t, m.Migrations["fyneDo"], "Fyne is told all UI calls go through fyne.Do")
	assert.Equal(t, "Paintpane", m.ID)
}

// A PNG source logo is decoded when the row is made, so refreshing the row (every theme switch
// refreshes every row) does not decode it again. Other formats stay resources.
func TestSourceRowDecodesPNGLogoOnce(t *testing.T) {
	s := newScenario(t)
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(1, 1, color.RGBA{R: 255, A: 255})
	require.NoError(t, png.Encode(&buf, img))

	findImage := func(row fyne.CanvasObject) *canvas.Image {
		var found *canvas.Image
		var walk func(o fyne.CanvasObject)
		walk = func(o fyne.CanvasObject) {
			switch x := o.(type) {
			case *canvas.Image:
				found = x
			case *fyne.Container:
				for _, c := range x.Objects {
					walk(c)
				}
			}
		}
		walk(row)
		return found
	}

	row := s.sa.prefsSM.newSourceRow(schema.SourceSchema{ID: "png", Title: "PNG", IconBytes: buf.Bytes()}, func() {})
	logo := findImage(row)
	require.NotNil(t, logo)
	assert.Nil(t, logo.Resource, "no resource to decode again")
	require.NotNil(t, logo.Image)
	assert.Equal(t, 4, logo.Image.Bounds().Dx())

	svg := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="4" height="4"><rect width="4" height="4"/></svg>`)
	row = s.sa.prefsSM.newSourceRow(schema.SourceSchema{ID: "svg", Title: "SVG", IconBytes: svg}, func() {})
	logo = findImage(row)
	require.NotNil(t, logo)
	assert.NotNil(t, logo.Resource, "an SVG logo stays a resource")
}
