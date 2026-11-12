package wallpaper

import (
	"context"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/config"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/util"
)

// A query goes on from the page it reached last time, across restarts, instead of fetching page 1
// (whose images the library already has) at every start.

// pagedSource serves pages of 3 images ("p<page>_<n>") up to lastPage, then empty pages, and
// records which pages were asked for. With cursors set it pages like Wikimedia: page N > 1 needs
// the cursor page N-1 returned, and is empty without it.
type pagedSource struct {
	MockImageProvider
	lastPage int
	cursors  bool

	mu     sync.Mutex
	asked  []int
	tokens map[int]string // page -> cursor
}

func newPagedSource(lastPage int, cursors bool) *pagedSource {
	s := &pagedSource{lastPage: lastPage, cursors: cursors, tokens: map[int]string{}}
	s.On("ID").Return("A")
	s.On("Type").Return(provider.TypeCommunity)
	return s
}

func (s *pagedSource) FetchImages(_ context.Context, _ string, page int) ([]provider.Image, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.asked = append(s.asked, page)
	if s.cursors && page > 1 {
		if _, ok := s.tokens[page]; !ok {
			return nil, nil
		}
	}
	if page > s.lastPage {
		return nil, nil
	}
	var out []provider.Image
	for i := 1; i <= 3; i++ {
		out = append(out, provider.Image{ID: fmt.Sprintf("p%d_%d", page, i)})
	}
	if s.cursors {
		s.tokens[page+1] = fmt.Sprintf("after-%d", page)
	}
	return out, nil
}

func (s *pagedSource) pagesAsked() []int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]int(nil), s.asked...)
}

// cursorSource is a pagedSource that hands its cursors to the app (provider.PageCursorProvider).
type cursorSource struct{ *pagedSource }

func (c cursorSource) PageCursor(_ string, page int) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	t, ok := c.tokens[page]
	return t, ok
}

func (c cursorSource) SetPageCursor(_ string, page int, cursor string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tokens[page] = cursor
}

// session is one run of the app: a plugin that loads the saved pages (Init) and restores cursors
// (Activate), with the source given.
func session(t *testing.T, src provider.ImageProvider) *Plugin {
	t.Helper()
	wp := newTestPlugin(t)
	wp.store.SetAsyncSave(false)
	wp.cfg = &Config{
		Preferences: NewMockPreferences(),
		Queries:     []ImageQuery{{ID: "q1", Provider: "A", Active: true, URL: "category:Paintings"}},
	}
	wp.cfg.SetCacheSize(Cache100Images)
	mockOS := &MockOS{}
	mockOS.On("GetMonitors").Return([]Monitor{{ID: 0, Rect: image.Rect(0, 0, 100, 100)}}, nil)
	wp.os = mockOS
	wp.jobSubmitter = &recordingSubmitter{}
	wp.ctx = context.Background()
	wp.providers["A"] = src
	require.NoError(t, os.MkdirAll(filepath.Dir(queryStatePath("query_pages.json")), 0o755))
	wp.loadQueryPages()
	wp.restorePageCursors()
	return wp
}

func isolateWorkingDir(t *testing.T) {
	t.Helper()
	t.Setenv("LOCALAPPDATA", t.TempDir()) // config.GetWorkingDir follows it; never the person's data
	require.Contains(t, config.GetWorkingDir(), os.Getenv("LOCALAPPDATA"))
}

func TestStartsGoOnFromTheSavedPage(t *testing.T) {
	isolateWorkingDir(t)
	src := newPagedSource(3, false)

	runFetchRound(t, session(t, src)) // first start: page 1
	runFetchRound(t, session(t, src)) // second start: page 2, not page 1 again
	runFetchRound(t, session(t, src)) // third start: page 3
	runFetchRound(t, session(t, src)) // fourth: past the end, so the query starts over
	runFetchRound(t, session(t, src)) // fifth: page 1 again
	assert.Equal(t, []int{1, 2, 3, 4, 1}, src.pagesAsked())
}

