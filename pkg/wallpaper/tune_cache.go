package wallpaper

import (
	"os"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
)

// tuneRenderCacheSize is how many renders of one picture Tune Image keeps (a few MB in all).
const tuneRenderCacheSize = 6

// tuneRenderKey is everything a render of the current picture depends on besides the picture and
// the screen: the tuning with its inherited values resolved, and the settings that change the
// automatic crop.
type tuneRenderKey struct {
	opts      provider.TuningOptions
	mode      ArtworkDisplayMode
	faceCrop  bool
	faceBoost bool
}

type tuneRender struct {
	key    tuneRenderKey
	data   []byte // the wallpaper file as written
	framed bool   // VirtualFramer drew a frame
}

// tuneRenderCache keeps the wallpaper files made for one picture while Tune Image is open, so going
// back to one of them (Reset after a drag, taking a frame off again) rewrites the file instead of
// decoding the original and running the automatic crop on it again. That analysis looks for faces
// in the full original and takes 6 to 12 seconds on a 40-80 megapixel scan; decoding takes 1 to 4.
type tuneRenderCache struct {
	imageID   string
	resKey    string
	shownPath string // the file this session last put on the desktop
	renders   []tuneRender
}

func (c *tuneRenderCache) get(key tuneRenderKey) (tuneRender, bool) {
	for _, r := range c.renders {
		if r.key == key {
			return r, true
		}
	}
	return tuneRender{}, false
}

func (c *tuneRenderCache) put(r tuneRender) {
	for i := range c.renders {
		if c.renders[i].key == r.key {
			c.renders[i] = r
			return
		}
	}
	if len(c.renders) >= tuneRenderCacheSize {
		c.renders = c.renders[1:]
	}
	c.renders = append(c.renders, r)
}

// tuneRenderKey returns the cache key for rendering img with opts at resKey.
func (mc *MonitorController) tuneRenderKey(opts provider.TuningOptions, img provider.Image, resKey string) tuneRenderKey {
	key := tuneRenderKey{opts: effectiveTuning(opts, img, resKey, mc.cfg)}
	if key.opts.Display != provider.DisplayCrop {
		// Where the crop would sit does not change a picture shown whole or framed.
		key.opts.ResetCrop()
	}
	if mc.cfg != nil {
		key.mode = mc.cfg.GetArtworkDisplayMode()
		key.faceCrop = mc.cfg.GetFaceCropEnabled()
		key.faceBoost = mc.cfg.GetFaceBoostEnabled()
	}
	return key
}

// tuneCacheFor returns the render cache for img while Tune Image is open, starting a new one when the
// picture or the screen size changed; nil when Tune Image is closed.
func (mc *MonitorController) tuneCacheFor(img provider.Image, resKey string) *tuneRenderCache {
	if !mc.State.TuningInProgress {
		mc.tuneCache = nil
		return nil
	}
	if mc.tuneCache == nil || mc.tuneCache.imageID != img.ID || mc.tuneCache.resKey != resKey {
		mc.tuneCache = &tuneRenderCache{imageID: img.ID, resKey: resKey}
	}
	return mc.tuneCache
}

// rememberShownRender keeps the wallpaper file on screen before it is replaced, under the tuning it
// was made with, unless that render is already kept.
func (c *tuneRenderCache) rememberShownRender(key tuneRenderKey, path string, framed bool) {
	if c == nil || path == "" {
		return
	}
	if _, ok := c.get(key); ok {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return
	}
	c.put(tuneRender{key: key, data: data, framed: framed})
}
