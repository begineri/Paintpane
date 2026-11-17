package wallpaper

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/util/log"
)

// P22: image_cache_map.json keeps the paths of downloaded files relative to the cache root
// (wallpaper_downloads), so the cache keeps working after its folder is renamed or moved, or the
// user name changes. In memory the store keeps absolute paths, as before; only reading and writing
// the file convert them.
//
// Files written up to v1.1 hold absolute paths. When such a path is gone, the file is looked for at
// the same place under the current cache root; one found there replaces the old path, and the cache
// is saved again. A file found in neither place keeps its old path, so the missing-file handling is
// unchanged.

// cacheRootLocked returns the directory that relative paths in the cache file start from: the
// FileManager's root, where every downloaded file is written, or else the cache file's folder.
// CALLER MUST HOLD s.mu (read or write).
func (s *ImageStore) cacheRootLocked() string {
	if s.fm != nil && s.fm.GetDownloadDir() != "" {
		return s.fm.GetDownloadDir()
	}
	if s.cachePath != "" {
		return filepath.Dir(s.cachePath)
	}
	return ""
}

// isURLPath reports whether p is a link rather than a file path (Met thumbnails keep their URL
// in DerivativePaths).
func isURLPath(p string) bool {
	return strings.Contains(p, "://")
}

// storedPath returns the form of p written to the cache file: relative to root, with forward
// slashes, when p is a file under root; p unchanged otherwise.
func storedPath(p, root string) string {
	if p == "" || root == "" || isURLPath(p) || !filepath.IsAbs(p) {
		return p
	}
	rel, err := filepath.Rel(root, p)
	if err != nil || !filepath.IsLocal(rel) {
		return p
	}
	return filepath.ToSlash(rel)
}

// relocationCandidates returns where an absolute path written under an earlier cache root could
// be under the current one, as paths relative to the root, most specific first: the part after the
// last folder named like the cache root (wallpaper_downloads), then the part from the last
// "fitted" folder for a derivative, or the file name alone for a master, which sits in the root.
func relocationCandidates(old string, rootNames []string) []string {
	parts := strings.FieldsFunc(old, func(r rune) bool { return r == '/' || r == '\\' })
	if len(parts) == 0 {
		return nil
	}
	var out []string
	add := func(segs []string) {
		if len(segs) == 0 {
			return
		}
		rel := filepath.Join(segs...)
		if !filepath.IsLocal(rel) {
			return
		}
		for _, have := range out {
			if have == rel {
				return
			}
		}
		out = append(out, rel)
	}

	for i := len(parts) - 2; i >= 0; i-- {
		matched := false
		for _, name := range rootNames {
			if name != "" && strings.EqualFold(parts[i], name) {
				matched = true
				break
			}
		}
		if matched {
			add(parts[i+1:])
			break
		}
	}

	fitted := -1
	for i := len(parts) - 2; i >= 0; i-- {
		if parts[i] == FittedRootDir {
			fitted = i
			break
		}
	}
	if fitted >= 0 {
		add(parts[fitted:])
	} else {
		add(parts[len(parts)-1:])
	}
	return out
}

// pathResolver turns paths read from the cache file into absolute ones.
type pathResolver struct {
	root      string
	rootNames []string
	stat      func(string) (os.FileInfo, error)

	upgraded  bool // at least one path was absolute and under the root: save in the new format
	relocated int  // absolute paths gone from their place and found under the root
}

func (s *ImageStore) newPathResolverLocked() *pathResolver {
	root := s.cacheRootLocked()
	names := []string{filepath.Base(root)}
	if s.cachePath != "" {
		if n := filepath.Base(filepath.Dir(s.cachePath)); n != names[0] {
			names = append(names, n)
		}
	}
	return &pathResolver{root: root, rootNames: names, stat: s.getStatFunc()}
}

// resolve returns the absolute path for p as read from the cache file.
func (r *pathResolver) resolve(p string) string {
	if p == "" || r.root == "" || isURLPath(p) {
		return p
	}
	if !filepath.IsAbs(p) {
		rel := filepath.FromSlash(p)
		if !filepath.IsLocal(rel) {
			return p
		}
		return filepath.Join(r.root, rel)
	}

	// Absolute: the format written up to v1.1.
	if rel, err := filepath.Rel(r.root, p); err == nil && filepath.IsLocal(rel) {
		r.upgraded = true
		return p
	}
	if _, err := r.stat(p); err == nil {
		return p // Still at its old place (outside the current root): use it there.
	}
	for _, rel := range relocationCandidates(p, r.rootNames) {
		candidate := filepath.Join(r.root, rel)
		if _, err := r.stat(candidate); err == nil {
			r.relocated++
			return candidate
		}
	}
	return p // Missing in both places: leave it to the missing-file handling.
}

// resolveCachePathsLocked makes every file path loaded from the cache file absolute, and reports
// whether the cache should be saved again (old format, or paths moved to the current root).
// CALLER MUST HOLD s.mu.Lock()
func (s *ImageStore) resolveCachePathsLocked() bool {
	r := s.newPathResolverLocked()
	if r.root == "" {
		return false
	}
	for i := range s.images {
		img := &s.images[i]
		img.FilePath = r.resolve(img.FilePath)
		for res, p := range img.DerivativePaths {
			if np := r.resolve(p); np != p {
				img.DerivativePaths[res] = np
			}
		}
	}
	if r.relocated > 0 {
		log.Printf("Migration: Found %d cached files under the moved cache folder %s.", r.relocated, r.root)
	}
	if r.upgraded || r.relocated > 0 {
		log.Printf("Migration: Saving the image cache with paths relative to %s.", r.root)
		return true
	}
	return false
}

// snapshotForSaveLocked deep-copies the images for writing to the cache file, with file paths
// under the cache root made relative to it.
// CALLER MUST HOLD s.mu (read or write).
func (s *ImageStore) snapshotForSaveLocked() []provider.Image {
	root := s.cacheRootLocked()
	snapshot := make([]provider.Image, len(s.images))
	for i, img := range s.images {
		// Deep copy the struct to avoid shared map references during JSON encoding
		snapshot[i] = img
		snapshot[i].FilePath = storedPath(img.FilePath, root)
		if img.ProcessingFlags != nil {
			flags := make(map[string]bool, len(img.ProcessingFlags))
			for k, v := range img.ProcessingFlags {
				flags[k] = v
			}
			snapshot[i].ProcessingFlags = flags
		}
		if img.DerivativePaths != nil {
			paths := make(map[string]string, len(img.DerivativePaths))
			for k, v := range img.DerivativePaths {
				paths[k] = storedPath(v, root)
			}
			snapshot[i].DerivativePaths = paths
		}
		if img.Tuning != nil {
			tuning := make(map[string]provider.TuningOptions, len(img.Tuning))
			for k, v := range img.Tuning {
				tuning[k] = v
			}
			snapshot[i].Tuning = tuning
		}
	}
	return snapshot
}
