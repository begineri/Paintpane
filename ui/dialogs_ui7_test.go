package ui

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/asset"
	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/setting"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/pexels"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/wallhaven"
	"github.com/dixieflatline76/Spice/v2/pkg/wallpaper/providers/wikimedia"
)

// Scenario tests for the dialogs of docs/ui_refresh.md §12 v34/v36: the licence window (E), the
// Add collection dialog (AQ), the delete-query confirmation (DQ) and the error dialog (ER).

// overlayAll returns every visible object of the dialog on top.
func overlayAll(s *scenario) []fyne.CanvasObject {
	top := s.w.Canvas().Overlays().Top()
	if top == nil {
		return nil
	}
	return shownObjects(top)
}

// overlayEntries returns the text entries of the dialog on top, top to bottom.
func overlayEntries(s *scenario) []*widget.Entry {
	var out []*widget.Entry
	for _, o := range overlayAll(s) {
		if e, ok := o.(*widget.Entry); ok {
			out = append(out, e)
		}
	}
	return out
}

func overlayTexts(s *scenario) string {
	var all []string
	for _, o := range overlayAll(s) {
		if text, ok := labelText(o); ok {
			all = append(all, text)
		}
	}
	return strings.Join(all, "\n")
}

// ---------------------------------------------------------------------------------------------
// AQ: Add collection
// ---------------------------------------------------------------------------------------------

// openAddDialog opens the dialog the way the Add new collection button of a source's page does.
func openAddDialog(t *testing.T, s *scenario, panel *schema.PanelSchema) {
	t.Helper()
	for _, sec := range panel.Sections {
		for _, item := range sec.Items {
			if b, ok := item.(schema.ButtonItem); ok && strings.HasSuffix(b.Name, "_add") {
				b.OnPressed()
				require.NotNil(t, s.w.Canvas().Overlays().Top(), "the dialog opens")
				return
			}
		}
	}
	t.Fatal("the source has no Add new collection button")
}

func TestScenarioAddCollectionDialog(t *testing.T) {
	inLanguages(t, func(t *testing.T, lang string) {
		want := map[string]struct{ title, urlLabel, name, active, cancel, add, bad, example string }{
			"en": {"Add Pexels collection", "Pexels URL", "Name", "Active", "Cancel", "Add", "That isn't a Pexels URL.", "e.g. Nature"},
			"zh": {"添加 Pexels 收藏", "Pexels 网址", "名称", "已启用", "取消", "添加", "这不是 Pexels 的网址。", "例如：自然"},
		}[lang]
		s := newScenarioWith(t, wallpaper.GetInstance())
		cfg := wallpaper.GetConfigInstance()
		p := pexels.NewProvider(cfg, http.DefaultClient)
		panel := p.CreateQueryPanel(s.sa.prefsSM, "")
		openAddDialog(t, s, panel)

		// AQ1-AQ6: a titled dialog with the source's own label, "Name", the Active switch and
		// the two buttons, and nothing of the old form.
		for _, text := range []string{want.title, want.urlLabel, want.name, want.active} {
			assert.True(t, overlayShows(s, text), "the dialog shows %q", text)
		}
		for _, old := range []string{"URL / search term:", "URL / 搜索词：", "Description:", "描述：", "Everything looks good", "一切看起来都很好", "Save", "保存", "已激活"} {
			assert.NotContains(t, overlayTexts(s), old, "the old form text is gone")
		}
		buttons := dialogButtons(s)
		require.Len(t, buttons, 2)
		assert.Equal(t, want.cancel, buttons[0].Text)
		assert.Equal(t, want.add, buttons[1].Text)
		assert.Equal(t, buttonPrimary, buttons[1].kind)
		assert.True(t, buttons[1].Disabled(), "Add is grey while the form is empty")
		assert.NotContains(t, overlayTexts(s), want.bad, "no red line while nothing is wrong")
		entries := overlayEntries(s)
		require.Len(t, entries, 2)
		assert.Equal(t, want.example, entries[1].PlaceHolder, "the name field hints at an example")
		var sw *toggleSwitch
		for _, o := range overlayAll(s) {
			if x, ok := o.(*toggleSwitch); ok {
				sw = x
			}
		}
		require.NotNil(t, sw, "an Active switch, not a check box")
		assert.True(t, sw.Checked, "on by default")

		// A wrong address gets the red line in plain words, and Add stays grey.
		entries[0].SetText("https://example.com/abc")
		entries[1].SetText("Impressionism")
		assert.True(t, overlayShows(s, want.bad), "the red line explains what is wrong")
		assert.True(t, buttons[1].Disabled())

		// Typing a right address takes the red line away and turns Add on; nothing green appears.
		entries[0].SetText("https://www.pexels.com/search/impressionism/")
		assert.False(t, overlayShows(s, want.bad), "the red line is gone")
		assert.False(t, buttons[1].Disabled(), "Add is on")
		assert.NotContains(t, overlayTexts(s), "Everything looks good")

		// Add adds the collection and closes the dialog.
		before := len(cfg.GetPexelsQueries())
		test.Tap(buttons[1])
		assert.Nil(t, s.w.Canvas().Overlays().Top(), "Add closes the dialog")
		queries := cfg.GetPexelsQueries()
		require.Len(t, queries, before+1)
		added := queries[len(queries)-1]
		assert.Equal(t, "Impressionism", added.Description, "the name is what the list shows")
		assert.True(t, added.Active)
		t.Cleanup(func() { _ = cfg.RemovePexelsQuery(added.ID) })
	})
}

