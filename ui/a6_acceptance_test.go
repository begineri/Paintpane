//go:build screenshots

package ui

// A6 acceptance (docs/prd.md): the preferences window has one level of navigation;
// from a source's detail page, choosing "Sources" on the left returns to the list;
// settings take effect right away. Run with the real wallpaper plugin and all seven
// museums registered, as in the shipped app:
//
//	go test -tags screenshots ./ui/ -run TestA6 -count=1 -v

import (
	"sort"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
)

// a6Names are what a user sees in each language.
var a6Names = map[string]struct {
	nav      []string
	museums  []string
	museumsH string // heading of the museums group
	apply    []string
}{
	"en": {
		nav:      []string{"Sources", "Wallpaper", "General", "Advanced"},
		museums:  []string{"Art Institute of Chicago", "Cleveland Museum of Art", "National Palace Museum", "Rijksmuseum", "Statens Museum for Kunst", "The Getty", "The Metropolitan Museum of Art"},
		museumsH: "Museums · 7",
		apply:    []string{"Apply Changes", "Apply", "Save", "Close", "OK", "Cancel"},
	},
	"zh": {
		nav:      []string{"图源", "壁纸", "常规", "高级"},
		museums:  []string{"丹麦国家美术馆", "克利夫兰艺术博物馆", "台北故宫博物院", "大都会艺术博物馆", "盖蒂博物馆", "芝加哥艺术博物馆", "荷兰国立博物馆"},
		museumsH: "博物馆 · 7",
		apply:    []string{"应用更改", "应用", "保存", "关闭", "确定", "取消"},
	},
}

// a6Scenario opens the real preferences window and lays it out once more: the
// headless driver draws the first list row at a stale position until it is resized.
func a6Scenario(t *testing.T) *scenario {
	t.Helper()
	s := newScenarioWith(t, wallpaper.GetInstance())
	s.w.Resize(fyne.NewSize(900, 700))
	s.w.Resize(fyne.NewSize(defaultPrefsWindowWidth, defaultPrefsWindowHeight))
	return s
}

// tapNav taps the side-navigation entry (the match left of the page area).
func (s *scenario) tapNav(text string) {
	s.t.Helper()
	var target fyne.CanvasObject
	for _, o := range shownObjects(s.w.Canvas().Content()) {
		if got, ok := labelText(o); ok && got == text {
			if pos, _ := s.absRect(o); pos.X < navWidth {
				target = o
			}
		}
	}
	require.NotNil(s.t, target, "no navigation entry reads %q", text)
	s.tapObject(target)
}

func (s *scenario) tapObject(o fyne.CanvasObject) {
	s.t.Helper()
	pos, size := s.absRect(o)
	test.TapCanvas(s.w.Canvas(), pos.Add(fyne.NewPos(size.Width/2, size.Height/2)))
}

// visibleTexts lists every visible label and button text, to tell pages apart.
func (s *scenario) visibleTexts() []string {
	var out []string
	for _, o := range shownObjects(s.w.Canvas().Content()) {
		if t, ok := labelText(o); ok && t != "" {
			out = append(out, t)
		}
	}
	return out
}

func TestA6_OneLevelOfNavigation(t *testing.T) {
	inLanguages(t, func(t *testing.T, lang string) {
		want := a6Names[lang]
		s := a6Scenario(t)

		// Exactly four pages, in this order.
		var got []string
		for _, p := range s.sa.prefsNav.pages {
			got = append(got, p.title)
		}
		assert.Equal(t, want.nav, got)

		for i, p := range s.sa.prefsNav.pages {
			s.tapNav(want.nav[i])
			require.Equal(t, p.id, s.sa.prefsNav.Current(), "tapping %q opens its page", want.nav[i])

			// No tab bars or accordions anywhere: one level only.
			for _, o := range shownObjects(s.w.Canvas().Content()) {
				switch o.(type) {
				case *container.AppTabs, *container.DocTabs, *widget.Accordion:
					t.Errorf("%s page shows a %T: more than one level of navigation", p.id, o)
				}
			}
			// No Apply or Close buttons: settings apply on their own.
			for _, txt := range s.visibleTexts() {
				for _, banned := range want.apply {
					if txt == banned {
						t.Errorf("%s page shows a %q control", p.id, txt)
					}
				}
			}
		}
	})
}

func TestA6_EveryMuseumOpensAndNavigationReturnsToTheList(t *testing.T) {
	inLanguages(t, func(t *testing.T, lang string) {
		want := a6Names[lang]
		s := a6Scenario(t)
		require.True(t, s.shows(want.museumsH), "the list starts with all seven museums")

		for _, museum := range want.museums {
			require.True(t, s.shows(museum), "%q is listed", museum)
			s.tapText(museum)
			// The detail replaces the list: the group heading is gone and the museum name is the title.
			assert.False(t, s.shows(want.museumsH), "%q: the list is replaced by the detail", museum)
			assert.True(t, s.shows(museum), "%q: its name is the page title", museum)
			var switches int
			for _, o := range shownObjects(s.w.Canvas().Content()) {
				if _, ok := o.(*toggleSwitch); ok {
					switches++
				}
			}
			assert.Greater(t, switches, 0, "%q: the detail has collection switches", museum)
			t.Logf("%s %s: %d switches; texts: %v", lang, museum, switches, firstN(s.visibleTexts(), 14))

			// A real tap on "Sources" in the left navigation goes back to the list.
			s.tapNav(want.nav[0])
			assert.Equal(t, pageSources, s.sa.prefsNav.Current())
			assert.True(t, s.shows(want.museumsH), "%q: back on the list", museum)
		}
	})
}

