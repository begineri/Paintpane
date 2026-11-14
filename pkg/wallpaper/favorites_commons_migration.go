package wallpaper

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/dixieflatline76/Spice/v2/config"
	"github.com/dixieflatline76/Spice/v2/util/log"
)

// commonsFavoriteLinksMigration is the AppliedMigrations name of FixCommonsFavoriteLinksStep.
const commonsFavoriteLinksMigration = "favorites_commons_file_pages"

// commonsFilePageBase starts the page of a file on Wikimedia Commons, as imageinfo's
// descriptionurl gives it.
const commonsFilePageBase = "https://commons.wikimedia.org/wiki/File:"

// favoritesDirForMigrations is the Favorites folder the config migrations work on. In a test
// binary it is empty unless the test sets it: config.GetAppDir is fixed when the process starts,
// from the real LOCALAPPDATA, so every test that loads a Config would otherwise rewrite the
// favourites of whoever runs the tests.
var favoritesDirForMigrations = func() string {
	if testing.Testing() {
		return ""
	}
	return filepath.Join(config.GetAppDir(), FavoritesCollection)
}

// FixCommonsFavoriteLinksStep points Wikimedia favourites saved before 2026-11-12 at the file's
// page on Commons. The Favorites folder keeps each favourite's page in metadata.json
// (product_url), and for Wikimedia that used to be the image file itself
// (https://upload.wikimedia.org/...), so the tray's first row opened the bare picture instead of
// the page with its title, author and licence. Wikimedia images in the image cache were fixed in
// ImageStore.fixCommonsViewURLsLocked; this does the same for favourites.
//
// It runs once (AppliedMigrations). Links it does not recognise stay as they are, and only
// metadata.json is written, never the favourite images. If metadata.json cannot be rewritten the
// step is not recorded, so it is tried again at the next start.
func FixCommonsFavoriteLinksStep(cfg *Config) (bool, error) {
	if slices.Contains(cfg.AppliedMigrations, commonsFavoriteLinksMigration) {
		return false, nil
	}
	dir := favoritesDirForMigrations()
	if dir == "" {
		return false, nil
	}
	fixed, err := fixCommonsFavoriteLinks(dir)
	if err != nil {
		// Not returned: the chain would stop here and skip the steps after it.
		log.Printf("Migration: Wikimedia favourites keep their old links for now: %v", err)
		return false, nil
	}
	if fixed > 0 {
		log.Printf("Migration: %d Wikimedia favourites now link to their pages on Commons.", fixed)
	}
	cfg.AppliedMigrations = append(cfg.AppliedMigrations, commonsFavoriteLinksMigration)
	return true, nil
}

// fixCommonsFavoriteLinks rewrites the Commons file links in dir/metadata.json and returns how many
// it changed. A missing or unreadable metadata.json has nothing to fix.
func fixCommonsFavoriteLinks(dir string) (int, error) {
	metaFile := filepath.Join(dir, "metadata.json")
	data, err := os.ReadFile(metaFile)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	var meta map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber() // write numbers back exactly as they were
	if err := dec.Decode(&meta); err != nil {
		log.Printf("Migration: Favourites metadata.json is not valid JSON, left as it is: %v", err)
		return 0, nil
	}
	files, _ := meta["files"].(map[string]any)

	fixed := 0
	for filename, v := range files {
		switch entry := v.(type) {
		case map[string]any:
			old, _ := entry["product_url"].(string)
			if page, ok := commonsFilePage(old, filename); ok {
				entry["product_url"] = page
				fixed++
			}
		case string: // the oldest form: the page alone
			if page, ok := commonsFilePage(entry, filename); ok {
				files[filename] = page
				fixed++
			}
		}
	}
	if fixed == 0 {
		return 0, nil
	}

	out, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return 0, err
	}
	// Write a new file and move it over the old one, so metadata.json is never half written.
	tmp := metaFile + ".tmp"
	if err := os.WriteFile(tmp, out, 0600); err != nil {
		return 0, err
	}
	if err := os.Rename(tmp, metaFile); err != nil {
		_ = os.Remove(tmp)
		return 0, fmt.Errorf("replace %s: %w", metaFile, err)
	}
	return fixed, nil
}

// commonsFilePage returns the Commons page for link when link is a Commons image file
// (https://upload.wikimedia.org/wikipedia/commons/a/ab/Name.jpg, or a scaled copy under
// .../commons/thumb/a/ab/Name.jpg/1280px-Name.jpg): https://commons.wikimedia.org/wiki/File:Name.jpg.
// A Commons file link whose name cannot be read (an old version under .../commons/archive/, say)
// falls back to the page by its page ID when the favourite was saved from the Wikimedia source
// (filename Wikimedia_<page ID>.jpg), as the provider does. Anything else is not a Commons file
// link, or not one it can place, and ok is false.
func commonsFilePage(link, filename string) (page string, ok bool) {
	u, err := url.Parse(link)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || !strings.EqualFold(u.Host, "upload.wikimedia.org") {
		return "", false
	}
	segs := strings.Split(strings.TrimPrefix(u.EscapedPath(), "/"), "/")
	if len(segs) < 3 || segs[0] != "wikipedia" || segs[1] != "commons" {
		return "", false
	}

	// wikipedia/commons/[thumb/]<h>/<hh>/<Name>[/<width>px-<Name>]
	i := 2
	if segs[i] == "thumb" {
		i++
	}
	if len(segs) > i+2 && isHashDir(segs[i], 1) && isHashDir(segs[i+1], 2) &&
		strings.HasPrefix(segs[i+1], segs[i]) && segs[i+2] != "" {
		return commonsFilePageBase + segs[i+2], true
	}

	stem := strings.TrimSuffix(filename, filepath.Ext(filename))
	if pageID, found := strings.CutPrefix(stem, "Wikimedia_"); found && pageID != "" && strings.Trim(pageID, "0123456789") == "" {
		return "https://commons.wikimedia.org/w/index.php?curid=" + pageID, true
	}
	return "", false
}

// isHashDir reports whether s is one of the hex folders Commons files are spread over.
func isHashDir(s string, n int) bool {
	return len(s) == n && strings.Trim(s, "0123456789abcdef") == ""
}