func TestScenarioAddCollectionDialogCancelAndSources(t *testing.T) {
	inLanguages(t, func(t *testing.T, lang string) {
		want := map[string]struct{ whTitle, whLabel, whBad, wmTitle, wmLabel, wmEg string }{
			"en": {"Add wallhaven collection", "wallhaven URL", "That isn't a wallhaven search or collection URL.",
				"Add Wikimedia collection", "Category or search term", "Category:Nature or Search:Pacific Ocean"},
			"zh": {"添加 wallhaven 收藏", "wallhaven 网址", "这不是 wallhaven 的搜索或收藏网址。",
				"添加维基共享资源收藏", "分类或搜索词", "Category:Nature 或 Search:Pacific Ocean"},
		}[lang]
		s := newScenarioWith(t, wallpaper.GetInstance())
		cfg := wallpaper.GetConfigInstance()

		// wallhaven: its own label and its own sentence for a wrong address; Cancel adds nothing.
		wh := wallhaven.NewProvider(cfg, http.DefaultClient)
		openAddDialog(t, s, wh.CreateQueryPanel(s.sa.prefsSM, ""))
		assert.True(t, overlayShows(s, want.whTitle))
		assert.True(t, overlayShows(s, want.whLabel))
		before := len(cfg.GetImageQueries())
		entries := overlayEntries(s)
		entries[0].SetText("not a url")
		assert.True(t, overlayShows(s, want.whBad))
		test.Tap(dialogButtons(s)[0])
		assert.Nil(t, s.w.Canvas().Overlays().Top(), "Cancel closes the dialog")
		assert.Len(t, cfg.GetImageQueries(), before, "and adds nothing")

		// Wikimedia Commons: the address is a category or a search term, anything is accepted.
		wm := wikimedia.NewProvider(cfg, http.DefaultClient)
		openAddDialog(t, s, wm.CreateQueryPanel(s.sa.prefsSM, ""))
		assert.True(t, overlayShows(s, want.wmTitle))
		assert.True(t, overlayShows(s, want.wmLabel))
		entries = overlayEntries(s)
		assert.Equal(t, want.wmEg, entries[0].PlaceHolder)
		entries[0].SetText("Category:Impressionist paintings")
		entries[1].SetText("Impressionist paintings")
		assert.False(t, dialogButtons(s)[1].Disabled(), "any text is a fine search term")
		test.Tap(dialogButtons(s)[0])
	})
}

// ---------------------------------------------------------------------------------------------
// DQ: delete a query
// ---------------------------------------------------------------------------------------------

