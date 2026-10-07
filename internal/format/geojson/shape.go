package geojson

import (
	// D11 promises the same bytes from the same seed, so a deliberate,
	// reproducible generator is the product rather than a weakness. Nothing
	// here ever makes a secret.
	// nosemgrep: go.lang.security.audit.crypto.math_random.math-random-used
	"math/rand/v2"
	"slices"
)

// kind is one of the seven geometry types of RFC 7946.
type kind int

const (
	kindPoint kind = iota
	kindLine
	kindPolygon
	kindMultiPoint
	kindMultiLine
	kindMultiPolygon
	kindCollection
)

// typeName is how each kind is written in the file.
var typeName = [...]string{
	kindPoint:        "Point",
	kindLine:         "LineString",
	kindPolygon:      "Polygon",
	kindMultiPoint:   "MultiPoint",
	kindMultiLine:    "MultiLineString",
	kindMultiPolygon: "MultiPolygon",
	kindCollection:   "GeometryCollection",
}

var kindOf = map[string]kind{
	Point:              kindPoint,
	LineString:         kindLine,
	Polygon:            kindPolygon,
	MultiPoint:         kindMultiPoint,
	MultiLineString:    kindMultiLine,
	MultiPolygon:       kindMultiPolygon,
	GeometryCollection: kindCollection,
}

// everyKind is mixed: all seven, in the order RFC 7946 introduces them.
var everyKind = []kind{kindPoint, kindLine, kindPolygon, kindMultiPoint, kindMultiLine, kindMultiPolygon, kindCollection}

func kindsOf(geometry string) []kind {
	if geometry == Mixed {
		return everyKind
	}
	return []kind{kindOf[geometry]}
}

// piece is what a geometry is drawn from. A MultiPoint is drawn as a line's
// points and written without the line, which is why there is no fourth piece.
type piece int

const (
	piecePoint piece = iota
	pieceLine
	pieceRing
)

// pieces is what each kind is drawn from, one band of latitude each, from the
// bottom up. A multi geometry has two parts, and a collection holds one point,
// one line and one polygon.
var pieces = [...][]piece{
	kindPoint:        {piecePoint},
	kindLine:         {pieceLine},
	kindPolygon:      {pieceRing},
	kindMultiPoint:   {pieceLine},
	kindMultiLine:    {pieceLine, pieceLine},
	kindMultiPolygon: {pieceRing, pieceRing},
	kindCollection:   {piecePoint, pieceLine, pieceRing},
}

// grid is the precision as whole numbers: a coordinate is a count of steps of
// 10^-places degrees, and a height a count of the same steps of a metre.
type grid struct {
	places int
	scale  int64
	lon    int64 // 180 degrees
	lat    int64 // 85 degrees
	low    int64 // -400 metres
	high   int64 // 8848 metres
}

// lowestLand and highestLand are the heights a position may carry: a little
// below the shore of the Dead Sea and the top of Everest.
const (
	lowestLand  = -400
	highestLand = 8848
	// widestLatitude keeps every shape inside the band a web map in the Web
	// Mercator projection can show, which ends just past 85 degrees.
	widestLatitude = 85
)

func gridFor(places int) grid {
	scale := int64(1)
	for i := 0; i < places; i++ {
		scale *= 10
	}
	return grid{places: places, scale: scale, lon: 180 * scale, lat: widestLatitude * scale,
		low: lowestLand * scale, high: highestLand * scale}
}

// room is the widest a shape may be, in steps of longitude: short of the whole
// globe by a step on each side, so a shape never meets itself round the back.
func (g grid) room() int64 { return 2*g.lon - 2 }

// needs is the fewest steps of longitude a piece of n points spans. A line
// moves right at every point. An outline goes right along the bottom and back
// left along the top, so it needs room for the longer of its two halves.
func needs(p piece, n int64) int64 {
	switch p {
	case pieceLine:
		return n - 1
	case pieceRing:
		return (n - 2) - (n-2)/2 + 1
	default:
		return 0
	}
}

func widest(ps []piece, n int64) int64 {
	var most int64
	for _, p := range ps {
		most = max(most, needs(p, n))
	}
	return most
}

// mostPoints is the largest vertices the globe has room for at this precision,
// given the kinds a file draws - needs turned round, for the piece that needs
// the most. Points alone use no vertices at all, so they set no limit.
func mostPoints(kinds []kind, g grid) int64 {
	switch {
	case draws(kinds, pieceLine):
		return g.room() + 1
	case draws(kinds, pieceRing):
		return 2 * g.room()
	default:
		return maxVertices
	}
}

func draws(kinds []kind, p piece) bool {
	for _, k := range kinds {
		if slices.Contains(pieces[k], p) {
			return true
		}
	}
	return false
}

// drawing is one geometry's positions, unwrapped: a longitude past 180 stays
// past it here and is wrapped only when written, so the shape can be built and
// checked as one piece wherever it lies.
type drawing struct {
	pos    []int64 // x, y and, with altitude, z
	ends   []int   // where each piece ends, counted in positions
	stride int

	// What one feature is drawn with, set by draw so the functions that draw
	// its pieces do not carry it in every call.
	rng      *rand.Rand
	g        grid
	altitude bool
}

// box is the space one geometry is drawn in.
type box struct {
	x0, w int64 // the left edge and the width, in steps of longitude
	h     int64 // how far a piece reaches above and below its middle
}

