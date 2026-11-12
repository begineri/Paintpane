package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf16"

	fyneapp "fyne.io/fyne/v2/app"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const samplePrefs = `{"app_language":"zh","wallpaper_cache_size_key":4,"title":"星夜"}`

func writePrefs(t *testing.T, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "preferences.json")
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}

func backups(t *testing.T, path string) []string {
	t.Helper()
	m, err := filepath.Glob(path + ".broken-*")
	require.NoError(t, err)
	return m
}

func TestRepairPreferencesFile_UTF8BOMRemoved(t *testing.T) {
	path := writePrefs(t, append([]byte{0xEF, 0xBB, 0xBF}, samplePrefs...))

	note, err := RepairPreferencesFile(path)
	require.NoError(t, err)
	assert.Contains(t, note, "UTF-8 byte order mark")
	assert.Contains(t, note, path, "the log says which file")
	assert.Equal(t, samplePrefs, readFile(t, path), "the settings are kept, only the mark goes")
	assert.Empty(t, backups(t, path))

	// Nothing left to do the second time.
	note, err = RepairPreferencesFile(path)
	require.NoError(t, err)
	assert.Empty(t, note)
}

func utf16Bytes(s string, bigEndian bool) []byte {
	var out []byte
	if bigEndian {
		out = []byte{0xFE, 0xFF}
	} else {
		out = []byte{0xFF, 0xFE}
	}
	for _, u := range utf16.Encode([]rune(s)) {
		if bigEndian {
			out = append(out, byte(u>>8), byte(u))
		} else {
			out = append(out, byte(u), byte(u>>8))
		}
	}
	return out
}

func TestRepairPreferencesFile_UTF16Converted(t *testing.T) {
	for _, be := range []bool{false, true} {
		path := writePrefs(t, utf16Bytes(samplePrefs, be))
		note, err := RepairPreferencesFile(path)
		require.NoError(t, err)
		assert.Contains(t, note, "UTF-16")
		assert.Equal(t, samplePrefs, readFile(t, path), "Chinese text survives the conversion")
	}
}

func TestRepairPreferencesFile_ValidFileUntouched(t *testing.T) {
	path := writePrefs(t, []byte(samplePrefs))
	before, err := os.Stat(path)
	require.NoError(t, err)

	note, err := RepairPreferencesFile(path)
	require.NoError(t, err)
	assert.Empty(t, note)
	after, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, before.ModTime(), after.ModTime(), "a good file is not rewritten")
	assert.Empty(t, backups(t, path))
}

func TestRepairPreferencesFile_MissingOrEmpty(t *testing.T) {
	note, err := RepairPreferencesFile(filepath.Join(t.TempDir(), "preferences.json"))
	require.NoError(t, err)
	assert.Empty(t, note, "a first start has no file")

	path := writePrefs(t, nil)
	note, err = RepairPreferencesFile(path)
	require.NoError(t, err)
	assert.Empty(t, note)
}

func TestRepairPreferencesFile_BrokenFileBackedUp(t *testing.T) {
	broken := append([]byte{0xEF, 0xBB, 0xBF}, `{"app_language":"zh",`...) // cut off half-way
	path := writePrefs(t, broken)

	note, err := RepairPreferencesFile(path)
	require.NoError(t, err)
	b := backups(t, path)
	require.Len(t, b, 1)
	assert.Equal(t, string(broken), readFile(t, b[0]), "the backup is the file exactly as it was")
	assert.Equal(t, string(broken), readFile(t, path), "the original is left for Fyne as it was")
	assert.Contains(t, note, "not valid JSON")
	assert.Contains(t, note, b[0], "the log says where the settings were kept")
}

func TestFynePreferencesPath(t *testing.T) {
	p := FynePreferencesPath("Paintpane")
	assert.True(t, strings.HasSuffix(p, filepath.Join("fyne", "Paintpane", "preferences.json")), p)
}

// The real Fyne app reads a repaired file: what the person had is what the app sees.
func TestRepairPreferencesFile_FyneLoadsRepairedFile(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("FynePreferencesPath follows USERPROFILE on Windows only")
	}
	home := t.TempDir()
	t.Setenv("USERPROFILE", home) // never the person's own preferences
	const id = "PaintpaneBOMTest"
	path := FynePreferencesPath(id)
	require.True(t, strings.HasPrefix(path, home), path)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, append([]byte{0xEF, 0xBB, 0xBF}, samplePrefs...), 0o600))

	_, err := RepairPreferencesFile(path)
	require.NoError(t, err)

	a := fyneapp.NewWithID(id)
	assert.Equal(t, "zh", a.Preferences().String("app_language"))
	assert.Equal(t, "星夜", a.Preferences().String("title"))
}
