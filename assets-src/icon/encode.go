package main

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
)

func writePNG(path string, img image.Image) {
	f, err := os.Create(path)
	check(err)
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	check(enc.Encode(f, img))
	check(f.Close())
}

// writeICO writes a multi-size .ico. Sizes below 256 are stored as classic
// 32-bit DIBs (with an AND mask) for the widest compatibility with Explorer,
// rsrc and Inno Setup; 256 is stored as PNG, as Windows expects.
func writeICO(path string, imgs []*image.NRGBA) {
	var dir, data bytes.Buffer
	le := binary.LittleEndian
	_ = binary.Write(&dir, le, [3]uint16{0, 1, uint16(len(imgs))})
	offset := 6 + 16*len(imgs)
	for _, img := range imgs {
		blob := icoEntry(img)
		w := img.Bounds().Dx()
		b := uint8(w)
		if w >= 256 {
			b = 0
		}
		dir.Write([]byte{b, b, 0, 0})
		_ = binary.Write(&dir, le, uint16(1))  // planes
		_ = binary.Write(&dir, le, uint16(32)) // bit count
		_ = binary.Write(&dir, le, uint32(len(blob)))
		_ = binary.Write(&dir, le, uint32(offset))
		offset += len(blob)
		data.Write(blob)
	}
	check(os.WriteFile(path, append(dir.Bytes(), data.Bytes()...), 0o644))
}

func icoEntry(img *image.NRGBA) []byte {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	if w >= 256 {
		var b bytes.Buffer
		check(png.Encode(&b, img))
		return b.Bytes()
	}
	var b bytes.Buffer
	le := binary.LittleEndian
	maskStride := ((w + 31) / 32) * 4
	_ = binary.Write(&b, le, struct {
		Size                   uint32
		Width, Height          int32
		Planes, BitCount       uint16
		Compression, SizeImage uint32
		XPels, YPels           int32
		ClrUsed, ClrImportant  uint32
	}{40, int32(w), int32(2 * h), 1, 32, 0, uint32(w*h*4 + maskStride*h), 0, 0, 0, 0})
	for y := h - 1; y >= 0; y-- {
		for x := 0; x < w; x++ {
			c := img.NRGBAAt(x, y)
			b.Write([]byte{c.B, c.G, c.R, c.A})
		}
	}
	for y := h - 1; y >= 0; y-- {
		row := make([]byte, maskStride)
		for x := 0; x < w; x++ {
			if img.NRGBAAt(x, y).A == 0 {
				row[x/8] |= 0x80 >> (x % 8)
			}
		}
		b.Write(row)
	}
	return b.Bytes()
}

// writeBMP writes a 24-bit bottom-up BMP, the format Inno Setup's
// WizardImageFile expects.
func writeBMP(path string, img *image.NRGBA) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	stride := (w*3 + 3) &^ 3
	size := 54 + stride*h
	var b bytes.Buffer
	le := binary.LittleEndian
	b.WriteString("BM")
	_ = binary.Write(&b, le, uint32(size))
	_ = binary.Write(&b, le, uint32(0))
	_ = binary.Write(&b, le, uint32(54))
	_ = binary.Write(&b, le, struct {
		Size                   uint32
		Width, Height          int32
		Planes, BitCount       uint16
		Compression, SizeImage uint32
		XPels, YPels           int32
		ClrUsed, ClrImportant  uint32
	}{40, int32(w), int32(h), 1, 24, 0, uint32(stride * h), 3780, 3780, 0, 0})
	for y := h - 1; y >= 0; y-- {
		row := make([]byte, stride)
		for x := 0; x < w; x++ {
			c := img.NRGBAAt(x, y)
			row[3*x], row[3*x+1], row[3*x+2] = c.B, c.G, c.R
		}
		b.Write(row)
	}
	check(os.WriteFile(path, b.Bytes(), 0o644))
}

// writeGIF writes frames with an exact palette built from their colours (the
// art uses far fewer than 256), so no dithering or quantisation happens.
func writeGIF(path string, frames []*image.NRGBA, delays []int) {
	idx := map[color.NRGBA]uint8{}
	var pal color.Palette
	for _, f := range frames {
		for i := 0; i < len(f.Pix); i += 4 {
			c := color.NRGBA{f.Pix[i], f.Pix[i+1], f.Pix[i+2], 255}
			if _, ok := idx[c]; !ok {
				if len(pal) == 256 {
					panic("gif: more than 256 colours")
				}
				idx[c] = uint8(len(pal))
				pal = append(pal, c)
			}
		}
	}
	g := &gif.GIF{LoopCount: -1}
	for i, f := range frames {
		pm := image.NewPaletted(f.Bounds(), pal)
		for p := 0; p < len(f.Pix); p += 4 {
			pm.Pix[p/4] = idx[color.NRGBA{f.Pix[p], f.Pix[p+1], f.Pix[p+2], 255}]
		}
		g.Image = append(g.Image, pm)
		g.Delay = append(g.Delay, delays[i])
		g.Disposal = append(g.Disposal, gif.DisposalNone)
	}
	out, err := os.Create(path)
	check(err)
	check(gif.EncodeAll(out, g))
	check(out.Close())
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
