package guard

import (
	"fmt"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/video"
)

// annexATiles is MaxTiles and MaxTileCols of each level a film can declare, by
// seq_level_idx, copied from the specification's own table (AOMediaCodec/av1-spec
// 5e04f3f, annex.a.levels.md lines 99-115) rather than from the package, so
// the two are held to the source and not to each other. The maximum parameters
// level, 31, has no row: no level constraint applies to it.
var annexATiles = map[int][2]int{
	0: {8, 4}, 1: {8, 4}, 4: {16, 6}, 5: {16, 6}, 8: {32, 8}, 9: {32, 8},
	12: {64, 8}, 13: {64, 8}, 14: {64, 8}, 15: {64, 8},
	16: {128, 16}, 17: {128, 16}, 18: {128, 16}, 19: {128, 16},
}

// Every way a film's picture is cut into tiles is one the AV1 specification
// allows - over every size the format declares, up to its bound.
//
// A picture is cut by a rule (internal/format/video, grid.go) rather than by
// asking an encoder, and a decoder takes a layout it should refuse without a
// word: libaom, dav1d and Chromium all played a film whose last row of tiles
// was one pixel tall, which Annex A forbids (docs/WEBM-LIMIT-2026-10-07.md
// section 2 - 81 900 such layouts in a sweep before the rule was mended). So
// this asks the specification, not a decoder: tile_info's own bounds
// (06.bitstream.syntax.md lines 1176-1268 - a column at most 64 superblocks,
// a row at most the area tile_info leaves over the widest column, one tile
// only where the frame fits one), MAX_TILE_AREA, 64 columns and rows at most,
// and Annex A for the lowest level the size and rate allow (annex.a.levels.md
// lines 240-271 - tiles and columns per level, the last tile at least 8 pixels
// each way, the largest tile times the frames a second under 588 251 136).
//
// The sizes are every side from 1 to 17 and each side next to a multiple of
// 64, where a superblock starts or ends, paired up to the declared bound. Two
// clocks and two rates, alternating, because the clock decides the columns
// and the rate the level.
func TestEveryFilmLayoutIsOneTheAV1SpecificationAllows(t *testing.T) {
	d, err := format.Get("webm")
	if err != nil {
		t.Fatal(err)
	}
	if len(d.JointLimits) != 1 {
		t.Fatalf("webm declares %d joint limits and this guard reads one", len(d.JointLimits))
	}
	most := d.JointLimits[0].Max
	var sides []int
	for s := 1; s <= 17; s++ {
		sides = append(sides, s)
	}
	for k := 1; k <= 256; k++ {
		for _, s := range []int{64*k - 1, 64 * k, 64*k + 1, 64*k + 8} {
			if s > 17 && s <= 16384 {
				sides = append(sides, s)
			}
		}
	}
	var films [2]video.Timeline
	for i, v := range [][2]int64{{1000, 30}, {500, 60}} {
		if films[i], err = video.NewTimeline("webm", 10_000, 60_000, v[0], int(v[1])); err != nil {
			t.Fatal(err)
		}
	}
	labels := [2]string{core.Label("webm", 268435456, 7741), ""}
	seen := layoutsSeen{}
	n := 0
	for _, w := range sides {
		for _, h := range sides {
			if int64(w)*int64(h) > most {
				continue
			}
			film := films[n%2]
			cols, rows := video.Layout(w, h, labels[n/2%2], film)
			n++
			for _, why := range layoutBreaks(w, h, film.FPS, cols, rows, &seen) {
				if seen.reported < 20 {
					t.Errorf("%dx%d at %d frames a second, columns %v, rows %v: %s", w, h, film.FPS, cols, rows, why)
				}
				seen.reported++
			}
		}
	}
	// The guard has to have asked about the layouts the rule cuts, or it says
	// nothing about them (O118).
	if seen.wide < 1000 || seen.byArea < 1000 || seen.narrowed < 100 || seen.mostTiles < 32 {
		t.Fatalf("%d layouts, %d with a column cut for width, %d with rows cut for area, %d with columns narrowed for a row of two, %d tiles at most - the sweep did not reach the layouts it is for",
			n, seen.wide, seen.byArea, seen.narrowed, seen.mostTiles)
	}
	t.Logf("%d layouts, %d wide, %d cut for area, %d narrowed, %d tiles at most", n, seen.wide, seen.byArea, seen.narrowed, seen.mostTiles)
	if seen.reported > 0 {
		t.Errorf("%d breaks in %d layouts", seen.reported, n)
	}
}

type layoutsSeen struct {
	wide, byArea, narrowed, mostTiles, reported int
}

// layoutBreaks is every rule of the specification this layout of a w by h
// picture at fps breaks, and counts what kind of layout it is.
func layoutBreaks(w, h, fps int, cols, rows []int, seen *layoutsSeen) []string {
	var out []string
	sbCols, sbRows := (w+63)/64, (h+63)/64
	if sum(cols) != sbCols || sum(rows) != sbRows {
		return []string{fmt.Sprintf("the sizes add up to %dx%d superblocks and the picture is %dx%d", sum(cols), sum(rows), sbCols, sbRows)}
	}
	minTiles := max(log2Up(64, sbCols), log2Up(2304, sbRows*sbCols))
	area := sbRows * sbCols
	if minTiles > 0 {
		area >>= minTiles + 1
	}
	widest := 0
	for _, c := range cols {
		widest = max(widest, c)
	}
	tiles := len(cols) * len(rows)
	switch {
	case tiles == 1 && minTiles > 0:
		out = append(out, "one tile, and tile_info asks this frame for more")
	case tiles > 1:
		for _, r := range rows {
			if r > max(area/widest, 1) {
				out = append(out, fmt.Sprintf("a row of %d superblocks, and tile_info codes at most %d", r, max(area/widest, 1)))
			}
		}
	}
	if widest > 64 {
		out = append(out, fmt.Sprintf("a column of %d superblocks, wider than MAX_TILE_WIDTH", widest))
	}
	if len(cols) > 64 || len(rows) > 64 {
		out = append(out, "more than 64 columns or rows")
	}
	largest := 0
	for _, r := range rows {
		largest = max(largest, r*widest*64*64)
	}
	if largest > 4096*2304 {
		out = append(out, "a tile larger than MAX_TILE_AREA")
	}
	if limits, ok := annexATiles[video.LevelFor(w, h, fps, 0, 1)]; ok {
		if tiles > limits[0] || len(cols) > limits[1] {
			out = append(out, fmt.Sprintf("%d tiles in %d columns, and the level allows %d in %d", tiles, len(cols), limits[0], limits[1]))
		}
		if len(cols) > 1 && w-64*(sbCols-cols[len(cols)-1]) < 8 {
			out = append(out, "the last column is under 8 pixels")
		}
		if len(rows) > 1 && h-64*(sbRows-rows[len(rows)-1]) < 8 {
			out = append(out, "the last row is under 8 pixels")
		}
		if largest*fps > 588_251_136 {
			out = append(out, "the largest tile times the frames a second is over 588 251 136")
		}
	}
	if sbCols > 64 {
		seen.wide++
	}
	if minTiles > 0 && sbCols <= 64 {
		seen.byArea++
	}
	if sbCols > 64 && sbRows > 1 && area/2 < 64 {
		seen.narrowed++
	}
	seen.mostTiles = max(seen.mostTiles, tiles)
	return out
}

func sum(sizes []int) int {
	n := 0
	for _, s := range sizes {
		n += s
	}
	return n
}
