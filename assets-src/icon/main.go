// Command icon regenerates every Paintpane icon and splash image from the
// pixel drawings in this directory. Run it through export.sh.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"
)

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

// wizardImage is the 240 x 458 Inno Setup side panel: the large icon above
// the name, centred on cream.
func wizardImage() *image.NRGBA {
	const w, h = 240, 458
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	fill(img, img.Bounds(), cCream)
	ic := b2x256.scaled(1, 2).render() // 128 px canvas, 112 px tile
	const scale, gap = 3, 20
	tw, th := textWidth("Paintpane")*scale, 7*scale
	top := (h - (112 + gap + th)) / 2 // centre the tile and the text as a group
	over(img, ic, (w-128)/2, top-8)
	drawText(img, "Paintpane", (w-tw)/2, top+112+gap, scale, cInk, color.NRGBA{})
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

	sizes := []int{16, 20, 24, 32, 40, 48, 64, 96, 256}
	var icons []*image.NRGBA
	for _, s := range sizes {
		icons = append(icons, icon(s))
	}
	big := icon(256)
	writeICO(out("assets-src/installer/Paintpane.ico"), icons)
	writePNG(out("asset/icons/tray.png"), trayIcon())
	writePNG(out("asset/icons/app.png"), big)
	writePNG(out("site/assets/icon-256.png"), big)
	writePNG(out("site/assets/favicon-32.png"), icon(32))
	writePNG(out("docs/mockups/intro-page/icon-256.png"), big)
	// The about animation still shows the old framed Bedroom (owner's call,
	// 2026-11-15); it is regenerated unchanged.
	frames, delays := aboutFrames()
	writeGIF(out("asset/images/about_splash.gif"), frames, delays)
	writeBMP(out("assets-src/installer/Paintpane.bmp"), wizardImage())

	if *preview != "" {
		base := strings.TrimSuffix(*preview, ".png")
		writePNG(*preview, previewSheet(append(icons[:6:6], trayIcon())))
		for _, s := range []int{16, 20, 24} {
			writePNG(fmt.Sprintf("%s-%d.png", base, s), icon(s)) // 1x, to check against final/*.svg
		}
		writePNG(base+"-wizard.png", wizardImage())
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
	over(dst, nearest(src, z), x0, y0)
}
