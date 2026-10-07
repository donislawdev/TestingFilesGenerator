package video

import (
	"image"
	"slices"
)

// grid is how a film's picture is cut into AV1 tiles: columns and rows of
// superblocks, 64 by 64 pixels, adding up to the picture's own.
//
// Why there are tiles. A film's pictures differ only in the clock and the
// square, and an AV1 tile is decoded without looking past its own edges - its
// entropy and its neighbours start afresh at every edge (AV1 specification,
// AOMediaCodec/av1-spec 5e04f3f: 06.bitstream.syntax.md lines 1821-1880 and
// 4106-4111, 09.parsing.process.md lines 38-70). So a tile gav1d codes as a
// picture of its own is the same tile in a frame of many, and a tile no
// picture changes is coded once a film. Measured on 2026-10-06 against
// libaom, dav1d and gav1d's own decoder, bit for bit, at five sizes. Per
// picture it is 17.7 times faster at 1920x1080 and 43.4 at 3840x2160
// (docs/WEBM-WYDAJNOSC-2026-10-06.md section 8).
//
// The layout is part of the bytes of every film (D11), so it is a rule rather
// than a search, measured against two others (section 9.1): one tile row under
// the clock's band, two around the square's, each superblock the clock's
// characters reach a column of its own and the rest of the width one column -
// cut further only where a frame cannot carry it (cutToFit).
// The clock's tile is coded at every change, so it is small. The square has
// ten places, so its tiles are coded ten times a film at most, and only have
// to be low. The rest is coded once. More columns cost bytes - every seam
// loses the prediction across it - and buy no time.
type grid struct {
	width, height int
	cols, rows    []int
}

const superblock = 64

// sbOf is how many superblocks a side of this many pixels takes, which is
// what tile_info counts: MiCols rounded to whole superblocks.
func sbOf(px int) int { return (px + superblock - 1) / superblock }

func oneTile(width, height int) grid {
	return grid{width: width, height: height, cols: []int{sbOf(width)}, rows: []int{sbOf(height)}}
}

// gridFor is the layout of a film's picture.
//
// The limits are those of the lowest level the picture's size and rate allow,
// before its bytes are known: the level a film declares is chosen from its
// bytes, the bytes depend on the tiles, and a level never allows fewer tiles
// than one below it (annex.a.levels.md lines 99-114, MaxTiles and MaxTileCols
// rise with the level), so a layout that fits the lowest fits every level the
// film can end up declaring. Over the limits, the square's rows go first, then
// the clock's columns are joined, then the picture is the fewest tiles it can
// be - one, for any picture of 4096 by 2304 or less.
//
// A picture is that fewest when its clock's tiles would be more than a quarter
// of it - coding them at every change would save less than three quarters of
// the time a whole picture takes, and every seam costs bytes. That is every
// rung of the ladder from 160x90 down.
func gridFor(g geometry, fps int) grid {
	maxTiles, maxCols := tileLimits(chooseLevel(g.width, g.height, fps, 0, 1))
	for _, try := range [...]struct{ square, apart bool }{{true, true}, {false, true}, {false, false}} {
		out := cutToFit(grid{width: g.width, height: g.height, rows: tileRows(g, try.square), cols: tileCols(g, try.apart)})
		if out.tiles() > maxTiles || len(out.cols) > maxCols {
			continue
		}
		if out.tiles() == 1 || 4*out.clockArea(g) > g.width*g.height {
			break
		}
		return out
	}
	return cutToFit(oneTile(g.width, g.height))
}

// Layout is the columns and rows, in superblocks, a film's picture is cut
// into - for the guard that holds every layout to what tile_info and Annex A
// allow, from the specification rather than from this package.
func Layout(width, height int, label string, t Timeline) (cols, rows []int) {
	g := gridFor(geometryOf(width, height, label, t), t.FPS)
	return g.cols, g.rows
}

// cutToFit cuts a layout further where a frame cannot carry it: a column wider
// than MAX_TILE_WIDTH, and a row taller than tile_info lets a tile be once the
// frame is too large for one (tileBounds.maxArea). Each is cut into the fewest
// equal parts, the larger ones last. A picture of 4096 by 2304 or less needs
// neither, so its layout is the one it had before pictures this large were
// allowed - every such size in a sweep of 10.4 million layouts on 2026-10-07
// (docs/WEBM-LIMIT-2026-10-07.md section 2), and the golden film of tiles.
//
// A column is also no wider than half that area when the frame has more than
// one row of superblocks, so that a row of tiles can be two superblocks tall.
// Without it, a picture wider than 4096 and only a little over 64 tall came out
// with a last row of one superblock holding a pixel or two, and Annex A asks
// every tile to be at least 8 pixels tall (CroppedTileHeight, annex.a.levels.md
// lines 265-267) - 81 900 layouts of that sweep did. The larger parts go last
// for the same reason: the last part is then at least two superblocks.
func cutToFit(g grid) grid {
	b := boundsOf(g.width, g.height)
	area := b.maxArea()
	widest := maxTileWidth / superblock
	if b.sbRows > 1 {
		widest = min(widest, max(area/2, 1))
	}
	g.cols = cutEach(g.cols, widest)
	g.rows = cutEach(g.rows, max(area/slices.Max(g.cols), 1))
	return g
}

// cutEach cuts every size over most into the fewest parts no larger than it,
// as equal as whole superblocks allow, the larger ones last.
func cutEach(sizes []int, most int) []int {
	out := make([]int, 0, len(sizes))
	for _, s := range sizes {
		n := (s + most - 1) / most
		for k := range n {
			// (k+s%n)/n is one for the last s%n parts and nought before them.
			out = append(out, s/n+(k+s%n)/n)
		}
	}
	return out
}

