package wallpaper

import (
	"strings"

	"github.com/dixieflatline76/Spice/v2/pkg/curation"
	"github.com/dixieflatline76/Spice/v2/pkg/provider"
)

const (
	wallhavenProviderID = "Wallhaven"
	puritySFW           = "sfw"
)

// isAutoDetectedNude reports whether an image is identified as nude from its source
// data alone. A Wallhaven image with no rating yet is treated as nude, so an image
// cached before ratings were stored stays hidden until a later fetch rates it.
func isAutoDetectedNude(img provider.Image) bool {
	if curation.GetManager().IsNudeID(img.ID) {
		return true
	}
	return img.Provider == wallhavenProviderID && !strings.EqualFold(img.Purity, puritySFW)
}

// GetShowNudes returns whether nude images may be shown. Defaults to hidden.
func (c *Config) GetShowNudes() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.BoolWithFallback(ShowNudesPrefKey, false)
}

// SetShowNudes sets whether nude images may be shown.
func (c *Config) SetShowNudes(show bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.SetBool(ShowNudesPrefKey, show)
}

// IsNudeMarked returns whether an image was manually marked as nude.
func (c *Config) IsNudeMarked(id string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.NudeMarks[id]
}

// SetNudeMarked adds or removes an image from the manual nude marks.
func (c *Config) SetNudeMarked(id string, marked bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if marked {
		if c.NudeMarks == nil {
			c.NudeMarks = make(map[string]bool)
		}
		c.NudeMarks[id] = true
	} else {
		delete(c.NudeMarks, id)
	}
	c.save()
}

// IsImageHidden returns whether an image must not be shown under the current setting.
func (c *Config) IsImageHidden(img provider.Image) bool {
	if c.GetShowNudes() {
		return false
	}
	return isAutoDetectedNude(img) || c.IsNudeMarked(img.ID)
}
