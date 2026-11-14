package wallpaper

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommonsFilePage(t *testing.T) {
	const file = "https://commons.wikimedia.org/wiki/File:"
	cases := []struct {
		name, link, filename, want string
	}{
		{"original file", "https://upload.wikimedia.org/wikipedia/commons/e/ec/Mona_Lisa%2C_by_Leonardo_da_Vinci.jpg", "Wikimedia_1.jpg",
			file + "Mona_Lisa%2C_by_Leonardo_da_Vinci.jpg"},
		{"scaled copy", "https://upload.wikimedia.org/wikipedia/commons/thumb/a/a5/Tsunami_by_hokusai_19th_century.jpg/3840px-Tsunami_by_hokusai_19th_century.jpg", "Wikimedia_2.jpg",
			file + "Tsunami_by_hokusai_19th_century.jpg"},
		{"http and upper-case host", "http://Upload.Wikimedia.org/wikipedia/commons/0/0a/A.png", "x.jpg", file + "A.png"},
		{"old version, page ID from the filename", "https://upload.wikimedia.org/wikipedia/commons/archive/0/0a/20200101000000%21A.png", "Wikimedia_778899.jpg",
			"https://commons.wikimedia.org/w/index.php?curid=778899"},
		{"old version, no page ID", "https://upload.wikimedia.org/wikipedia/commons/archive/0/0a/20200101000000%21A.png", "Custom.jpg", ""},
		{"page ID only after Wikimedia_", "https://upload.wikimedia.org/wikipedia/commons/archive/x", "Wikimedia_12a.jpg", ""},
		{"already the page", file + "A.png", "Wikimedia_3.jpg", ""},
		{"page by ID", "https://commons.wikimedia.org/w/index.php?curid=3", "Wikimedia_3.jpg", ""},
		{"another wiki", "https://upload.wikimedia.org/wikipedia/en/0/0a/A.png", "Custom.jpg", ""},
		{"museum page", "https://www.metmuseum.org/art/collection/search/436535", "MetMuseum_436535.jpg", ""},
		{"local file", "file:///C:/Users/x/Favorites/Local.jpg", "Local.jpg", ""},
		{"empty", "", "Wikimedia_4.jpg", ""},
		{"not hash folders", "https://upload.wikimedia.org/wikipedia/commons/x/yz/A.png", "Custom.jpg", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := commonsFilePage(c.link, c.filename)
			assert.Equal(t, c.want != "", ok)
			assert.Equal(t, c.want, got)
		})
	}
}

// Favourites as an older version saved them: Wikimedia ones with the image file as their page.
const oldFavoritesMetadata = `{
  "description": "My favourites",
  "files": {
    "Wikimedia_101.jpg": {"attribution": "Leonardo (Public domain)", "product_url": "https://upload.wikimedia.org/wikipedia/commons/e/ec/Mona_Lisa.jpg"},
    "Wikimedia_102.jpg": {"attribution": "Hokusai", "product_url": "https://upload.wikimedia.org/wikipedia/commons/thumb/a/a5/Tsunami.jpg/3840px-Tsunami.jpg"},
    "Wikimedia_103.jpg": {"attribution": "Already fixed", "product_url": "https://commons.wikimedia.org/wiki/File:Starry_Night.jpg"},
    "Wikimedia_104.jpg": {"attribution": "Old version", "product_url": "https://upload.wikimedia.org/wikipedia/commons/archive/0/0a/20200101000000%21A.png"},
    "MetMuseum_436535.jpg": {"attribution": "Van Gogh", "product_url": "https://www.metmuseum.org/art/collection/search/436535"},
    "Custom.jpg": {"attribution": "?", "product_url": "https://upload.wikimedia.org/wikipedia/commons/archive/0/0a/20200101000000%21B.png"},
    "Local.jpg": {"attribution": "", "product_url": ""},
    "Legacy.jpg": "https://upload.wikimedia.org/wikipedia/commons/1/1b/Legacy.jpg"
  },
  "count": 8
}`

var favoriteImages = []string{
	"Wikimedia_101.jpg", "Wikimedia_102.jpg", "Wikimedia_103.jpg", "Wikimedia_104.jpg",
	"MetMuseum_436535.jpg", "Custom.jpg", "Local.jpg", "Legacy.jpg",
}

