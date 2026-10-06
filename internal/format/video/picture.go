package video

import (
	"image"
	"image/color"

	"github.com/donislawdev/TestingFilesGenerator/internal/format/imagelabel"
)

// Planes is a picture in the form the encoder takes: 8 bit luma, and two
// chroma planes at half the resolution each way, rounded up.
type Planes struct {
	Width, Height int
	Y, U, V       []uint8
}

// Picture draws the one picture a film from this tool shows: the gradient
// every image format here draws, moved by the seed, with the label burned into
// the top of it when there is room.
//
// One picture for the whole film, by the owner's decision of 2026-10-06
// (docs/WIDEO-2026-10-06.md section 12): it is coded once, so planning stays
// arithmetic and a run of ten thousand films costs ten thousand encodings
// rather than ten thousand times the number of key frames.
func Picture(width, height int, seed uint64, label string) Planes {
	off := int(seed % 256)
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8((x + off) % 256),
				G: uint8((y + off) % 256),
				B: uint8((x + y + off) % 256),
				A: 255,
			})
		}
	}
	if Labelled(width, label) {
		imagelabel.Draw(img, label)
	}
	return toPlanes(img)
}

// Labelled says whether a picture this wide carries a readable label - asked
// while planning, to report a film without one, and while drawing, so the
// manifest cannot claim a label the picture lacks.
func Labelled(width int, label string) bool {
	return label != "" && imagelabel.Fits(width, len(label))
}

// toPlanes converts to BT.709 studio range in whole numbers.
//
// Whole numbers rather than the floats gav1d converts AVIF pictures with, and
// that is about the byte stability contract (D11): this project has measured
// arm64 rounding a float differently from amd64 (O120), and a film whose bytes
// depend on the machine that made it breaks the one promise a seed makes. The
// coefficients are BT.709's scaled by 256 and rounded, with each chroma row
// adjusted to sum to zero so a grey stays exactly grey:
//
//	Y  = 16  + ( 47 R + 157 G +  16 B) / 256
//	Cb = 128 + (-26 R -  86 G + 112 B) / 256
//	Cr = 128 + (112 R - 102 G -  10 B) / 256
//
// Chroma is the rounded mean of the two by two block it covers, so an odd
// width or height averages the pixels that are there.
func toPlanes(img *image.RGBA) Planes {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	cw, ch := (w+1)/2, (h+1)/2
	p := Planes{Width: w, Height: h, Y: make([]uint8, w*h), U: make([]uint8, cw*ch), V: make([]uint8, cw*ch)}

	for y := range h {
		row := img.Pix[y*img.Stride:]
		for x := range w {
			r, g, b := int(row[4*x]), int(row[4*x+1]), int(row[4*x+2])
			p.Y[y*w+x] = uint8(16 + (47*r+157*g+16*b+128)>>8)
		}
	}
	for cy := range ch {
		for cx := range cw {
			p.U[cy*cw+cx], p.V[cy*cw+cx] = chroma(img, 2*cx, 2*cy)
		}
	}
	return p
}

// chroma is the Cb and Cr of the two by two block whose top left is x0, y0,
// over the pixels of it that are inside the picture.
func chroma(img *image.RGBA, x0, y0 int) (cb, cr uint8) {
	sumB, sumR, n := 0, 0, 0
	for y := y0; y < min(y0+2, img.Rect.Dy()); y++ {
		row := img.Pix[y*img.Stride:]
		for x := x0; x < min(x0+2, img.Rect.Dx()); x++ {
			r, g, b := int(row[4*x]), int(row[4*x+1]), int(row[4*x+2])
			sumB += -26*r - 86*g + 112*b
			sumR += 112*r - 102*g - 10*b
			n++
		}
	}
	return uint8(128 + roundedMean(sumB, n*256)), uint8(128 + roundedMean(sumR, n*256))
}

// roundedMean is sum divided by n, rounded half away from zero, so a block
// and its mirror image come out the same distance from 128.
func roundedMean(sum, n int) int {
	if sum < 0 {
		return -((-sum + n/2) / n)
	}
	return (sum + n/2) / n
}
