package ui

// Text fit: in every UI language, every text the app draws fits where it is drawn.
//
// What is opened (headless, with the app's own theme and the real wallpaper plugin):
//   - the preferences window at its default size (880×640): the four pages, the shortcuts page
//     and every source's detail page;
//   - on those pages, every dropdown, opened, and with each option shown as the selected value;
//   - the dialogs: add a collection (on every source that has one, also with the "not a valid
//     URL" line showing), the confirmation behind every red button (delete a collection, clear
//     cache), save a collection, and the error dialog for each kind of error;
//   - the license agreement window at its size (800×600);
//   - the Tune image window in its three modes;
//   - the tray menu (with every row the app can show), its display-mode submenu and a display's
//     submenu.
//
// A text does not fit when
//  1. a window's content needs more room than the window has: Fyne would widen the window (or
//     show the page cut off), so the window's MinSize must stay within its size;
//  2. the drawn text (its natural width, placed by its alignment) sticks out of the widget that
//     draws it (a label, button, segment, dropdown item…), out of a scroll area, a dialog panel
//     or the window, by more than fitTolerance horizontally, or out of its widget vertically;
//  3. the text is cut with "…" where the full text should show (a dropdown's selected value, a
//     tray row): a drawn text that ends in "…" and is not itself one of the app's strings;
//  4. two texts drawn in the same window or dialog overlap;
//  5. outside Chinese and Japanese, a punctuation mark or symbol is drawn about as wide as a
//     Chinese character (Noto Sans SC draws · “ ” … full width; font_narrow.go), which leaves
//     wide gaps around it in Latin text.
//
// The tray menu is drawn by the app (trayMenuView), so its rows are checked like any window; a tip
// (an icon button's name) is checked on its own, as it is drawn over the rows on purpose.
//
// Problems whose fix is a layout change can wait for the owner (knownFitIssues): they are logged, not
// failed. TestTextFitsInEveryLanguage runs the check for every shipped language (about 1.5 s each);
// TestTextFitCatchesPseudo runs it for the pseudo language (each vowel doubled, wrapped in
// "[!! … !!]") and requires it to find problems, so the check cannot silently pass everything.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/asset"
	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/setting"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
	// Every source the app ships.
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/artic"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/cleveland"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/favorites"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/getty"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/googlephotos"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/localfolder"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/metmuseum"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/nga"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/npm"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/parismusees"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/pexels"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/rijksmuseum"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/smk"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/wallhaven"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/wikimedia"
	_ "github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/yale"
)

// fitTolerance absorbs rounding of positions and text widths.
const fitTolerance float32 = 1

func TestTextFitsInEveryLanguage(t *testing.T) {
	for _, lang := range shippedLanguages() {
		t.Run(lang, func(t *testing.T) {
			problems, known := textFitProblems(t, lang)
			for _, p := range problems {
				t.Error(p)
			}
			for _, p := range known {
				t.Log("known, awaiting a layout decision: " + p)
			}
		})
	}
}

func TestTextFitCatchesPseudo(t *testing.T) {
	supported := false
	for _, l := range i18n.SupportedLanguages {
		supported = supported || l.Code == "pseudo"
	}
	if !supported {
		t.Skip("the pseudo language is left out of release builds")
	}
	problems, _ := textFitProblems(t, "pseudo")
	t.Logf("pseudo: %d problems", len(problems))
	for i, p := range problems {
		if i == 15 {
			break
		}
		t.Log("  " + p)
	}
	require.NotEmpty(t, problems, "lengthened text must not fit everywhere; if it does, the check finds nothing")
}

// shippedLanguages are the UI languages people can choose. Under -race, which makes the check
// about ten times slower (230 s for all of them) and finds nothing a layout depends on, only
// English and German, the longest, are checked.
func shippedLanguages() []string {
	var out []string
	for _, l := range i18n.SupportedLanguages {
		if raceEnabled && l.Code != "en" && l.Code != "de" {
			continue
		}
		if l.Code != "pseudo" {
			out = append(out, l.Code)
		}
	}
	return out
}

