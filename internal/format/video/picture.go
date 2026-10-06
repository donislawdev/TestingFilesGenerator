package video

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"slices"

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
// square at its step. It is drawn the way a film draws it, and a film draws
// its pictures through painters of its own, so this is for the probes and
// guards that measure any one of them.
func Picture(width, height int, seed uint64, label string, t Timeline, c int64) Planes {
	f := newFilm(width, height, seed, label, t)
	return f.painter().draw(f.lookOf(c))
}

// WholePicture is picture c painted whole - the gradient copied, the clock and
// the square drawn on the copy, and every pixel of it converted - which is how
// every picture of a film was painted until 2026-10-06. A film now paints only
// the rows that can change (painter.draw), and a guard holds the two to the
// same planes, because the bytes of every film depend on them being the same.
func WholePicture(width, height int, seed uint64, label string, t Timeline, c int64) Planes {
	f := newFilm(width, height, seed, label, t)
	return f.whole(f.lookOf(c))
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

// film is what every picture of one film has in common, made once and only
// read after: the gradient with the label burned in, the same gradient in
// planes, and where the clock and the square go. The painters of one film
// share it, which is what lets several of them paint at once (ahead.go).
type film struct {
	t          Timeline
	base       *image.RGBA
	basePlanes Planes
	planesBuf  []uint8 // what basePlanes lie over, copied whole by a painter
	clockFrom  int     // the row the clock's band starts at
	clockEnd   int     // the row under the band, where Draw stops painting it
	showClock  bool
	side       int
	squareY    int
	square     *image.Uniform
	// strips are the only rows a picture can differ from the gradient in -
	// the clock's band and the square's - each widened to whole pairs of
	// rows, because one chroma sample covers two, and merged where they meet.
	strips []image.Rectangle
}

func newFilm(width, height int, seed uint64, label string, t Timeline) *film {
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
	f := &film{
		t: t, base: base,
		clockFrom: labelBand(width, label),
		showClock: ClockShown(width, height, label, t),
		side:      min(max(width/squareDivisor, 1), maxSquareSide, height),
		square:    image.NewUniform(ink),
	}
	f.basePlanes, f.planesBuf = newPlanes(width, height)
	toPlanes(base, f.basePlanes)
	f.squareY = (height - f.side) * 3 / 4
	f.clockEnd = f.clockFrom
	if f.showClock {
		f.clockEnd = min(height, f.clockFrom+imagelabel.BandHeight(width, len(Clock(0, t))))
	}
	f.strips = changingRows(width, height, [][2]int{{f.clockFrom, f.clockEnd}, {f.squareY, f.squareY + f.side}})
	return f
}

// newPlanes is the three planes of a picture this size in one allocation,
// and that allocation, which a painter copies whole (film.painter).
func newPlanes(width, height int) (Planes, []uint8) {
	cw, ch := (width+1)/2, (height+1)/2
	buf := make([]uint8, width*height+2*cw*ch)
	return planesOver(buf, width, height), buf
}

// planesOver lays the three planes out over buf, luma first. Each is capped
// at its own length, so no plane can grow into the next.
func planesOver(buf []uint8, width, height int) Planes {
	n, c := width*height, ((width+1)/2)*((height+1)/2)
	return Planes{Width: width, Height: height, Y: buf[:n:n], U: buf[n : n+c : n+c], V: buf[n+c : n+2*c : n+2*c]}
}

// changingRows turns runs of rows into strips the width of the picture,
// each starting on an even row and ending on an even row or the last one, so
// that every chroma sample a strip converts covers only rows the strip holds,
// and merges the strips that overlap or touch. A run of no rows - the clock's,
// on a picture with no room for it - gives no strip.
func changingRows(width, height int, runs [][2]int) []image.Rectangle {
	var spans [][2]int
	for _, r := range runs {
		if r[0] < r[1] {
			spans = append(spans, [2]int{r[0] &^ 1, min(height, (r[1]+1)&^1)})
		}
	}
	slices.SortFunc(spans, func(a, b [2]int) int { return a[0] - b[0] })
	var out []image.Rectangle
	for _, s := range spans {
		if n := len(out); n > 0 && s[0] <= out[n-1].Max.Y {
			out[n-1].Max.Y = max(out[n-1].Max.Y, s[1])
			continue
		}
		out = append(out, image.Rect(0, s[0], width, s[1]))
	}
	return out
}

// look is what tells two pictures of one film apart: the clock's text when it
// is drawn, and where the square stands. Two changes with the same look are
// the same picture - a film too small for the clock whose square has nowhere
// to go - and the second one is not coded again.
type look struct {
	clock string
	x     int
}

func (f *film) lookOf(c int64) look {
	l := look{x: (f.base.Rect.Dx() - f.side) * int(c%squareSteps) / (squareSteps - 1)}
	if f.showClock {
		l.clock = Clock(c, f.t)
	}
	return l
}

// whole paints a picture the long way, into planes of its own: the whole
// gradient copied, the clock and the square drawn on the copy, every pixel
// converted. It is the reference painter.draw is held to (WholePicture).
func (f *film) whole(l look) Planes {
	work := image.NewRGBA(f.base.Rect)
	copy(work.Pix, f.base.Pix)
	if f.showClock {
		b := work.Rect
		imagelabel.Draw(work.SubImage(image.Rect(0, f.clockFrom, b.Dx(), b.Dy())).(*image.RGBA), l.clock)
	}
	draw.Draw(work, image.Rect(l.x, f.squareY, l.x+f.side, f.squareY+f.side), f.square, image.Point{}, draw.Src)
	planes, _ := newPlanes(f.base.Rect.Dx(), f.base.Rect.Dy())
	toPlanes(work, planes)
	return planes
}

// painter draws pictures of one film into planes of its own. They start as
// the gradient's, and every row outside the film's strips stays the
// gradient's in every picture, so a picture repaints and converts the strips
// and nothing else - at 1920x1080 the clock's band and the square's are about
// a sixth of the rows. One painter is used by one goroutine at a time.
type painter struct {
	f      *film
	planes Planes
	strips []image.RGBA
}

// painter is a painter of this film, in four allocations whatever the size:
// itself, its planes, its strips, and the pixels of all of them. Every helper
// coding a film makes one, and the allocations a file may cost are counted
// (the AllocCeiling of the formats built on this).
func (f *film) painter() *painter {
	w := f.base.Rect.Dx()
	stride := 4 * w
	rows := 0
	for _, r := range f.strips {
		rows += r.Dy()
	}
	pix := make([]uint8, rows*stride)
	p := &painter{f: f, planes: planesOver(slices.Clone(f.planesBuf), w, f.base.Rect.Dy()), strips: make([]image.RGBA, len(f.strips))}
	for i, r := range f.strips {
		n := r.Dy() * stride
		p.strips[i] = image.RGBA{Pix: pix[:n:n], Stride: stride, Rect: r}
		pix = pix[n:]
	}
	return p
}

// draw paints the picture with this look into the painter's planes and
// returns them. They are overwritten by the next call.
//
// Each strip is the gradient's rows copied, the clock drawn when its band
// starts in the strip, the square drawn where it crosses the strip, and the
// rows converted - the same steps, in the same order, as whole, on fewer
// rows. The clock's band always lies whole inside one strip, because the
// strips were cut around it, so Draw sizes it as it would on the whole
// picture.
func (p *painter) draw(l look) Planes {
	f := p.f
	w := f.base.Rect.Dx()
	for i := range p.strips {
		s := &p.strips[i]
		r := s.Rect
		copy(s.Pix, f.base.Pix[r.Min.Y*f.base.Stride:r.Max.Y*f.base.Stride])
		if f.showClock && r.Min.Y <= f.clockFrom && f.clockFrom < r.Max.Y {
			imagelabel.Draw(s.SubImage(image.Rect(0, f.clockFrom, w, r.Max.Y)).(*image.RGBA), l.clock)
		}
		draw.Draw(s, image.Rect(l.x, f.squareY, l.x+f.side, f.squareY+f.side), f.square, image.Point{}, draw.Src)
		convertRows(s, p.planes, r.Min.Y, r.Max.Y)
	}
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
func toPlanes(img *image.RGBA, p Planes) { convertRows(img, p, 0, p.Height) }

// convertRows converts rows r0 up to r1 of the picture p is, read from img -
// the whole picture, or a strip of it that starts at row r0. r0 is even and r1
// is even or the picture's height, so every chroma sample written here covers
// rows img holds.
func convertRows(img *image.RGBA, p Planes, r0, r1 int) {
	w, h := p.Width, p.Height
	top := img.Rect.Min.Y
	for y := r0; y < r1; y++ {
		row := img.Pix[(y-top)*img.Stride:]
		for x := range w {
			r, g, b := int(row[4*x]), int(row[4*x+1]), int(row[4*x+2])
			p.Y[y*w+x] = uint8(16 + (47*r+157*g+16*b+128)>>8)
		}
	}
	cw := (w + 1) / 2
	for cy := r0 / 2; cy < (r1+1)/2; cy++ {
		for cx := range cw {
			p.U[cy*cw+cx], p.V[cy*cw+cx] = chroma(img, 2*cx, 2*cy, w, h)
		}
	}
}

// chroma is the Cb and Cr of the two by two block whose top left is x0, y0,
// over the pixels of it that are inside a picture w by h.
func chroma(img *image.RGBA, x0, y0, w, h int) (cb, cr uint8) {
	sumB, sumR, n := 0, 0, 0
	top := img.Rect.Min.Y
	for y := y0; y < min(y0+2, h); y++ {
		row := img.Pix[(y-top)*img.Stride:]
		for x := x0; x < min(x0+2, w); x++ {
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
