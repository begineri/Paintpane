package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"
	"unicode/utf16"
)

// Fyne reads preferences.json with a plain JSON decoder. A file that an editor saved with a byte
// order mark (old Notepad, PowerShell 5.1's Out-File -Encoding utf8) fails at the first byte, Fyne
// then runs on empty preferences, and its first save writes them over the person's file (A2 §14.9
// #4). RepairPreferencesFile runs before Fyne loads the file.

var (
	bomUTF8    = []byte{0xEF, 0xBB, 0xBF}
	bomUTF16LE = []byte{0xFF, 0xFE}
	bomUTF16BE = []byte{0xFE, 0xFF}
)

// FynePreferencesPath returns where Fyne keeps the preferences of the app with this ID, the same
// way fyne.io/fyne/v2/internal/app's rootConfigDir does on desktops.
func FynePreferencesPath(appID string) string {
	var root string
	switch runtime.GOOS {
	case "windows":
		home, _ := os.UserHomeDir()
		root = filepath.Join(home, "AppData", "Roaming")
	case "darwin":
		home, _ := os.UserHomeDir()
		root = filepath.Join(home, "Library", "Preferences")
	default:
		root, _ = os.UserConfigDir()
	}
	return filepath.Join(root, "fyne", appID, "preferences.json")
}

// RepairPreferencesFile makes the preferences file readable by Fyne before it is loaded:
//
//   - A UTF-8 byte order mark is removed, and a UTF-16 file with a byte order mark is converted
//     to UTF-8. The file is rewritten only if the result is valid JSON.
//   - A file that still is not valid JSON is copied next to itself as
//     preferences.json.broken-<time>, so the settings in it survive Fyne's first save.
//
// It returns a sentence for the log when it did something, or "" when the file was fine or absent.
func RepairPreferencesFile(path string) (string, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- the app's own preferences file
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return "", nil // Fyne treats an empty store as a fresh start
	}

	fixed, encoding := withoutByteOrderMark(data)
	if json.Valid(fixed) {
		if encoding == "" {
			return "", nil
		}
		if err := writeFileAtomically(path, fixed); err != nil {
			return "", fmt.Errorf("remove the %s byte order mark from %s: %w", encoding, path, err)
		}
		return fmt.Sprintf("Preferences file %s started with a %s byte order mark, which Fyne cannot read; removed it, the settings are kept.", path, encoding), nil
	}

	backup := fmt.Sprintf("%s.broken-%s", path, time.Now().Format("20060102-150405"))
	if err := os.WriteFile(backup, data, 0o600); err != nil {
		return "", fmt.Errorf("preferences file %s is not valid JSON and could not be backed up: %w", path, err)
	}
	return fmt.Sprintf("Preferences file %s is not valid JSON; Fyne will start with default settings and overwrite it. The original is kept as %s.", path, backup), nil
}

// withoutByteOrderMark returns data as UTF-8 without a byte order mark, and the name of the mark
// it found ("" for none).
func withoutByteOrderMark(data []byte) ([]byte, string) {
	switch {
	case bytes.HasPrefix(data, bomUTF8):
		return data[len(bomUTF8):], "UTF-8"
	case bytes.HasPrefix(data, bomUTF16LE):
		return utf16ToUTF8(data[len(bomUTF16LE):], false), "UTF-16 LE"
	case bytes.HasPrefix(data, bomUTF16BE):
		return utf16ToUTF8(data[len(bomUTF16BE):], true), "UTF-16 BE"
	}
	return data, ""
}

func utf16ToUTF8(b []byte, bigEndian bool) []byte {
	units := make([]uint16, len(b)/2)
	for i := range units {
		if bigEndian {
			units[i] = uint16(b[2*i])<<8 | uint16(b[2*i+1])
		} else {
			units[i] = uint16(b[2*i+1])<<8 | uint16(b[2*i])
		}
	}
	return []byte(string(utf16.Decode(units)))
}

// writeFileAtomically replaces path with data through a temporary file in the same folder, so a
// failure half-way leaves the old file in place.
func writeFileAtomically(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	_, err = tmp.Write(data)
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
	}
	return err
}