func TestScenarioDeleteQueryConfirmation(t *testing.T) {
	inLanguages(t, func(t *testing.T, lang string) {
		want := map[string]struct{ title, cancel, del string }{
			"en": {"Delete “Random General”?", "Cancel", "Delete"},
			"zh": {"删除“随机 · 综合”？", "取消", "删除"},
		}[lang]
		s := newScenarioWith(t, wallpaper.GetInstance())
		var removed []string
		queries := []schema.Query{
			{ID: "q1", Description: "Random General", URL: "https://wallhaven.cc/search", Active: true},
			{ID: "fav", Description: "Favorite Images", URL: "favorites://", Active: true, Managed: true},
		}
		list := s.sa.prefsSM.renderQueryList(schema.QueryListItem{
			GetQueries:   func() []schema.Query { return queries },
			EnableQuery:  func(string) error { return nil },
			DisableQuery: func(string) error { return nil },
			RemoveQuery:  func(id string) error { removed = append(removed, id); return nil },
		})
		w := test.NewWindow(list)
		defer w.Close()
		w.Resize(fyne.NewSize(600, 200))

		// Delete is red text, like every destructive button (v43).
		var deletes []*dangerButton
		for _, o := range shownObjects(list) {
			if b, ok := o.(*dangerButton); ok && b.Text == want.del {
				deletes = append(deletes, b)
			}
		}
		require.Len(t, deletes, 2, "a Delete button per row")
		assert.True(t, deletes[1].Disabled(), "the Favorites row keeps its grey Delete, there is no Clear")

		// The question carries the name and nothing else; the delete button is red.
		test.Tap(deletes[0])
		assert.True(t, overlayShows(s, want.title), "the title names the query")
		assert.NotContains(t, overlayTexts(s), "Please confirm")
		assert.NotContains(t, overlayTexts(s), "Are you sure")
		assert.NotContains(t, overlayTexts(s), "请确认")
		buttons := dialogButtons(s)
		require.Len(t, buttons, 2)
		assert.Equal(t, want.cancel, buttons[0].Text)
		assert.Equal(t, want.del, buttons[1].Text)
		assert.Equal(t, buttonDanger, buttons[1].kind)

		test.Tap(buttons[0])
		assert.Nil(t, s.w.Canvas().Overlays().Top())
		assert.Empty(t, removed, "Cancel deletes nothing")

		test.Tap(deletes[0])
		test.Tap(dialogButtons(s)[1])
		assert.Equal(t, []string{"q1"}, removed)

		// A disabled Delete opens nothing.
		test.Tap(deletes[1])
		assert.Nil(t, s.w.Canvas().Overlays().Top())
	})
}

// ---------------------------------------------------------------------------------------------
// ER: error dialog
// ---------------------------------------------------------------------------------------------

func TestScenarioErrorDialog(t *testing.T) {
	_, shortKeyErr := func() (struct{}, error) {
		return struct{}{}, pexels.CheckPexelsAPIKeyWithContext(context.Background(), "short")
	}()
	require.Error(t, shortKeyErr)

	cases := []struct {
		name          string
		err           error
		en, zh        [2]string // title, sentence
		showsOriginal bool
	}{
		{"invalid key", shortKeyErr,
			[2]string{"Couldn't verify the key", "Pexels didn't accept this API key. Check it and try again."},
			[2]string{"无法验证密钥", "Pexels 不接受这个 API 密钥，请检查后再试。"}, false},
		{"no connection", &url.Error{Op: "Get", URL: "https://api.pexels.com/v1", Err: errors.New("dial tcp: no route")},
			[2]string{"Couldn't connect", "Check your connection and try again."},
			[2]string{"无法连接", "请检查网络后再试。"}, false},
		{"unknown user", setting.Known(setting.ErrorUnknownUser, "wallhaven", errors.New("user not found on Wallhaven")),
			[2]string{"Couldn't verify the username", "That user wasn't found, or the account is private."},
			[2]string{"无法验证用户名", "找不到这个用户，或者该账户是私密的。"}, false},
		{"no collections", setting.Known(setting.ErrorNoCollection, "wallhaven", errors.New("API key owner has no collections; favorites sync unavailable")),
			[2]string{"Can't sync favorites", "This wallhaven account has no collections yet."},
			[2]string{"无法同步收藏", "这个 wallhaven 账户还没有收藏夹。"}, false},
		{"anything else", errors.New("failed to decode collections list: unexpected EOF"),
			[2]string{"Something went wrong", "Try again later."},
			[2]string{"出错了", "请稍后再试。"}, true},
	}
	inLanguages(t, func(t *testing.T, lang string) {
		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				want, ok := c.en, "OK"
				if lang == "zh" {
					want, ok = c.zh, "好"
				}
				s := newScenarioWith(t, wallpaper.GetInstance())
				s.sa.prefsSM.ShowError(c.err)

				assert.True(t, overlayShows(s, want[0]), "the title says what failed")
				assert.True(t, overlayShows(s, want[1]), "one plain sentence in the app language")
				assert.NotContains(t, overlayTexts(s), "Error", "not Fyne's stock title")
				assert.Equal(t, c.showsOriginal, overlayShows(s, c.err.Error()),
					"the original text appears only for errors we do not know")
				buttons := dialogButtons(s)
				require.Len(t, buttons, 1, "one button")
				assert.Equal(t, ok, buttons[0].Text)
				assert.Equal(t, buttonPrimary, buttons[0].kind)
				test.Tap(buttons[0])
				assert.Nil(t, s.w.Canvas().Overlays().Top(), "OK closes the dialog")
			})
		}
	})
}

