package wallpaper

import (
	"image"

	"github.com/dixieflatline76/Spice/v2/pkg/provider"
)

// RenderFramePreview renders the artwork in a frame on a wall of targetW×targetH
// pixels, exactly as the wallpaper is made, for the Tune Image preview. The
// options say the frame size, paper mat and wall colour; unset ones fall back
// to the built-in defaults.
func RenderFramePreview(img image.Image, targetW, targetH int, opts provider.TuningOptions) (image.Image, error) {
	framer := NewVirtualFramer(nil, &Config{VirtualWallColor: WallAlgorithmic})
	return framer.renderGalleryWall(img, targetW, targetH, opts)
}
