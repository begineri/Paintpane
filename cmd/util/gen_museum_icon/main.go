// gen_museum_icon draws the neutral placeholder icon used for museum sources: a rounded
// square in one colour with the museum's initial in white. It exists because official
// museum logos may not be used without permission (docs/museum-logos.md).
//
//	go run ./cmd/util/gen_museum_icon -letter N -color 2F5D8A -out pkg/wallpaper/providers/nga/nga.png
//
// The style is a placeholder until the UI prototype session confirms the final icon design.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"strconv"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

func main() {
	letter := flag.String("letter", "M", "initial drawn in the icon")
	hex := flag.String("color", "444444", "background colour, RRGGBB")
	out := flag.String("out", "icon.png", "output PNG path")
	size := flag.Int("size", 64, "icon size in pixels (square)")
	glyph := flag.String("glyph", "", "draw a solid white glyph (heart or folder) instead of the letter")
	flag.Parse()

	rgb, err := strconv.ParseUint(*hex, 16, 32)
	if err != nil {
		fmt.Fprintln(os.Stderr, "bad -color:", err)
		os.Exit(1)
	}
	bg := color.NRGBA{R: uint8(rgb >> 16), G: uint8(rgb >> 8), B: uint8(rgb), A: 255}

	img := image.NewNRGBA(image.Rect(0, 0, *size, *size))
	drawRoundedSquare(img, bg, float64(*size)*0.22)

	if *glyph != "" {
		path, ok := glyphPaths[*glyph]
		if !ok {
			fmt.Fprintln(os.Stderr, "bad -glyph (want heart or folder):", *glyph)
			os.Exit(1)
		}
		drawGlyph(img, path, color.White, 0.58)
		write(img, *out)
		return
	}

	ttf, err := opentype.Parse(gobold.TTF)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	face, err := opentype.NewFace(ttf, &opentype.FaceOptions{Size: float64(*size) * 0.62, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	d := &font.Drawer{Dst: img, Src: image.NewUniform(color.White), Face: face}
	adv := d.MeasureString(*letter)
	metrics := face.Metrics()
	capHeight := ttf2cap(face)
	d.Dot = fixed.Point26_6{
		X: (fixed.I(*size) - adv) / 2,
		Y: fixed.I(*size)/2 + capHeight/2,
	}
	_ = metrics
	d.DrawString(*letter)

	write(img, *out)
}

func write(img image.Image, path string) {
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// ttf2cap is the height of a capital letter, used to centre the initial vertically.
func ttf2cap(face font.Face) fixed.Int26_6 {
	bounds, _, _ := face.GlyphBounds('H')
	return -bounds.Min.Y
}

// drawRoundedSquare fills the image with a rounded square and leaves the corners transparent.
func drawRoundedSquare(img *image.NRGBA, c color.NRGBA, radius float64) {
	b := img.Bounds()
	w, h := float64(b.Dx()), float64(b.Dy())
	draw.Draw(img, b, image.Transparent, image.Point{}, draw.Src)
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			// Distance from the nearest corner arc centre; 4x4 supersampling smooths the edge.
			covered := 0
			for sy := 0; sy < 4; sy++ {
				for sx := 0; sx < 4; sx++ {
					px := float64(x) + (float64(sx)+0.5)/4
					py := float64(y) + (float64(sy)+0.5)/4
					if inRounded(px, py, w, h, radius) {
						covered++
					}
				}
			}
			if covered > 0 {
				cc := c
				cc.A = uint8(int(c.A) * covered / 16)
				img.SetNRGBA(x, y, cc)
			}
		}
	}
}

func inRounded(px, py, w, h, r float64) bool {
	cx, cy := px, py
	switch {
	case px < r:
		cx = r
	case px > w-r:
		cx = w - r
	}
	switch {
	case py < r:
		cy = r
	case py > h-r:
		cy = h - r
	}
	dx, dy := px-cx, py-cy
	return dx*dx+dy*dy <= r*r
}