// textFitProblems opens every screen in lang and returns what does not fit, one line each, as
// "<screen>: <problem>": the new problems and those in knownFitIssues.
func textFitProblems(t *testing.T, lang string) (problems, known []string) {
	t.Helper()
	i18n.SetLanguage(lang)
	t.Cleanup(func() { i18n.SetLanguage("en") })
	f := &fitRun{t: t, appStrings: appStrings(t), screens: map[string]bool{}, markWidths: map[string][2]float32{},
		cjk: lang == "ja" || strings.HasPrefix(lang, "zh")}

	s := newScenarioWith(t, wallpaper.GetInstance())
	s.sa.App.Settings().SetTheme(newPaintpaneTheme(themeModeLight))
	s.w.Resize(fyne.NewSize(900, 700))
	s.w.Resize(fyne.NewSize(defaultPrefsWindowWidth, defaultPrefsWindowHeight))
	f.s = s

	for _, id := range []string{pageSources, pageWallpaper, pageGeneral, pageAdvanced} {
		s.sa.prefsNav.Select(id)
		f.screen("page "+id, true)
		if id == pageGeneral && f.followLink(i18n.T("View all shortcuts →")) {
			f.screen("page shortcuts", true)
			s.sa.prefsNav.Select(pageGeneral)
		}
	}

	s.sa.prefsNav.Select(pageSources)
	for _, title := range sourceTitles(t, s) {
		s.searchEntry().SetText(title)
		s.tapText(title)
		where := "source " + title
		f.screen(where, true)
		if s.shows(i18n.T("Add new collection")) {
			s.tapText(i18n.T("Add new collection"))
			f.checkDialog(where + " › add dialog")
			// Something that is not a URL shows the dialog's error line, if it has one.
			if e := f.overlayEntry(); e != nil {
				e.SetText("x")
				f.checkDialog(where + " › add dialog with an error")
			}
			f.closeOverlays()
		}
		s.sa.prefsNav.Select(pageSources)
		s.searchEntry().SetText("")
	}

	// Dialogs opened from code paths the pages do not reach in a test.
	s.sa.prefsSM.showSaveCollectionDialog("Random General", func(string, bool) error { return nil }, func() {})
	f.checkDialog("save collection dialog")
	f.closeOverlays()
	for _, err := range []error{
		setting.Known(setting.ErrorInvalidKey, "wallhaven", errors.New("401")),
		setting.Known(setting.ErrorInvalidKey, "", errors.New("401")),
		&url.Error{Op: "Get", URL: "https://example.invalid", Err: errors.New("no route to host")},
		setting.Known(setting.ErrorUnknownUser, "wallhaven", errors.New("404")),
		setting.Known(setting.ErrorNoCollection, "wallhaven", errors.New("empty")),
		errors.New("unexpected status 500 from https://api.example.invalid/v1/collections/123456"),
	} {
		showErrorDialog(s.w, err)
		f.checkDialog("error dialog " + fmt.Sprintf("%T %v", err, err))
		f.closeOverlays()
	}

	f.checkLicense()
	f.checkTune()
	f.checkTray()

	sort.Strings(f.problems)
	sort.Strings(f.known)
	t.Logf("%s: %d screens, %d texts, %d dropdowns checked; %d problems, %d known", lang, len(f.screens), f.texts, f.selects, len(f.problems), len(f.known))
	return f.problems, f.known
}

// knownFitIssues are problems whose fix changes the layout, not a translation. The owner decides
// layout changes first, so such a problem can be listed here while it waits: it is then logged
// instead of failing the test. Remove the entry once its fix is in. Empty since the three found
// with this test (dialog titles, the tray's display mode row, collection list links) were fixed.
var knownFitIssues []struct {
	id    string
	match func(f *fitRun, where, text string) bool
}

type fitRun struct {
	t          *testing.T
	s          *scenario
	appStrings map[string]bool // every string of the app in this language and in English
	problems   []string
	known      []string // problems listed in knownFitIssues
	screens    map[string]bool
	texts      int
	selects    int
	// tip is the text of the tip being checked: a tip is drawn over the rows on purpose, and the
	// rows were checked without it, so only the tip's own text counts then.
	tip string
	// cjk is set for Chinese and Japanese, whose typesetting wants full-width punctuation.
	cjk        bool
	markWidths map[string][2]float32 // a mark's width and a Chinese character's, by mark, size and style
}

