package builder

import (
	"math"

	"github.com/fogleman/gg"
)

func GenerateOG(postTitle string, outPath string, config OGImageConfig) {
	const width = 1200
	const height = 630 // Common OG image size

	dc := gg.NewContext(width, height)
	r := float64(config.BgR) / 255
	g := float64(config.BgG) / 255
	b := float64(config.BgB) / 255

	// Set background color
	dc.SetRGB(r, g, b)
	dc.Clear()

	// Load a custom font
	fontSize := config.FontSize
	if err := dc.LoadFontFace(config.FontPath, fontSize); err != nil {
		panic(err)
	}

	// The text to be drawn (top of the image)
	text := postTitle
	padding := 40.0
	wrapped := dc.WordWrap(text, float64(width)-2*padding)
	lineHeight := fontSize * 1.5
	startY := padding

	// Set text color and draw the top text
	tR := float64(config.TextR) / 255
	tG := float64(config.TextG) / 255
	tB := float64(config.TextB) / 255
	dc.SetRGB(tR, tG, tB)
	for i, line := range wrapped {
		x := padding
		y := startY + float64(i)*lineHeight + lineHeight*0.8
		dc.DrawString(line, x, y)
	}

	if config.IconPath != "" {
		img, err := gg.LoadImage(config.IconPath)
		if err != nil {
			panic(err)
		}

		radius := 50.0
		cx, cy := padding+radius, float64(height)-padding-radius

		imgW := float64(img.Bounds().Dx())
		imgH := float64(img.Bounds().Dy())
		scale := math.Min((2*radius)/imgW, (2*radius)/imgH)

		dc.Push()
		dc.DrawCircle(cx, cy, radius)
		dc.Clip()

		dc.Translate(cx, cy)
		dc.Scale(scale, scale)
		dc.Translate(-imgW/2, -imgH/2)
		dc.DrawImage(img, 0, 0)
		dc.Pop()
	}

	// Save the result
	dc.SavePNG(outPath)
}
