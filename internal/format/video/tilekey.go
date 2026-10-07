package video

import "image"

// tileKey is everything the pixels of one tile of a film depend on, so two
// tiles with one key are the same tile and the second is not coded again.
//
// A tile is the gradient and the label, which no picture changes, the clock's
// characters drawn in it and the part of the square inside it - and nothing
// else, because a character is drawn from its own cell, the square is one
// colour, and a chroma sample covers two by two pixels that never straddle a
// tile's edge (tiles start on multiples of 64). So the key is the tile, those
// characters, and the square's columns in it - exact by construction, with no
// hash to collide, and a guard holds it to the pixels of pictures painted
// whole.
type tileKey struct {
	tile int
	// clock is the clock's characters drawn in the tile, four bits each - the
	// low four bits of each, which tell the ten digits, ':' and '.' apart. The
	// same tile always holds the same character positions, so the packed
	// characters of one tile are comparable.
	clock uint64
	// from and to are the square's columns inside the tile, both nought when
	// it is not there.
	from, to int
}

// showsClock is whether the tile changes with the clock - the tiles a film
// keeps only while there is room (pictures.go).
func (k tileKey) showsClock(ks keyer) bool { return ks.tiles[k.tile].chars[1] > 0 }

// keyer works out the keys of a film's tiles: for each tile, which of the
// clock's characters fall in it and whether it crosses the square's band.
type keyer struct {
	geo   geometry
	grid  grid
	tiles []tilePlace
}

type tilePlace struct {
	rect image.Rectangle
	// chars are the first and the after-last of the clock's characters drawn
	// in the tile, both nought when none is.
	chars  [2]int
	square bool
}

func newKeyer(geo geometry, g grid) keyer {
	ks := keyer{geo: geo, grid: g, tiles: make([]tilePlace, g.tiles())}
	band := image.Rect(0, geo.clockFrom, geo.width, geo.clockEnd)
	squareRows := image.Rect(0, geo.squareY, geo.width, geo.squareY+geo.side)
	for i := range ks.tiles {
		r := g.rect(i)
		p := tilePlace{rect: r, square: r.Overlaps(squareRows)}
		if geo.showClock() && r.Overlaps(band) {
			p.chars = clockCharsIn(geo, r)
		}
		ks.tiles[i] = p
	}
	return ks
}

// clockCharsIn is the run of the clock's characters whose cells cross the
// columns of r.
func clockCharsIn(geo geometry, r image.Rectangle) [2]int {
	first, last := -1, -1
	for k := range geo.clockChars {
		x := geo.cells.First + k*geo.cells.Step
		if x >= r.Max.X || x+geo.cells.Width <= r.Min.X {
			continue
		}
		if first < 0 {
			first = k
		}
		last = k
	}
	if first < 0 {
		return [2]int{}
	}
	return [2]int{first, last + 1}
}

// key is tile i's key in the picture with this look.
func (ks keyer) key(i int, l look) tileKey {
	p := ks.tiles[i]
	k := tileKey{tile: i}
	for c := p.chars[0]; c < p.chars[1]; c++ {
		k.clock = k.clock<<4 | uint64(l.clock[c]&0x0f)
	}
	if p.square {
		from, to := max(l.x, p.rect.Min.X), min(l.x+ks.geo.side, p.rect.Max.X)
		if from < to {
			k.from, k.to = from, to
		}
	}
	return k
}
