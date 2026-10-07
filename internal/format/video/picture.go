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
// square at its step. It is put together the way a film codes it, tile by
// tile, each from where the film takes it (painter.source), so this is for
// the probes and guards that measure any one of them.
func Picture(width, height int, seed uint64, label string, t Timeline, c int64) Planes {
	f := newFilm(width, height, seed, label, t)
	ks := newKeyer(f.geometry, gridFor(f.geometry, t.FPS))
	p, l := f.painter(ks), f.lookOf(c)
	out, _ := newPlanes(width, height)
	for _, tile := range ks.tiles {
		src, at := p.source(l, tile.rect)
		copyRect(out, tile.rect.Min, src, at)
	}
	return out
}

// WholePicture is picture c painted whole - the gradient copied, the clock and
// the square drawn on the copy, and every pixel of it converted - which is how
// every picture of a film was painted until 2026-10-06. A film now paints only
// the tiles that can change, and only their rows that can (painter.source), and
// a guard holds the two to the same planes, because the bytes of every film
// depend on them being the same.
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

// geometry is where the pictures of a film can differ from one another: the
// clock's band and where each of its characters stands, and the square's
// band. Worked out from the size, the label and the timeline without painting
// anything, because planning needs it to cut the picture into tiles before
// any picture exists (grid.go), and a film paints by the same numbers.
type geometry struct {
	width, height int
	clockFrom     int // the row the clock's band starts at
	clockEnd      int // the row under the band, where Draw stops painting it
	clockChars    int // nought when the picture shows no clock
	cells         imagelabel.Cells
	side          int
	squareY       int
}

func geometryOf(width, height int, label string, t Timeline) geometry {
	g := geometry{width: width, height: height, clockFrom: labelBand(width, label)}
	g.clockEnd = g.clockFrom
	if ClockShown(width, height, label, t) {
		g.clockChars = len(Clock(0, t))
		g.clockEnd = min(height, g.clockFrom+imagelabel.BandHeight(width, g.clockChars))
		g.cells = imagelabel.CellsOf(width, g.clockChars)
	}
	g.side = min(max(width/squareDivisor, 1), maxSquareSide, height)
	g.squareY = (height - g.side) * 3 / 4
	return g
}

func (g geometry) showClock() bool { return g.clockChars > 0 }

// squareX is where the square stands in picture c.
func (g geometry) squareX(c int64) int {
	return (g.width - g.side) * int(c%squareSteps) / (squareSteps - 1)
}

// clockRight is the column after the clock's last character, nought without
// a clock.
func (g geometry) clockRight() int {
	if !g.showClock() {
		return 0
	}
	return g.cells.First + (g.clockChars-1)*g.cells.Step + g.cells.Width
}

// film is what every picture of one film has in common, made once and only
// read after: the gradient with the label burned in, in planes, the same
// gradient in pixels along the rows a picture can change in, and where the
// clock and the square go. The painters of one film share it, which is what
// lets several of them paint at once (ahead.go).
//
// The gradient is kept in pixels along the strips and nowhere else, and a
// painter has room for one tile and nothing else, because the whole picture
// in each was what a film's memory was: at 7680x4320, 133 MB of pixels the
// painters read a sixth of, and 50 MB of planes copied to every one of
// fifteen helpers - 1.3 GB at the peak and a live heap of 1028 MB, against
// 236 MB on one thread. Without them the same film peaks at 187 to 241 MB, and
// sixteen such films at once at 1.1 GB where they took 3.8 (2026-10-07, three
// rounds each, docs/WEBM-LIMIT-2026-10-07.md section 11).
type film struct {
	geometry
	t          Timeline
	seed       uint64
	label      string
	basePlanes Planes
	square     *image.Uniform
	// strips are the only rows a picture can differ from the gradient in -
	// the clock's band and the square's - each widened to whole pairs of
	// rows, because one chroma sample covers two, and merged where they meet.
	// baseStrips are the gradient and the label along each of them, which a
	// painter starts every picture's strip from.
	strips     []image.Rectangle
	baseStrips []image.RGBA
}

func newFilm(width, height int, seed uint64, label string, t Timeline) *film {
	f := &film{geometry: geometryOf(width, height, label, t), t: t, seed: seed, label: label, square: image.NewUniform(ink)}
	f.strips = changingRows(width, height, [][2]int{{f.clockFrom, f.clockEnd}, {f.squareY, f.squareY + f.side}})
	f.baseStrips = stripsOver(f.strips, width)
	g := newGround(width, height, seed, label)
	for i := range f.baseStrips {
		g.fill(&f.baseStrips[i])
	}
	f.basePlanes, _ = newPlanes(width, height)
	g.fillPlanes(f.basePlanes)
	return f
}

// ground makes the picture every picture of a film starts from - the
// gradient, moved by the seed, with the label burned into the top - a band of
// rows at a time, so the film never holds it whole: at 7680x4320 that whole
// was 133 MB that every film of a run made at once held at its start. The
// label's band is the one part that is not a sum of the row and the column,
// and Draw paints it solid before the text, so it is drawn once on a band of
// its own and copied - the pixels Draw makes on the whole picture, which the
// reference painter (gradient, whole) still draws whole and a guard compares.
type ground struct {
	off   int
	label *image.RGBA
}

