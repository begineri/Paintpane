package wallpaper

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/user"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/asset"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
)

// S4-B: blocking keeps the image's details and a small thumbnail, because the
// image's files are deleted; the blocked-images page lists them, and Unblock
// and Reset undo it.

func newBlockedTestPlugin(t *testing.T) (*Plugin, *Config) {
	t.Helper()
	u, _ := user.Current()
	cfg := &Config{Preferences: NewMockPreferences(), AvoidSet: map[string]bool{}, userid: u.Uid, assetMgr: asset.NewManager()}
	cfg.loadFromPrefs()
	fm := NewFileManager(filepath.Join(t.TempDir(), "wallpaper_downloads"))
	require.NoError(t, os.MkdirAll(fm.GetDownloadDir(), 0o755))
	store := NewImageStore()
	store.SetFileManager(fm, filepath.Join(fm.GetDownloadDir(), "image_cache_map.json"))
	return &Plugin{cfg: cfg, fm: fm, store: store, providers: map[string]provider.ImageProvider{}}, cfg
}

// writeJPEG writes a w×h test image.
func writeJPEG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 255 / w), G: uint8(y * 255 / h), B: 0x80, A: 0xFF})
		}
	}
	f, err := os.Create(path)
	require.NoError(t, err)
	defer f.Close()
	require.NoError(t, jpeg.Encode(f, img, &jpeg.Options{Quality: 90}))
}

func waitForFile(t *testing.T, path string) os.FileInfo {
	t.Helper()
	for i := 0; i < 100; i++ {
		if fi, err := os.Stat(path); err == nil {
			return fi
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s was not written", path)
	return nil
}

func TestBlockImageKeepsDetailsAndThumbnail(t *testing.T) {
	wp, cfg := newBlockedTestPlugin(t)
	master := filepath.Join(wp.fm.GetDownloadDir(), "MetMuseum_1.jpg")
	writeJPEG(t, master, 1600, 1000)
	fitted := filepath.Join(wp.fm.GetDownloadDir(), "fitted.jpg")
	writeJPEG(t, fitted, 800, 500)
	img := provider.Image{ID: "MetMuseum_1", Provider: "MetMuseum", Title: "Irises",
		Artist: "Vincent van Gogh (Dutch, 1853–1890)", FilePath: fitted}
	require.True(t, wp.store.Add(img))

	blockImage(cfg, wp.store, img)

	assert.True(t, cfg.InAvoidSet(img.ID))
	assert.False(t, wp.store.Exists(img.ID), "the image leaves the store")
	info := cfg.GetBlockedImages()[img.ID]
	assert.Equal(t, "Irises", info.Title)
	assert.Equal(t, "Vincent van Gogh", info.Artist, "the artist's name only, as in the tray")
	assert.Equal(t, "MetMuseum", info.Provider)

	thumb := blockedThumbPath(blockedThumbDir(wp.fm), img.ID)
	fi := waitForFile(t, thumb)
	assert.Less(t, fi.Size(), int64(80_000), "tens of KB")
	f, err := os.Open(thumb)
	require.NoError(t, err)
	conf, err := jpeg.DecodeConfig(f)
	f.Close()
	require.NoError(t, err)
	assert.Equal(t, 360, conf.Width, "made from the original, the whole image within 360 px")
	assert.Equal(t, 225, conf.Height)
	assert.NotContains(t, filepath.Dir(thumb), "wallpaper_downloads", "clearing the cache leaves thumbnails")

	// Only the thumbnail is left in the folder: the temporary copy is gone.
	time.Sleep(50 * time.Millisecond)
	entries, err := os.ReadDir(filepath.Dir(thumb))
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestBlockedImageListUnblockAndReset(t *testing.T) {
	wp, cfg := newBlockedTestPlugin(t)
	cfg.AddToAvoidSet("Wallhaven_old") // blocked before details were kept
	now := time.Now()
	cfg.BlockImage(BlockedImage{ID: "A_1", Title: "First", Artist: "Ann", Provider: "A", BlockedAt: now.Add(-time.Hour)})
	cfg.BlockImage(BlockedImage{ID: "B_2", Title: "Second", Provider: "B", BlockedAt: now})
	dir := blockedThumbDir(wp.fm)
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(blockedThumbPath(dir, "A_1"), []byte("jpeg"), 0o644))

	list := wp.BlockedImageList()
	require.Len(t, list, 3)
	assert.Equal(t, []string{"B_2", "A_1", "Wallhaven_old"}, []string{list[0].ID, list[1].ID, list[2].ID},
		"newest first, then those with an ID only")
	assert.Equal(t, "Ann", list[1].Artist)
	assert.NotEmpty(t, list[1].ThumbPath)
	assert.Empty(t, list[0].ThumbPath, "no thumbnail on disk")
	assert.Empty(t, list[2].Title)

	// The store refuses blocked images until they are unblocked.
	wp.store.LoadAvoidSet(cfg.GetAvoidSet())
	assert.False(t, wp.store.Add(provider.Image{ID: "A_1"}))
	wp.UnblockImage("A_1")
	assert.False(t, cfg.InAvoidSet("A_1"))
	assert.NotContains(t, cfg.GetBlockedImages(), "A_1")
	assert.NoFileExists(t, blockedThumbPath(dir, "A_1"), "its thumbnail is deleted")
	assert.True(t, wp.store.Add(provider.Image{ID: "A_1"}), "it may be downloaded again")
	assert.Len(t, wp.BlockedImageList(), 2)

	wp.ResetBlockedImages()
	assert.Empty(t, wp.BlockedImageList())
	assert.Empty(t, cfg.GetBlockedImages())
	assert.NoDirExists(t, dir)
}

func TestBlockedImagesPersist(t *testing.T) {
	_, cfg := newBlockedTestPlugin(t)
	at := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	cfg.BlockImage(BlockedImage{ID: "Getty_9", Title: "Irises", Artist: "Vincent van Gogh", Provider: "Getty", BlockedAt: at})

	cfg2 := &Config{Preferences: cfg.Preferences, AvoidSet: map[string]bool{}, userid: cfg.userid, assetMgr: asset.NewManager()}
	cfg2.loadFromPrefs()
	assert.True(t, cfg2.InAvoidSet("Getty_9"))
	got := cfg2.GetBlockedImages()["Getty_9"]
	assert.Equal(t, "Irises", got.Title)
	assert.True(t, at.Equal(got.BlockedAt))
}