func firstN(in []string, n int) []string {
	if len(in) > n {
		return in[:n]
	}
	return in
}

func TestA6_SettingsTakeEffectWithoutApply(t *testing.T) {
	inLanguages(t, func(t *testing.T, lang string) {
		modes := map[string][]string{
			"en": {"Full artwork", "Museum frame", "Smart crop"},
			"zh": {"完整显示", "博物馆画框", "智能裁切"},
		}[lang]
		cfg := wallpaper.GetConfigInstance()
		cfg.SetArtworkDisplayMode(wallpaper.ArtworkDisplayFull)
		cfg.SetShowNudes(false)
		t.Cleanup(func() {
			cfg.SetArtworkDisplayMode(wallpaper.ArtworkDisplayFull)
			cfg.SetShowNudes(false)
		})
		s := a6Scenario(t)
		s.tapNav(a6Names[lang].nav[1])

		// A segmented choice is pending right after the tap and is saved by the apply timer
		// (800 ms after the last change) with no Apply button involved. The test applies it
		// with FlushPending, which is what the timer and closing the window call; waiting for
		// the real timer here trips the race detector inside Fyne's test driver.
		s.tapText(modes[1])
		assert.True(t, s.sa.prefsSM.hasPending(), "the change waits for the apply timer")
		assert.NotNil(t, s.sa.prefsSM.applyTimer, "an apply timer is running")
		assert.Equal(t, 800*time.Millisecond, s.sa.prefsSM.applyDelay)
		s.sa.prefsSM.FlushPending()
		assert.Equal(t, wallpaper.ArtworkDisplayFramed, cfg.GetArtworkDisplayMode(), "display mode saved")

		// A switch is a pending change that applies by itself (or on flush); no button is needed.
		sw, ok := s.sa.prefsSM.allWidgets["showNudes"].(*toggleSwitch)
		if !ok {
			for name, w := range s.sa.prefsSM.allWidgets {
				if _, isSw := w.(*toggleSwitch); isSw {
					t.Logf("switch key: %s", name)
				}
			}
			require.FailNow(t, "no switch named showNudes")
		}
		assert.False(t, cfg.GetShowNudes())
		test.Tap(sw)
		s.sa.prefsSM.FlushPending()
		assert.True(t, cfg.GetShowNudes(), "NSFW switch saved")
		test.Tap(sw)
		s.sa.prefsSM.FlushPending()
		assert.False(t, cfg.GetShowNudes())
	})
}

// A7: in each language every visible text is in that language. Opens all four pages and
// every source's detail page (Yours and Community included) and lists the texts that
// still look untranslated: no Chinese characters in Chinese, Chinese characters in English.
func TestA7_NoUntranslatedTextOnAnyPage(t *testing.T) {
	inLanguages(t, func(t *testing.T, lang string) {
		s := a6Scenario(t)
		sm := s.sa.prefsSM
		schema := wallpaper.GetInstance().CreatePrefsSchema(sm)
		var titles []string
		for _, item := range schema.Items {
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

		seen := map[string]string{}
		collect := func(where string) {
			for _, txt := range s.visibleTexts() {
				if _, ok := seen[txt]; !ok {
					seen[txt] = where
				}
			}
		}
		for _, id := range []string{pageSources, pageWallpaper, pageGeneral, pageAdvanced} {
			s.sa.prefsNav.Select(id)
			collect("page " + id)
		}
		s.sa.prefsNav.Select(pageSources)
		for _, title := range titles {
			s.searchEntry().SetText(title)
			s.tapText(title)
			collect("source " + title)
			s.tapNav(a6Names[lang].nav[0])
			s.searchEntry().SetText("")
		}

		var flagged []string
		for txt, where := range seen {
			hasCJK := strings.ContainsFunc(txt, func(r rune) bool { return r >= 0x4e00 && r <= 0x9fff })
			hasWord := false
			for _, w := range strings.FieldsFunc(txt, func(r rune) bool { return !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z') }) {
				if len(w) >= 3 {
					hasWord = true
				}
			}
			switch {
			case lang == "zh" && !hasCJK && hasWord:
				flagged = append(flagged, where+": "+txt)
			case lang == "en" && hasCJK:
				flagged = append(flagged, where+": "+txt)
			}
		}
		sort.Strings(flagged)
		t.Logf("%s: %d distinct texts on 4 pages and %d source pages; %d to review", lang, len(seen), len(titles), len(flagged))
		for _, f := range flagged {
			t.Logf("  REVIEW %s", f)
		}
	})
}
