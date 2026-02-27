//go:build ignore

// Generates PWA icons for Comrad Watch.
// Run: go run generate.go
package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

func main() {
	generateIcon("icon-192.png", 192)
	generateIcon("icon-512.png", 512)
}

func generateIcon(name string, size int) {
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	bg := color.RGBA{10, 10, 10, 255}    // #0A0A0A
	red := color.RGBA{255, 59, 48, 255}   // #FF3B30

	cx := float64(size) / 2
	cy := float64(size) / 2
	outerR := float64(size) * 0.42
	innerR := float64(size) * 0.28

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			dist := math.Sqrt(dx*dx + dy*dy)

			if dist <= innerR {
				img.Set(x, y, red)
			} else if dist <= outerR {
				// Smooth ring between innerR and outerR
				img.Set(x, y, bg)
			} else {
				img.Set(x, y, bg)
			}
		}
	}

	f, _ := os.Create(name)
	defer f.Close()
	png.Encode(f, img)
}
