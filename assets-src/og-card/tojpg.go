//go:build ignore

// tojpg converts the PNG that Edge renders into the JPEG used as the share card.
//
//	go run tojpg.go card-en.png ../../site/assets/img/en/og-card.jpg
package main

import (
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"os"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: go run tojpg.go <in.png> <out.jpg>")
		os.Exit(2)
	}
	in, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer in.Close()
	img, _, err := image.Decode(in)
	if err != nil {
		panic(err)
	}
	out, err := os.Create(os.Args[2])
	if err != nil {
		panic(err)
	}
	defer out.Close()
	if err := jpeg.Encode(out, img, &jpeg.Options{Quality: 86}); err != nil {
		panic(err)
	}
	fmt.Println(os.Args[2], img.Bounds().Size())
}
