// The pixels: what the picture shows and how a row of it reaches the coder.
//
// Split out of webp.go on 2026-08-29, when the format crossed the line count
// this project crowds against. The three files are three jobs - the container
// and the size arithmetic, this one, and the bitstream in vp8l.go.
package webp

import (
	"context"
	"image"
	"image/color"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/format/imagelabel"
)

// writePicture codes the picture straight into w and reports how many bytes it
// handed over.
//
// Nothing but one row and the label band is held: the bit writer drains into w
// as it fills, so a request for gigabytes costs the same memory as a small one.
func writePicture(ctx context.Context, w io.Writer, m memo) (int64, error) {
	off := int(m.seed % 256)
	band := labelBand(m, off)
	bandH := 0
	if band != nil {
		bandH = band.Bounds().Dy()
	}

	row := make([]byte, m.width*samplesPerPixel)
	bw := newBitWriter(w)
	var stopped error
	writeStream(bw, m.width, m.height, func(y int) []byte {
		if y%64 == 0 && stopped == nil {
			stopped = interrupted(ctx)
		}
		if y < bandH {
			copyBandRow(row, band, y, m.width)
		} else {
			fillRow(row, y, m.width, off)
		}
		return row
	})
	written, err := bw.flush()
	if stopped != nil {
		return written, stopped
	}
	return written, err
}

// interrupted reports a cancelled run, and exists so the row callback stays two
// levels deep rather than three. This project counts how many functions nest
// that far and the count is meant to fall.
func interrupted(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func fillRow(row []byte, y, width, off int) {
	for x := 0; x < width; x++ {
		row[x*samplesPerPixel] = uint8((x + off) % 256)
		row[x*samplesPerPixel+1] = uint8((y + off) % 256)
		row[x*samplesPerPixel+2] = uint8((x + y + off) % 256)
	}
}

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

func copyBandRow(row []byte, band *image.RGBA, y, width int) {
	base := band.PixOffset(0, y)
	for x := 0; x < width; x++ {
		p := base + x*4
		row[x*samplesPerPixel] = band.Pix[p]
		row[x*samplesPerPixel+1] = band.Pix[p+1]
		row[x*samplesPerPixel+2] = band.Pix[p+2]
	}
}
