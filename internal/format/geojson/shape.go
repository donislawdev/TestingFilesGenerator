package geojson

import (
	// D11 promises the same bytes from the same seed, so a deliberate,
	// reproducible generator is the product rather than a weakness. Nothing
	// here ever makes a secret.
	// nosemgrep: go.lang.security.audit.crypto.math_random.math-random-used
	"math/rand/v2"
	"slices"
)

// kind is one of the seven geometry types of RFC 7946, or no geometry at all.
type kind int

const (
	kindPoint kind = iota
	kindLine
	kindPolygon
	kindMultiPoint
	kindMultiLine
	kindMultiPolygon
	kindCollection
	// kindNone is a feature with no place, written as geometry null. It is
	// drawn from no pieces, so it draws nothing.
	kindNone
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
	kindNone:         "",
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

// kindsOf is the kinds a file draws. With every feature unlocated it draws
// none, whatever geometry says, so nothing asks for room on the globe.
func kindsOf(geometry, unlocated string) []kind {
	switch {
	case unlocated == All:
		return []kind{kindNone}
	case geometry == Mixed:
		return everyKind
	default:
		return []kind{kindOf[geometry]}
	}
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
	kindNone:         nil,
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

// wrap is a longitude as written: one drawn past 180 degrees comes back on the
// other side of the antimeridian.
func (g grid) wrap(x int64) int64 {
	if x > g.lon {
		return x - 2*g.lon
	}
	return x
}

// needs is the fewest steps of longitude a piece of n points spans. A line
// moves right at every point. An outline goes right along the bottom and back
// left along the top, so it needs room for the longer of its two halves - and
// with holes, room for them as well: a cell of four steps each between the two
// columns a step in from the ends (see pierced).
func needs(p piece, n, holes int64) int64 {
	switch p {
	case pieceLine:
		return n - 1
	case pieceRing:
		half := (n - 2) - (n-2)/2 + 1
		if holes > 0 {
			return max(half, holeCell*holes+2)
		}
		return half
	default:
		return 0
	}
}

func widest(ps []piece, n, holes int64) int64 {
	var most int64
	for _, p := range ps {
		most = max(most, needs(p, n, holes))
	}
	return most
}

// holeCell is the fewest steps of longitude one hole takes: two for the hole
// and one either side of it.
const holeCell = 4

// mostHoles is the most holes an outline can hold at this precision - needs
// turned round for holes, in half the globe rather than all of it. An outline
// widened for holes may have only six points, so an edge of it can run from
// one end nearly to the other, and an edge as long as half the globe or more
// reads as the short way round the other side to a reader that draws edges on
// the sphere - an outline crossing the antimeridian nobody asked for (measured
// 2026-10-07 with 118 holes at precision 0, before this limit).
func mostHoles(g grid) int64 { return (g.lon - 2) / holeCell }

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
//
// A feature with no place draws nothing and takes nothing from rng.
func (d *drawing) draw(rng *rand.Rand, s settings, g grid, k kind) {
	d.reset(s)
	if k == kindNone {
		return
	}
	d.rng, d.g, d.altitude = rng, g, s.altitude
	ps := pieces[k]
	n := int64(s.vertices)
	holes := holesIn(ps, s)
	b := box{w: max(widest(ps, n, holes), g.scale/5+rng.Int64N(g.scale*4/5+1))}
	if s.antimeridian {
		b.w = max(b.w, 2)
	}
	b.h = max(1, g.scale/20+rng.Int64N(g.scale/5+1))
	if holes > 0 {
		// Two steps either side of the middle at the least: the outline keeps
		// to the outer one and the holes reach no further than the inner one.
		b.h = max(b.h, 2)
	}
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
			d.polygon(b, c, n, holes)
		}
		d.ends = append(d.ends, len(d.pos)/d.stride)
	}
}

// holesIn is the holes the kind drawn from ps has: the setting for a kind with
// an outline, and none for points and lines, which holes leave as they were,
// byte for byte.
func holesIn(ps []piece, s settings) int64 {
	if slices.Contains(ps, pieceRing) {
		return int64(s.holes)
	}
	return 0
}

