// The pixels: what an AVIF from this tool actually shows.
//
// Unlike the formats this project codes by hand, the whole picture is built in
// memory before anything is encoded, because the encoder takes an image rather
// than a row at a time. That is what the megapixel limit in avif.go is for -
// the picture is bounded, so the memory is bounded, and the file size above it
// is carried by padding that streams.
package avif

import (
	"image"
	"image/color"

	"github.com/donislawdev/TestingFilesGenerator/internal/format/imagelabel"
)

// picture builds the image for one file: a gradient that moves with the seed,
// and the label burned into the top of it when there is room.
func picture(m memo) *image.RGBA {
	off := int(m.seed % 256)
	img := image.NewRGBA(image.Rect(0, 0, m.width, m.height))

	for y := 0; y < m.height; y++ {
		for x := 0; x < m.width; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8((x + off) % 256),
				G: uint8((y + off) % 256),
				B: uint8((x + y + off) % 256),
				A: 255,
			})
		}
	}

	if labelled(m.width, m.label) {
		imagelabel.Draw(img, m.label)
	}
	return img
}

// labelled says whether this picture carries a readable label, and is asked
// both while planning, to report a file that will not have one, and while
// drawing. One answer, so the manifest cannot claim a label the picture lacks.
func labelled(width int, label string) bool {
	return label != "" && imagelabel.Fits(width, len(label))
}