// ---------------------------------------------------------------------------------------------
// E: licence window
// ---------------------------------------------------------------------------------------------

func TestEULATextFollowsTheSystemLanguage(t *testing.T) {
	am := asset.NewManager()
	english, err := am.GetText("eula.txt")
	require.NoError(t, err)
	chinese, err := am.GetText("eula.zh.txt")
	require.NoError(t, err)

	for lang, wantZh := range map[string]bool{
		"zh": true, "zh-Hant": true, // Simplified or Traditional Chinese: the translation
		"en": false, "de": false, "ja": false, "": false, // anything else, or not recognised: English
	} {
		got, err := eulaTextFor(am, lang)
		require.NoError(t, err)
		if wantZh {
			assert.Equal(t, chinese, got, "language %q", lang)
		} else {
			assert.Equal(t, english, got, "language %q", lang)
		}
	}
}

func TestEULAWindowContent(t *testing.T) {
	inLanguages(t, func(t *testing.T, lang string) {
		want := map[string]struct{ title, heading, section, decline, accept string }{
			"en": {"License agreement", "End-User License Agreement (EULA) for Paintpane", "2.6 National Palace Museum, Taipei", "Decline", "Accept"},
			"zh": {"许可协议", "观画壁纸 最终用户许可协议", "2.6 台北故宫博物院", "拒绝", "接受"},
		}[lang]
		assert.Equal(t, want.title, i18n.T("License agreement"), "the window title")

		text, err := eulaTextFor(asset.NewManager(), i18n.EffectiveLanguage())
		require.NoError(t, err)

		var accepted, declined int
		w := test.NewWindow(newEULAContent(text, func() { declined++ }, func() { accepted++ }))
		defer w.Close()
		w.Resize(fyne.NewSize(800, 600))

		var all []string
		var buttons []*dialogButton
		for _, o := range shownObjects(w.Canvas().Content()) {
			if txt, ok := labelText(o); ok {
				all = append(all, txt)
			}
			if b, ok := o.(*dialogButton); ok {
				buttons = append(buttons, b)
			}
		}
		joined := strings.Join(all, "\n")
		assert.Contains(t, joined, want.heading)
		assert.Contains(t, joined, want.section)
		assert.Contains(t, joined, "國立故宮博物院，臺北，CC BY 4.0 @ www.npm.gov.tw")
		assert.NotContains(t, joined, "Google", "no Google Photos section")
		assert.NotContains(t, joined, "To continue using Paintpane", "no prompt line")
		assert.NotContains(t, joined, "要继续使用观画壁纸", "no prompt line")

		require.Len(t, buttons, 2)
		assert.Equal(t, want.decline, buttons[0].Text)
		assert.Equal(t, want.accept, buttons[1].Text)
		assert.Equal(t, buttonNeutral, buttons[0].kind, "Decline is a plain button")
		assert.Equal(t, buttonPrimary, buttons[1].kind, "Accept is the accent button")
		test.Tap(buttons[0])
		assert.Equal(t, 1, declined)
		test.Tap(buttons[1])
		assert.Equal(t, 1, accepted)
	})
}
