package main

import (
	"image"
	"image/color"
	"strconv"
	"strings"
	"unicode"
)

// glyphPaths are solid 24×24 outlines (Material "favorite" and "folder", Apache 2.0) used for
// the Favorites and Local Folders tiles: a white heart and a white folder (S1-10).
var glyphPaths = map[string]string{
	"heart":  "M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z",
	"folder": "M10 4H4c-1.1 0-1.99.9-1.99 2L2 18c0 1.1.9 2 2 2h16c1.1 0 2-.9 2-2V8c0-1.1-.9-2-2-2h-8l-2-2z",
}

type pt struct{ x, y float64 }

// parsePath flattens an SVG path (commands M m L l H h V v C c z, with repeated coordinates) into
// one polygon per subpath.
func parsePath(d string) [][]pt {
	var polys [][]pt
	var cur []pt
	var tokens []string
	var num strings.Builder
	flush := func() {
		if num.Len() > 0 {
			tokens = append(tokens, num.String())
			num.Reset()
		}
	}
	for _, r := range d {
		switch {
		case unicode.IsLetter(r):
			flush()
			tokens = append(tokens, string(r))
		case r == '-':
			flush()
			num.WriteRune(r)
		case r == ' ' || r == ',':
			flush()
		case r == '.':
			// A second dot starts a new number: "1.1.9" is 1.1 and .9.
			if strings.Contains(num.String(), ".") {
				flush()
			}
			num.WriteRune(r)
		default:
			num.WriteRune(r)
		}
	}
	flush()

	var x, y float64
	var cmd string
	i := 0
	next := func() float64 {
		v, _ := strconv.ParseFloat(tokens[i], 64)
		i++
		return v
	}
	isNum := func() bool {
		if i >= len(tokens) {
			return false
		}
		_, err := strconv.ParseFloat(tokens[i], 64)
		return err == nil
	}
	for i < len(tokens) {
		if !isNum() {
			cmd = tokens[i]
			i++
			if cmd == "z" || cmd == "Z" {
				if len(cur) > 0 {
					polys = append(polys, cur)
					cur = nil
				}
				continue
			}
		}
		rel := unicode.IsLower(rune(cmd[0]))
		switch strings.ToUpper(cmd) {
		case "M", "L":
			nx, ny := next(), next()
			if rel {
				nx, ny = x+nx, y+ny
			}
			x, y = nx, ny
			cur = append(cur, pt{x, y})
			if strings.ToUpper(cmd) == "M" {
				cmd = map[bool]string{true: "l", false: "L"}[rel] // further pairs are lines
			}
		case "H":
			nx := next()
			if rel {
				nx += x
			}
			x = nx
			cur = append(cur, pt{x, y})
		case "V":
			ny := next()
			if rel {
				ny += y
			}
			y = ny
			cur = append(cur, pt{x, y})
		case "C":
			var c [6]float64
			for k := range c {
				c[k] = next()
			}
			if rel {
				for k := 0; k < 6; k += 2 {
					c[k] += x
					c[k+1] += y
				}
			}
			p0, p1, p2, p3 := pt{x, y}, pt{c[0], c[1]}, pt{c[2], c[3]}, pt{c[4], c[5]}
			const steps = 24
			for s := 1; s <= steps; s++ {
				t := float64(s) / steps
				u := 1 - t
				cur = append(cur, pt{
					u*u*u*p0.x + 3*u*u*t*p1.x + 3*u*t*t*p2.x + t*t*t*p3.x,
					u*u*u*p0.y + 3*u*u*t*p1.y + 3*u*t*t*p2.y + t*t*t*p3.y,
				})
			}
			x, y = p3.x, p3.y
		default:
			panic("unsupported path command " + cmd)
		}
	}
	if len(cur) > 0 {
		polys = append(polys, cur)
	}
	return polys
}

// drawGlyph fills the path, scaled to fraction of the icon and centred, with 4×4 supersampling.
func drawGlyph(img *image.NRGBA, path string, c color.Color, fraction float64) {
	polys := parsePath(path)
	minX, minY, maxX, maxY := 1e9, 1e9, -1e9, -1e9
	for _, p := range polys {
		for _, q := range p {
			minX, maxX = min(minX, q.x), max(maxX, q.x)
			minY, maxY = min(minY, q.y), max(maxY, q.y)
		}
	}
	b := img.Bounds()
	size := float64(b.Dx())
	scale := size * fraction / max(maxX-minX, maxY-minY)
	offX := (size - (maxX-minX)*scale) / 2
	offY := (size - (maxY-minY)*scale) / 2

	inside := func(px, py float64) bool {
		// Back to path space, then count crossings (even-odd; each glyph is one simple outline).
		x := (px-offX)/scale + minX
		y := (py-offY)/scale + minY
		in := false
		for _, poly := range polys {
			for i, j := 0, len(poly)-1; i < len(poly); j, i = i, i+1 {
				a, bb := poly[i], poly[j]
				if (a.y > y) != (bb.y > y) && x < (bb.x-a.x)*(y-a.y)/(bb.y-a.y)+a.x {
					in = !in
				}
			}
		}
		return in
	}
	cr, cg, cb, _ := c.RGBA()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			covered := 0
			for sy := 0; sy < 4; sy++ {
				for sx := 0; sx < 4; sx++ {
					if inside(float64(x)+(float64(sx)+0.5)/4, float64(y)+(float64(sy)+0.5)/4) {
						covered++
					}
				}
			}
			if covered == 0 {
				continue
			}
			// Blend the white glyph over the tile, keeping the tile's alpha.
			dst := img.NRGBAAt(x, y)
			a := float64(covered) / 16
			dst.R = uint8(float64(dst.R)*(1-a) + float64(cr>>8)*a)
			dst.G = uint8(float64(dst.G)*(1-a) + float64(cg>>8)*a)
			dst.B = uint8(float64(dst.B)*(1-a) + float64(cb>>8)*a)
			img.SetNRGBA(x, y, dst)
		}
	}
}
