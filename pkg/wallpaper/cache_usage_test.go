package wallpaper

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
)

func TestFormatBytes(t *testing.T) {
	cases := map[int64]string{
		0:              "0 B",
		999:            "999 B",
		1500:           "2 KB",
		850_000_000:    "850 MB",
		1_200_000_000:  "1.2 GB",
		10_000_000_000: "10 GB",
	}
	for n, want := range cases {
		assert.Equal(t, want, FormatBytes(n), "%d bytes", n)
	}
}

func TestCacheUsageSentences(t *testing.T) {
	t.Cleanup(func() { i18n.SetLanguage("en") })
	u := CacheUsage{Images: 312, Bytes: 1_200_000_000}

	i18n.SetLanguage("en")
	assert.Equal(t, "312 images, 1.2 GB", u.Summary())
	assert.Equal(t, "No images", CacheUsage{}.Summary())
	assert.Equal(t, "This deletes 312 downloaded images (1.2 GB). You will need internet to see new wallpapers.", u.ClearMessage())

	i18n.SetLanguage("zh")
	assert.Equal(t, "312 张，共 1.2 GB", u.Summary())
	assert.Equal(t, "0 张", CacheUsage{}.Summary())
	assert.Equal(t, "将删除 312 张已下载的图片（1.2 GB）。之后需要联网才能看到新壁纸。", u.ClearMessage())
}

func TestDirSizeCountsEveryFileUnderTheFolder(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "fitted", "quality"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.jpg"), make([]byte, 100), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "fitted", "quality", "b.jpg"), make([]byte, 250), 0o644))
	assert.Equal(t, int64(350), dirSize(dir))
	assert.Equal(t, int64(0), dirSize(filepath.Join(dir, "missing")))
}
