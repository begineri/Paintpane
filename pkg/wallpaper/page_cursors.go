package wallpaper

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/dixieflatline76/Spice/v2/config"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/util/log"
)

// Each query remembers which page it fetches next (query_pages.json), across restarts, so a start
// or a nightly round asks for images the library does not have yet instead of page 1 again. A page
// that comes back empty means the query reached its end and it starts over at page 1
// (fetchFromProvider); a changed search or collection gets a new query ID and starts at page 1.
//
// Every round goes on from the saved page: a start (startupFetch), the nightly refresh, the round
// started when every image has been shown (RequestFetch), and the round after the person changes
// sources in Settings (RefreshImagesAndPulse). Only adding a favourite sets the Favorites query back
// to page 1, so the new file is found at once. A page that still has images the cache had no room
// for, or that a stopped fetch did not finish queuing, is fetched again rather than skipped.
//
// Most sources work out a page from its number alone (a slice of a fixed list, or a page number
// sent to their API). Wikimedia Commons needs a continuation cursor from the page before
// (provider.PageCursorProvider); those cursors are saved next to the page numbers, in
// query_cursors.json, and given back at start.

// savedCursor is the cursor a query needs to fetch its next page.
type savedCursor struct {
	Page   int    `json:"page"`
	Cursor string `json:"cursor"`
}

func queryStatePath(name string) string {
	return filepath.Join(config.GetWorkingDir(), strings.ToLower(pluginName)+"_downloads", name)
}

// cursorQueries returns, for each configured query whose source pages with a cursor, the query and
// its source.
func (wp *Plugin) cursorQueries() map[string]struct {
	q ImageQuery
	p provider.PageCursorProvider
} {
	out := map[string]struct {
		q ImageQuery
		p provider.PageCursorProvider
	}{}
	if wp.cfg == nil {
		return out
	}
	for _, q := range wp.cfg.GetQueries() {
		if cp, ok := wp.providers[q.Provider].(provider.PageCursorProvider); ok {
			out[q.ID] = struct {
				q ImageQuery
				p provider.PageCursorProvider
			}{q, cp}
		}
	}
	return out
}

// savePageCursors writes the cursor of every cursor-paged query's next page.
func (wp *Plugin) savePageCursors(pages map[string]int) {
	saved := map[string]savedCursor{}
	for id, c := range wp.cursorQueries() {
		page := pages[id]
		if page <= 1 {
			continue // page 1 needs no cursor
		}
		if cursor, ok := c.p.PageCursor(c.q.URL, page); ok {
			saved[id] = savedCursor{Page: page, Cursor: cursor}
		}
	}
	data, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return
	}
	if err := os.WriteFile(queryStatePath("query_cursors.json"), data, 0o600); err != nil {
		log.Printf("[ERROR] Failed to write query_cursors.json: %v", err)
	}
}

// restorePageCursors gives each cursor-paged query back the cursor for the page it is on. A query
// whose cursor is missing (saved by an older version) or for another page starts over at page 1,
// which is what the source would do anyway without a cursor.
func (wp *Plugin) restorePageCursors() {
	saved := map[string]savedCursor{}
	if data, err := os.ReadFile(queryStatePath("query_cursors.json")); err == nil {
		if err := json.Unmarshal(data, &saved); err != nil {
			log.Printf("[ERROR] Failed to parse query_cursors.json: %v", err)
		}
	}
	for id, c := range wp.cursorQueries() {
		wp.downloadMutex.RLock()
		pg, ok := wp.queryPages[id]
		wp.downloadMutex.RUnlock()
		if !ok || pg.Value() <= 1 {
			continue
		}
		if _, has := c.p.PageCursor(c.q.URL, pg.Value()); has {
			continue // activated again in the same run: the source still has it
		}
		if s, ok := saved[id]; ok && s.Page == pg.Value() {
			c.p.SetPageCursor(c.q.URL, s.Page, s.Cursor)
			continue
		}
		log.Debugf("Query %s has no cursor for page %d; starting it over at page 1", id, pg.Value())
		pg.Set(1)
	}
}