// useFavoritesDir gives the migrations a Favorites folder with oldFavoritesMetadata and an image
// for each entry, and returns it.
func useFavoritesDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), FavoritesCollection)
	require.NoError(t, os.MkdirAll(dir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "metadata.json"), []byte(oldFavoritesMetadata), 0600))
	for _, name := range favoriteImages {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("image "+name), 0600))
	}
	old := favoritesDirForMigrations
	favoritesDirForMigrations = func() string { return dir }
	t.Cleanup(func() { favoritesDirForMigrations = old })
	return dir
}

func readFavoritesMetadata(t *testing.T, dir string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
	require.NoError(t, err)
	var meta map[string]any
	require.NoError(t, json.Unmarshal(data, &meta))
	return meta
}

// favoritePages maps each favourite to its page, as the local listing reads them.
func favoritePages(t *testing.T, dir string) map[string]string {
	t.Helper()
	pages := map[string]string{}
	for name, v := range readFavoritesMetadata(t, dir)["files"].(map[string]any) {
		if m, ok := v.(map[string]any); ok {
			pages[name], _ = m["product_url"].(string)
		} else {
			pages[name], _ = v.(string)
		}
	}
	return pages
}

func assertImagesKept(t *testing.T, dir string) {
	t.Helper()
	for _, name := range favoriteImages {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if assert.NoError(t, err, name) {
			assert.Equal(t, "image "+name, string(data), name)
		}
	}
	_, err := os.Stat(filepath.Join(dir, "metadata.json.tmp"))
	assert.True(t, os.IsNotExist(err), "no temporary file is left behind")
}

func TestFixCommonsFavoriteLinksStep(t *testing.T) {
	dir := useFavoritesDir(t)
	cfg := &Config{}

	changed, err := FixCommonsFavoriteLinksStep(cfg)
	require.NoError(t, err)
	assert.True(t, changed, "the step records itself, so the config is saved")
	assert.Equal(t, []string{commonsFavoriteLinksMigration}, cfg.AppliedMigrations)

	assert.Equal(t, map[string]string{
		// old format: the image file becomes its page on Commons
		"Wikimedia_101.jpg": "https://commons.wikimedia.org/wiki/File:Mona_Lisa.jpg",
		"Wikimedia_102.jpg": "https://commons.wikimedia.org/wiki/File:Tsunami.jpg",
		"Wikimedia_104.jpg": "https://commons.wikimedia.org/w/index.php?curid=104",
		"Legacy.jpg":        "https://commons.wikimedia.org/wiki/File:Legacy.jpg",
		// already the page
		"Wikimedia_103.jpg": "https://commons.wikimedia.org/wiki/File:Starry_Night.jpg",
		// not from Wikimedia, or a link it cannot place: as it was
		"MetMuseum_436535.jpg": "https://www.metmuseum.org/art/collection/search/436535",
		"Custom.jpg":           "https://upload.wikimedia.org/wikipedia/commons/archive/0/0a/20200101000000%21B.png",
		"Local.jpg":            "",
	}, favoritePages(t, dir))

	meta := readFavoritesMetadata(t, dir)
	assert.Equal(t, "My favourites", meta["description"], "other fields are kept")
	assert.Equal(t, float64(8), meta["count"])
	entry := meta["files"].(map[string]any)["Wikimedia_101.jpg"].(map[string]any)
	assert.Equal(t, "Leonardo (Public domain)", entry["attribution"])
	assertImagesKept(t, dir)
}

