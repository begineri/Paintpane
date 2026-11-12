package wallpaper

import (
	"fmt"
	"math/rand"
	"sync"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
	"github.com/dixieflatline76/Spice/v2/util/log"
)

// lockScreenSetter is implemented by OS backends that can set the lock screen image.
type lockScreenSetter interface {
	SetLockScreen(imagePath string) error
}

// lockScreenSyncer applies lock screen updates off the monitor actor. Setting the
// image is slow, so a request made during an update replaces any older waiting one.
type lockScreenSyncer struct {
	setter lockScreenSetter

	mu      sync.Mutex
	pending string
	running bool
}

func (s *lockScreenSyncer) request(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending = path
	if !s.running {
		s.running = true
		go s.run()
	}
}

func (s *lockScreenSyncer) run() {
	for {
		s.mu.Lock()
		path := s.pending
		s.pending = ""
		if path == "" {
			s.running = false
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()

		if err := s.setter.SetLockScreen(path); err != nil {
			log.Printf("[ERROR] Failed to set lock screen: %v", err)
		}
	}
}

// updateLockScreen gives the lock screen a different image from the desktop. It only
// falls back to the desktop image when no other image may be shown.
func (mc *MonitorController) updateLockScreen(desktop provider.Image) {
	if mc.lockScreen == nil {
		return
	}
	img, path, ok := mc.pickLockScreenImage(desktop.ID, "")
	if !ok {
		if mc.isHidden(desktop) {
			log.Printf("[Monitor %d] No image can be shown on the lock screen. Keeping the current one.", mc.ID)
			return
		}
		img, path = desktop, mc.resolveImagePath(desktop)
	}
	mc.setLockScreenImage(img, path)
}

func (mc *MonitorController) setLockScreenImage(img provider.Image, path string) {
	if path == "" {
		return
	}
	mc.lockImageID = img.ID
	mc.lockScreen.request(path)
}

// imagesOnShow returns the IDs of the images every display and the lock screen show now. Pruning
// keeps them: their files are what Tune Image, Favourite and Block work on.
func (wp *Plugin) imagesOnShow() map[string]bool {
	wp.monMu.RLock()
	monitors := make([]*MonitorController, 0, len(wp.Monitors))
	for _, mc := range wp.Monitors {
		monitors = append(monitors, mc)
	}
	wp.monMu.RUnlock()

	ids := make(map[string]bool, 2*len(monitors))
	for _, mc := range monitors {
		mc.mu.RLock()
		for _, id := range []string{mc.State.CurrentImage.ID, mc.State.CurrentID, mc.lockImageID} {
			if id != "" {
				ids[id] = true
			}
		}
		mc.mu.RUnlock()
	}
	return ids
}

// SupportsLockScreen reports whether this monitor sets the lock screen: the primary monitor, on a
// platform that can.
func (mc *MonitorController) SupportsLockScreen() bool {
	return mc.lockScreen != nil
}

// nextLockScreen gives the lock screen another image that may be shown, not the desktop's and not
// the one it shows now. The desktop is left alone. With no such image it does nothing.
func (mc *MonitorController) nextLockScreen() {
	if mc.lockScreen == nil {
		return
	}
	img, path, ok := mc.pickLockScreenImage(mc.State.CurrentImage.ID, mc.lockImageID)
	if !ok {
		log.Printf("[Monitor %d] No other image can be shown on the lock screen. Keeping the current one.", mc.ID)
		return
	}
	log.Printf("[Monitor %d] Changing only the lock screen to %s", mc.ID, img.ID)
	mc.setLockScreenImage(img, path)
}

// enforceLockScreenFilter replaces the lock screen image if it must now be hidden. An
// unknown image is replaced too, because it may have been set before a restart.
func (mc *MonitorController) enforceLockScreenFilter() {
	if mc.lockScreen == nil {
		return
	}
	if mc.lockImageID != "" {
		current, ok := mc.Store.GetByID(mc.lockImageID)
		if ok && !mc.isHidden(current) {
			return
		}
	}
	mc.updateLockScreen(mc.State.CurrentImage)
}

// pickLockScreenImage returns a random image that may be shown, other than excludeID and alsoExclude
// (either may be empty).
func (mc *MonitorController) pickLockScreenImage(excludeID, alsoExclude string) (provider.Image, string, bool) {
	resKey := fmt.Sprintf("%dx%d", mc.Monitor.Rect.Dx(), mc.Monitor.Rect.Dy())
	ids := mc.Store.GetIDsForResolution(resKey)
	for _, i := range rand.Perm(len(ids)) { // #nosec G404 -- image choice is not security sensitive
		id := ids[i]
		if id == excludeID || (alsoExclude != "" && id == alsoExclude) || (mc.cfg != nil && mc.cfg.InAvoidSet(id)) {
			continue
		}
		img, ok := mc.Store.GetByID(id)
		if !ok || mc.isHidden(img) {
			continue
		}
		path := mc.resolveImagePath(img)
		if path == "" {
			continue
		}
		if _, err := mc.os.Stat(path); err != nil {
			continue
		}
		return img, path, true
	}
	return provider.Image{}, "", false
}
