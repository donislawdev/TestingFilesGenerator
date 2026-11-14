package tiff

import (
	"context"
	"image"
	"image/color"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/format/imagelabel"
)

// writePixels streams the picture a row at a time.
//
// Only the label band is ever built as an image, and it is at most 24 rows
// tall whatever the picture is. Everything else comes from the formula
// straight into one reused row buffer, so a 64 MiB picture costs a row rather
// than 64 MiB.
//
// A TIFF stores its rows top down, so the label band is written first - the
// one place this is simpler than BMP, which stores them the other way up.
func writePixels(ctx context.Context, w io.Writer, m memo) error {
	row := make([]byte, m.width*samplesPerPixel)
	off := int(m.seed % 256)

	band := labelBand(m, off)
	bandH := 0
	if band != nil {
		bandH = band.Bounds().Dy()
	}

	for y := 0; y < m.height; y++ {
		if y%64 == 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}
		if y < bandH {
			copyBandRow(row, band, y, m.width)
		} else {
			fillRow(row, y, m.width, off)
		}
		if _, err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}

// fillRow writes one row of the gradient in the order a TIFF wants it: red,
// green, blue.
func fillRow(row []byte, y, width, off int) {
	for x := 0; x < width; x++ {
		row[x*3] = uint8((x + off) % 256)
		row[x*3+1] = uint8((y + off) % 256)
		row[x*3+2] = uint8((x + y + off) % 256)
	}
}

func copyBandRow(row []byte, band *image.RGBA, y, width int) {
	for x := 0; x < width; x++ {
		i := band.PixOffset(x, y)
		row[x*3] = band.Pix[i]
		row[x*3+1] = band.Pix[i+1]
		row[x*3+2] = band.Pix[i+2]
	}
}

// labelBand rasterises the top strip of the picture, label included, or
// returns nil when there is no label to draw.
func labelBand(m memo, off int) *image.RGBA {
	if m.label == "" || !imagelabel.Fits(m.width, len(m.label)) {
		return nil
	}
	h := maxBandHeight
	if m.height < h {
		h = m.height
	}
	img := image.NewRGBA(image.Rect(0, 0, m.width, h))
	for y := 0; y < h; y++ {
		for x := 0; x < m.width; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8((x + off) % 256),
				G: uint8((y + off) % 256),
				B: uint8((x + y + off) % 256),
				A: 255,
			})
		}
	}
	imagelabel.Draw(img, m.label)
	return img
}
