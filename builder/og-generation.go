package builder

import (
	"math"

	"github.com/fogleman/gg"
)

func GenerateOG(postTitle string, outPath string, config OGImageConfig) {
	const width = 1200
	const height = 630 // Common OG image size

	dc := gg.NewContext(width, height)
	// Set background color
	dc.SetRGB(1, 1, float64(250)/255)
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
	dc.SetRGB(51/255.0, 51/255.0, 51/255.0)
	for i, line := range wrapped {
		x := padding
		y := startY + float64(i)*lineHeight + lineHeight*0.8
		dc.DrawString(line, x, y)
	}

	// Draw the circular headshot image
	img, err := gg.LoadImage(config.IconPath)
	if err != nil {
		panic(err)
	}

	radius := 50.0
	cx, cy := padding+radius, float64(height)-padding-radius

	// Compute scale to fit the image within a square of side 2*radius
	imgW := float64(img.Bounds().Dx())
	imgH := float64(img.Bounds().Dy())
	scale := math.Min((2*radius)/imgW, (2*radius)/imgH)

	dc.Push()
	dc.DrawCircle(cx, cy, radius)
	dc.Clip()

	// Center and scale the image inside the circle
	dc.Translate(cx, cy)
	dc.Scale(scale, scale)
	dc.Translate(-imgW/2, -imgH/2)
	dc.DrawImage(img, 0, 0)
	dc.Pop()

	// Save the result
	dc.SavePNG(outPath)
}
