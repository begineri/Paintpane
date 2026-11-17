package wallpaper

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// P22: image_cache_map.json keeps paths relative to wallpaper_downloads, and files written up to
// v1.1 (absolute paths) keep working after the cache folder is renamed or moved.

const p22Res = "2560x1600"

// p22Derivative is where a derivative of id sits, relative to the cache root.
func p22Derivative(id string) string {
	return filepath.Join(FittedRootDir, FullArtDir, StandardDir, p22Res, id+".jpg")
}

// p22Touch creates the file root/rel.
func p22Touch(t *testing.T, root, rel string) string {
	t.Helper()
	p := filepath.Join(root, rel)
	require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
	require.NoError(t, os.WriteFile(p, []byte("jpg"), 0o600))
	return p
}

// p22Image is a cached image whose files sit under root, as the downloader records it.
func p22Image(id, root string) provider.Image {
	d := filepath.Join(root, p22Derivative(id))
	return provider.Image{
		ID:              id,
		Provider:        "Rijksmuseum",
		FilePath:        d,
		DerivativePaths: map[string]string{p22Res: d, "primary": d},
		ProcessingFlags: map[string]bool{"FullArtwork": true},
	}
}

// p22WriteCache writes images to the cache file as-is (absolute paths: the v1.1 format).
func p22WriteCache(t *testing.T, cache string, images []provider.Image) {
	t.Helper()
	data, err := json.MarshalIndent(images, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(cache), 0o755))
	require.NoError(t, os.WriteFile(cache, data, 0o600))
}

// p22ReadCache returns the images as stored in the cache file.
func p22ReadCache(t *testing.T, cache string) map[string]provider.Image {
	t.Helper()
	data, err := os.ReadFile(cache)
	require.NoError(t, err)
	var images []provider.Image
	require.NoError(t, json.Unmarshal(data, &images))
	out := map[string]provider.Image{}
	for _, img := range images {
		out[img.ID] = img
	}
	return out
}

// p22Store loads the cache file of the cache root root, saving synchronously.
func p22Store(t *testing.T, root string) (*ImageStore, string) {
	t.Helper()
	cache := filepath.Join(root, "image_cache_map.json")
	store := NewImageStore()
	store.SetAsyncSave(false)
	store.SetFileManager(NewFileManager(root), cache)
	require.NoError(t, store.LoadCache())
	return store, cache
}

func p22Get(t *testing.T, store *ImageStore, id string) provider.Image {
	t.Helper()
	img, ok := store.GetByID(id)
	require.True(t, ok, id)
	return img
}

// A v1.1 cache (absolute paths in the current root) loads unchanged and is saved in the new format.
func TestP22LoadsAbsolutePathsOfV11Cache(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Paintpane", "wallpaper_downloads")
	want := p22Touch(t, root, p22Derivative("Rijksmuseum_1"))
	p22WriteCache(t, filepath.Join(root, "image_cache_map.json"), []provider.Image{p22Image("Rijksmuseum_1", root)})

	store, cache := p22Store(t, root)

	img := p22Get(t, store, "Rijksmuseum_1")
	assert.Equal(t, want, img.FilePath)
	assert.Equal(t, want, img.DerivativePaths[p22Res])
	assert.Equal(t, want, img.DerivativePaths["primary"])

	saved := p22ReadCache(t, cache)["Rijksmuseum_1"]
	rel := filepath.ToSlash(p22Derivative("Rijksmuseum_1"))
	assert.Equal(t, rel, saved.FilePath, "the load saves the cache with relative paths")
	assert.Equal(t, rel, saved.DerivativePaths[p22Res])
	assert.Equal(t, rel, saved.DerivativePaths["primary"])
}

// The cache folder renamed and moved (another user name, MuseWall -> Paintpane): every path is
// found at the same place under the new root, and nothing reports a missing file.
func TestP22RelocatesPathsAfterCacheFolderMoved(t *testing.T) {
	base := t.TempDir()
	oldRoot := filepath.Join(base, "Users", "Old", "AppData", "Local", "MuseWall", "wallpaper_downloads")
	newRoot := filepath.Join(base, "Users", "New", "AppData", "Local", "Paintpane", "wallpaper_downloads")

	master := provider.Image{
		ID: "Rijksmuseum_2", Provider: "Rijksmuseum",
		FilePath:        filepath.Join(oldRoot, "Rijksmuseum_2.jpg"),
		DerivativePaths: map[string]string{"primary": filepath.Join(oldRoot, "Rijksmuseum_2.jpg")},
	}
	p22WriteCache(t, filepath.Join(newRoot, "image_cache_map.json"),
		[]provider.Image{p22Image("Rijksmuseum_1", oldRoot), master})
	derivative := p22Touch(t, newRoot, p22Derivative("Rijksmuseum_1"))
	masterNew := p22Touch(t, newRoot, "Rijksmuseum_2.jpg")

	store, cache := p22Store(t, newRoot)

	img := p22Get(t, store, "Rijksmuseum_1")
	assert.Equal(t, derivative, img.FilePath)
	assert.Equal(t, derivative, img.DerivativePaths[p22Res])
	assert.Equal(t, derivative, img.DerivativePaths["primary"])
	img2 := p22Get(t, store, "Rijksmuseum_2")
	assert.Equal(t, masterNew, img2.FilePath)
	assert.Equal(t, masterNew, img2.DerivativePaths["primary"])

	// The monitor controller's missing-file check passes for the path it would set.
	for _, p := range []string{img.DerivativePaths[p22Res], img2.FilePath} {
		_, err := os.Stat(p)
		assert.NoError(t, err, p)
	}

	// MarkSeen finds the image by its new path.
	store.MarkSeen(derivative)
	assert.True(t, p22Get(t, store, "Rijksmuseum_1").Seen)

	saved := p22ReadCache(t, cache)
	assert.Equal(t, filepath.ToSlash(p22Derivative("Rijksmuseum_1")), saved["Rijksmuseum_1"].DerivativePaths[p22Res])
	assert.Equal(t, "Rijksmuseum_2.jpg", saved["Rijksmuseum_2"].FilePath)
}

