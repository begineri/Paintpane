package main

import (
	"image"
	"image/color"
)

// drawFrame paints a gilded pixel frame filling outer and returns the
// rectangle left for the painting. All widths are in blocks of block pixels.
//
//	outline: dark edge that keeps the icon readable on light taskbars
//	gold:    gilded moulding, lit from the top left
//	inner:   dark lip between moulding and canvas (0 to omit)
//	studs:   light pixels at the corners and edge centres (large tiers)
func drawFrame(dst *image.NRGBA, outer image.Rectangle, block, gold, inner int, studs bool) image.Rectangle {
	fill(dst, outer, cOutline)
	rc := outer.Inset(block)
	for i := 0; i < gold; i++ {
		tl, br := cGold, cGold
		switch {
		case gold >= 2 && i == 0:
			tl, br = cGoldLight, cGoldShade
		case gold >= 3 && i == gold-1:
			tl, br = cGoldShade, cGoldLight
		}
		bevel(dst, rc, block, tl, br)
		rc = rc.Inset(block)
	}
	if studs && gold >= 3 {
		mid := outer.Inset(block * (1 + gold/2))
		b := block
		cx := (mid.Min.X+mid.Max.X)/2 - b/2
		cx = mid.Min.X + (cx-mid.Min.X)/b*b
		cy := mid.Min.Y + ((mid.Min.Y+mid.Max.Y)/2-b/2-mid.Min.Y)/b*b
		for _, q := range []image.Point{
			{mid.Min.X, mid.Min.Y}, {mid.Max.X - b, mid.Min.Y},
			{mid.Min.X, mid.Max.Y - b}, {mid.Max.X - b, mid.Max.Y - b},
			{cx, mid.Min.Y}, {cx, mid.Max.Y - b}, {mid.Min.X, cy}, {mid.Max.X - b, cy},
		} {
			fill(dst, image.Rect(q.X, q.Y, q.X+b, q.Y+b), cGoldLight)
		}
	}
	for i := 0; i < inner; i++ {
		fill(dst, rc, cGoldInner)
		rc = rc.Inset(block)
	}
	return rc
}

// bevel fills a one-block ring just inside rc: tl colour on the top and left
// edges, br colour on the bottom and right edges.
func bevel(dst *image.NRGBA, rc image.Rectangle, b int, tl, br color.NRGBA) {
	ctl, cbr := tl, br
	fill(dst, image.Rect(rc.Min.X, rc.Min.Y, rc.Max.X, rc.Min.Y+b), ctl)
	fill(dst, image.Rect(rc.Min.X, rc.Min.Y, rc.Min.X+b, rc.Max.Y), ctl)
	fill(dst, image.Rect(rc.Min.X, rc.Max.Y-b, rc.Max.X, rc.Max.Y), cbr)
	fill(dst, image.Rect(rc.Max.X-b, rc.Min.Y+b, rc.Max.X, rc.Max.Y), cbr)
}
