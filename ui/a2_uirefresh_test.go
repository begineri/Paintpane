//go:build screenshots

package ui

// A2 acceptance of the interface refresh (docs/ui_refresh.md section 12): the rows the
// owner confirmed, checked against the real preferences window with the real
// wallpaper plugin, in English and Simplified Chinese.
//
//	go test -tags screenshots ./ui/ -run TestA2 -count=1 -v
//	A2_DUMP=out.txt go test -tags screenshots ./ui/ -run TestA2_DumpPages   (every visible text)

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
)

type a2Words struct {
	personalization, accent, singleDisplay, displayMode, picture string
	blocked, view, cacheNone, npmLink, ccZero                    string
	yours                                                        string
	// gone are texts the confirmed rows removed from the pages.
	gonePages  []string
	goneSource []string
}

var a2Names = map[string]a2Words{
	"en": {
		personalization: "Personalization", accent: "Accent color",
		singleDisplay: "Single-display shortcuts (hold Alt + display number)",
		displayMode:   "Display mode", picture: "Picture",
		blocked: "Blocked images", view: "View", cacheNone: "No images",
		npmLink: "Open data (CC BY 4.0)", ccZero: "Open Access (CC0)", yours: "Yours · 2",
		gonePages: []string{"Crop strategy", "Face priority", "Frame images that don't fit", "Per-display shortcuts",
			"Displays", "Refresh displays", "Detect displays", "Google Photos", "An unofficial build of Spice.",
			"Download new wallpapers each night.", "Switching rebuilds your wallpapers in the background."},
	},
	"zh": {
		personalization: "个性化", accent: "主题色",
		singleDisplay: "单台显示器快捷键（按住 Alt + 显示器编号）",
		displayMode:   "显示方式", picture: "画面",
		blocked: "已屏蔽的图片", view: "查看", cacheNone: "0 张",
		npmLink: "开放资料（CC BY 4.0）", ccZero: "开放获取（CC0）", yours: "我的 · 2",
		gonePages: []string{"裁切策略", "人脸优先", "加框显示不合适的图", "按显示器操作", "刷新显示器", "Google 相册",
			"这是 Spice 的非官方修改版。", "每晚下载新的壁纸。"},
	},
}

func TestA2_ConfirmedRowsOfTheInterfaceRefresh(t *testing.T) {
	inLanguages(t, func(t *testing.T, lang string) {
		w, nav := a2Names[lang], a6Names[lang]
		s := a6Scenario(t)

		var all []string
		for i := range s.sa.prefsNav.pages {
			s.tapNav(nav.nav[i])
			all = append(all, s.visibleTexts()...)
		}
		joined := "\n" + strings.Join(all, "\n") + "\n"
		for _, gone := range w.gonePages {
			assert.NotContains(t, joined, "\n"+gone+"\n", "%s: %q was removed by a confirmed row", lang, gone)
		}

		s.tapNav(nav.nav[0])
		assert.True(t, s.shows(w.yours), "%s: Yours group", lang)
		s.tapNav(nav.nav[1])
		assert.True(t, s.shows(w.picture) && s.shows(w.displayMode), "%s: Wallpaper page has Picture / Display mode", lang)
		s.tapNav(nav.nav[2])
		assert.True(t, s.shows(w.personalization) && s.shows(w.accent) && s.shows(w.singleDisplay), "%s: General page", lang)
		s.tapNav(nav.nav[3])
		assert.True(t, s.shows(w.blocked) && s.shows(w.view) && s.shows(w.cacheNone), "%s: Advanced page", lang)

		// National Palace Museum: the first link is CC BY 4.0, the others keep CC0.
		s.tapNav(nav.nav[0])
		npm := map[string]string{"en": "National Palace Museum", "zh": "台北故宫博物院"}[lang]
		s.searchEntry().SetText(npm)
		s.tapText(npm)
		assert.True(t, s.shows(w.npmLink), "%s: NPM detail page shows %q", lang, w.npmLink)
		s.tapNav(nav.nav[0])
		s.searchEntry().SetText("")
		met := map[string]string{"en": "The Metropolitan Museum of Art", "zh": "大都会艺术博物馆"}[lang]
		s.searchEntry().SetText(met)
		s.tapText(met)
		assert.True(t, s.shows(w.ccZero), "%s: Met detail page shows %q", lang, w.ccZero)
	})
}

func TestA2_DumpPages(t *testing.T) {
	var b strings.Builder
	inLanguages(t, func(t *testing.T, lang string) {
		want := a6Names[lang]
		s := a6Scenario(t)
		dump := func(where string) {
			fmt.Fprintf(&b, "## [%s] %s\n", lang, where)
			seen := map[string]bool{}
			for _, txt := range s.visibleTexts() {
				if !seen[txt] {
					seen[txt] = true
					fmt.Fprintf(&b, "  %s\n", txt)
				}
			}
		}
		for i := range s.sa.prefsNav.pages {
			s.tapNav(want.nav[i])
			dump("page " + want.nav[i])
		}
		s.tapNav(want.nav[0])
		schema := wallpaper.GetInstance().CreatePrefsSchema(s.sa.prefsSM)
		for _, item := range schema.Items {
			if item.Sources == nil {
				continue
			}
			for _, g := range item.Sources.Groups {
				for _, src := range g.Items {
					s.searchEntry().SetText(src.Title)
					s.tapText(src.Title)
					dump("source " + src.Title)
					s.tapNav(want.nav[0])
					s.searchEntry().SetText("")
				}
			}
		}
	})
	if p := os.Getenv("A2_DUMP"); p != "" {
		if err := os.WriteFile(p, []byte(b.String()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
