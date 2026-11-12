package main

import (
	"image"
	"image/color"
	"strings"
)

// A 5 x 7 pixel font with just the glyphs the splash and installer art need.
// Rows 0-6 sit on the baseline; g and p add two descender rows below it.
var glyphs = map[rune][]string{
	'A': {".###.", "#...#", "#...#", "#####", "#...#", "#...#", "#...#"},
	'D': {"####.", "#...#", "#...#", "#...#", "#...#", "#...#", "####."},
	'M': {"#...#", "##.##", "#.#.#", "#.#.#", "#...#", "#...#", "#...#"},
	'P': {"####.", "#...#", "#...#", "####.", "#....", "#....", "#...."},
	'W': {"#...#", "#...#", "#...#", "#.#.#", "#.#.#", "##.##", "#...#"},
	'a': {".....", ".....", ".###.", "....#", ".####", "#...#", ".####"},
	'c': {".....", ".....", ".###.", "#....", "#....", "#...#", ".###."},
	'd': {"....#", "....#", ".##.#", "#..##", "#...#", "#...#", ".####"},
	'e': {".....", ".....", ".###.", "#...#", "#####", "#....", ".###."},
	'f': {"..##.", ".#..#", ".#...", "###..", ".#...", ".#...", ".#..."},
	'g': {".....", ".....", ".####", "#...#", "#...#", "#...#", ".####", "....#", ".###."},
	'h': {"#....", "#....", "#.##.", "##..#", "#...#", "#...#", "#...#"},
	'i': {"..#..", ".....", ".##..", "..#..", "..#..", "..#..", ".###."},
	'l': {".##..", "..#..", "..#..", "..#..", "..#..", "..#..", ".###."},
	'n': {".....", ".....", "#.##.", "##..#", "#...#", "#...#", "#...#"},
	'o': {".....", ".....", ".###.", "#...#", "#...#", "#...#", ".###."},
	'p': {".....", ".....", "####.", "#...#", "#...#", "#...#", "####.", "#....", "#...."},
	'r': {".....", ".....", "#.##.", "##..#", "#....", "#....", "#...."},
	's': {".....", ".....", ".####", "#....", ".###.", "....#", "####."},
	't': {".#...", ".#...", "####.", ".#...", ".#...", ".#..#", "..##."},
	'u': {".....", ".....", "#...#", "#...#", "#...#", "#..##", ".##.#"},
	'.': {".....", ".....", ".....", ".....", ".....", ".##..", ".##.."},
}

// textWidth is the width in font pixels of s (5 per glyph, 1 between glyphs,
// 3 for a space).
func textWidth(s string) int {
	w := 0
	for i, ch := range s {
		if i > 0 {
			w++
		}
		if ch == ' ' {
			w += 3
		} else {
			w += 5
		}
	}
	return w
}

// drawText draws s with its top-left at (x, y), each font pixel scale x scale,
// with an optional one-pixel drop shadow (zero alpha to omit).
func drawText(dst *image.NRGBA, s string, x, y, scale int, c, shadow color.NRGBA) {
	if shadow.A != 0 {
		drawText(dst, s, x+scale, y+scale, scale, shadow, color.NRGBA{})
	}
	for _, ch := range s {
		if ch == ' ' {
			x += 4 * scale
			continue
		}
		g, ok := glyphs[ch]
		if !ok {
			panic("missing glyph " + strings.ToUpper(string(ch)) + " (" + string(ch) + ")")
		}
		for gy, row := range g {
			for gx := 0; gx < len(row); gx++ {
				if row[gx] == '#' {
					fill(dst, image.Rect(x+gx*scale, y+gy*scale, x+(gx+1)*scale, y+(gy+1)*scale), c)
				}
			}
		}
		x += 6 * scale
	}
}
