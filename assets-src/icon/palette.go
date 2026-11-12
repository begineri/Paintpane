package main

import "image/color"

// Palette for the pixel-art re-drawing of Van Gogh's "The Bedroom" (1889,
// Art Institute of Chicago 1926.417, public domain). Colours were picked by
// eye from the reference and pushed slightly brighter so they survive at
// 16 px; nothing is sampled from the photograph.
func hex(s string) color.NRGBA {
	var v [3]uint8
	for i := 0; i < 3; i++ {
		v[i] = hexByte(s[1+2*i], s[2+2*i])
	}
	return color.NRGBA{v[0], v[1], v[2], 255}
}

func hexByte(a, b byte) uint8 { return nib(a)<<4 | nib(b) }

func nib(c byte) uint8 {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	}
	panic("bad hex digit")
}

var (
	cClear = color.NRGBA{}

	// Room.
	cWall      = hex("#7B8EDB")
	cWallShade = hex("#6476C6")
	cWallLight = hex("#96A8EA")
	cDoor      = hex("#8EA3E4")
	cDoorDark  = hex("#5867B4")
	cWinFrame  = hex("#2F9446")
	cWinDark   = hex("#1E6A32")
	cPane      = hex("#C3DB55")
	cPaneLight = hex("#E2EE8C")
	cBed       = hex("#DB9A2C")
	cBedLight  = hex("#F0B84A")
	cBedDark   = hex("#A3651B")
	cPillow    = hex("#F4EDB4")
	cSheet     = hex("#E3DB98")
	cRed       = hex("#D8392B")
	cRedDark   = hex("#A3251E")
	cChair     = hex("#E4BC3A")
	cChairDark = hex("#AE8620")
	cStraw     = hex("#CDAA45")
	cTable     = hex("#C9773A")
	cTableDark = hex("#965226")
	cFloor     = hex("#BD7A58")
	cFloorDark = hex("#9C5E44")
	cFloorGrn  = hex("#79C196")
	cPicFrame  = hex("#B4823A")
	cPicGreen  = hex("#93C27A")
	cPicFace   = hex("#E9C99D")
	cJug       = hex("#A9D2C4")
	cCloth     = hex("#5F7FB0")

	// Gilded frame.
	cOutline   = hex("#3A2410")
	cGold      = hex("#E3A82E")
	cGoldLight = hex("#F9DA76")
	cGoldShade = hex("#A86E1A")
	cGoldInner = hex("#5C3A14")

	// Splash / installer backgrounds and text.
	cCream     = hex("#F4EAD3")
	cCreamDark = hex("#E3D3B0")
	cInk       = hex("#4A2C12")
	cNavy      = hex("#2B3360")
	cNavyLight = hex("#3A4478")
)
