package main

import (
	"image"
	"image/color"
)

// The painting for the 48 px and larger tiers is described as flat shapes in
// normalised painting coordinates (0..1 across, 0..1 down) and rasterised
// straight onto a coarse pixel grid at each target size. Every shape carries
// the lowest detail level that draws it, so each tier is its own drawing
// rather than a scaled copy of a bigger one.
//
// Positions were laid out by eye from the AIC reference (artwork 28560).

type pt struct{ x, y float64 }

type shape struct {
	pts    []pt // polygon, or 2 points (x0,y0)-(x1,y1) for a rectangle
	col    color.NRGBA
	detail int // drawn when the tier's detail >= this
}

const (
	detMid  = 1 // 48 / 64 / 96 px: bed outline, window, two chairs
	detHigh = 2 // 118 px and up, splash screens
)

func r(x0, y0, x1, y1 float64, c color.NRGBA, d int) shape {
	return shape{pts: []pt{{x0, y0}, {x1, y1}}, col: c, detail: d}
}

func p(c color.NRGBA, d int, xy ...float64) shape {
	s := shape{col: c, detail: d}
	for i := 0; i+1 < len(xy); i += 2 {
		s.pts = append(s.pts, pt{xy[i], xy[i+1]})
	}
	return s
}

// bedroom returns the shapes, back to front.
func bedroom() []shape {
	var s []shape
	add := func(x ...shape) { s = append(s, x...) }

	// Walls and doors.
	add(r(0, 0, 1, 1, cWall, detMid))
	add(r(0, 0, 0.085, 0.62, cWallShade, detMid)) // left door / side wall
	add(r(0.075, 0, 0.085, 0.60, cDoorDark, detHigh))
	add(r(0.92, 0, 1, 0.80, cDoor, detMid)) // right door
	add(r(0.92, 0, 0.93, 0.80, cDoorDark, detHigh))
	add(r(0.945, 0.08, 0.99, 0.30, cWallLight, detHigh))
	add(r(0.945, 0.40, 0.99, 0.66, cWallLight, detHigh))

	// Floor, with the green streaks of the AIC version.
	add(p(cFloor, detMid, 0, 0.54, 1, 0.48, 1, 1, 0, 1))
	for _, st := range [][4]float64{
		{0.20, 0.62, 0.30, 0.65}, {0.36, 0.72, 0.50, 0.75}, {0.12, 0.86, 0.26, 0.89},
		{0.40, 0.90, 0.56, 0.93}, {0.66, 0.92, 0.80, 0.95}, {0.04, 0.95, 0.14, 0.98},
	} {
		add(r(st[0], st[1], st[2], st[3], cFloorGrn, detMid))
	}
	for _, st := range [][4]float64{
		{0.26, 0.56, 0.34, 0.58}, {0.52, 0.80, 0.60, 0.83}, {0.86, 0.86, 0.96, 0.89},
		{0.30, 0.82, 0.38, 0.84}, {0.86, 0.97, 0.98, 1.0},
	} {
		add(r(st[0], st[1], st[2], st[3], cFloorDark, detHigh))
	}

	// Window: two green shutters opened inwards.
	add(r(0.215, 0, 0.405, 0.31, cWinFrame, detMid))
	add(r(0.26, 0, 0.30, 0.27, cPane, detMid))
	add(r(0.33, 0, 0.37, 0.27, cPane, detMid))
	add(r(0.26, 0.13, 0.37, 0.15, cWinFrame, detMid))
	add(r(0.25, 0.085, 0.305, 0.10, cWinFrame, detHigh))
	add(r(0.32, 0.085, 0.375, 0.10, cWinFrame, detHigh))
	add(r(0.25, 0.175, 0.305, 0.19, cWinFrame, detHigh))
	add(r(0.32, 0.175, 0.375, 0.19, cWinFrame, detHigh))
	add(r(0.26, 0.01, 0.285, 0.08, cPaneLight, detHigh))
	add(r(0.33, 0.01, 0.355, 0.08, cPaneLight, detHigh))
	add(r(0.215, 0.29, 0.405, 0.31, cWinDark, detHigh))

	// Pictures on the wall (high detail only).
	add(r(0.42, 0.07, 0.55, 0.18, cPicFrame, detHigh))
	add(r(0.435, 0.09, 0.535, 0.16, cPicGreen, detHigh))
	add(p(cPicFrame, detHigh, 0.615, 0.02, 0.675, 0.0, 0.695, 0.10, 0.635, 0.12))
	add(r(0.64, 0.03, 0.67, 0.08, cPicFace, detHigh))
	add(p(cPicFrame, detHigh, 0.71, 0.0, 0.80, 0.0, 0.80, 0.08, 0.72, 0.09))
	add(r(0.735, 0.01, 0.775, 0.06, cPicFace, detHigh))
	add(p(cPaneLight, detHigh, 0.675, 0.17, 0.72, 0.16, 0.735, 0.26, 0.69, 0.27))
	add(p(cPaneLight, detHigh, 0.775, 0.19, 0.86, 0.18, 0.865, 0.26, 0.785, 0.27))
	add(r(0.155, 0.10, 0.215, 0.22, cPicFrame, detHigh)) // mirror
	add(r(0.165, 0.115, 0.205, 0.205, cPillow, detHigh))
	add(r(0.095, 0.09, 0.14, 0.48, cSheet, detHigh)) // towel
	add(r(0.112, 0.09, 0.12, 0.48, cRed, detHigh))
	add(r(0.41, 0.22, 0.58, 0.235, cTableDark, detHigh)) // peg rail
	add(r(0.43, 0.23, 0.475, 0.38, cCloth, detHigh))     // clothes on pegs

	// Table with jug and basin.
	add(r(0.15, 0.35, 0.355, 0.40, cTable, detMid))
	add(r(0.165, 0.40, 0.19, 0.61, cTableDark, detMid))
	add(r(0.315, 0.40, 0.34, 0.57, cTableDark, detMid))
	add(r(0.19, 0.40, 0.315, 0.43, cTableDark, detHigh))
	add(r(0.17, 0.28, 0.20, 0.35, cJug, detHigh))
	add(r(0.21, 0.31, 0.29, 0.35, cJug, detHigh))
	add(r(0.30, 0.31, 0.32, 0.35, cPillow, detHigh))

	// Back chair, between table and bed.
	add(r(0.36, 0.30, 0.38, 0.56, cChairDark, detMid)) // back legs
	add(r(0.44, 0.30, 0.46, 0.50, cChairDark, detMid))
	add(r(0.36, 0.31, 0.46, 0.33, cChair, detMid)) // ladder back
	add(r(0.36, 0.36, 0.46, 0.375, cChair, detHigh))
	add(r(0.35, 0.42, 0.48, 0.47, cStraw, detMid)) // seat
	add(r(0.35, 0.47, 0.37, 0.58, cChair, detMid)) // front legs
	add(r(0.46, 0.47, 0.48, 0.56, cChair, detMid))

	// Bed: headboard, pillows, sheet, red cover, side rail, footboard.
	add(p(cBed, detMid, 0.48, 0.28, 0.58, 0.25, 0.69, 0.27, 0.69, 0.50, 0.48, 0.52))
	add(p(cPillow, detMid, 0.50, 0.31, 0.68, 0.29, 0.68, 0.37, 0.50, 0.39))
	add(r(0.585, 0.29, 0.595, 0.37, cSheet, detHigh))
	add(p(cSheet, detMid, 0.48, 0.38, 0.52, 0.38, 0.55, 0.62, 0.50, 0.62))
	add(p(cRed, detMid, 0.51, 0.39, 0.60, 0.38, 0.60, 0.58, 0.535, 0.60))
	add(p(cRedDark, detHigh, 0.54, 0.55, 0.60, 0.53, 0.60, 0.58, 0.545, 0.60))
	add(p(cBedDark, detMid, 0.475, 0.50, 0.50, 0.50, 0.62, 0.86, 0.585, 0.86)) // side rail
	add(p(cBed, detMid, 0.585, 0.33, 0.66, 0.29, 0.76, 0.28, 0.86, 0.30, 0.94, 0.33,
		0.94, 0.78, 0.585, 0.85))
	add(r(0.585, 0.31, 0.625, 0.86, cBedLight, detMid))                        // left post
	add(r(0.90, 0.31, 0.94, 0.79, cBedDark, detMid))                           // right post
	add(p(cBedDark, detMid, 0.625, 0.66, 0.90, 0.63, 0.90, 0.66, 0.625, 0.69)) // rail
	add(p(cBedLight, detHigh, 0.66, 0.34, 0.86, 0.33, 0.86, 0.35, 0.66, 0.36))
	add(r(0.70, 0.40, 0.71, 0.62, cBedDark, detHigh)) // board seams
	add(r(0.78, 0.39, 0.79, 0.61, cBedDark, detHigh))

	// Front chair, bottom left.
	add(r(0.0, 0.40, 0.03, 0.88, cChairDark, detMid)) // back post
	add(r(0.115, 0.38, 0.145, 0.60, cChair, detMid))
	add(r(0.0, 0.44, 0.13, 0.465, cChair, detMid)) // ladder rungs
	add(r(0.0, 0.50, 0.13, 0.52, cChair, detHigh))
	add(p(cStraw, detMid, 0.0, 0.58, 0.17, 0.56, 0.20, 0.66, 0.0, 0.69)) // seat
	add(p(cChairDark, detHigh, 0.0, 0.66, 0.20, 0.64, 0.20, 0.66, 0.0, 0.69))
	add(r(0.03, 0.69, 0.055, 0.88, cChair, detMid)) // legs
	add(r(0.165, 0.66, 0.19, 0.86, cChair, detMid))
	add(r(0.03, 0.78, 0.19, 0.80, cChairDark, detHigh))

	return s
}

