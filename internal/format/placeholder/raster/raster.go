// Package raster turns outlines into how much of each pixel they cover, with
// whole numbers only.
//
// Whole numbers because the pixels are bytes somebody holds a hash of (D11).
// Go lets a compiler fuse a multiplication and an addition into one
// instruction where the processor has one, and arm64 has one, so the same
// floating point rasteriser can give a Mac different pixels from a PC. Here
// every step is an integer operation the language defines exactly, so a
// letter is the same pixels on every system (docs/PLACEHOLDER-2026-11-14.md
// section 5).
//
// How it covers a pixel. Each row of pixels is sampled along Rows lines, and
// along each line the inside of the outline is measured to a 1/One of a pixel,
// by the non-zero winding rule. A pixel's coverage is the share of it those
// spans cover. Memory is one row of the outline's width, whatever its height,
// so a line of text across a 20000 pixel picture costs a row, not a picture.
package raster

import "slices"

const (
	// Frac is how many bits of a coordinate are a fraction of a pixel.
	Frac = 8
	// One is one pixel in coordinates.
	One = 1 << Frac
	// Rows is how many lines each row of pixels is sampled along.
	Rows = 16

	full = Rows * One
)

type edge struct {
	x0, y0, x1, y1 int64
	dir            int
}

// Path is an outline in coordinates: pixels times One, y growing downwards.
// Contours close themselves - a MoveTo or the end of the path joins the last
// point back to the first.
type Path struct {
	edges []edge
	// segs is the outline as it was drawn, curves and all, for a format that
	// writes shapes rather than pixels (Segments).
	segs           []Segment
	startX, startY int64
	curX, curY     int64
	open           bool
	minX, minY     int64
	maxX, maxY     int64
	empty          bool
}

// NewPath is an empty outline.
func NewPath() *Path { return &Path{empty: true} }

// Op is what one segment of an outline does.
type Op uint8

// The four things an outline does, as an SVG path names them.
const (
	OpMove Op = iota
	OpLine
	OpQuad
	OpClose
)

// Segment is one step of an outline as it was drawn: to X, Y, through CX, CY
// for a curve.
type Segment struct {
	Op           Op
	CX, CY, X, Y int64
}

// Segments is the outline as it was drawn, every contour ending in OpClose. The
// curves are the curves rather than the lines Fill covers, so a drawing in
// vectors keeps them smooth at any scale.
func (p *Path) Segments() []Segment {
	p.Close()
	return p.segs
}

// MoveTo starts a contour at x, y.
func (p *Path) MoveTo(x, y int64) {
	p.Close()
	p.segs = append(p.segs, Segment{Op: OpMove, X: x, Y: y})
	p.startX, p.startY, p.curX, p.curY = x, y, x, y
	p.open = true
	p.grow(x, y)
}

// LineTo draws a straight line from the current point to x, y.
func (p *Path) LineTo(x, y int64) {
	p.segs = append(p.segs, Segment{Op: OpLine, X: x, Y: y})
	p.lineTo(x, y)
}

func (p *Path) lineTo(x, y int64) {
	p.addEdge(p.curX, p.curY, x, y)
	p.curX, p.curY = x, y
	p.grow(x, y)
}

// QuadTo draws a quadratic curve from the current point through the control
// point cx, cy to x, y, as straight lines short enough that no point of the
// curve is further than a sixteenth of a pixel from them.
func (p *Path) QuadTo(cx, cy, x, y int64) {
	p.segs = append(p.segs, Segment{Op: OpQuad, CX: cx, CY: cy, X: x, Y: y})
	x0, y0 := p.curX, p.curY
	dev := max(abs(x0-2*cx+x), abs(y0-2*cy+y))
	// A curve flattened into n lines strays from them by at most dev/(8n^2),
	// so n = sqrt(dev/(8*tol)) with tol a sixteenth of a pixel, One/16.
	n := isqrt(dev/(One/2)) + 1
	n = min(n, 256)
	nn := n * n
	for i := int64(1); i <= n; i++ {
		a, b, c := (n-i)*(n-i), 2*i*(n-i), i*i
		p.lineTo(divRound(a*x0+b*cx+c*x, nn), divRound(a*y0+b*cy+c*y, nn))
	}
	p.grow(cx, cy)
}

// Close joins the current point back to where its contour started.
func (p *Path) Close() {
	if p.open {
		p.segs = append(p.segs, Segment{Op: OpClose})
	}
	if p.open && (p.curX != p.startX || p.curY != p.startY) {
		p.addEdge(p.curX, p.curY, p.startX, p.startY)
	}
	p.curX, p.curY = p.startX, p.startY
	p.open = false
}

func (p *Path) addEdge(x0, y0, x1, y1 int64) {
	if y0 == y1 {
		return
	}
	dir := 1
	if y0 > y1 {
		x0, y0, x1, y1 = x1, y1, x0, y0
		dir = -1
	}
	p.edges = append(p.edges, edge{x0, y0, x1, y1, dir})
}

func (p *Path) grow(x, y int64) {
	if p.empty {
		p.minX, p.maxX, p.minY, p.maxY = x, x, y, y
		p.empty = false
		return
	}
	p.minX, p.maxX = min(p.minX, x), max(p.maxX, x)
	p.minY, p.maxY = min(p.minY, y), max(p.maxY, y)
}

type crossing struct {
	x   int64
	dir int
}

