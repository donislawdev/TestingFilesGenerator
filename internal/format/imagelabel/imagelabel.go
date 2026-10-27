// Package imagelabel draws the self describing label into pixels.
//
// The image formats and the films burn the same label, so they share one
// rasteriser rather than carrying a copy each.
package imagelabel

import (
	"image"
	"image/color"
	"image/draw"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
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
//
// It decides whether Draw is called, and only that. Whether the label can be
// READ is Readable, because a label is drawn by its width and can still be cut
// off at the foot of a low picture.
func Fits(width, chars int) bool {
	return scaleFor(width, chars) > 0
}

// Readable says whether a label drawn across width is whole in a picture of
// this height - every row of every glyph inside it. The padding below the text
// may be cut, the text itself may not.
//
// What the manifest says of a picture is this, not Fits. Until 2026-10-27 it
// was Fits, and a picture 512 px wide and 1 px tall said it carried its label
// with not one pixel of text in it (docs/STABILITY-REVIEW-2026-10-26.md, R14).
// Draw is unchanged, so the pixels of such a picture are what they were - the
// owner's decision, because they are bytes somebody's suite has a hash of
// (D11) - and only what is said about them moved.
func Readable(width, height, chars int) bool {
	return Fits(width, chars) && height >= textHeight(width, chars)
}

// Claim is what a plan says about a label: drawn is whether it is drawn at
// all, which the caller decides with Fits and reports in its own words when it
// is not. A label drawn and cut off at the foot of the picture is not carried
// either, and the note saying so goes after the notes given.
//
// One place for the eleven formats that draw the label, so the sentence and
// the answer cannot come apart between them.
func Claim(drawn bool, width, height, chars int, notes []format.Note) (bool, []format.Note) {
	if !drawn || Readable(width, height, chars) {
		return drawn, notes
	}
	return false, append(notes, format.Note{
		Code: "label_omitted",
		Detail: core.Says("imagelabel.ThePictureIsPxTallAnd", "The picture is %d px tall and the label needs %d px, so its text is cut off and this file carries no readable label. Its name and the manifest still identify it.",
			core.A("Height", height), core.A("Needs", textHeight(width, chars))),
	})
}

// textHeight is how many rows from the top the text of a label reaches.
func textHeight(width, chars int) int {
	return pad + glyphHeight*scaleFor(width, chars)
}

// Cells is where Draw puts the characters of a text: character k covers the
// columns from First + k*Step, Width of them, and every row of the band. A
// film asks it to know which characters of its clock fall in which part of
// the picture, so it is Draw's own arithmetic rather than a copy of it.
type Cells struct {
	First, Step, Width int
}

// CellsOf is where Draw puts a text of chars characters across width - the
// zero Cells when Draw would draw nothing.
func CellsOf(width, chars int) Cells {
	s := scaleFor(width, chars)
	if s == 0 {
		return Cells{}
	}
	return Cells{First: pad, Step: (glyphWidth + 1) * s, Width: glyphWidth * s}
}

// BandHeight is how tall the band Draw paints for a label of chars characters
// across width is, before it is cut to the picture - 0 when the label does not
// fit. A film draws a second line under the first and asks this before it
// draws, so planning can say whether that line will be there.
func BandHeight(width, chars int) int {
	s := scaleFor(width, chars)
	if s == 0 {
		return 0
	}
	return glyphHeight*s + 2*pad
}