// paintScene rasterises the bedroom into dst within rect, sampling one colour
// per block x block cell so the result stays on a crisp pixel grid.
func paintScene(dst *image.NRGBA, rect image.Rectangle, block, detail int) {
	paintShapes(dst, rect, block, detail, bedroom(), cWall)
}

// paintShapes is paintScene with an explicit shape list and base colour; the
// about animation uses it to lay the painting down a few shapes at a time.
func paintShapes(dst *image.NRGBA, rect image.Rectangle, block, detail int, shapes []shape, base color.NRGBA) {
	w, h := rect.Dx(), rect.Dy()
	for by := 0; by < h; by += block {
		for bx := 0; bx < w; bx += block {
			cx := (float64(bx) + float64(minInt(block, w-bx))/2) / float64(w)
			cy := (float64(by) + float64(minInt(block, h-by))/2) / float64(h)
			c := base
			for _, sh := range shapes {
				if sh.detail > detail {
					continue
				}
				if sh.contains(cx, cy) {
					c = sh.col
				}
			}
			fill(dst, image.Rect(rect.Min.X+bx, rect.Min.Y+by,
				rect.Min.X+minInt(bx+block, w), rect.Min.Y+minInt(by+block, h)), c)
		}
	}
}

func (s shape) contains(x, y float64) bool {
	if len(s.pts) == 2 {
		a, b := s.pts[0], s.pts[1]
		return x >= a.x && x < b.x && y >= a.y && y < b.y
	}
	in := false
	n := len(s.pts)
	for i, j := 0, n-1; i < n; j, i = i, i+1 {
		pi, pj := s.pts[i], s.pts[j]
		if (pi.y > y) != (pj.y > y) && x < (pj.x-pi.x)*(y-pi.y)/(pj.y-pi.y)+pi.x {
			in = !in
		}
	}
	return in
}

func fill(dst *image.NRGBA, rc image.Rectangle, c color.NRGBA) {
	rc = rc.Intersect(dst.Bounds())
	for y := rc.Min.Y; y < rc.Max.Y; y++ {
		for x := rc.Min.X; x < rc.Max.X; x++ {
			dst.SetNRGBA(x, y, c)
		}
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