func TestStartsGoOnWithASavedCursor(t *testing.T) {
	isolateWorkingDir(t)

	first := cursorSource{newPagedSource(5, true)}
	runFetchRound(t, session(t, first))
	runFetchRound(t, session(t, first)) // same process: the source still has its cursor
	assert.Equal(t, []int{1, 2}, first.pagesAsked())

	// A restart: a new source that knows no cursors gets the saved one back and goes on at page 3.
	second := cursorSource{newPagedSource(5, true)}
	wp := session(t, second)
	cursor, ok := second.PageCursor("category:Paintings", 3)
	require.True(t, ok, "the cursor for page 3 is given back at start")
	assert.Equal(t, "after-2", cursor)
	runFetchRound(t, wp)
	assert.Equal(t, []int{3}, second.pagesAsked())
	assert.Contains(t, submittedIDs(wp), "A_p3_1")
}

func TestStartWithoutTheCursorStartsTheQueryOver(t *testing.T) {
	isolateWorkingDir(t)
	runFetchRound(t, session(t, cursorSource{newPagedSource(5, true)}))
	runFetchRound(t, session(t, cursorSource{newPagedSource(5, true)})) // page 2, cursor saved
	require.NoError(t, os.Remove(queryStatePath("query_cursors.json"))) // as saved by an older version

	src := cursorSource{newPagedSource(5, true)}
	wp := session(t, src)
	assert.Equal(t, 1, wp.queryPages["q1"].Value(), "without a cursor the query starts over at page 1")
	runFetchRound(t, wp)
	assert.Equal(t, []int{1}, src.pagesAsked())
}

func submittedIDs(wp *Plugin) []string {
	return wp.jobSubmitter.(*recordingSubmitter).ids()
}

// A fetch that is stopped while it queues a page stays on that page: pages are kept across
// restarts, so moving on would skip the images not queued.
func TestStoppedFetchStaysOnThePage(t *testing.T) {
	isolateWorkingDir(t)
	wp := session(t, newPagedSource(3, false))
	wp.jobSubmitter = refusingSubmitter{}
	var mu sync.Mutex
	wp.fetchFromProvider(context.Background(), wp.cfg.Queries[0], wp.providers["A"], false, 10, &mu, map[string]bool{}, util.NewSafeInt())
	assert.Equal(t, 1, wp.queryPages["q1"].Value())
}

type refusingSubmitter struct{}

func (refusingSubmitter) Submit(context.Context, DownloadJob) bool { return false }

// The nightly round goes on from where the starts left off (scheduler.go never resets pages), and
// so does the round started when the images run out (RequestFetch → FetchNewImages).
func TestNightlyAndStarvationRoundsGoOnFromTheSavedPage(t *testing.T) {
	isolateWorkingDir(t)
	src := newPagedSource(5, false)
	runFetchRound(t, session(t, src))
	wp := session(t, src)
	wp.RequestFetch()
	require.Eventually(t, func() bool { return len(src.pagesAsked()) == 2 }, 5*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool { return !wp.fetchingInProgress.Value() }, 5*time.Second, 10*time.Millisecond)
	assert.Equal(t, []int{1, 2}, src.pagesAsked())
	runFetchRound(t, wp) // the nightly refresh calls FetchNewImages(false) the same way
	assert.Equal(t, []int{1, 2, 3}, src.pagesAsked())
}

// Activate used to set every query back to page 1, discarding the pages loaded at Init.
func TestActivateKeepsTheSavedPages(t *testing.T) {
	isolateWorkingDir(t)
	defer ResetConfig()
	app := newE2EApp(t, t.TempDir(), NewMockPreferences(), e2eMonitor(0, 800, 500))
	app.wp.queryPages["q1"] = util.NewSafeIntWithValue(4)
	app.wp.Activate()
	defer app.stop()
	assert.Equal(t, 4, app.wp.queryPages["q1"].Value())
}
