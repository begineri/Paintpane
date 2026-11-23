package wallpaper

import (
	"os"
	"path/filepath"
	"sync"
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

// recordShown collects the texts the Advanced page would show under "Downloaded wallpapers".
type recordShown struct {
	mu    sync.Mutex
	texts []string
}

func (r *recordShown) show(text string) {
	r.mu.Lock()
	r.texts = append(r.texts, text)
	r.mu.Unlock()
}

func (r *recordShown) all() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.texts...)
}

// Review R4: the page opens and starts counting the cache; the cache is cleared before the count
// ends. The old count must not come back over "No images". The order is forced with channels.
func TestCacheUsage_ClearDuringCountKeepsNoImages(t *testing.T) {
	var usage cacheUsageCache
	var shown recordShown
	counting, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	old := CacheUsage{Images: 148, Bytes: 900_000_000}

	go func() {
		defer close(done)
		usage.measure(func() CacheUsage {
			close(counting)
			<-release // Still walking the folder when Clear cache is confirmed
			return old
		}, shown.show)
	}()
	<-counting
	usage.cleared(shown.show)
	close(release)
	<-done

	assert.Equal(t, []string{CacheUsage{}.Summary()}, shown.all(), "the stale count is not shown")
	got, ok := usage.get()
	require.True(t, ok)
	assert.Equal(t, CacheUsage{}, got, "the dialog would not offer to delete images already gone")

	// A count started after the clear is shown as usual.
	fresh := CacheUsage{Images: 3, Bytes: 4_000_000}
	usage.measure(func() CacheUsage { return fresh }, shown.show)
	assert.Equal(t, []string{CacheUsage{}.Summary(), fresh.Summary()}, shown.all())
	got, _ = usage.get()
	assert.Equal(t, fresh, got)
}

func TestCacheUsage_CountWithoutClear(t *testing.T) {
	var usage cacheUsageCache
	var shown recordShown
	_, ok := usage.get()
	assert.False(t, ok, "nothing measured yet")

	u := CacheUsage{Images: 148, Bytes: 900_000_000}
	usage.measure(func() CacheUsage { return u }, shown.show)
	assert.Equal(t, []string{u.Summary()}, shown.all())
	got, ok := usage.get()
	require.True(t, ok)
	assert.Equal(t, u, got)

	usage.cleared(nil) // The help line was never built: nothing to show, the count is still reset
	got, _ = usage.get()
	assert.Equal(t, CacheUsage{}, got)
}
