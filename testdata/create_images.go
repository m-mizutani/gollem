package main

import (
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
)

func main() {
	// Create a small test JPEG (100x100 red square)
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255}) // Red
		}
	}
	f, _ := os.Create("test_image.jpg")
	jpeg.Encode(f, img, &jpeg.Options{Quality: 85})
	f.Close()

	// Create a small test PNG (100x100 blue square)
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{0, 0, 255, 255}) // Blue
		}
	}
	f, _ = os.Create("test_image.png")
	png.Encode(f, img)
	f.Close()

	// Create a small test GIF (50x50 green square)
	gifImg := image.NewPaletted(image.Rect(0, 0, 50, 50), color.Palette{
		color.RGBA{0, 0, 0, 255},   // Black
		color.RGBA{0, 255, 0, 255}, // Green
	})
	for y := 0; y < 50; y++ {
		for x := 0; x < 50; x++ {
			gifImg.SetColorIndex(x, y, 1) // Green
		}
	}
	f, _ = os.Create("test_image.gif")
	gif.Encode(f, gifImg, nil)
	f.Close()

	println("Created test images")
}