// polygon is an outline and its holes. Without holes it is the outline every
// earlier file drew, with the same draws in the same order.
func (d *drawing) polygon(b box, c, n, holes int64) {
	if holes == 0 {
		d.ring(b, c, n)
		return
	}
	d.pierced(b, c, n, holes)
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

// holePoints is how many points every hole has, and holePositions what one
// writes: those and the first again.
const (
	holePoints    = 4
	holePositions = holePoints + 1
)

// pierced is an outline of n points with holes inside it, every one of them
// strictly inside and none touching another - simple by construction, like
// the outline alone.
//
// The outline alone lets its halves come back to the middle row between
// points, and with two points a half the free part of the middle can be a
// single step wide. So here each half keeps gap rows or more from the middle,
// and its first and last points stand one step in from the ends. Between
// those two columns the band of 2*gap-1 rows round the middle is inside the
// outline whatever was drawn. The holes are drawn in that band, one to a cell,
// each by ring at four points.
//
// What keeps a hole off the outline and off its neighbours is a margin, and the
// margin is a share of the space rather than a step. At fifteen decimal places
// a step is 10^-15 degrees, below what a double tells apart at a longitude
// of a hundred - read by GEOS, holes a step apart touched and cut the polygon
// in two (measured 2026-10-07). So a hole keeps a quarter of its cell clear on
// either side and reaches only half way to the outline above and below it.
// needs makes the box wide enough, and refuseHoles keeps n at six or more,
// two points a half.
func (d *drawing) pierced(b box, c, n, holes int64) {
	first := len(d.pos)
	bottom := (n - 2) / 2
	gap := max(2, b.h/2)
	d.add(b.x0, c)
	d.pinned(b, bottom, c-gap, -1, b.h-gap)
	d.add(b.x0+b.w, c)
	from := len(d.pos)
	d.pinned(b, n-2-bottom, c+gap, 1, b.h-gap)
	d.reverse(from)
	d.pos = append(d.pos, d.pos[first:first+d.stride]...)

	// A cell no wider than a quarter of the globe, so no edge of a hole is
	// half the globe long either, when the outline is wide for its points.
	cell := min((b.w-2)/holes, d.g.lon/2)
	margin := max(1, cell/holeCell)
	for j := int64(0); j < holes; j++ {
		at := b.x0 + 1 + j*cell + margin
		d.ring(box{x0: at, w: cell - 2*margin, h: max(1, gap/2)}, c, holePoints)
	}
}

// pinned is one half of a pierced outline: count points, the first one step in
// from the left end and the last one step in from the right, the rest between
// them in slots as half places them, all from edge outwards by up to spread.
func (d *drawing) pinned(b box, count, edge, sign, spread int64) {
	d.add(b.x0+1, edge+sign*d.rng.Int64N(spread+1))
	d.half(box{x0: b.x0 + 1, w: b.w - 2, h: spread + 1}, count-2, edge, sign)
	d.add(b.x0+b.w-1, edge+sign*d.rng.Int64N(spread+1))
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
	holes := int(holesIn(pieces[k], s))
	for _, p := range pieces[k] {
		for i := 0; i < positions(p, s.vertices, holes); i++ {
			d.pos = append(d.pos, widest...)
		}
		d.ends = append(d.ends, len(d.pos)/d.stride)
	}
}

// positions is how many positions a piece of n points writes. An outline
// repeats its first point at the end, and so does every hole after it.
func positions(p piece, n, holes int) int {
	switch p {
	case pieceLine:
		return n
	case pieceRing:
		return n + 1 + holes*holePositions
	default:
		return 1
	}
}

// extent is the box a set of positions lies in, in the coordinates they were
// drawn in - a longitude past 180 degrees not yet wrapped round - so a box
// across the antimeridian is one interval here and two only when written.
type extent struct {
	lo, hi [3]int64
	set    bool
}

// extent is the box this drawing lies in. A drawing of no positions, a
// feature with no place, has none.
func (d *drawing) extent() extent {
	var x extent
	for i := 0; i+d.stride <= len(d.pos); i += d.stride {
		x.add(d.pos[i : i+d.stride])
	}
	return x
}

func (x *extent) add(p []int64) {
	if !x.set {
		copy(x.lo[:], p)
		copy(x.hi[:], p)
		x.set = true
		return
	}
	for j, v := range p {
		x.lo[j] = min(x.lo[j], v)
		x.hi[j] = max(x.hi[j], v)
	}
}

// grow widens x to take in y as well.
func (x *extent) grow(y extent, stride int) {
	if y.set {
		x.add(y.lo[:stride])
		x.add(y.hi[:stride])
	}
}