func newGround(width, height int, seed uint64, label string) ground {
	g := ground{off: int(seed % 256), label: &image.RGBA{}}
	if band := labelBand(width, label); band > 0 {
		g.label = image.NewRGBA(image.Rect(0, 0, width, min(height, band)))
		imagelabel.Draw(g.label, label)
	}
	return g
}

// fill paints the ground's rows into dst, which is the width of the picture.
func (g ground) fill(dst *image.RGBA) {
	for y := dst.Rect.Min.Y; y < dst.Rect.Max.Y; y++ {
		row := dst.Pix[(y-dst.Rect.Min.Y)*dst.Stride:][:dst.Stride]
		if y < g.label.Rect.Max.Y {
			copy(row, g.label.Pix[y*g.label.Stride:])
			continue
		}
		for x := range dst.Rect.Dx() {
			row[4*x], row[4*x+1], row[4*x+2], row[4*x+3] = uint8((x+g.off)%256), uint8((y+g.off)%256), uint8((x+y+g.off)%256), 255
		}
	}
}

// fillPlanes converts the whole ground into p, sixty four rows at a time - an
// even number, so no chroma sample straddles two bands.
func (g ground) fillPlanes(p Planes) {
	stride := 4 * p.Width
	pix := make([]uint8, min(p.Height, superblock)*stride)
	for y := 0; y < p.Height; y += superblock {
		r := image.Rect(0, y, p.Width, min(p.Height, y+superblock))
		band := &image.RGBA{Pix: pix[:r.Dy()*stride], Stride: stride, Rect: r}
		g.fill(band)
		convertRect(band, p, image.Point{}, r)
	}
}

// gradient is the ground drawn whole, the way every picture of a film was
// until 2026-10-07, for the reference painter only (whole).
func gradient(width, height int, seed uint64, label string) *image.RGBA {
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
	return base
}

// stripsOver is a picture's strips in pixels, the width of the picture, all
// of them over one allocation.
func stripsOver(strips []image.Rectangle, width int) []image.RGBA {
	stride := 4 * width
	rows := 0
	for _, r := range strips {
		rows += r.Dy()
	}
	pix := make([]uint8, rows*stride)
	out := make([]image.RGBA, len(strips))
	for i, r := range strips {
		n := r.Dy() * stride
		out[i] = image.RGBA{Pix: pix[:n:n], Stride: stride, Rect: r}
		pix = pix[n:]
	}
	return out
}

// changes says whether a picture can differ from the gradient anywhere in r.
func (f *film) changes(r image.Rectangle) bool {
	for _, s := range f.strips {
		if s.Overlaps(r) {
			return true
		}
	}
	return false
}

// newPlanes is the three planes of a picture this size in one allocation,
// and that allocation.
func newPlanes(width, height int) (Planes, []uint8) {
	buf := make([]uint8, planesSize(width, height))
	return planesOver(buf, width, height), buf
}