// add records a problem with text (empty when it is not about one text).
func (f *fitRun) add(where, text, format string, args ...any) {
	if f.tip != "" && text != f.tip {
		return
	}
	line := where + ": " + fmt.Sprintf(format, args...)
	for _, k := range knownFitIssues {
		if text != "" && k.match(f, where, text) {
			f.known = append(f.known, line+" ["+k.id+"]")
			return
		}
	}
	f.problems = append(f.problems, line)
}

// screen checks the preferences window as it is, then its dropdowns and red buttons.
func (f *fitRun) screen(where string, withControls bool) {
	f.check(where)
	if !withControls {
		return
	}
	for _, o := range shownObjects(f.s.w.Canvas().Content()) {
		if sel, ok := o.(*widget.Select); ok {
			f.checkSelect(where, sel)
		}
	}
	for _, o := range shownObjects(f.s.w.Canvas().Content()) {
		if b, ok := o.(*dangerButton); ok && !b.Disabled() {
			test.Tap(b)
			if f.s.w.Canvas().Overlays().Top() != nil {
				f.checkDialog(where + " › " + b.Text + " dialog")
				f.closeOverlays()
			}
		}
	}
}

// check applies rules 1–4 to the preferences window and whatever dialog is open over it.
func (f *fitRun) check(where string) {
	flushUI()
	f.checkCanvas(where, f.s.w.Canvas(), f.s.w.Canvas().Size())
}

// checkDialog applies rules 2–4 to what is open over the preferences window; the page under it
// was checked on its own.
func (f *fitRun) checkDialog(where string) {
	flushUI()
	c := f.s.w.Canvas()
	if c.Overlays().Top() == nil {
		f.add(where, "", "nothing opened")
		return
	}
	f.checkLayers(where, c.Size(), c.Overlays().List())
}

// checkSelect shows each option as the selected value (without choosing it) and opens the list.
func (f *fitRun) checkSelect(where string, sel *widget.Select) {
	f.selects++
	r := test.WidgetRenderer(sel)
	orig := sel.Selected
	for _, opt := range sel.Options {
		sel.Selected = opt
		r.Refresh()
		var shown []string
		for _, o := range shownObjects(sel) {
			if txt, ok := o.(*canvas.Text); ok {
				shown = append(shown, txt.Text)
			}
		}
		if got := strings.Join(shown, ""); got != opt {
			f.add(where, opt, "dropdown shows %q as %q", opt, got)
		}
	}
	sel.Selected = orig
	r.Refresh()

	test.Tap(sel)
	f.checkDialog(where + " › dropdown " + strings.Join(sel.Options, " / "))
	f.closeOverlays()
}

func (f *fitRun) closeOverlays() {
	ov := f.s.w.Canvas().Overlays()
	for ov.Top() != nil {
		ov.Remove(ov.Top())
	}
	flushUI()
}

func (f *fitRun) overlayEntry() *widget.Entry {
	top := f.s.w.Canvas().Overlays().Top()
	if top == nil {
		return nil
	}
	for _, o := range shownObjects(top) {
		if e, ok := o.(*widget.Entry); ok {
			return e
		}
	}
	return nil
}

// followLink taps the link reading text, as the screenshot test does.
func (f *fitRun) followLink(text string) bool {
	for _, o := range shownObjects(f.s.w.Canvas().Content()) {
		var h *widget.Hyperlink
		switch x := o.(type) {
		case *widget.Hyperlink:
			h = x
		case *accentLink:
			h = &x.Hyperlink
		}
		if h != nil && h.Text == text {
			pos := fyne.CurrentApp().Driver().AbsolutePositionForObject(o)
			test.TapCanvas(f.s.w.Canvas(), pos.Add(fyne.NewPos(o.Size().Width/2, o.Size().Height/2)))
			return true
		}
	}
	f.add("page general", "", "no link %q", text)
	return false
}

func (f *fitRun) checkLicense() {
	text, err := eulaTextFor(asset.NewManager(), i18n.EffectiveLanguage())
	require.NoError(f.t, err)
	w := test.NewWindow(newEULAContent(text, func() {}, func() {}))
	defer w.Close()
	w.SetPadded(false)
	w.Resize(fyne.NewSize(800, 600)) // as showEULA
	f.checkCanvas("license agreement window", w.Canvas(), w.Canvas().Size())
}

