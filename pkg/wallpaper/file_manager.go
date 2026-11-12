package wallpaper

import (
	"context"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dixieflatline76/Spice/v2/util/log"

	"github.com/dixieflatline76/Spice/v2/config"
)

// FileManager handles all file system operations for wallpaper images.
// It enforces the directory structure for Source + Derivative architecture.
type FileManager struct {
	derivativeDeleteMu   sync.Mutex    // Protects lastDerivativeDelete
	lastDerivativeDelete chan struct{} // Closed when the most recent delete batch finishes
	rootDir              string
	wg                   sync.WaitGroup
	closing              atomic.Bool
}

// NewFileManager creates a new FileManager with the given root directory.
// The rootDir is typically ".../wallpaper_downloads".
func NewFileManager(rootDir string) *FileManager {
	return &FileManager{
		rootDir: config.ResolveLinkedDir(rootDir),
	}
}

// GetDownloadDir returns the root directory where images are downloaded.
func (fm *FileManager) GetDownloadDir() string {
	return fm.rootDir
}

// EnsureDirs creates necessary subdirectories for derivatives.
func (fm *FileManager) EnsureDirs() error {
	// Root dirs
	if err := os.MkdirAll(fm.rootDir, 0755); err != nil {
		return fmt.Errorf("failed to create root directory %s: %w", fm.rootDir, err)
	}

	fittedRoot := filepath.Join(fm.rootDir, FittedRootDir)

	// Create structure: fitted/{quality,flexibility}/{standard,faceboost,facecrop}
	modes := []string{QualityDir, FlexibilityDir}
	types := []string{StandardDir, FaceBoostDir, FaceCropDir}

	for _, mode := range modes {
		for _, t := range types {
			dir := filepath.Join(fittedRoot, mode, t)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", dir, err)
			}
		}
	}

	// Whole-image display modes have no face-processing variants.
	for _, mode := range []string{FullArtDir, FramedArtDir} {
		dir := filepath.Join(fittedRoot, mode, StandardDir)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// On macOS, sweep orphaned .spice_tmp hardlinks left behind by previous runs
	if runtime.GOOS == "darwin" {
		if tmps, err := filepath.Glob(filepath.Join(fittedRoot, "*", "*", "*.spice_tmp")); err == nil {
			for _, f := range tmps {
				_ = os.Remove(f)
			}
		}
	}

	return nil
}

// validateID ensures the ID does not contain path traversal characters.
func (fm *FileManager) validateID(id string) error {
	if strings.Contains(id, "..") || strings.Contains(id, string(filepath.Separator)) {
		return fmt.Errorf("invalid id: contains illegal characters")
	}
	return nil
}

// GetMasterPath returns the absolute path for the Master (Raw) image.
// Master images are stored directly in the root directory.
func (fm *FileManager) GetMasterPath(id string, ext string) (string, error) {
	if err := fm.validateID(id); err != nil {
		return "", err
	}
	if strings.Contains(ext, "..") || strings.Contains(ext, string(filepath.Separator)) {
		return "", fmt.Errorf("invalid extension")
	}
	return filepath.Join(fm.rootDir, id+ext), nil
}

// GetDerivativePath returns the path for a processed image based on the type.
// derivativeType is now a relative path segment (e.g., "fitted/quality/standard")
func (fm *FileManager) GetDerivativePath(id string, ext string, derivativeType string) (string, error) {
	if err := fm.validateID(id); err != nil {
		return "", err
	}
	// Basic sanitation for derivative path segments
	if strings.Contains(derivativeType, "..") {
		return "", fmt.Errorf("invalid derivative type")
	}
	return filepath.Join(fm.rootDir, derivativeType, id+ext), nil
}

