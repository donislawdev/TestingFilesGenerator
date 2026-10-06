package video

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"

	"github.com/donislawdev/TestingFilesGenerator/internal/format/imagelabel"
)

// Planes is a picture in the form the encoder takes: 8 bit luma, and two
// chroma planes at half the resolution each way, rounded up.
type Planes struct {
	Width, Height int
	Y, U, V       []uint8
}

// The square that steps across the picture is the GIF animation's marker -
// an eighth of the width, never under a pixel, never over 128 - so the two
// formats that move move the same way. squareSteps is how many places it
// takes: at the default change a second it crosses the picture in ten
// seconds, once in a default film.
const (
	squareDivisor = 8
	maxSquareSide = 128
	squareSteps   = 10
)

// ink is the colour the square is drawn in, the label's own.
var ink = color.RGBA{R: 240, G: 240, B: 240, A: 255}

// Picture draws picture c of a film: the gradient every image format here
// draws, moved by the seed, with the label burned into the top of it when
// there is room, the clock under it reading when the picture starts, and the
// square at its step. A film draws its pictures through one painter, so this
// is for planning, which codes the first, and for the probes and guards that
// measure any one of them.
func Picture(width, height int, seed uint64, label string, t Timeline, c int64) Planes {
	return newPainter(width, height, seed, label, t).paint(c)
}

// Labelled says whether a picture this wide carries a readable label - asked
// while planning, to report a film without one, and while drawing, so the
// manifest cannot claim a label the picture lacks.
func Labelled(width int, label string) bool {
	return label != "" && imagelabel.Fits(width, len(label))
}

// ClockShown says whether a picture of this size shows the clock - under the
// label when the label is there, at the top when it is not, and only when the
// whole of it fits across and there is height left below the label for it.
func ClockShown(width, height int, label string, t Timeline) bool {
	return imagelabel.Fits(width, len(Clock(0, t))) && labelBand(width, label) < height
}

func labelBand(width int, label string) int {
	if !Labelled(width, label) {
		return 0
	}
	return imagelabel.BandHeight(width, len(label))
}

// Clock is what the clock in picture c reads: when the picture starts, as a
// player's position shows it - hours, minutes and seconds, and milliseconds
// only when the changes do not fall on whole seconds, by the owner's decision
// of 2026-10-06 (docs/WIDEO-2026-10-06.md section 15). Every picture of a film
// reads the same number of characters, so a clock that fits the first fits
// them all.
func Clock(c int64, t Timeline) string {
	ms := c * t.ChangeMs()
	hms := fmt.Sprintf("%02d:%02d:%02d", ms/3_600_000, ms/60_000%60, ms/1000%60)
	if t.ChangeMs()%1000 == 0 {
		return hms
	}
	return fmt.Sprintf("%s.%03d", hms, ms%1000)
}

// painter draws the pictures of one film. The gradient and the label are the
// same in every one of them, so they are drawn once, and each change copies
// them and adds only what moves - the clock and the square.
type painter struct {
	t         Timeline
	base      *image.RGBA
	work      *image.RGBA
	planes    Planes
	clockFrom int // the row the clock's band starts at
	showClock bool
	side      int
	square    *image.Uniform
}

func newPainter(width, height int, seed uint64, label string, t Timeline) *painter {
	off := int(seed % 256)
	base := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		row := base.Pix[y*base.Stride:]
		for x := range width {
			row[4*x], row[4*x+1], row[4*x+2], row[4*x+3] = uint8((x+off)%256), uint8((y+off)%256), uint8((x+y+off)%256), 255
		}
	}
	if Labelled(width, label) {
		imagelabel.Draw(base, label)
	}
	cw, ch := (width+1)/2, (height+1)/2
	return &painter{
		t: t, base: base, work: image.NewRGBA(base.Rect),
		planes:    Planes{Width: width, Height: height, Y: make([]uint8, width*height), U: make([]uint8, cw*ch), V: make([]uint8, cw*ch)},
		clockFrom: labelBand(width, label),
		showClock: ClockShown(width, height, label, t),
		side:      min(max(width/squareDivisor, 1), maxSquareSide, height),
		square:    image.NewUniform(ink),
	}
}

// look is what tells two pictures of one film apart: the clock's text when it
// is drawn, and where the square stands. Two changes with the same look are
// the same picture - a film too small for the clock whose square has nowhere
// to go - and the second one is not coded again.
type look struct {
	clock string
	x     int
}

func (p *painter) lookOf(c int64) look {
	l := look{x: (p.base.Rect.Dx() - p.side) * int(c%squareSteps) / (squareSteps - 1)}
	if p.showClock {
		l.clock = Clock(c, p.t)
	}
	return l
}

// paint draws picture c into the painter's own planes and returns them. They
// are overwritten by the next call.
func (p *painter) paint(c int64) Planes { return p.draw(p.lookOf(c)) }

// draw is paint for a picture whose look is already worked out.
func (p *painter) draw(l look) Planes {
	copy(p.work.Pix, p.base.Pix)
	if p.showClock {
		b := p.work.Rect
		imagelabel.Draw(p.work.SubImage(image.Rect(0, p.clockFrom, b.Dx(), b.Dy())).(*image.RGBA), l.clock)
	}
	y := (p.base.Rect.Dy() - p.side) * 3 / 4
	draw.Draw(p.work, image.Rect(l.x, y, l.x+p.side, y+p.side), p.square, image.Point{}, draw.Src)
	toPlanes(p.work, p.planes)
	return p.planes
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
func toPlanes(img *image.RGBA, p Planes) {
	w, h := img.Rect.Dx(), img.Rect.Dy()
	cw, ch := (w+1)/2, (h+1)/2
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