// A second start does not run the migration again, even if an old link turned up since.
func TestFixCommonsFavoriteLinksStep_OnceAcrossStarts(t *testing.T) {
	dir := useFavoritesDir(t)
	p := NewMockPreferences()
	p.SetString(WallhavenConfigPrefKey, `{"queries": []}`)

	ResetConfig()
	t.Cleanup(ResetConfig)
	cfg := GetConfig(p)
	assert.Equal(t, 1, countMigration(cfg.AppliedMigrations), "recorded")
	var saved struct {
		AppliedMigrations []string `json:"applied_migrations"`
	}
	require.NoError(t, json.Unmarshal([]byte(p.String(WallhavenConfigPrefKey)), &saved))
	assert.Equal(t, 1, countMigration(saved.AppliedMigrations), "saved with the settings")
	assert.Equal(t, "https://commons.wikimedia.org/wiki/File:Mona_Lisa.jpg", favoritePages(t, dir)["Wikimedia_101.jpg"])

	// Put an old link back, as if the user restored an old metadata.json.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "metadata.json"), []byte(oldFavoritesMetadata), 0600))
	ResetConfig()
	cfg = GetConfig(p)
	assert.Equal(t, 1, countMigration(cfg.AppliedMigrations), "recorded once")
	assert.Equal(t, "https://upload.wikimedia.org/wikipedia/commons/e/ec/Mona_Lisa.jpg", favoritePages(t, dir)["Wikimedia_101.jpg"],
		"not migrated again")
	assertImagesKept(t, dir)
}

func TestFixCommonsFavoriteLinksStep_NoFavorites(t *testing.T) {
	dir := filepath.Join(t.TempDir(), FavoritesCollection)
	old := favoritesDirForMigrations
	favoritesDirForMigrations = func() string { return dir }
	t.Cleanup(func() { favoritesDirForMigrations = old })

	cfg := &Config{}
	changed, err := FixCommonsFavoriteLinksStep(cfg)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, []string{commonsFavoriteLinksMigration}, cfg.AppliedMigrations, "nothing to fix, done")
	_, err = os.Stat(dir)
	assert.True(t, os.IsNotExist(err), "no folder or file is created")
}

func TestFixCommonsFavoriteLinksStep_BrokenMetadataLeftAlone(t *testing.T) {
	dir := useFavoritesDir(t)
	broken := []byte(`{"files": {"Wikimedia_101.jpg": `)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "metadata.json"), broken, 0600))

	cfg := &Config{}
	_, err := FixCommonsFavoriteLinksStep(cfg)
	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
	require.NoError(t, err)
	assert.Equal(t, broken, data)
	assert.Equal(t, []string{commonsFavoriteLinksMigration}, cfg.AppliedMigrations)
}

// When metadata.json cannot be written the step is not recorded, so the next start tries again.
func TestFixCommonsFavoriteLinksStep_RetriedAfterWriteFailure(t *testing.T) {
	dir := useFavoritesDir(t)
	blocker := filepath.Join(dir, "metadata.json.tmp")
	require.NoError(t, os.Mkdir(blocker, 0755)) // the temporary file cannot be created

	cfg := &Config{}
	changed, err := FixCommonsFavoriteLinksStep(cfg)
	require.NoError(t, err, "a failure does not stop the other migrations")
	assert.False(t, changed)
	assert.Empty(t, cfg.AppliedMigrations)
	assert.Equal(t, "https://upload.wikimedia.org/wikipedia/commons/e/ec/Mona_Lisa.jpg", favoritePages(t, dir)["Wikimedia_101.jpg"])

	require.NoError(t, os.Remove(blocker))
	changed, err = FixCommonsFavoriteLinksStep(cfg)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, "https://commons.wikimedia.org/wiki/File:Mona_Lisa.jpg", favoritePages(t, dir)["Wikimedia_101.jpg"])
	assertImagesKept(t, dir)
}

// Tests never touch the favourites of whoever runs them.
func TestFavoritesDirForMigrations_EmptyInTests(t *testing.T) {
	assert.Equal(t, "", favoritesDirForMigrations())
	cfg := &Config{}
	changed, err := FixCommonsFavoriteLinksStep(cfg)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Empty(t, cfg.AppliedMigrations)
}

func TestMigrationChain_FixesCommonsFavorites(t *testing.T) {
	found := false
	for _, step := range NewMigrationChain().Steps {
		if funcName(step) == funcName(FixCommonsFavoriteLinksStep) {
			found = true
		}
	}
	assert.True(t, found, "FixCommonsFavoriteLinksStep is in the migration chain")
}

func funcName(f MigrationFunc) string {
	return runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
}

// countMigration counts FixCommonsFavoriteLinksStep's entries; other steps record themselves too.
func countMigration(applied []string) int {
	n := 0
	for _, name := range applied {
		if name == commonsFavoriteLinksMigration {
			n++
		}
	}
	return n
}
