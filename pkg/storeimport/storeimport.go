// Package storeimport brings the portable exe's data into the Microsoft Store edition
// (docs/store-feasibility.md §7.2 and §10 S5; docs/ui_refresh.md §12 v51, X–X6).
//
// The two editions keep their data apart (config.DataID): the portable exe uses
// %APPDATA%\fyne\Paintpane and %LOCALAPPDATA%\Paintpane, the Store edition PaintpaneStore. On the
// Store edition's first start, after the licence, the app asks once whether to bring over the
// portable exe's settings, favorites and blocked images. Importing copies the favorite images and
// the blocked-image thumbnails, writes the portable exe's settings to a pending file and restarts
// the app; the next start puts that file in place before Fyne reads its preferences
// (ApplyPending), so nothing that is already loaded has to be reloaded. The portable exe's files
// are only read.
//
// Inside the package, %LOCALAPPDATA% is virtualized: reading a file the package has no copy of
// reads the real one, which is how the portable exe's favorites are found. %APPDATA% is not
// virtualized (§11.3).
package storeimport

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/dixieflatline76/Spice/v2/config"
)

const (
	// AskedKey is the Store edition's preference that records that the question was answered (or
	// that there was nothing to import), so it is asked at most once.
	AskedKey = "store_import_asked"
	// PendingName is the settings file an import leaves for the next start.
	PendingName = "import_pending.json"
	// RestartFlag is what the restart after an import passes, so the new process waits for the old
	// one to let go of the single-instance lock.
	RestartFlag = "--after-import"
)

// Folders, relative to the data folder, that hold what the person chose rather than what the app
// can download again: the favorite images (with their metadata.json) and the thumbnails of blocked
// images (pkg/wallpaper: FavoritesCollection, blockedThumbDirName).
var DataFolders = []string{"favorite_images", "blocked_thumbs"}

// NotImported are preference keys the import leaves out: the licence acceptance (the Store
// edition's own acceptance is kept) and the portable exe's first-run registration with Windows.
var NotImported = []string{"eula_acceptance", "autostart_first_run_done"}

// WebPrefsPath is the portable exe's settings file.
func WebPrefsPath() string { return config.FynePreferencesPath(config.AppName) }

// WebDataDir is the portable exe's data folder (%LOCALAPPDATA%\Paintpane).
func WebDataDir() string {
	cache, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(cache, strings.ToLower(config.AppName))
}

// PendingPath is where an import leaves the settings for the next start, in this edition's data
// folder.
func PendingPath() string { return filepath.Join(config.GetWorkingDir(), PendingName) }

// Found reports whether the portable exe's settings file is there and has something in it.
func Found(webPrefs string) bool {
	info, err := os.Stat(webPrefs)
	return err == nil && !info.IsDir() && info.Size() > 2
}

// Prepare reads the portable exe's settings, leaves out the keys in drop, sets the values in keep
// (this edition's licence acceptance, AskedKey) and writes the result to pending.
func Prepare(webPrefs, pending string, drop []string, keep map[string]any) error {
	data, err := os.ReadFile(webPrefs)
	if err != nil {
		return fmt.Errorf("read the settings to import: %w", err)
	}
	data = trimBOM(data)
	values := map[string]any{}
	if err := json.Unmarshal(data, &values); err != nil {
		return fmt.Errorf("read the settings to import: %w", err)
	}
	for _, k := range drop {
		delete(values, k)
	}
	for k, v := range keep {
		values[k] = v
	}
	out, err := json.Marshal(values)
	if err != nil {
		return err
	}
	return writeAtomic(pending, out)
}

// ApplyPending moves a pending import over this edition's settings file, before Fyne reads it. It
// reports whether there was one.
func ApplyPending(pending, prefs string) (bool, error) {
	if _, err := os.Stat(pending); errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	data, err := os.ReadFile(pending)
	if err != nil {
		return false, err
	}
	if err := writeAtomic(prefs, data); err != nil {
		return false, err
	}
	return true, os.Remove(pending)
}

// CopyData copies DataFolders from the portable exe's data folder to this edition's, keeping files
// that are already there. It returns how many files it copied. A folder the portable exe does not
// have is skipped.
func CopyData(webDir, storeDir string) (int, error) {
	copied := 0
	for _, name := range DataFolders {
		src := filepath.Join(webDir, name)
		if info, err := os.Stat(src); err != nil || !info.IsDir() {
			continue
		}
		dst := filepath.Join(storeDir, name)
		err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(src, path)
			if err != nil {
				return err
			}
			target := filepath.Join(dst, rel)
			if d.IsDir() {
				return os.MkdirAll(target, 0o755)
			}
			if !d.Type().IsRegular() {
				return nil
			}
			if _, err := os.Stat(target); err == nil {
				return nil
			}
			if err := copyFile(path, target); err != nil {
				return err
			}
			copied++
			return nil
		})
		if err != nil {
			return copied, fmt.Errorf("copy %s: %w", name, err)
		}
	}
	return copied, nil
}

func trimBOM(b []byte) []byte {
	return bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, dst)
	}
	if err != nil {
		_ = os.Remove(tmp)
	}
	return err
}