// planesSize is the bytes of the three planes of a picture this size.
func planesSize(width, height int) int {
	return width*height + 2*((width+1)/2)*((height+1)/2)
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

func (f *film) lookOf(c int64) look { return lookAt(f.geometry, f.t, c) }

// lookAt is the look of picture c, from the film's geometry alone - planning
// walks the looks of a whole film without painting it (Stream.Work).
func lookAt(g geometry, t Timeline, c int64) look {
	l := look{x: g.squareX(c)}
	if g.showClock() {
		l.clock = Clock(c, t)
	}
	return l
}

// whole paints a picture the long way, into planes of its own: the whole
// gradient drawn again, the clock and the square drawn on it, every pixel
// converted. It is the reference painter.source is held to (WholePicture).
func (f *film) whole(l look) Planes {
	work := gradient(f.width, f.height, f.seed, f.label)
	if f.showClock() {
		b := work.Rect
		imagelabel.Draw(work.SubImage(image.Rect(0, f.clockFrom, b.Dx(), b.Dy())).(*image.RGBA), l.clock)
	}
	draw.Draw(work, image.Rect(l.x, f.squareY, l.x+f.side, f.squareY+f.side), f.square, image.Point{}, draw.Src)
	planes, _ := newPlanes(f.width, f.height)
	toPlanes(work, planes)
	return planes
}

// painter draws the tiles of one film's pictures that can change into planes
// of its own, one tile at a time. Every row outside the film's strips stays
// the gradient's in every picture, so a tile is the film's planes copied and
// its strips repainted and converted over them - at 1920x1080 the clock's band
// and the square's are about a sixth of the rows. One painter is used by one
// goroutine at a time.
type painter struct {
	f      *film
	tile   []uint8 // the planes of the largest tile that can change
	strips []image.RGBA
}

// painter is a painter of this film's tiles, in four allocations whatever the
// size: itself, its tile, its strips, and the pixels of all of them. Every
// helper coding a film makes one, and the allocations a file may cost are
// counted (the AllocCeiling of the formats built on this).
func (f *film) painter(ks keyer) *painter {
	most := 0
	for _, t := range ks.tiles {
		if f.changes(t.rect) {
			most = max(most, planesSize(t.rect.Dx(), t.rect.Dy()))
		}
	}
	return &painter{f: f, tile: make([]uint8, most), strips: stripsOver(f.strips, f.width)}
}

// source is the planes tile r of the picture with look l is coded from, and
// where r lies in them: the film's own planes for a tile no picture changes,
// read by every goroutine coding the film and written by none, and for a tile
// that can change, the painter's, overwritten by its next call.
func (p *painter) source(l look, r image.Rectangle) (Planes, image.Rectangle) {
	if !p.f.changes(r) {
		return p.f.basePlanes, r
	}
	return p.drawIn(l, r), image.Rectangle{Max: r.Size()}
}

// drawIn paints tile r of the picture with this look into the painter's planes
// of the tile's size: the film's planes there copied, and each strip crossing
// r the gradient's rows copied, the clock drawn when its band starts in the
// strip, the square drawn where it crosses the strip, and the part of it inside
// r converted - the same steps, in the same order, as whole, on fewer pixels.
// r starts on an even row and an even column, as every tile does.
//
// The strip's rows are copied across the whole width, because the clock's
// characters are placed from the picture's left edge. The clock's band always
// lies whole inside one strip, because the strips were cut around it, so Draw
// sizes it as it would on the whole picture.
func (p *painter) drawIn(l look, r image.Rectangle) Planes {
	f := p.f
	out := planesOver(p.tile, r.Dx(), r.Dy())
	copyRect(out, image.Point{}, f.basePlanes, r)
	for i := range p.strips {
		s, base := &p.strips[i], &f.baseStrips[i]
		in := s.Rect.Intersect(r)
		if in.Empty() {
			continue
		}
		from, to := (in.Min.Y-s.Rect.Min.Y)*s.Stride, (in.Max.Y-s.Rect.Min.Y)*s.Stride
		copy(s.Pix[from:to], base.Pix[from:to])
		if f.showClock() && s.Rect.Min.Y <= f.clockFrom && f.clockFrom < s.Rect.Max.Y {
			imagelabel.Draw(s.SubImage(image.Rect(0, f.clockFrom, f.width, s.Rect.Max.Y)).(*image.RGBA), l.clock)
		}
		draw.Draw(s, image.Rect(l.x, f.squareY, l.x+f.side, f.squareY+f.side), f.square, image.Point{}, draw.Src)
		convertRect(s, out, r.Min, in)
	}
	return out
}

// copyRect copies the part r of src into dst with its top left at at. Both
// start on an even row and column, so the chroma samples r covers are whole
// samples of both.
func copyRect(dst Planes, at image.Point, src Planes, r image.Rectangle) {
	for y := r.Min.Y; y < r.Max.Y; y++ {
		copy(dst.Y[(at.Y+y-r.Min.Y)*dst.Width+at.X:][:r.Dx()], src.Y[y*src.Width+r.Min.X:])
	}
	scw, dcw, cw := (src.Width+1)/2, (dst.Width+1)/2, (r.Dx()+1)/2
	for cy := r.Min.Y / 2; cy < (r.Max.Y+1)/2; cy++ {
		d, s := (at.Y/2+cy-r.Min.Y/2)*dcw+at.X/2, cy*scw+r.Min.X/2
		copy(dst.U[d:][:cw], src.U[s:])
		copy(dst.V[d:][:cw], src.V[s:])
	}
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
func toPlanes(img *image.RGBA, p Planes) { convertRect(img, p, image.Point{}, img.Rect) }

// convertRect converts the pixels inside r of a picture, read from img - the
// whole picture, or a strip of it whose rows hold r's - into p, the planes of
// the part of the picture whose top left is at: the whole of it, or one tile.
// r starts on an even row and column and ends on even ones or the picture's
// edge, so every chroma sample written here covers pixels inside r. p ends
// where the picture does or on a multiple of 64, so a two by two block cut at
// p's edge is cut at the picture's.
func convertRect(img *image.RGBA, p Planes, at image.Point, r image.Rectangle) {
	w := p.Width
	top := img.Rect.Min.Y
	for y := r.Min.Y; y < r.Max.Y; y++ {
		row := img.Pix[(y-top)*img.Stride:]
		for x := r.Min.X; x < r.Max.X; x++ {
			red, g, b := int(row[4*x]), int(row[4*x+1]), int(row[4*x+2])
			p.Y[(y-at.Y)*w+x-at.X] = uint8(16 + (47*red+157*g+16*b+128)>>8)
		}
	}
	cw := (w + 1) / 2
	for cy := r.Min.Y / 2; cy < (r.Max.Y+1)/2; cy++ {
		for cx := r.Min.X / 2; cx < (r.Max.X+1)/2; cx++ {
			i := (cy-at.Y/2)*cw + cx - at.X/2
			p.U[i], p.V[i] = chroma(img, 2*cx, 2*cy, at.X+w, at.Y+p.Height)
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