// tileLimits is MaxTiles and MaxTileCols of a level, annex.a.levels.md lines
// 99-114 (read 2026-10-06). The maximum parameters level has none of its own,
// which leaves the specification's MAX_TILE_ROWS and MAX_TILE_COLS, 64 each.
func tileLimits(level int) (tiles, cols int) {
	switch {
	case level <= 1:
		return 8, 4
	case level <= 5:
		return 16, 6
	case level <= 9:
		return 32, 8
	case level <= 15:
		return 64, 8
	case level <= 19:
		return 128, 16
	}
	return 64 * 64, 64
}

// tileRows cuts the rows under the clock's band, which is always in the first
// row of superblocks, and - when square - above and under the square's band.
func tileRows(g geometry, square bool) []int {
	var cuts []int
	if g.showClock() {
		cuts = append(cuts, sbOf(g.clockEnd))
	}
	if square {
		cuts = append(cuts, g.squareY/superblock, sbOf(g.squareY+g.side))
	}
	return sizesBetween(cuts, sbOf(g.height), g.height)
}

// tileCols gives each superblock the clock's characters reach a column of its
// own when apart, or one column for all of them, and the rest of the width one
// column.
func tileCols(g geometry, apart bool) []int {
	clock := sbOf(g.clockRight())
	var cuts []int
	if apart {
		for c := 1; c < clock; c++ {
			cuts = append(cuts, c)
		}
	}
	cuts = append(cuts, clock)
	return sizesBetween(cuts, sbOf(g.width), g.width)
}

// sizesBetween turns cuts, in superblocks, into the sizes between them - cuts
// at the edges or past them dropped, and a last size under 8 pixels joined to
// the one before it, because Annex A asks every tile to be at least 8 by 8 of
// the picture (CroppedTileWidth and CroppedTileHeight, annex.a.levels.md
// lines 265-267).
func sizesBetween(cuts []int, total, px int) []int {
	slices.Sort(cuts)
	cuts = slices.Compact(cuts)
	sizes := make([]int, 0, len(cuts)+1)
	prev := 0
	for _, c := range cuts {
		if c > prev && c < total {
			sizes = append(sizes, c-prev)
			prev = c
		}
	}
	sizes = append(sizes, total-prev)
	if n := len(sizes); n > 1 && px-superblock*(total-sizes[n-1]) < 8 {
		sizes[n-2] += sizes[n-1]
		sizes = sizes[:n-1]
	}
	return sizes
}

func (g grid) tiles() int { return len(g.cols) * len(g.rows) }

// rect is tile i in pixels, in the order the tiles follow one another in a
// frame - across, then down - cut to the picture.
func (g grid) rect(i int) image.Rectangle {
	col, row := i%len(g.cols), i/len(g.cols)
	x0, y0 := 0, 0
	for _, c := range g.cols[:col] {
		x0 += superblock * c
	}
	for _, r := range g.rows[:row] {
		y0 += superblock * r
	}
	return image.Rect(x0, y0, min(g.width, x0+superblock*g.cols[col]), min(g.height, y0+superblock*g.rows[row]))
}

// writeTileInfo writes tile_info for this layout, 06.bitstream.syntax.md
// lines 1176-1268, with tileSizeBytes for the sizes of every tile but the last.
//
// One tile is written the way gav1d writes it - uniform spacing, no increment
// - so a film of one tile has the bytes it had before there were tiles, and
// the golden films of one tile hold the writer to that. Several are written
// with their sizes (uniform_tile_spacing_flag 0), because the layout's sizes
// are its own and not the uniform ones the specification would derive.
func (g grid) writeTileInfo(w *bitWriter, tileSizeBytes int) {
	b := boundsOf(g.width, g.height)
	if g.tiles() == 1 {
		w.bit(1) // uniform_tile_spacing_flag
		if b.minLog2Cols < b.maxLog2Cols {
			w.bit(0) // increment_tile_cols_log2
		}
		if max(b.minLog2Tiles-b.minLog2Cols, 0) < b.maxLog2Rows {
			w.bit(0) // increment_tile_rows_log2
		}
		return
	}
	w.bit(0)
	widest, start := 0, 0
	for _, s := range g.cols {
		w.ns(s-1, min(b.sbCols-start, maxTileWidth/superblock)) // width_in_sbs_minus_1
		widest, start = max(widest, s), start+s
	}
	start = 0
	for _, s := range g.rows {
		w.ns(s-1, min(b.sbRows-start, max(b.maxArea()/widest, 1))) // height_in_sbs_minus_1
		start += s
	}
	// context_update_tile_id says which tile's probabilities a frame keeps for
	// the next, and every frame here keeps none (disable_frame_end_update_cdf),
	// so it is the first.
	w.bits(0, tileLog2(1, len(g.cols))+tileLog2(1, len(g.rows)))
	w.bits(uint32(tileSizeBytes-1), 2) // tile_size_bytes_minus_1
}

// tileSizeBytes is how many bytes each tile's size takes in a frame of this
// film: enough for the largest tile a picture may have, which is no larger
// than the picture's reserve. One number for the film rather than one per
// frame, so a frame's length is known before its tiles are coded.
func tileSizeBytes(reserve int) int {
	n := 1
	for n < 4 && max(reserve-1, 0)>>(8*n) != 0 {
		n++
	}
	return n
}

// clockArea is the pixels of the tiles the clock's characters are drawn in.
func (g grid) clockArea(geo geometry) int {
	if !geo.showClock() {
		return 0
	}
	band := image.Rect(0, geo.clockFrom, geo.clockRight(), geo.clockEnd)
	area := 0
	for i := range g.tiles() {
		if r := g.rect(i); r.Overlaps(band) {
			area += r.Dx() * r.Dy()
		}
	}
	return area
}