// checkTune opens the Tune image window as ShowTuneImagePopup does: the window is as big as its
// content wants, so rule 1 cannot fail; the content keeps a fixed width that its texts must fit.
func (f *fitRun) checkTune() {
	preview := filepath.Join("..", "pkg", "wallpaper", "testdata", "tuning_images", "4ld7or.jpg")
	base := provider.TuningOptions{CropOffset: 30, HasCropOffset: true, FrameOverride: provider.FrameOverrideForceOff,
		FrameSize: 0.85, Matting: provider.MattingOverrideOn, WallColor: provider.WallColorOverrideAlgorithmic}
	framed := base
	framed.FrameOverride = provider.FrameOverrideForceOn
	for name, req := range map[string]tuneRequest{
		"crop":  {previewPath: preview, opts: base, global: provider.DisplayCrop},
		"frame": {previewPath: preview, opts: framed, global: provider.DisplayCrop},
		"whole": {previewPath: preview, opts: base, global: provider.DisplayFull},
	} {
		content := newTunePanel(req)
		w := test.NewWindow(content)
		w.SetPadded(false)
		w.Resize(content.MinSize())
		f.checkCanvas("tune window ("+name+")", w.Canvas(), w.Canvas().Size())
		w.Close()
	}
}

// checkTray draws the tray menu with every row the app can put in it.
func (f *fitRun) checkTray() {
	items, footer := sampleTrayItems(nil)
	at := len(items) - 2 // before Display mode and Show NSFW images, as on Windows
	extra := []schema.MenuItemSchema{
		{Label: i18n.T("Next lock screen picture"), Glyph: "lock"},
		{Label: i18n.T("Remove NSFW mark"), Glyph: "eyeOff"},
	}
	all := append(append(append([]schema.MenuItemSchema{}, items[:at]...), extra...), items[at:]...)
	modes := items[len(items)-2].SubMenu
	for _, mode := range []string{"Full artwork", "Museum frame", "Smart crop"} {
		all = append(all, schema.MenuItemSchema{Label: i18n.T("Display mode"), Detail: i18n.T(mode), Glyph: "photo", SubMenu: modes})
	}
	// With several displays, each display's rows (all but "now showing") sit in its own submenu.
	perDisplay := append([]schema.MenuItemSchema{}, all[1:]...)
	display := i18n.Tf("Display {{.ID}} ({{.Name}})", map[string]any{"ID": 2, "Name": "DELL U2723QE"})
	all = append(all,
		schema.MenuItemSchema{Label: display, SubMenu: &schema.MenuSchema{Label: display, Items: perDisplay}},
		schema.MenuItemSchema{IsSeparator: true},
		schema.MenuItemSchema{Label: i18n.Tf("Update to {{.Version}}", map[string]any{"Version": "v1.12.10"}), Glyph: "download", Emphasis: true},
	)
	alt := append([]schema.MenuItemSchema{}, all...)
	for i := range alt { // the other state of the toggling rows
		switch alt[i].Label {
		case i18n.T("Pause rotation"):
			alt[i].Label = i18n.T("Resume rotation")
		case i18n.T("Remove from favorites"):
			alt[i].Label = i18n.T("Add to favorites")
		}
	}

	f.checkTrayView("tray menu", newTrayMenuView(all, footer, trayMinWidth))
	f.checkTrayView("tray menu (other states)", newTrayMenuView(alt, footer, trayMinWidth))
	f.checkTrayView("tray display-mode submenu", newTrayMenuView(modes.Items, nil, subMenuWidth(modes.Items)))
	f.checkTrayView("tray display submenu", newTrayMenuView(perDisplay, nil, subMenuWidth(perDisplay)))
}

func (f *fitRun) checkTrayView(where string, v *trayMenuView) {
	w := test.NewWindow(v)
	defer w.Close()
	w.SetPadded(false)
	w.Resize(v.MinSize())
	f.checkCanvas(where, w.Canvas(), w.Canvas().Size())
	// The icon buttons show their names only as a tip; show each one.
	for i, r := range v.regions {
		if r.kind != trayRegionButton {
			continue
		}
		v.mu.Lock()
		v.tip = i
		v.mu.Unlock()
		v.Refresh()
		f.tip = r.item.Label
		f.checkCanvas(where+" › tip "+r.item.Label, w.Canvas(), w.Canvas().Size())
		f.tip = ""
	}
}

