//go:build windows

package wallpaper

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dixieflatline76/Spice/v2/config"
)

// The Store build copies every wallpaper into Pictures\Paintpane, because Explorer cannot read
// the package's private AppData. Without cleanup that folder grows by one image per change and
// survives uninstall. These functions keep only the copies the desktop (one per monitor) and the
// lock screen are using now; everything else in the folder that we could have made is deleted.
// Outside a package all of this is a no-op.

const (
	msixLockScreenRole = "lockscreen"
	msixStagingState   = "msix_staged.json"
)

var (
	// msixStagingMu guards copies into the picture folder, the in-use list and the pending set.
	msixStagingMu sync.Mutex
	// msixInUse maps a role ("desktop-0", "lockscreen") to the file name it shows. Loaded lazily.
	msixInUse map[string]string
	// msixPending counts copies that are made but not yet set, so another monitor's cleanup
	// does not delete a file just before Windows reads it.
	msixPending = map[string]int{}
	// msixStateFile is where msixInUse is kept between runs (in the package's private AppData).
	msixStateFile = func() string { return filepath.Join(config.GetWorkingDir(), msixStagingState) }
)

// msixStagedExts are the files cleanup may delete: the images we copy and our unfinished copies.
// Anything else the user puts in the folder stays.
var msixStagedExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".bmp": true, ".webp": true, ".tmp": true}

func msixDesktopRole(monitorID int) string { return fmt.Sprintf("desktop-%d", monitorID) }

// withMSIXPicture stages imagePath where Windows can read it, calls set with that path and, if set
// succeeds, records it as the picture role shows and deletes copies nothing shows any more.
func withMSIXPicture(imagePath, role string, set func(path string) error) error {
	if msixPackageFamilyName == "" || msixPictureStaging == "" {
		return set(imagePath)
	}

	msixStagingMu.Lock()
	staged := resolveMSIXStagedPath(imagePath, msixPictureStaging, true)
	name, ours := msixStagedName(staged)
	if ours {
		msixPending[name]++
	}
	msixStagingMu.Unlock()

	err := set(staged)

	msixStagingMu.Lock()
	defer msixStagingMu.Unlock()
	if ours {
		if msixPending[name]--; msixPending[name] <= 0 {
			delete(msixPending, name)
		}
	}
	if err == nil && ours {
		msixLoadInUse()
		if msixInUse[role] != name {
			msixInUse[role] = name
			msixSaveInUse()
		}
		msixPruneStaging()
	}
	return err
}

// msixStagedName returns the file name of path if it lies directly in the picture folder.
func msixStagedName(path string) (string, bool) {
	if msixPictureStaging == "" || !strings.EqualFold(filepath.Clean(filepath.Dir(path)), filepath.Clean(msixPictureStaging)) {
		return "", false
	}
	return filepath.Base(path), true
}

func msixLoadInUse() {
	if msixInUse != nil {
		return
	}
	msixInUse = map[string]string{}
	data, err := os.ReadFile(msixStateFile())
	if err != nil {
		return
	}
	if err := json.Unmarshal(data, &msixInUse); err != nil {
		log.Printf("MSIX: ignoring unreadable %s: %v", msixStagingState, err)
		msixInUse = map[string]string{}
	}
}

func msixSaveInUse() {
	path := msixStateFile()
	data, err := json.Marshal(msixInUse)
	if err == nil {
		err = os.MkdirAll(filepath.Dir(path), 0o755)
	}
	if err == nil {
		err = os.WriteFile(path, data, 0o600)
	}
	if err != nil {
		log.Printf("MSIX: could not save %s: %v", msixStagingState, err)
	}
}

// msixPruneStaging deletes the files in the picture folder that no role shows and no set in
// progress is about to show. Only files with msixStagedExts directly in the folder are deleted.
// Callers hold msixStagingMu.
func msixPruneStaging() {
	keep := map[string]bool{}
	for _, name := range msixInUse {
		keep[strings.ToLower(name)] = true
	}
	for name := range msixPending {
		keep[strings.ToLower(name)] = true
	}

	entries, err := os.ReadDir(msixPictureStaging)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.Type().IsRegular() || keep[strings.ToLower(e.Name())] {
			continue
		}
		if !msixStagedExts[strings.ToLower(filepath.Ext(e.Name()))] {
			continue
		}
		if err := os.Remove(filepath.Join(msixPictureStaging, e.Name())); err != nil {
			log.Printf("MSIX: could not remove old copy %s: %v", e.Name(), err)
		}
	}
}