// draw makes the geometry of one feature.
//
// Every shape is simple by construction rather than by luck: a line moves
// right at every point, and an outline runs right along the bottom below its
// middle and back left along the top above it, so the two halves meet only at
// their ends. The parts of a multi geometry sit in separate bands of latitude,
// so they never overlap. Nothing is checked after the fact, because nothing
// can go wrong that a check would have to catch.
func (d *drawing) draw(rng *rand.Rand, s settings, g grid, k kind) {
	d.reset(s)
	d.rng, d.g, d.altitude = rng, g, s.altitude
	ps := pieces[k]
	n := int64(s.vertices)
	b := box{w: max(widest(ps, n), g.scale/5+rng.Int64N(g.scale*4/5+1))}
	if s.antimeridian {
		b.w = max(b.w, 2)
	}
	b.h = max(1, g.scale/20+rng.Int64N(g.scale/5+1))
	pitch := 2*b.h + 3
	if s.antimeridian {
		// Starting at least a step short of 180 and ending at least a step past
		// it, so every line and outline crosses.
		b.x0 = g.lon - b.w + 1 + rng.Int64N(b.w-1)
	} else {
		b.x0 = -g.lon + rng.Int64N(2*g.lon-b.w+1)
	}
	y0 := -g.lat + rng.Int64N(2*g.lat-int64(len(ps))*pitch+1)
	for j, p := range ps {
		c := y0 + int64(j)*pitch + b.h + 1
		switch p {
		case piecePoint:
			d.add(b.x0+rng.Int64N(b.w+1), c-b.h+rng.Int64N(2*b.h+1))
		case pieceLine:
			d.line(b, c, n)
		case pieceRing:
			d.ring(b, c, n)
		}
		d.ends = append(d.ends, len(d.pos)/d.stride)
	}
}

func (d *drawing) reset(s settings) {
	d.pos = d.pos[:0]
	d.ends = d.ends[:0]
	d.stride = 2
	if s.altitude {
		d.stride = 3
	}
}

func (d *drawing) add(x, y int64) {
	d.pos = append(d.pos, x, y)
	if d.altitude {
		d.pos = append(d.pos, height(d.rng, d.g))
	}
}

// height is a height between the lowest and the highest, drawn as an unsigned
// count. At fifteen places the span is 9248 * 10^15 steps, past the largest
// int64 - found 2026-10-07, when Int64N was handed the overflow and stopped the
// run. Every height itself fits, so the sum wraps back into range.
func height(rng *rand.Rand, g grid) int64 {
	span := uint64(g.high) + uint64(-g.low) + 1
	return int64(uint64(g.low) + rng.Uint64N(span))
}

// line is n points, each a step or more right of the one before, the first on
// the left edge of the box.
func (d *drawing) line(b box, c, n int64) {
	step := b.w / (n - 1)
	for i := int64(0); i < n; i++ {
		x := b.x0 + i*step
		if i > 0 && i < n-1 {
			x += d.rng.Int64N(step)
		}
		d.add(x, c-b.h+d.rng.Int64N(2*b.h+1))
	}
}

// ring is a closed outline of n points, counter-clockwise: from the left end
// right along the bottom, below the middle, to the right end, back left along
// the top, above the middle, and the first position again. RFC 7946 asks for
// the last position to repeat the first exactly, height included.
func (d *drawing) ring(b box, c, n int64) {
	first := len(d.pos)
	bottom := (n - 2) / 2
	top := n - 2 - bottom
	d.add(b.x0, c)
	d.half(b, bottom, c-1, -1)
	d.add(b.x0+b.w, c)
	from := len(d.pos)
	d.half(b, top, c+1, 1)
	// Drawn left to right like the bottom, and walked right to left.
	d.reverse(from)
	d.pos = append(d.pos, d.pos[first:first+d.stride]...)
}

// half is the points strictly between the two ends of an outline, from left to
// right, each in a slot of its own so no two share a longitude. edge is the
// row nearest the middle and sign the side of it they keep to.
func (d *drawing) half(b box, count, edge, sign int64) {
	slot := b.w / (count + 1)
	for i := int64(0); i < count; i++ {
		x := b.x0 + i*slot + 1 + d.rng.Int64N(slot)
		d.add(x, edge+sign*d.rng.Int64N(b.h))
	}
}

// reverse turns round the positions from index from to the end.
func (d *drawing) reverse(from int) {
	st := d.stride
	for i, j := from, len(d.pos)-st; i < j; i, j = i+st, j-st {
		for k := 0; k < st; k++ {
			d.pos[i+k], d.pos[j+k] = d.pos[j+k], d.pos[i+k]
		}
	}
}

// worst fills the drawing with the widest numbers every position can be
// written with, in the counts kind k has. It is what the smallest closing
// feature is measured from, and it has to be at least as long as every draw.
func (d *drawing) worst(s settings, g grid, k kind) {
	d.reset(s)
	widest := []int64{-g.lon, -g.lat, g.low}[:d.stride]
	for _, p := range pieces[k] {
		for i := 0; i < positions(p, s.vertices); i++ {
			d.pos = append(d.pos, widest...)
		}
		d.ends = append(d.ends, len(d.pos)/d.stride)
	}
}

// positions is how many positions a piece of n points writes. An outline
// repeats its first point at the end.
func positions(p piece, n int) int {
	switch p {
	case pieceLine:
		return n
	case pieceRing:
		return n + 1
	default:
		return 1
	}
}