// One file holding relative paths, absolute ones in the current root, absolute ones from a moved
// root, a file outside the cache, and a thumbnail URL.
func TestP22LoadsMixedPathFormats(t *testing.T) {
	base := t.TempDir()
	oldRoot := filepath.Join(base, "old", "wallpaper_downloads")
	root := filepath.Join(base, "new", "wallpaper_downloads")
	elsewhere := p22Touch(t, base, filepath.Join("Pictures", "own.jpg"))

	relative := p22Image("Rijksmuseum_rel", root)
	relative.FilePath = filepath.ToSlash(p22Derivative("Rijksmuseum_rel"))
	relative.DerivativePaths = map[string]string{p22Res: relative.FilePath, "primary": relative.FilePath}
	outside := provider.Image{ID: "LocalFolder_own", Provider: "LocalFolder", FilePath: elsewhere,
		DerivativePaths: map[string]string{"primary": elsewhere}}
	thumbURL := "https://images.metmuseum.org/CRDImages/ep/web-large/DT1567.jpg"
	met := p22Image("MetMuseum_1", root)
	met.Provider = "MetMuseum"
	met.DerivativePaths["thumbnail"] = thumbURL

	p22WriteCache(t, filepath.Join(root, "image_cache_map.json"), []provider.Image{
		relative, p22Image("Rijksmuseum_abs", root), p22Image("Rijksmuseum_old", oldRoot), outside, met,
	})
	for _, id := range []string{"Rijksmuseum_rel", "Rijksmuseum_abs", "Rijksmuseum_old", "MetMuseum_1"} {
		p22Touch(t, root, p22Derivative(id))
	}

	store, cache := p22Store(t, root)
	require.Equal(t, 5, store.Count())

	for _, id := range []string{"Rijksmuseum_rel", "Rijksmuseum_abs", "Rijksmuseum_old", "MetMuseum_1"} {
		img := p22Get(t, store, id)
		want := filepath.Join(root, p22Derivative(id))
		assert.Equal(t, want, img.FilePath, id)
		assert.Equal(t, want, img.DerivativePaths[p22Res], id)
		assert.Equal(t, want, img.DerivativePaths["primary"], id)
	}
	assert.Equal(t, elsewhere, p22Get(t, store, "LocalFolder_own").FilePath)
	assert.Equal(t, thumbURL, p22Get(t, store, "MetMuseum_1").DerivativePaths["thumbnail"])

	saved := p22ReadCache(t, cache)
	for _, id := range []string{"Rijksmuseum_rel", "Rijksmuseum_abs", "Rijksmuseum_old", "MetMuseum_1"} {
		assert.Equal(t, filepath.ToSlash(p22Derivative(id)), saved[id].FilePath, id)
	}
	assert.Equal(t, elsewhere, saved["LocalFolder_own"].FilePath, "files outside the cache keep absolute paths")
	assert.Equal(t, thumbURL, saved["MetMuseum_1"].DerivativePaths["thumbnail"], "URLs are not paths")
}

// What a save writes: forward-slash paths relative to the cache root, written atomically; the
// file loads again from a copy of the cache in another folder.
func TestP22SavedFormat(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "a", "wallpaper_downloads")
	store, cache := p22Store(t, root)
	p22Touch(t, root, p22Derivative("Rijksmuseum_1"))
	require.True(t, store.Add(p22Image("Rijksmuseum_1", root)))
	store.SaveCache()

	data, err := os.ReadFile(cache)
	require.NoError(t, err)
	assert.NotContains(t, string(data), filepath.ToSlash(base), "no absolute path in the file")
	assert.NotContains(t, string(data), `\\`, "forward slashes only")
	saved := p22ReadCache(t, cache)["Rijksmuseum_1"]
	assert.Equal(t, "fitted/full/standard/2560x1600/Rijksmuseum_1.jpg", saved.FilePath)
	assert.Equal(t, saved.FilePath, saved.DerivativePaths["primary"])
	assert.Equal(t, saved.FilePath, saved.DerivativePaths[p22Res])
	_, err = os.Stat(cache + ".tmp")
	assert.True(t, os.IsNotExist(err), "the temporary file is renamed over the cache")

	// The in-memory paths stay absolute after a save.
	assert.Equal(t, filepath.Join(root, p22Derivative("Rijksmuseum_1")), p22Get(t, store, "Rijksmuseum_1").FilePath)

	// The same file read from another root.
	moved := filepath.Join(base, "b", "wallpaper_downloads")
	p22Touch(t, moved, p22Derivative("Rijksmuseum_1"))
	require.NoError(t, os.WriteFile(filepath.Join(moved, "image_cache_map.json"), data, 0o600))
	store2, _ := p22Store(t, moved)
	assert.Equal(t, filepath.Join(moved, p22Derivative("Rijksmuseum_1")), p22Get(t, store2, "Rijksmuseum_1").FilePath)
}