// Fill measures the outline over a picture width by height pixels and calls
// plot for every pixel it covers, with how much of it from 1 to 255. Rows go
// top to bottom and pixels left to right, each pixel once.
func Fill(p *Path, width, height int, plot func(x, y int, alpha uint8)) {
	p.Close()
	box, ok := p.box(int64(width), int64(height))
	if !ok {
		return
	}
	f := newFiller(p.edges, box)
	for row := box.top; row < box.bottom; row++ {
		f.row(row)
		f.emit(row, plot)
	}
}

// box is the part of a picture an outline can touch, in whole pixels.
type box struct{ left, right, top, bottom int64 }

func (p *Path) box(width, height int64) (box, bool) {
	if p.empty || len(p.edges) == 0 || width <= 0 || height <= 0 {
		return box{}, false
	}
	b := box{
		left:   clamp(floorDiv(p.minX, One), 0, width),
		right:  clamp(floorDiv(p.maxX, One)+1, 0, width),
		top:    clamp(floorDiv(p.minY, One), 0, height),
		bottom: clamp(floorDiv(p.maxY, One)+1, 0, height),
	}
	return b, b.left < b.right && b.top < b.bottom
}

// filler is one Fill in progress: the edges by where they start, the ones the
// current sampling line crosses, and the coverage of the current row.
type filler struct {
	box    box
	edges  []edge
	next   int
	active []edge
	cross  []crossing
	part   []int64
	whole  []int64
}

func newFiller(edges []edge, b box) *filler {
	span := b.right - b.left
	f := &filler{box: b, edges: slices.Clone(edges), part: make([]int64, span+1), whole: make([]int64, span+1)}
	slices.SortFunc(f.edges, byStart)
	return f
}

// byStart orders edges by every field, so two edges starting at one height
// are taken in the same order on every system.
func byStart(a, b edge) int {
	for _, d := range [4][2]int64{{a.y0, b.y0}, {a.x0, b.x0}, {a.y1, b.y1}, {a.x1, b.x1}} {
		if c := cmp64(d[0], d[1]); c != 0 {
			return c
		}
	}
	return a.dir - b.dir
}

// row samples one row of pixels along Rows lines.
func (f *filler) row(row int64) {
	clear(f.part)
	clear(f.whole)
	for k := int64(0); k < Rows; k++ {
		f.line(row*One + k*(One/Rows) + One/(2*Rows))
	}
}

// line measures the inside of the outline along one sampling line at y.
func (f *filler) line(y int64) {
	for f.next < len(f.edges) && f.edges[f.next].y0 <= y {
		f.active = append(f.active, f.edges[f.next])
		f.next++
	}
	f.active = slices.DeleteFunc(f.active, func(e edge) bool { return e.y1 <= y })
	f.cross = f.cross[:0]
	for _, e := range f.active {
		x := e.x0 + floorDiv((y-e.y0)*(e.x1-e.x0), e.y1-e.y0)
		f.cross = append(f.cross, crossing{x, e.dir})
	}
	slices.SortFunc(f.cross, func(a, b crossing) int {
		if c := cmp64(a.x, b.x); c != 0 {
			return c
		}
		return a.dir - b.dir
	})
	lo, hi := f.box.left*One, f.box.right*One
	winding := 0
	var from int64
	for _, c := range f.cross {
		was := winding
		winding += c.dir
		if was == 0 && winding != 0 {
			from = c.x
		} else if was != 0 && winding == 0 {
			cover(f.part, f.whole, clamp(from, lo, hi)-lo, clamp(c.x, lo, hi)-lo)
		}
	}
}

// emit hands the coverage of one finished row to plot.
func (f *filler) emit(row int64, plot func(x, y int, alpha uint8)) {
	var run int64
	for i := int64(0); i < f.box.right-f.box.left; i++ {
		run += f.whole[i]
		c := run + f.part[i]
		if c <= 0 {
			continue
		}
		if a := min((c*255+full/2)/full, 255); a > 0 {
			plot(int(f.box.left+i), int(row), uint8(a))
		}
	}
}

// cover adds the span from a to b, in coordinates from the left edge of the
// outline, to one sampling line: the pixels at its ends in part, the whole
// pixels between as a step in whole that the row sums up.
func cover(part, whole []int64, a, b int64) {
	if b <= a {
		return
	}
	ia, ib := a>>Frac, b>>Frac
	if ia == ib {
		part[ia] += b - a
		return
	}
	part[ia] += One - (a & (One - 1))
	whole[ia+1] += One
	whole[ib] -= One
	part[ib] += b & (One - 1)
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

func cmp64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func clamp(v, lo, hi int64) int64 { return min(max(v, lo), hi) }

// floorDiv is a / b rounded down, b positive.
func floorDiv(a, b int64) int64 {
	q := a / b
	if (a%b != 0) && (a < 0) {
		q--
	}
	return q
}

// divRound is a / b to the nearest whole number, halves up, b positive.
func divRound(a, b int64) int64 { return floorDiv(2*a+b, 2*b) }

// isqrt is the whole part of the square root of v, v not negative.
func isqrt(v int64) int64 {
	if v < 2 {
		return max(v, 0)
	}
	x := v
	y := (x + 1) / 2
	for y < x {
		x = y
		y = (x + v/x) / 2
	}
	return x
}

// Isqrt is the whole part of the square root of v, for the shapes drawn
// beside the letters, which need a length.
func Isqrt(v int64) int64 { return isqrt(v) }
