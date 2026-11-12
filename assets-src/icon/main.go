// Command icon regenerates every Paintpane icon and splash image from the
// pixel drawings in this directory. Run it through export.sh.
package main

import (
	"flag"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
)

// icon returns the app icon drawn for one size. 16-32 px are hand-placed
// grids; larger sizes rasterise the shape scene at their own block size and
// detail tier.
func icon(size int) *image.NRGBA {
	if size <= 32 {
		return smallIcon(size)
	}
	type tier struct{ block, gold, inner, detail int }
	t := map[int]tier{
		48:  {1, 3, 1, detMid},
		64:  {1, 3, 1, detMid},
		96:  {2, 3, 1, detMid}, // the 48 px drawing at 200 % DPI
		118: {2, 4, 1, detHigh},
		128: {2, 4, 1, detHigh},
		256: {4, 4, 1, detHigh}, // the 128 px drawing at 200 % DPI
	}[size]
	if t.block == 0 {
		panic("no tier for size")
	}
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	in := drawFrame(img, img.Bounds(), t.block, t.gold, t.inner, size >= 64)
	paintScene(img, in, t.block, t.detail)
	return img
}

// framedPainting draws a landscape frame + painting at block size b with its
// outer top-left at (x, y) and size w x h (all in pixels, multiples of b).
// shapes limits the painting to the first n scene shapes (-1 for all).
func framedPainting(dst *image.NRGBA, x, y, w, h, b, n int) {
	in := drawFrame(dst, image.Rect(x, y, x+w, y+h), b, 3, 1, true)
	sh := bedroom()
	base := cWall
	if n >= 0 && n < len(sh) {
		sh = sh[:n]
		base = cCreamDark // bare primed canvas before the first stroke
	}
	paintShapes(dst, in, b, detHigh, sh, base)
}

// splash draws the 300 px start-up splash, or the 600 px about image when
// scale is 2. It is laid out on a 75 x 75 grid of 4 px blocks.
func splash(scale, shapes int, title bool) *image.NRGBA {
	b := 4 * scale
	size := 75 * b
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	fill(img, img.Bounds(), cCreamDark)
	fill(img, img.Bounds().Inset(b), cCream)
	framedPainting(img, 8*b, 7*b, 59*b, 46*b, b, shapes)
	if title {
		tw := textWidth("Paintpane")
		drawText(img, "Paintpane", (75-tw)/2*b, 58*b, b, cInk, cCreamDark)
	}
	return img
}

// wizardImage is the 240 x 458 Inno Setup side panel.
func wizardImage() *image.NRGBA {
	const b = 4
	img := image.NewNRGBA(image.Rect(0, 0, 240, 458))
	fill(img, img.Bounds(), cNavy)
	fill(img, image.Rect(0, 100*b, 240, 458), cNavyLight) // wainscot
	fill(img, image.Rect(0, 99*b, 240, 100*b), cGoldShade)
	framedPainting(img, 4*b, 24*b, 52*b, 40*b, b, -1)
	tw := textWidth("Paintpane")
	drawText(img, "Paintpane", (60-tw)/2*b, 72*b, b, cCream, cOutline)
	return img
}

// aboutFrames lays the painting down in strokes, then holds the finished
// picture with its title; about 7.8 s in all, like the original animation.
func aboutFrames() ([]*image.NRGBA, []int) {
	n := len(bedroom())
	var frames []*image.NRGBA
	var delays []int
	frames = append(frames, splash(2, 0, false))
	delays = append(delays, 40)
	steps := 16
	for i := 1; i <= steps; i++ {
		frames = append(frames, splash(2, n*i/steps, false))
		delays = append(delays, 18)
	}
	frames = append(frames, splash(2, -1, true))
	delays = append(delays, 450)
	return frames, delays
}

func main() {
	root := flag.String("root", "../..", "repository root to write into")
	preview := flag.String("preview", "", "if set, also write a zoomed preview sheet to this PNG")
	flag.Parse()
	out := func(p string) string { return filepath.Join(*root, filepath.FromSlash(p)) }

	sizes := []int{16, 24, 32, 48, 64, 96, 128, 256}
	var icons []*image.NRGBA
	for _, s := range sizes {
		icons = append(icons, icon(s))
	}
	writeICO(out("installer/Paintpane.ico"), icons)
	writePNG(out("asset/icons/tray.png"), icon(118))
	writePNG(out("asset/images/splash.png"), splash(1, -1, true))
	frames, delays := aboutFrames()
	writeGIF(out("asset/images/about_splash.gif"), frames, delays)
	writeBMP(out("installer/Paintpane.bmp"), wizardImage())

	if *preview != "" {
		writePNG(*preview, previewSheet(append(icons[:3:3], icon(48), icon(64), icon(118))))
		writePNG(strings.TrimSuffix(*preview, ".png")+"-wizard.png", wizardImage())
		f, d := aboutFrames()
		writePNG(strings.TrimSuffix(*preview, ".png")+"-about-mid.png", f[len(d)/2])
	}
	os.Stdout.WriteString("icons written\n")
}

// previewSheet shows each icon at 1x and zoomed, on a dark and a light
// taskbar colour.
func previewSheet(icons []*image.NRGBA) *image.NRGBA {
	const zoomTo = 192
	pad := 16
	w := pad
	for range icons {
		w += zoomTo + pad
	}
	w += 60
	h := 2*(240+pad+140) + pad
	sheet := image.NewNRGBA(image.Rect(0, 0, w, h))
	bgs := []color.NRGBA{{0x20, 0x20, 0x24, 255}, {0xF3, 0xF3, 0xF3, 255}}
	for row, bg := range bgs {
		y0 := row * (h / 2)
		fill(sheet, image.Rect(0, y0, w, y0+h/2), bg)
		x := pad
		for _, ic := range icons {
			s := ic.Bounds().Dx()
			z := zoomTo / s
			if z < 2 {
				z = 2
			}
			blit(sheet, ic, x, y0+pad, 1)
			blit(sheet, ic, x, y0+pad+140, z)
			x += zoomTo + pad
		}
	}
	return sheet
}

func blit(dst, src *image.NRGBA, x0, y0, z int) {
	b := src.Bounds()
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			c := src.NRGBAAt(x, y)
			if c.A == 0 {
				continue
			}
			fill(dst, image.Rect(x0+x*z, y0+y*z, x0+(x+1)*z, y0+(y+1)*z), c)
		}
	}
}
