package wallpaper

import (
	"io/fs"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/dixieflatline76/Spice/v2/pkg/i18n"
)

// CacheUsage is what the image cache holds: how many images and how many bytes
// its folder takes on disk (originals and the copies fitted to each screen).
type CacheUsage struct {
	Images int
	Bytes  int64
}

// CacheUsage measures the cache. It walks the download folder, so call it off
// the UI thread.
func (wp *Plugin) CacheUsage() CacheUsage {
	var u CacheUsage
	if wp.store != nil {
		u.Images = wp.store.Count()
	}
	if wp.fm != nil {
		u.Bytes = dirSize(wp.fm.GetDownloadDir())
	}
	return u
}

// dirSize adds up the size of every file under root; unreadable entries count as zero.
func dirSize(root string) int64 {
	var total int64
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil {
			total += info.Size()
		}
		return nil
	})
	return total
}

// FormatBytes writes a size the way people read it: "850 MB", "1.2 GB". Units
// are decimal, as file managers show them.
func FormatBytes(n int64) string {
	const kb = 1000
	switch {
	case n >= kb*kb*kb:
		return trimFloat(float64(n)/(kb*kb*kb)) + " GB"
	case n >= kb*kb:
		return strconv.FormatInt((n+kb*kb/2)/(kb*kb), 10) + " MB"
	case n >= kb:
		return strconv.FormatInt((n+kb/2)/kb, 10) + " KB"
	default:
		return strconv.FormatInt(n, 10) + " B"
	}
}

// trimFloat writes one decimal place: 1.2, 10.0 -> "10".
func trimFloat(v float64) string {
	s := strconv.FormatFloat(v, 'f', 1, 64)
	if len(s) > 2 && s[len(s)-2:] == ".0" {
		s = s[:len(s)-2]
	}
	return s
}

// Summary is the line under "Downloaded wallpapers" on the Advanced page.
func (u CacheUsage) Summary() string {
	if u.Images == 0 {
		return i18n.T("No images")
	}
	return i18n.Tf("{{.Count}} images, {{.Size}}", map[string]any{"Count": u.Images, "Size": FormatBytes(u.Bytes)})
}

// ClearMessage is the text of the clear-cache dialog: it says how much will be deleted.
func (u CacheUsage) ClearMessage() string {
	return i18n.Tf("This deletes {{.Count}} downloaded images ({{.Size}}). You will need internet to see new wallpapers.",
		map[string]any{"Count": u.Images, "Size": FormatBytes(u.Bytes)})
}

// cacheUsageCache remembers the last measurement so the dialog opens at once.
type cacheUsageCache struct {
	mu   sync.Mutex
	last *CacheUsage
}

func (c *cacheUsageCache) set(u CacheUsage) {
	c.mu.Lock()
	c.last = &u
	c.mu.Unlock()
}

func (c *cacheUsageCache) get() (CacheUsage, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.last == nil {
		return CacheUsage{}, false
	}
	return *c.last, true
}
