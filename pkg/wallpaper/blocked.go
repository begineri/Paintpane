package wallpaper

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/disintegration/imaging"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/pkg/ui/schema"
	"github.com/dixieflatline76/Spice/v2/util/log"
)

// BlockedImage is what the blocked-images page (S4-B) shows for an image the
// user blocked. Blocking deletes the image's files, so these details and a
// small thumbnail are kept when it is blocked.
type BlockedImage struct {
	ID        string    `json:"id"`
	Title     string    `json:"title,omitempty"`
	Artist    string    `json:"artist,omitempty"` // the name only, as in the tray (T-R3)
	Provider  string    `json:"provider,omitempty"`
	BlockedAt time.Time `json:"blocked_at"`
}

// Blocked-image thumbnails: small enough for tens of KB, sharp at 112 px wide
// on a 2x screen.
const (
	blockedThumbMax     = 360
	blockedThumbQuality = 85
	blockedThumbDirName = "blocked_thumbs"
)

// blockedThumbDir is where thumbnails of blocked images are kept: next to the
// download folder, so clearing the cache or removing orphans leaves them.
func blockedThumbDir(fm *FileManager) string {
	if fm == nil {
		return ""
	}
	return filepath.Join(filepath.Dir(fm.GetDownloadDir()), blockedThumbDirName)
}

// blockedThumbPath names an image's thumbnail by a hash of its ID, which may
// be a file path (local folders).
func blockedThumbPath(dir, id string) string {
	if dir == "" {
		return ""
	}
	sum := sha1.Sum([]byte(id))
	return filepath.Join(dir, hex.EncodeToString(sum[:])+".jpg")
}

// blockImage blocks img: it is removed from the store (which deletes its
// files) and added to the avoid set with its details, and a thumbnail is saved
// from its file first.
func blockImage(cfg *Config, store StoreInterface, img provider.Image) {
	var fm *FileManager
	if s, ok := store.(*ImageStore); ok {
		fm = s.fm
	}
	// Take the file before the store deletes it; the thumbnail is made in the
	// background so the next wallpaper is not held up.
	if dir := blockedThumbDir(fm); dir != "" {
		if src := blockedThumbSource(fm, img); src != "" {
			if tmp, err := holdFile(src, dir); err != nil {
				log.Printf("Blocked image %s: cannot keep a copy for the thumbnail: %v", img.ID, err)
			} else {
				go func() {
					defer os.Remove(tmp)
					if err := saveBlockedThumb(tmp, blockedThumbPath(dir, img.ID)); err != nil {
						log.Printf("Blocked image %s: cannot save the thumbnail: %v", img.ID, err)
					}
				}()
			}
		}
	}

	if store != nil {
		store.Remove(img.ID)
	}
	if cfg != nil {
		title, artist := titleAndArtist(img)
		cfg.BlockImage(BlockedImage{ID: img.ID, Title: title, Artist: artist, Provider: img.Provider, BlockedAt: time.Now()})
	}
}

// blockedThumbSource is the best file to make a thumbnail from: the original
// download (the artwork alone), else the wallpaper file.
func blockedThumbSource(fm *FileManager, img provider.Image) string {
	if fm != nil && img.FilePath != "" {
		if master, err := fm.GetMasterPath(img.ID, filepath.Ext(img.FilePath)); err == nil {
			if _, err := os.Stat(master); err == nil {
				return master
			}
		}
	}
	if img.FilePath != "" {
		if _, err := os.Stat(img.FilePath); err == nil {
			return img.FilePath
		}
	}
	return ""
}

// holdFile links (or, failing that, copies) src into dir under a temporary
// name, so it survives src being deleted.
func holdFile(src, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	f, err := os.CreateTemp(dir, "hold-*"+filepath.Ext(src))
	if err != nil {
		return "", err
	}
	tmp := f.Name()
	f.Close()
	os.Remove(tmp)
	if err := os.Link(src, tmp); err == nil {
		return tmp, nil
	}
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	out, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return "", err
	}
	return tmp, out.Close()
}

// saveBlockedThumb writes a small JPEG of src to dst, the whole image within
// blockedThumbMax pixels; the page crops it to its tiles.
func saveBlockedThumb(src, dst string) error {
	release, err := acquireMasterDecode(context.Background(), src)
	if err != nil {
		return err
	}
	defer release()
	img, err := imaging.Open(src, imaging.AutoOrientation(true))
	if err != nil {
		return err
	}
	thumb := imaging.Fit(img, blockedThumbMax, blockedThumbMax, imaging.Lanczos)
	tmp := dst + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if err := jpeg.Encode(f, thumb, &jpeg.Options{Quality: blockedThumbQuality}); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

// BlockedImageList lists the blocked images for the blocked-images page:
// those with details newest first, then those blocked before details were
// kept, by ID.
func (wp *Plugin) BlockedImageList() []schema.BlockedImageSchema {
	ids := wp.cfg.GetAvoidSet()
	infos := wp.cfg.GetBlockedImages()
	dir := blockedThumbDir(wp.fm)

	type entry struct {
		info BlockedImage
		ok   bool
	}
	entries := make([]entry, 0, len(ids))
	for id := range ids {
		info, ok := infos[id]
		if !ok {
			info = BlockedImage{ID: id}
		}
		entries = append(entries, entry{info: info, ok: ok})
	}
	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.ok != b.ok {
			return a.ok
		}
		if a.ok && !a.info.BlockedAt.Equal(b.info.BlockedAt) {
			return a.info.BlockedAt.After(b.info.BlockedAt)
		}
		return a.info.ID < b.info.ID
	})

	out := make([]schema.BlockedImageSchema, 0, len(entries))
	for _, e := range entries {
		item := schema.BlockedImageSchema{ID: e.info.ID}
		if e.ok {
			item.Title, item.Artist = e.info.Title, e.info.Artist
			item.Source = wp.sourceName(e.info.Provider)
			if path := blockedThumbPath(dir, e.info.ID); path != "" {
				if _, err := os.Stat(path); err == nil {
					item.ThumbPath = path
				}
			}
		}
		out = append(out, item)
	}
	return out
}

// sourceName is a source's full name in the UI language.
func (wp *Plugin) sourceName(providerID string) string {
	if p, ok := wp.providers[providerID]; ok {
		return p.Name()
	}
	return providerID
}

// UnblockImage takes one image off the blocked list and deletes its thumbnail.
// It may be downloaded and shown again.
func (wp *Plugin) UnblockImage(id string) {
	wp.cfg.RemoveFromAvoidSet(id)
	if wp.store != nil {
		wp.store.LoadAvoidSet(wp.cfg.GetAvoidSet())
	}
	if path := blockedThumbPath(blockedThumbDir(wp.fm), id); path != "" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			log.Printf("Unblock %s: cannot delete the thumbnail: %v", id, err)
		}
	}
}

// ResetBlockedImages unblocks every image and deletes the thumbnails.
func (wp *Plugin) ResetBlockedImages() {
	wp.cfg.ResetAvoidSet()
	if wp.store != nil {
		wp.store.LoadAvoidSet(nil)
	}
	if dir := blockedThumbDir(wp.fm); dir != "" {
		if err := os.RemoveAll(dir); err != nil {
			log.Printf("Reset blocked images: cannot delete the thumbnails: %v", err)
		}
	}
}
