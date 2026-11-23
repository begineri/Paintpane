package main

import "image"

// MSIX package logos (msix/Assets, referenced by msix/AppxManifest.xml). Each is an existing tier
// enlarged by a whole factor and centred on a transparent canvas of the size Windows asks for, so
// block edges stay on whole pixels, as in the .ico sizes.
func msixLogos() map[string]*image.NRGBA {
	return map[string]*image.NRGBA{
		"Square44x44Logo.png":   centred(icon(40), 44, 44),                      // 20 px drawing x2
		"StoreLogo.png":         centred(icon(48), 50, 50),                      // 24 px drawing x2
		"Square150x150Logo.png": centred(b2x24.scaled(6, 1).render(), 150, 150), // 24 px drawing x6
		"Wide310x150Logo.png":   centred(b2x24.scaled(6, 1).render(), 310, 150),
	}
}

// centred returns a transparent w x h canvas with src in the middle.
func centred(src *image.NRGBA, w, h int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	b := src.Bounds()
	x0, y0 := (w-b.Dx())/2, (h-b.Dy())/2
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			dst.SetNRGBA(x0+x, y0+y, src.NRGBAAt(x, y))
		}
	}
	return dst
}
