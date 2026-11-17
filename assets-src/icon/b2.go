package main

import (
	"fmt"
	"image"
	"image/color"
	"math"
)

// The Paintpane icon, P19 design "B2 pure blocks": a rounded tile holding the
// five colour blocks of Van Gogh's The Bedroom (wall, window, bed with red
// cover and pillow, floor). No frame, outline, shadow or gradient.
//
// Each tier below is copied rectangle for rectangle from
// docs/mockups/p19-icons/final/paintpane-<size>.svg. The 16, 20 and 24 px
// tiers are pixel drawings; never derive one from another or from the 256.

type block struct {
	x, y, w, h int
	c          color.NRGBA
}

type tier struct {
	size   int             // canvas edge in pixels
	tile   image.Rectangle // the rounded tile; blocks are clipped to it
	radius int             // tile corner radius
	blocks []block         // back to front
}

var (
	b2x16 = tier{16, image.Rect(1, 1, 15, 15), 3, []block{
		{0, 0, 16, 9, cWall},
		{0, 9, 16, 7, cFloor},
		{3, 3, 3, 5, cWinFrame},
		{4, 4, 1, 3, cPane},
		{8, 7, 8, 6, cBed},
		{8, 10, 8, 2, cRed},
		{12, 8, 2, 1, cPillow},
	}}
	b2x20 = tier{20, image.Rect(1, 1, 19, 19), 4, []block{
		{0, 0, 20, 11, cWall},
		{0, 11, 20, 9, cFloor},
		{4, 4, 5, 6, cWinFrame},
		{5, 5, 1, 4, cPane},
		{7, 5, 1, 4, cPane},
		{10, 9, 10, 7, cBed},
		{10, 12, 10, 2, cRed},
		{14, 10, 3, 1, cPillow},
	}}
	b2x24 = tier{24, image.Rect(2, 2, 22, 22), 4, []block{
		{0, 0, 24, 14, cWall},
		{0, 14, 24, 10, cFloor},
		{5, 5, 5, 7, cWinFrame},
		{6, 6, 1, 5, cPane},
		{8, 6, 1, 5, cPane},
		{12, 11, 12, 8, cBed},
		{12, 15, 12, 2, cRed},
		{17, 12, 4, 2, cPillow},
	}}
	b2x256 = tier{256, image.Rect(16, 16, 240, 240), 40, []block{
		{0, 0, 256, 148, cWall},
		{0, 148, 256, 108, cFloor},
		{48, 44, 52, 84, cWinFrame},
		{66, 60, 16, 52, cPane},
		{132, 112, 124, 96, cBed},
		{132, 160, 124, 32, cRed},
		{190, 128, 36, 16, cPillow},
	}}
)

// scaled returns t with every coordinate multiplied by num/den. It panics
// unless all of them stay whole pixels, so block edges never land between
// pixels.
func (t tier) scaled(num, den int) tier {
	s := func(v int) int {
		if v*num%den != 0 {
			panic(fmt.Sprintf("tier %d x %d/%d: %d is not a whole pixel", t.size, num, den, v))
		}
		return v * num / den
	}
	out := tier{
		size:   s(t.size),
		tile:   image.Rect(s(t.tile.Min.X), s(t.tile.Min.Y), s(t.tile.Max.X), s(t.tile.Max.Y)),
		radius: s(t.radius),
	}
	for _, b := range t.blocks {
		out.blocks = append(out.blocks, block{s(b.x), s(b.y), s(b.w), s(b.h), b.c})
	}
	return out
}

// inTile reports whether the point (x, y) lies inside the rounded tile.
func (t tier) inTile(x, y float64) bool {
	r := t.tile
	if x < float64(r.Min.X) || y < float64(r.Min.Y) || x > float64(r.Max.X) || y > float64(r.Max.Y) {
		return false
	}
	rad := float64(t.radius)
	cx := math.Max(float64(r.Min.X)+rad, math.Min(x, float64(r.Max.X)-rad))
	cy := math.Max(float64(r.Min.Y)+rad, math.Min(y, float64(r.Max.Y)-rad))
	dx, dy := x-cx, y-cy
	return dx*dx+dy*dy <= rad*rad
}

// render draws t at its own size. Every pixel takes the colour of the topmost
// block covering it; its alpha is how much of it the rounded tile covers
// (16 x 16 samples). Block and tile edges fall on whole pixels, so only the
// corners come out partly transparent, matching how the browser draws
// final/*.svg.
func (t tier) render() *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, t.size, t.size))
	const ss = 16
	for y := 0; y < t.size; y++ {
		for x := 0; x < t.size; x++ {
			n := 0
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					if t.inTile(float64(x)+(float64(sx)+0.5)/ss, float64(y)+(float64(sy)+0.5)/ss) {
						n++
					}
				}
			}
			if n == 0 {
				continue
			}
			var c color.NRGBA
			for _, b := range t.blocks {
				if x >= b.x && x < b.x+b.w && y >= b.y && y < b.y+b.h {
					c = b.c
				}
			}
			c.A = uint8(math.Round(255 * float64(n) / (ss * ss)))
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// icon returns the app icon at one of the .ico sizes. 16, 20 and 24 are the
// pixel drawings; 32-96 enlarge the matching drawing by a whole factor with
// anti-aliased corners; 256 is the large drawing.
func icon(size int) *image.NRGBA {
	switch size {
	case 16:
		return b2x16.render()
	case 20:
		return b2x20.render()
	case 24:
		return b2x24.render()
	case 32:
		return b2x16.scaled(2, 1).render()
	case 40:
		return b2x20.scaled(2, 1).render()
	case 48:
		return b2x24.scaled(2, 1).render()
	case 64:
		return b2x16.scaled(4, 1).render()
	case 96:
		return b2x24.scaled(4, 1).render()
	case 256:
		return b2x256.render()
	}
	panic(fmt.Sprintf("no icon tier for %d px", size))
}

// trayIcon is the 16 px drawing with every pixel enlarged to 4 x 4 and no
// further smoothing, so Windows' whole-number reductions (64 -> 32 -> 16 at 100 %,
// 64 -> 32 at 200 %) give back the pixel drawing exactly.
func trayIcon() *image.NRGBA {
	return nearest(b2x16.render(), 4)
}

func nearest(src *image.NRGBA, k int) *image.NRGBA {
	b := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx()*k, b.Dy()*k))
	for y := 0; y < dst.Bounds().Dy(); y++ {
		for x := 0; x < dst.Bounds().Dx(); x++ {
			dst.SetNRGBA(x, y, src.NRGBAAt(x/k, y/k))
		}
	}
	return dst
}

// over composites src onto dst with its top-left at (x0, y0).
func over(dst, src *image.NRGBA, x0, y0 int) {
	b := src.Bounds()
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			s := src.NRGBAAt(x, y)
			if s.A == 0 {
				continue
			}
			d := dst.NRGBAAt(x0+x, y0+y)
			a := float64(s.A) / 255
			mix := func(sv, dv uint8) uint8 { return uint8(math.Round(float64(sv)*a + float64(dv)*(1-a))) }
			dst.SetNRGBA(x0+x, y0+y, color.NRGBA{mix(s.R, d.R), mix(s.G, d.G), mix(s.B, d.B), 255})
		}
	}
}
