package wallpaper

import (
	"encoding/json"
	"fmt"

	"github.com/dixieflatline76/Spice/v2/util/log"
)

// A start keeps the wallpaper from last time (owner's choice, 2026-11-08), so the app must also know
// what that wallpaper is: the tray's first row, Tune Image, favourite and block all act on the
// current image. Each display's last shown image ID is kept in the preferences and read back at
// start, without setting the wallpaper again (A2 §14.9 #2).

// lastShownPrefKey holds a JSON object from a display key (monitorKey) to the ID of the image last
// set as that display's wallpaper.
const lastShownPrefKey = pluginPrefix + "last_shown_images"

// lastShownLocked returns the saved map. The caller holds c.mu.
func (c *Config) lastShownLocked() map[string]string {
	m := map[string]string{}
	if raw := c.String(lastShownPrefKey); raw != "" {
		if err := json.Unmarshal([]byte(raw), &m); err != nil {
			log.Printf("Ignoring unreadable %s: %v", lastShownPrefKey, err)
			return map[string]string{}
		}
	}
	return m
}

// LastShownImage returns the ID of the image last set as the wallpaper of the display with this
// key, or "".
func (c *Config) LastShownImage(displayKey string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastShownLocked()[displayKey]
}

// SetLastShownImage records the image just set as the wallpaper of the display with this key.
func (c *Config) SetLastShownImage(displayKey, imageID string) {
	if displayKey == "" || imageID == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	m := c.lastShownLocked()
	if m[displayKey] == imageID {
		return
	}
	m[displayKey] = imageID
	data, err := json.Marshal(m)
	if err != nil {
		return
	}
	c.SetString(lastShownPrefKey, string(data))
}

// monitorKey identifies the display across starts: the OS device path, which the pause state
// uses too, or the display number when the OS gives none.
func (mc *MonitorController) monitorKey() string {
	if mc.Monitor.DevicePath != "" {
		return mc.Monitor.DevicePath
	}
	return fmt.Sprintf("display-%d", mc.ID)
}

// rememberShown records the image with this ID as this display's wallpaper for the next start. The caller holds mc.mu.
func (mc *MonitorController) rememberShown(id string) {
	if mc.cfg != nil {
		mc.cfg.SetLastShownImage(mc.monitorKey(), id)
	}
}

// restoreLastShown makes the image last shown on this display the current image again, without
// setting the wallpaper: the desktop still shows it. It does nothing when the display already has a
// current image, or when that image is no longer in the library, blocked or now hidden; the display
// then has no current image until the next change, as with an empty library.
func (mc *MonitorController) restoreLastShown() bool {
	if mc.cfg == nil {
		return false
	}
	mc.mu.Lock()
	defer mc.mu.Unlock()
	if mc.State.CurrentID != "" || mc.State.CurrentImage.ID != "" {
		return false
	}
	id := mc.cfg.LastShownImage(mc.monitorKey())
	if id == "" {
		return false
	}
	img, ok := mc.Store.GetByID(id)
	switch {
	case !ok:
		log.Printf("[Monitor %d] The wallpaper from last time (%s) is no longer in the library.", mc.ID, id)
		return false
	case mc.cfg.InAvoidSet(id) || mc.isHidden(img):
		log.Printf("[Monitor %d] The wallpaper from last time (%s) is blocked or hidden now; not restoring it.", mc.ID, id)
		return false
	}
	mc.State.CurrentID = id
	mc.State.CurrentImage = img
	mc.State.CurrentImage.FilePath = mc.resolveImagePath(img)
	mc.State.History = append(mc.State.History[:0], id)
	log.Printf("[Monitor %d] Restored the wallpaper from last time: %s", mc.ID, id)
	return true
}

// restoreLastShownImages restores every display's wallpaper from last time (see restoreLastShown).
func (wp *Plugin) restoreLastShownImages() {
	wp.monMu.RLock()
	monitors := make([]*MonitorController, 0, len(wp.Monitors))
	for _, mc := range wp.Monitors {
		monitors = append(monitors, mc)
	}
	wp.monMu.RUnlock()
	for _, mc := range monitors {
		mc.restoreLastShown()
	}
}