// DerivativeExists checks if a specific derivative exists on disk.
// derivativeDir should be the resolution folder name (e.g. "1920x1080")
func (fm *FileManager) DerivativeExists(id string, ext string, derivativeDir string) bool {
	path, err := fm.GetDerivativePath(id, ext, derivativeDir)
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// DeepDeleteBatch removes all physical files (Master and Derivatives) associated with a list of IDs.
// It is significantly more efficient than calling DeepDelete in a loop for multiple IDs.
func (fm *FileManager) DeepDeleteBatch(ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	idMap := make(map[string]bool)
	for _, id := range ids {
		idMap[id] = true
	}

	filesToDelete := []string{}

	// 1. Scan Master (Root)
	entries, err := os.ReadDir(fm.rootDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			ext := filepath.Ext(name)
			fileID := strings.TrimSuffix(name, ext)
			if idMap[fileID] {
				filesToDelete = append(filesToDelete, filepath.Join(fm.rootDir, name))
				log.Debugf("DeepDeleteBatch: Found Master file %s", name)
			}
		}
	} else {
		log.Printf("DeepDeleteBatch: Failed to read root dir: %v", err)
	}

	// 2. Scan Derivatives (Recursive in FittedRoot)
	fittedRoot := filepath.Join(fm.rootDir, FittedRootDir)
	log.Debugf("DeepDeleteBatch: Scanning fitted root %s for %d IDs", fittedRoot, len(ids))

	err = filepath.Walk(fittedRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("DeepDeleteBatch: Error accessing path %s: %v", path, err)
			return nil // Skip access errors
		}
		if !info.IsDir() {
			name := info.Name()
			trimmed := strings.TrimSuffix(name, ".spice_tmp")
			ext := filepath.Ext(trimmed)
			fileID := strings.TrimSuffix(trimmed, ext)
			if idMap[fileID] {
				filesToDelete = append(filesToDelete, path)
				log.Debugf("DeepDeleteBatch: Found Derivative file %s", path)
			}
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		log.Printf("DeepDeleteBatch: Error walking fitted dir: %v", err)
	}

	log.Debugf("DeepDeleteBatch: Total files to delete: %d", len(filesToDelete))

	for _, f := range filesToDelete {
		if fm.closing.Load() {
			log.Debug("DeepDeleteBatch: Aborted remaining deletions due to shutdown.")
			return nil
		}
		if err := os.Remove(f); err != nil {
			if !os.IsNotExist(err) {
				log.Printf("DeepDeleteBatch: Failed to delete %s: %v", f, err)
			}
		} else {
			log.Debugf("DeepDeleteBatch: Successfully deleted %s", f)
		}
	}

	return nil
}

// DeepDelete removes the Master image and ALL its derivatives.
// It searches for files with the given ID in all known directories.
func (fm *FileManager) DeepDelete(id string) error {
	log.Printf("[DeepDelete] Requested for ID: %s", id)
	return fm.DeepDeleteBatch([]string{id})
}

// CleanupOrphans removes files from the root directory and subdirectories
// that are NOT present in the knownIDs map.
func (fm *FileManager) CleanupOrphans(knownIDs map[string]bool) {
	log.Print("FileManager: Starting orphan cleanup...")
	deletedCount := 0

	// 1. Clean Root (Masters)
	entries, err := os.ReadDir(fm.rootDir)
	if err == nil {
		for _, entry := range entries {
			if fm.closing.Load() {
				log.Debug("CleanupOrphans: Aborted orphan cleanup due to shutdown.")
				return
			}
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			ext := filepath.Ext(name)
			lowerExt := strings.ToLower(ext)

			if lowerExt != ".jpg" && lowerExt != ".jpeg" && lowerExt != ".png" {
				continue
			}

			id := strings.TrimSuffix(name, ext)

			if !knownIDs[id] {
				fullPath := filepath.Join(fm.rootDir, name)
				time.Sleep(50 * time.Millisecond) // Pacer
				if err := os.Remove(fullPath); err == nil {
					deletedCount++
				}
			}
		}
	}

	// 2. Clean Derivatives (Recursive in FittedRoot)
	fittedRoot := filepath.Join(fm.rootDir, FittedRootDir)
	err = filepath.Walk(fittedRoot, func(path string, info os.FileInfo, err error) error {
		if fm.closing.Load() {
			return filepath.SkipAll
		}
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			name := info.Name()
			trimmed := strings.TrimSuffix(name, ".spice_tmp")
			ext := filepath.Ext(trimmed)
			lowerExt := strings.ToLower(ext)

			if lowerExt != ".jpg" && lowerExt != ".jpeg" && lowerExt != ".png" {
				return nil
			}

			id := strings.TrimSuffix(trimmed, ext)

			if !knownIDs[id] {
				// Orphan
				time.Sleep(50 * time.Millisecond) // Pacer
				if err := os.Remove(path); err == nil {
					deletedCount++
				}
			}
		}
		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		log.Printf("FileManager: Error walking fitted dir during cleanup: %v", err)
	}

	log.Printf("FileManager: Orphan cleanup finished. Removed %d files.", deletedCount)
}

