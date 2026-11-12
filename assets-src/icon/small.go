package main

import (
	"fmt"
	"image"
	"image/color"
)

// Hand-placed pixel grids for the tray / taskbar sizes. At these sizes only
// the frame and the five strongest colour blocks of the painting survive:
// blue-violet wall, green window, yellow bed, red cover, orange-brown floor.

var key = map[byte]color.NRGBA{
	'b': cWall, 'B': cWallShade, 'd': cDoor,
	'G': cWinFrame, 'g': cWinDark, 'y': cPane, 'Y': cBed, 'L': cBedLight, 'o': cBedDark,
	'w': cPillow, 's': cSheet, 'R': cRed, 'r': cRedDark,
	'c': cChair, 'C': cChairDark, 'S': cStraw,
	'f': cFloor, 'F': cFloorGrn, 'k': cFloorDark,
}

// 12 x 12 canvas inside a 2 px frame.
var grid16 = []string{
	"bbbGGGbbbbbb",
	"bbbGyGbbbbbb",
	"bbbGyGbbbbbb",
	"bbbGGGbbbbbb",
	"bbbbbYYYbbbb",
	"bbbbbwwLYYYb",
	"bbbbbRRLYYYo",
	"ffffRRRLYYYo",
	"ffffRRRLYYYo",
	"ffffoooLooYo",
	"fFFffffLfffo",
	"ffffffffffff",
}

// 18 x 18 canvas inside a 3 px frame.
var grid24 = []string{
	"bbbGGGGGbbbbbbbbbb",
	"bbbGyGyGbbbbbbbbbb",
	"bbbGyGyGbbbbbbbbbb",
	"bbbGyGyGbbbbbbbbbb",
	"bbbGyGyGbbbbbbbbbb",
	"bbbGGGGGbbbbbbbbbb",
	"bbbbbbbYYYYbbbbbbb",
	"bbbbbbbYwwLYYYYbbb",
	"bbbbbbbswwLYYYYYob",
	"bbbbbbsRRRLYYYYYob",
	"ffffffsRRRLYYYYYob",
	"ffffffRRRRLYYYYYof",
	"ffffffoRRRLYYYYYof",
	"fffffffoRRLoooooof",
	"ffFFFffoooLYYYYYof",
	"fffffffffoLffffFof",
	"ffffffffffLfffffff",
	"fffFFFffffffffffff",
}

// 24 x 24 canvas inside a 4 px frame.
var grid32 = []string{
	"bbbbbGGGGGGGbbbbbbbbbbbb",
	"bbbbbGyyGyyGbbbbbbbbbbbb",
	"bbbbbGyyGyyGbbbbbbbbbbbb",
	"bbbbbGGGGGGGbbbbbbbbbbbb",
	"bbbbbGyyGyyGbbbbbbbbbbbb",
	"bbbbbGyyGyyGbbbbbbbbbbbb",
	"bbbbbGyyGyyGbbbbbbbbbbbb",
	"bbbbbgggggggbbbbbbbbbbbb",
	"bbbbbbbbbbYYYYYbbbbbbbbb",
	"bbbbbbbbbYwwwwLYYYYYbbbb",
	"bbbbbbbbbYwwwwLYYYYYYYbb",
	"bbbbbbbbbsRRRRLYYYYYYYob",
	"bbbbbbbbsRRRRRLYYYYYYYob",
	"bbbbbbbbsRRRRRLYYYYYYYob",
	"ffffffffsRRRRRLYYYYYYYob",
	"ffffffffRRRRRRLYYYYYYYof",
	"ffffffffoRRRRrLYYYYYYYof",
	"fffffffffoRrrrLoooooooof",
	"ffFFFfffffooooLYYYYYYYof",
	"ffffffffffffooLYYYYYYYof",
	"fffffffffffffoLfffFFfFof",
	"ffffFFFfffffffLfffffffff",
	"ffffffffffffffffffffffff",
	"fffffffffFFFFfffffffffff",
}

func smallIcon(size int) *image.NRGBA {
	var grid []string
	var gold, inner int
	switch size {
	case 16:
		grid, gold, inner = grid16, 1, 0
	case 24:
		grid, gold, inner = grid24, 2, 0
	case 32:
		grid, gold, inner = grid32, 2, 1
	default:
		panic(fmt.Sprintf("no hand grid for %d", size))
	}
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	in := drawFrame(img, img.Bounds(), 1, gold, inner, false)
	if len(grid) != in.Dy() {
		panic(fmt.Sprintf("grid%d: %d rows, want %d", size, len(grid), in.Dy()))
	}
	for y, row := range grid {
		if len(row) != in.Dx() {
			panic(fmt.Sprintf("grid%d row %d: %d cols, want %d", size, y, len(row), in.Dx()))
		}
		for x := 0; x < len(row); x++ {
			c, ok := key[row[x]]
			if !ok {
				panic(fmt.Sprintf("grid%d: unknown key %q", size, row[x]))
			}
			img.SetNRGBA(in.Min.X+x, in.Min.Y+y, c)
		}
	}
	return img
}
