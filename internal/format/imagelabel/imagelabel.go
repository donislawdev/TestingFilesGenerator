// Package imagelabel draws the self describing label into pixels.
//
// Seven image formats burn the same label, so they share one rasteriser
// rather than carrying seven copies of it.
package imagelabel

import (
	"image"
	"image/color"
	"image/draw"
)

// Padding around the text inside its band, in scaled pixels.
const pad = 2

// Draw burns text into the top left of img and returns the height of the band
// it used.
//
// The band gets a solid background so the text stays readable whatever the
// picture underneath looks like. Without that, a label over a bright gradient
// is invisible in exactly the screenshot somebody attaches to a bug report.
//
// It draws what fits and stops. A picture too small for the label is not a
// reason to fail - the caller decides whether to ask for a label at all, and
// says so in the manifest when it is left out.
func Draw(img draw.Image, text string) int {
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 || text == "" {
		return 0
	}

	scale := scaleFor(b.Dx(), len(text))
	if scale == 0 {
		return 0
	}

	bandH := glyphHeight*scale + 2*pad
	if bandH > b.Dy() {
		bandH = b.Dy()
	}

	band := image.Rect(b.Min.X, b.Min.Y, b.Max.X, b.Min.Y+bandH)
	draw.Draw(img, band, image.NewUniform(color.RGBA{R: 16, G: 16, B: 16, A: 255}), image.Point{}, draw.Src)

	ink := color.RGBA{R: 240, G: 240, B: 240, A: 255}
	x := b.Min.X + pad
	for _, r := range text {
		if x+glyphWidth*scale > b.Max.X {
			break
		}
		drawGlyph(img, glyph(r), x, b.Min.Y+pad, scale, ink)
		x += (glyphWidth + 1) * scale
	}
	return bandH
}

// scaleFor picks the largest whole scale at which the whole label fits across
// the image. Whole numbers only, so a glyph is never resampled and the result
// is identical on every machine.
func scaleFor(width, chars int) int {
	if chars <= 0 {
		return 0
	}
	needed := chars*(glyphWidth+1) + 2*pad
	for s := 4; s >= 1; s-- {
		if needed*s <= width {
			return s
		}
	}
	return 0
}

func drawGlyph(img draw.Image, g string, x, y, scale int, ink color.Color) {
	for row := 0; row < glyphHeight; row++ {
		for col := 0; col < glyphWidth; col++ {
			if g[row*glyphWidth+col] != '#' {
				continue
			}
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					img.Set(x+col*scale+dx, y+row*scale+dy, ink)
				}
			}
		}
	}
}

// Fits says whether a label would be drawn at all at this width. The caller
// needs to know before planning, so that a picture too small to carry one is
// reported rather than quietly unlabelled.
func Fits(width, chars int) bool {
	return scaleFor(width, chars) > 0
}