// DeleteDerivatives removes ONLY the processed versions of an image, keeping the Master.
// This is used when invalidating cache due to settings changes (Smart Fit, etc.).
func (fm *FileManager) DeleteDerivatives(id string) error {
	filesToDelete := []string{}

	// Recursive walk in fitted directory
	fittedRoot := filepath.Join(fm.rootDir, FittedRootDir)
	err := filepath.Walk(fittedRoot, func(path string, info os.FileInfo, err error) error {
		if fm.closing.Load() {
			return filepath.SkipAll
		}
		if err != nil {
			return nil // Skip access errors
		}
		if !info.IsDir() {
			name := info.Name()
			trimmed := strings.TrimSuffix(name, ".spice_tmp")
			ext := filepath.Ext(trimmed)
			fileID := strings.TrimSuffix(trimmed, ext)
			if fileID == id {
				filesToDelete = append(filesToDelete, path)
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("DeleteDerivatives: Error walking fitted dir: %v", err)
	}

	for _, f := range filesToDelete {
		if fm.closing.Load() {
			return nil
		}
		if err := os.Remove(f); err != nil {
			// Suppress benign errors
			if strings.Contains(err.Error(), "used by another process") || strings.Contains(err.Error(), "access is denied") {
				log.Debugf("DeleteDerivatives: Skipped locked file %s: %v", f, err)
			} else {
				log.Printf("DeleteDerivatives: Failed to delete %s: %v", f, err)
			}
		}
	}

	return nil
}

// AsyncDeepDelete launches DeepDelete in a tracked background goroutine.
func (fm *FileManager) AsyncDeepDelete(id string) {
	if fm.closing.Load() {
		return
	}
	fm.wg.Add(1)
	go func() {
		defer fm.wg.Done()
		_ = fm.DeepDelete(id)
	}()
}

// AsyncDeepDeleteBatch launches DeepDeleteBatch in a tracked background goroutine.
func (fm *FileManager) AsyncDeepDeleteBatch(ids []string) {
	if fm.closing.Load() || len(ids) == 0 {
		return
	}
	fm.wg.Add(1)
	go func() {
		defer fm.wg.Done()
		_ = fm.DeepDeleteBatch(ids)
	}()
}

// AsyncCleanupOrphans launches CleanupOrphans in a tracked background goroutine.
func (fm *FileManager) AsyncCleanupOrphans(knownIDs map[string]bool) {
	if fm.closing.Load() {
		return
	}
	fm.wg.Add(1)
	go func() {
		defer fm.wg.Done()
		fm.CleanupOrphans(knownIDs)
	}()
}

// AsyncDeleteDerivatives launches DeleteDerivatives in a tracked background goroutine.
func (fm *FileManager) AsyncDeleteDerivatives(id string) {
	if fm.closing.Load() {
		return
	}
	fm.wg.Add(1)
	go func() {
		defer fm.wg.Done()
		_ = fm.DeleteDerivatives(id)
	}()
}

// DeleteOtherDerivativesAtResolution removes an image's derivatives for one resolution in every
// other mode directory, keeping keepPath. Tuning applies across modes, so those files are stale.
func (fm *FileManager) DeleteOtherDerivativesAtResolution(id, resolution, keepPath string) {
	keep, err := os.Stat(keepPath)
	if err != nil {
		return // Without the file to keep, it cannot be told apart from the others.
	}
	err = filepath.Walk(filepath.Join(fm.rootDir, FittedRootDir), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Base(filepath.Dir(path)) != resolution {
			return nil
		}
		name := info.Name()
		if strings.TrimSuffix(name, filepath.Ext(name)) != id || os.SameFile(keep, info) {
			return nil
		}
		if err := os.Remove(path); err != nil {
			log.Printf("DeleteOtherDerivativesAtResolution: failed to remove %s: %v", path, err)
		}
		return nil
	})
	if err != nil {
		log.Printf("DeleteOtherDerivativesAtResolution: error walking fitted dir: %v", err)
	}
}

// AsyncDeleteDerivativesBatch launches DeleteDerivatives for a slice of IDs.
func (fm *FileManager) AsyncDeleteDerivativesBatch(ids []string) {
	if fm.closing.Load() || len(ids) == 0 {
		return
	}

	// Batches run one at a time, so waiting for the latest one covers all of them.
	done := make(chan struct{})
	fm.derivativeDeleteMu.Lock()
	previous := fm.lastDerivativeDelete
	fm.lastDerivativeDelete = done
	fm.derivativeDeleteMu.Unlock()

	fm.wg.Add(1)
	go func() {
		defer fm.wg.Done()
		defer close(done)
		if previous != nil {
			<-previous
		}
		for _, id := range ids {
			if fm.closing.Load() {
				return
			}
			_ = fm.DeleteDerivatives(id)
		}
	}()
}

// WaitForDerivativeDeletes blocks until every batch started so far has finished. It returns false
// if ctx ends first. Unlike WaitTimeout, it is safe while new work is being started.
func (fm *FileManager) WaitForDerivativeDeletes(ctx context.Context) bool {
	fm.derivativeDeleteMu.Lock()
	done := fm.lastDerivativeDelete
	fm.derivativeDeleteMu.Unlock()
	if done == nil {
		return true
	}
	select {
	case <-done:
		return true
	case <-ctx.Done():
		return false
	}
}

// WaitTimeout waits for all in-flight asynchronous operations to finish up to timeout.
// Returns true if all finished cleanly, false if timed out.
func (fm *FileManager) WaitTimeout(timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		fm.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return true
	case <-time.After(timeout):
		log.Printf("FileManager: in-flight operations timed out after %v during shutdown", timeout)
		return false
	}
}

// Close marks the FileManager as closing and drains in-flight operations with a bounded timeout.
func (fm *FileManager) Close() {
	fm.closing.Store(true)
	fm.WaitTimeout(500 * time.Millisecond)
}

// RenameAllAssets renames the Master image and ALL its derivatives to a new ID.
// This is used for namespacing migrations.
func (fm *FileManager) RenameAllAssets(oldID, newID string) error {
	log.Printf("[FileManager] Renaming assets: %s -> %s", oldID, newID)

	// 1. Rename Master (Root)
	entries, err := os.ReadDir(fm.rootDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			ext := filepath.Ext(name)
			fileID := strings.TrimSuffix(name, ext)
			if fileID == oldID {
				oldPath := filepath.Join(fm.rootDir, name)
				newPath := filepath.Join(fm.rootDir, newID+ext)
				log.Debugf("FileManager: Renaming Master %s -> %s", oldPath, newPath)
				if err := os.Rename(oldPath, newPath); err != nil {
					log.Printf("FileManager: Failed to rename Master %s: %v", oldPath, err)
				}
			}
		}
	}

	// 2. Rename Derivatives (Recursive in FittedRoot)
	fittedRoot := filepath.Join(fm.rootDir, FittedRootDir)
	err = filepath.Walk(fittedRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			name := info.Name()
			ext := filepath.Ext(name)
			fileID := strings.TrimSuffix(name, ext)
			if fileID == oldID {
				newPath := filepath.Join(filepath.Dir(path), newID+ext)
				log.Debugf("FileManager: Renaming Derivative %s -> %s", path, newPath)
				if err := os.Rename(path, newPath); err != nil {
					log.Printf("FileManager: Failed to rename Derivative %s: %v", path, err)
				}
			}
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		log.Printf("FileManager: Error walking fitted dir during rename: %v", err)
	}

	return nil
}

// GetDimensions returns the width and height of an image file on disk.
func (fm *FileManager) GetDimensions(path string) (int, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	img, _, err := image.DecodeConfig(file)
	if err != nil {
		return 0, 0, err
	}
	return img.Width, img.Height, nil
}