// Files missing from both the old place and the new root keep their old paths, so the
// missing-file handling stays as it was; other entries are untouched. A missing derivative is not
// pointed at the master that has the same name.
func TestP22KeepsPathsOfMissingFiles(t *testing.T) {
	base := t.TempDir()
	oldRoot := filepath.Join(base, "old", "wallpaper_downloads")
	root := filepath.Join(base, "new", "wallpaper_downloads")

	gone := p22Image("Rijksmuseum_gone", oldRoot)
	noDerivative := p22Image("Rijksmuseum_master", oldRoot)
	p22WriteCache(t, filepath.Join(root, "image_cache_map.json"),
		[]provider.Image{gone, noDerivative, p22Image("Rijksmuseum_ok", oldRoot)})
	p22Touch(t, root, "Rijksmuseum_master.jpg") // the master only
	ok := p22Touch(t, root, p22Derivative("Rijksmuseum_ok"))

	store, cache := p22Store(t, root)
	require.Equal(t, 3, store.Count())

	assert.Equal(t, gone.FilePath, p22Get(t, store, "Rijksmuseum_gone").FilePath)
	assert.Equal(t, gone.DerivativePaths, p22Get(t, store, "Rijksmuseum_gone").DerivativePaths)
	assert.Equal(t, noDerivative.DerivativePaths, p22Get(t, store, "Rijksmuseum_master").DerivativePaths)
	assert.Equal(t, ok, p22Get(t, store, "Rijksmuseum_ok").FilePath)

	saved := p22ReadCache(t, cache)
	assert.Equal(t, gone.FilePath, saved["Rijksmuseum_gone"].FilePath, "a missing file keeps its old path")
	assert.Equal(t, filepath.ToSlash(p22Derivative("Rijksmuseum_ok")), saved["Rijksmuseum_ok"].FilePath)

	// The existing stale handling: clearing the derivatives of the missing image.
	require.True(t, store.ClearDerivatives("Rijksmuseum_gone"))
	assert.Empty(t, p22Get(t, store, "Rijksmuseum_gone").DerivativePaths)
	assert.Equal(t, ok, p22Get(t, store, "Rijksmuseum_ok").FilePath)
}

// A cache with nothing missing and already relative is not rewritten at load.
func TestP22RelativeCacheIsNotRewrittenAtLoad(t *testing.T) {
	root := filepath.Join(t.TempDir(), "wallpaper_downloads")
	p22Touch(t, root, p22Derivative("Rijksmuseum_1"))
	img := p22Image("Rijksmuseum_1", root)
	rel := filepath.ToSlash(p22Derivative("Rijksmuseum_1"))
	img.FilePath = rel
	img.DerivativePaths = map[string]string{p22Res: rel, "primary": rel}
	cache := filepath.Join(root, "image_cache_map.json")
	p22WriteCache(t, cache, []provider.Image{img})

	store := NewImageStore()
	store.SetAsyncSave(false)
	saves := 0
	store.saveFunc = func() { saves++ }
	store.SetFileManager(NewFileManager(root), cache)
	require.NoError(t, store.LoadCache())
	assert.Equal(t, 0, saves)
	assert.Equal(t, filepath.Join(root, p22Derivative("Rijksmuseum_1")), p22Get(t, store, "Rijksmuseum_1").FilePath)
}

// Old paths are taken apart whichever separator they were written with.
func TestP22RelocationCandidates(t *testing.T) {
	names := []string{"wallpaper_downloads"}
	win := `C:\Users\alice\AppData\Local\paintpane\wallpaper_downloads\fitted\full\standard\2560x1600\Rijksmuseum_1.jpg`
	assert.Equal(t, []string{p22Derivative("Rijksmuseum_1")}, relocationCandidates(win, names))

	assert.Equal(t, []string{"Rijksmuseum_1.jpg"},
		relocationCandidates(`C:\Users\alice\AppData\Local\MuseWall\wallpaper_downloads\Rijksmuseum_1.jpg`, names))

	// A root linked to a folder with another name: the fitted folder still anchors derivatives.
	assert.Equal(t, []string{p22Derivative("Rijksmuseum_1")},
		relocationCandidates("/mnt/d/cache/fitted/full/standard/2560x1600/Rijksmuseum_1.jpg", names))

	assert.Empty(t, relocationCandidates("", names))
}
