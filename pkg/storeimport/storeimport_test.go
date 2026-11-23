package storeimport

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	return m
}

func TestFound(t *testing.T) {
	dir := t.TempDir()
	assert.False(t, Found(filepath.Join(dir, "missing.json")))
	write(t, filepath.Join(dir, "empty.json"), "{}")
	assert.False(t, Found(filepath.Join(dir, "empty.json")), "an empty settings file has nothing to import")
	write(t, filepath.Join(dir, "p.json"), `{"theme":"dark"}`)
	assert.True(t, Found(filepath.Join(dir, "p.json")))
	assert.False(t, Found(dir), "a folder is not a settings file")
}

func TestPrepareAndApply(t *testing.T) {
	dir := t.TempDir()
	web := filepath.Join(dir, "web", "preferences.json")
	pending := filepath.Join(dir, "store-data", PendingName)
	prefs := filepath.Join(dir, "store-prefs", "preferences.json")
	// A byte order mark, as some editors leave it (config.RepairPreferencesFile).
	write(t, web, string([]byte{0xEF, 0xBB, 0xBF})+`{"theme":"dark","eula_acceptance":"web","autostart_first_run_done":true,"wallpaper_queries":"[1,2]","freq":3}`)
	write(t, prefs, `{"eula_acceptance":"store","theme":"light"}`)

	keep := map[string]any{"eula_acceptance": "store", AskedKey: true}
	require.NoError(t, Prepare(web, pending, NotImported, keep))
	assert.FileExists(t, pending)
	assert.Equal(t, `{"eula_acceptance":"store","theme":"light"}`, mustRead(t, prefs), "the settings change only at the next start")

	applied, err := ApplyPending(pending, prefs)
	require.NoError(t, err)
	assert.True(t, applied)
	assert.NoFileExists(t, pending, "applied once")
	got := readJSON(t, prefs)
	assert.Equal(t, map[string]any{
		"theme":             "dark",
		"eula_acceptance":   "store",
		"wallpaper_queries": "[1,2]",
		"freq":              float64(3),
		AskedKey:            true,
	}, got)
	assert.Contains(t, mustRead(t, web), `"eula_acceptance":"web"`, "the portable exe's file is only read")

	applied, err = ApplyPending(pending, prefs)
	require.NoError(t, err)
	assert.False(t, applied, "nothing pending")
}

func TestPrepareRejectsABrokenFile(t *testing.T) {
	dir := t.TempDir()
	web := filepath.Join(dir, "preferences.json")
	write(t, web, `{"theme":`)
	pending := filepath.Join(dir, PendingName)
	assert.Error(t, Prepare(web, pending, NotImported, nil))
	assert.NoFileExists(t, pending)
}

func TestCopyData(t *testing.T) {
	dir := t.TempDir()
	web := filepath.Join(dir, "Paintpane")
	store := filepath.Join(dir, "PaintpaneStore")
	write(t, filepath.Join(web, "favorite_images", "metadata.json"), `{"files":{"a.jpg":{}}}`)
	write(t, filepath.Join(web, "favorite_images", "a.jpg"), "a")
	write(t, filepath.Join(web, "blocked_thumbs", "ab", "x.jpg"), "x")
	write(t, filepath.Join(web, "wallpaper_downloads", "big.jpg"), "downloaded")
	write(t, filepath.Join(web, "Paintpane.log"), "log")
	// Already in the Store edition: kept as it is.
	write(t, filepath.Join(store, "favorite_images", "a.jpg"), "store's own")

	n, err := CopyData(web, store)
	require.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, "store's own", mustRead(t, filepath.Join(store, "favorite_images", "a.jpg")))
	assert.FileExists(t, filepath.Join(store, "favorite_images", "metadata.json"))
	assert.Equal(t, "x", mustRead(t, filepath.Join(store, "blocked_thumbs", "ab", "x.jpg")))
	assert.NoDirExists(t, filepath.Join(store, "wallpaper_downloads"), "downloaded wallpapers are downloaded again")
	assert.NoFileExists(t, filepath.Join(store, "Paintpane.log"))
	assert.Equal(t, "a", mustRead(t, filepath.Join(web, "favorite_images", "a.jpg")), "the portable exe keeps its files")

	n, err = CopyData(filepath.Join(dir, "none"), store)
	require.NoError(t, err)
	assert.Zero(t, n, "nothing to copy")
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(b)
}