// ---------------------------------------------------------------------------
// The check itself
// ---------------------------------------------------------------------------

type fitRect struct{ x0, y0, x1, y1 float32 }

func rectOf(pos fyne.Position, size fyne.Size) fitRect {
	return fitRect{pos.X, pos.Y, pos.X + size.Width, pos.Y + size.Height}
}

// fitOwner is the widget that draws a text.
type fitOwner struct {
	fitRect
	kind string
}

type drawnText struct {
	text string
	r    fitRect
}

// checkCanvas applies rules 1–4 to one window: its content and every pop-up over it.
func (f *fitRun) checkCanvas(where string, c fyne.Canvas, size fyne.Size) {
	if m := c.Content().MinSize(); m.Width > size.Width+fitTolerance || m.Height > size.Height+fitTolerance {
		f.add(where, "", "window content needs %.0f×%.0f, the window is %.0f×%.0f", m.Width, m.Height, size.Width, size.Height)
	}
	f.checkLayers(where, size, append([]fyne.CanvasObject{c.Content()}, c.Overlays().List()...))
}

func (f *fitRun) checkLayers(where string, size fyne.Size, layers []fyne.CanvasObject) {
	f.screens[where] = true
	window := fitRect{0, 0, size.Width, size.Height}
	for _, layer := range layers {
		var texts []drawnText
		f.walk(where, layer, fyne.Position{}, nil, []fitRect{window}, nil, &texts)
		if f.tip == "" {
			f.checkOverlaps(where, texts)
		}
	}
}

// walk visits o and its children. parent is the absolute position of o's parent; owner is the
// rectangle of the nearest widget above; clips are the rectangles that cut off what is outside
// them (the window, scroll areas, dialog panels).
func (f *fitRun) walk(where string, o fyne.CanvasObject, parent fyne.Position, owner *fitOwner, clips []fitRect, clipObj fyne.CanvasObject, texts *[]drawnText) {
	if o == nil || !o.Visible() {
		return
	}
	pos := parent.Add(o.Position())
	r := rectOf(pos, o.Size())
	if o == clipObj {
		clips = append(clips[:len(clips):len(clips)], r)
	}

	if txt, ok := o.(*canvas.Text); ok {
		f.checkText(where, txt, pos, owner, clips, texts)
		return
	}

	var children []fyne.CanvasObject
	switch x := o.(type) {
	case *fyne.Container:
		children = x.Objects
	case fyne.Widget:
		switch w := x.(type) {
		case *container.Scroll:
			clips = append(clips[:len(clips):len(clips)], r)
		case *widget.PopUp:
			clipObj = w.Content // the dialog panel
		}
		if e, ok := x.(*widget.Entry); ok && e.PlaceHolder != "" {
			f.appStrings[e.PlaceHolder] = true // "https://wallhaven.cc/search?q=…" ends in … on purpose
		}
		owner = &fitOwner{r, fmt.Sprintf("%T", x)}
		children = test.WidgetRenderer(x).Objects()
	}
	for _, c := range children {
		f.walk(where, c, pos, owner, clips, clipObj, texts)
	}
}

