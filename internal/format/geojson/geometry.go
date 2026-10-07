package geojson

// geometry writes the geometry object of kind k from the drawing already made.
func (r *records) geometry(k kind) {
	e := &r.e
	e.open('{')
	e.key("type")
	e.text(typeName[k])
	if k == kindCollection {
		e.key("geometries")
		e.open('[')
		for j, p := range pieces[k] {
			e.next()
			r.member(p, j)
		}
		e.close(']')
	} else {
		e.key("coordinates")
		r.coordinates(k)
	}
	e.close('}')
}

// member is one geometry of a collection, written as the single geometry its
// piece makes.
func (r *records) member(p piece, j int) {
	e := &r.e
	e.open('{')
	e.key("type")
	e.text(typeName[pieceKind[p]])
	e.key("coordinates")
	r.piece(p, j)
	e.close('}')
}

// pieceKind is the single geometry each piece makes on its own.
var pieceKind = [...]kind{piecePoint: kindPoint, pieceLine: kindLine, pieceRing: kindPolygon}

// coordinates is the coordinates member of every kind but a collection. A
// MultiLineString and a MultiPolygon are an array of what one part would
// write. A MultiPoint is a line's points without the line, which is the same
// array of positions.
func (r *records) coordinates(k kind) {
	if k != kindMultiLine && k != kindMultiPolygon {
		r.piece(pieces[k][0], 0)
		return
	}
	r.e.open('[')
	for j, p := range pieces[k] {
		r.e.next()
		r.piece(p, j)
	}
	r.e.close(']')
}

// piece writes piece j of the drawing: a position for a point, an array of
// positions for a line, and an array holding one ring for an outline.
func (r *records) piece(p piece, j int) {
	from := 0
	if j > 0 {
		from = r.d.ends[j-1]
	}
	to := r.d.ends[j]
	switch p {
	case piecePoint:
		r.position(from)
	case pieceLine:
		r.run(from, to, false)
	case pieceRing:
		r.e.open('[')
		r.e.next()
		r.run(from, to, r.s.reversed)
		r.e.close(']')
	}
}

// run is the positions from one index to another, walked backwards for an
// outline asked for clockwise. The first and last positions of an outline are
// the same, so it stays closed either way round.
func (r *records) run(from, to int, backwards bool) {
	r.e.open('[')
	for i := range to - from {
		at := from + i
		if backwards {
			at = to - 1 - i
		}
		r.e.next()
		r.position(at)
	}
	r.e.close(']')
}

// position writes one position: longitude, latitude and, with altitude, the
// height. A longitude drawn past 180 degrees is written on the other side of
// the antimeridian, which is what crossing it without being cut looks like.
func (r *records) position(i int) {
	e := &r.e
	p := r.d.pos[i*r.d.stride : (i+1)*r.d.stride]
	x := p[0]
	if x > r.g.lon {
		x -= 2 * r.g.lon
	}
	e.open('[')
	for j, v := range p {
		if j == 0 {
			v = x
		}
		e.next()
		e.fixed(v, r.g.places, r.g.scale)
	}
	e.close(']')
}