func (f *fitRun) checkText(where string, txt *canvas.Text, pos fyne.Position, owner *fitOwner, clips []fitRect, texts *[]drawnText) {
	if strings.TrimSpace(txt.Text) == "" {
		return
	}
	m := txt.MinSize()
	x := pos.X
	switch txt.Alignment {
	case fyne.TextAlignCenter:
		x += (txt.Size().Width - m.Width) / 2
	case fyne.TextAlignTrailing:
		x += txt.Size().Width - m.Width
	}
	d := fitRect{x, pos.Y, x + m.Width, pos.Y + m.Height}
	*texts = append(*texts, drawnText{txt.Text, d})
	f.texts++

	if owner != nil {
		if out := fyne.Max(owner.x0-d.x0, d.x1-owner.x1); out > fitTolerance {
			f.add(where, txt.Text, "%q sticks out of its %s (%.0f px wide) by %.0f px", txt.Text, owner.kind, owner.x1-owner.x0, out)
		} else if out := fyne.Max(owner.y0-d.y0, d.y1-owner.y1); out > fitTolerance+1 {
			f.add(where, txt.Text, "%q sticks out of the top or bottom of its %s by %.0f px", txt.Text, owner.kind, out)
		}
	}
	for _, c := range clips {
		if out := fyne.Max(c.x0-d.x0, d.x1-c.x1); out > fitTolerance {
			f.add(where, txt.Text, "%q is cut off: %.0f px outside its scroll area, dialog or window", txt.Text, out)
			break
		}
	}
	if strings.HasSuffix(txt.Text, "…") && !f.appStrings[txt.Text] {
		f.add(where, txt.Text, "%q is cut with …", txt.Text)
	}
	if !f.cjk {
		for _, r := range txt.Text {
			if w, han, wide := f.wideMark(r, txt.TextSize, txt.TextStyle); wide {
				f.add(where, txt.Text, "%q draws %q (U+%04X) %.1f px wide, as wide as a Chinese character (%.1f px)", txt.Text, r, r, w, han)
			}
		}
	}
}

// wideAllowed are marks that are full width in Latin fonts too (arrows, the shapes the app uses
// as icons) or that belong to Chinese text quoted in another language (CJK and full-width
// punctuation, as in a museum's Chinese credit line).
func wideAllowed(r rune) bool {
	switch {
	case r >= 0x2190 && r <= 0x21FF, r >= 0x25A0 && r <= 0x26FF: // arrows, shapes, symbols
		return true
	case r >= 0x3000 && r <= 0x303F, r >= 0xFF00 && r <= 0xFFEF: // CJK and full-width punctuation
		return true
	}
	return false
}

// wideMark reports whether r is a punctuation mark or symbol outside ASCII drawn at least 90% as
// wide as a Chinese character (Latin fonts draw none so wide; © and a bold … come closest at 85%) of the same size and style (rule 5), with both widths.
func (f *fitRun) wideMark(r rune, size float32, style fyne.TextStyle) (w, han float32, wide bool) {
	if r < 0x80 || !(unicode.IsPunct(r) || unicode.IsSymbol(r)) || wideAllowed(r) {
		return 0, 0, false
	}
	key := fmt.Sprintf("%c/%g/%v", r, size, style)
	if v, ok := f.markWidths[key]; ok {
		return v[0], v[1], v[0] >= 0.9*v[1]
	}
	w = fyne.MeasureText(string(r), size, style).Width
	han = fyne.MeasureText("汉", size, style).Width
	f.markWidths[key] = [2]float32{w, han}
	return w, han, w >= 0.9*han
}

// checkOverlaps reports texts drawn over each other (rule 4).
func (f *fitRun) checkOverlaps(where string, texts []drawnText) {
	for i := range texts {
		for j := i + 1; j < len(texts); j++ {
			a, b := texts[i].r, texts[j].r
			w := fyne.Min(a.x1, b.x1) - fyne.Max(a.x0, b.x0)
			h := fyne.Min(a.y1, b.y1) - fyne.Max(a.y0, b.y0)
			if w > 2*fitTolerance && h > 4 {
				f.add(where, texts[i].text, "%q and %q overlap", texts[i].text, texts[j].text)
			}
		}
	}
}

// appStrings are the app's strings in the current language and in English; a text that ends in
// "…" and is one of them is meant to read so ("Settings…", "Calculating…").
func appStrings(t *testing.T) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "pkg", "i18n", "translations", "en.json"))
	require.NoError(t, err)
	var en map[string]string
	require.NoError(t, json.Unmarshal(data, &en))
	known := map[string]bool{}
	for k := range en {
		known[k] = true
		known[i18n.T(k)] = true
	}
	return known
}

// sourceTitles lists every source on the Sources page, as TestA7 does.
func sourceTitles(t *testing.T, s *scenario) []string {
	t.Helper()
	var titles []string
	for _, item := range wallpaper.GetInstance().CreatePrefsSchema(s.sa.prefsSM).Items {
		if item.Sources == nil {
			continue
		}
		for _, g := range item.Sources.Groups {
			for _, src := range g.Items {
				titles = append(titles, src.Title)
			}
		}
	}
	require.GreaterOrEqual(t, len(titles), 10, "sources listed: %v", titles)
	return titles
}
