package guard

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gen2brain/gav1d/av1"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/video"
)

// A film's picture is cut into AV1 tiles, each coded by gav1d as a picture of
// its own, and a tile nothing changes is coded once a film and taken from what
// was coded every time after (internal/format/video, grid.go and tilekey.go,
// and docs/WEBM-WYDAJNOSC-2026-10-06.md sections 8 and 9). Read back from the
// file: the reader below knows tile_info and the tile group from the AV1
// specification (AOMediaCodec/av1-spec 5e04f3f, 06.bitstream.syntax.md lines
// 1176-1268 and 1771-1816), not from the writer.

// filmBits reads a frame header a bit at a time. Reading past its end
// gives zeros and says so in overrun, which the reader turns into an error -
// a panic here would end every guard of the package, not this one.
type filmBits struct {
	b       []byte
	pos     int
	overrun bool
}

func (r *filmBits) bit() int {
	if r.pos/8 >= len(r.b) {
		r.overrun = true
		return 0
	}
	v := int(r.b[r.pos/8]>>(7-r.pos%8)) & 1
	r.pos++
	return v
}

func (r *filmBits) bits(n int) int {
	v := 0
	for range n {
		v = v<<1 | r.bit()
	}
	return v
}

// ns is 04.conventions.md "ns(n)".
func (r *filmBits) ns(n int) int {
	w := 0
	for x := n; x > 0; x >>= 1 {
		w++
	}
	m := (1 << w) - n
	v := r.bits(w - 1)
	if v < m {
		return v
	}
	return (v << 1) - m + r.bit()
}

func log2Up(blk, target int) int {
	k := 0
	for blk<<k < target {
		k++
	}
	return k
}

// filmTiles is what one frame OBU of a film carries: the tiles' rectangles in
// frame order and each tile's data.
type filmTiles struct {
	rects [][4]int // x0, y0, x1, y1
	data  [][]byte
}

// readFilmTiles reads tile_info, the rest of an intra frame's header and the
// tile group of a frame OBU whose header has before bits ahead of tile_info.
func readFilmTiles(frame []byte, before, width, height int) (filmTiles, error) {
	r := &filmBits{b: frame, pos: before}
	cols, rows, sizeBytes := readFilmTileInfo(r, width, height)
	// quantization_params to reduced_tx_set for an intra 4:2:0 frame with
	// CDEF and loop restoration off: the three delta_coded flags, the matrix
	// and segmentation have to be off, and with them the frame is lossless
	// exactly when base_q_idx is 0, which leaves out the loop filter and the
	// tx mode.
	base := r.bits(8)
	if r.bits(5) != 0 {
		return filmTiles{}, fmt.Errorf("a quantizer delta, a matrix or segmentation in a film frame")
	}
	if base > 0 {
		if r.bit() != 0 {
			return filmTiles{}, fmt.Errorf("delta_q_present in a film frame")
		}
		if l0, l1 := r.bits(6), r.bits(6); l0|l1 != 0 {
			r.bits(12)
		}
		r.bits(3) // loop_filter_sharpness
		if r.bit() != 0 {
			return filmTiles{}, fmt.Errorf("loop_filter_delta_enabled in a film frame")
		}
		r.bit() // tx_mode_select
	}
	r.bit() // reduced_tx_set
	if r.overrun {
		return filmTiles{}, fmt.Errorf("the frame ends inside its header")
	}
	pos := (r.pos + 7) / 8
	var out filmTiles
	y := 0
	for _, rh := range rows {
		x := 0
		for _, cw := range cols {
			out.rects = append(out.rects, [4]int{x, y, min(width, x+64*cw), min(height, y+64*rh)})
			x += 64 * cw
		}
		y += 64 * rh
	}
	if len(out.rects) > 1 {
		if pos >= len(frame) {
			return filmTiles{}, fmt.Errorf("the frame ends before its tile group")
		}
		if frame[pos] != 0 {
			return filmTiles{}, fmt.Errorf("tile_start_and_end_present_flag is set in an OBU_FRAME")
		}
		pos++
	}
	for i := range out.rects {
		size := len(frame) - pos
		if i < len(out.rects)-1 {
			if pos+sizeBytes > len(frame) {
				return filmTiles{}, fmt.Errorf("the frame ends inside the size of tile %d", i)
			}
			size = 1
			for b := range sizeBytes {
				size += int(frame[pos+b]) << (8 * b)
			}
			pos += sizeBytes
		}
		if size < 0 || pos+size > len(frame) {
			return filmTiles{}, fmt.Errorf("tile %d of %d says %d B and %d remain", i, len(out.rects), size, len(frame)-pos)
		}
		out.data = append(out.data, frame[pos:pos+size])
		pos += size
	}
	return out, nil
}

// readFilmTileInfo is tile_info: the columns and rows in superblocks, and the
// bytes of each tile size.
func readFilmTileInfo(r *filmBits, width, height int) (cols, rows []int, sizeBytes int) {
	sbCols, sbRows := (2*((width+7)>>3)+15)>>4, (2*((height+7)>>3)+15)>>4
	minCols := log2Up(64, sbCols)
	maxCols, maxRows := log2Up(1, min(sbCols, 64)), log2Up(1, min(sbRows, 64))
	minTiles := max(minCols, log2Up(2304, sbRows*sbCols))
	var colsLog2, rowsLog2 int
	if r.bit() == 1 {
		colsLog2 = minCols
		for colsLog2 < maxCols && r.bit() == 1 {
			colsLog2++
		}
		rowsLog2 = max(minTiles-colsLog2, 0)
		for rowsLog2 < maxRows && r.bit() == 1 {
			rowsLog2++
		}
		cols, rows = uniformSizes(sbCols, colsLog2), uniformSizes(sbRows, rowsLog2)
	} else {
		widest := 0
		for start := 0; start < sbCols; {
			s := r.ns(min(sbCols-start, 64)) + 1
			cols, widest, start = append(cols, s), max(widest, s), start+s
		}
		area := sbRows * sbCols
		if minTiles > 0 {
			area >>= minTiles + 1
		}
		for start := 0; start < sbRows; {
			s := r.ns(min(sbRows-start, max(area/widest, 1))) + 1
			rows, start = append(rows, s), start+s
		}
		colsLog2, rowsLog2 = log2Up(1, len(cols)), log2Up(1, len(rows))
	}
	sizeBytes = 4
	if colsLog2 > 0 || rowsLog2 > 0 {
		r.bits(colsLog2 + rowsLog2)
		sizeBytes = r.bits(2) + 1
	}
	return cols, rows, sizeBytes
}

func uniformSizes(sb, log2 int) []int {
	step := (sb + (1 << log2) - 1) >> log2
	var out []int
	for s := 0; s < sb; s += step {
		out = append(out, min(step, sb-s))
	}
	return out
}

// frameOBU is the payload of the one frame OBU in a sample, and how many bits
// of its header come before tile_info: eight for a shown key frame, eighteen
// for a hidden intra only copy (the two this tool writes, film_test.go).
func frameOBU(sample []byte) ([]byte, int, error) {
	for len(sample) > 0 {
		size, used := 0, 0
		for i := 1; i < len(sample) && i < 9; i++ {
			size |= int(sample[i]&0x7f) << (7 * (i - 1))
			if sample[i]&0x80 == 0 {
				used = i
				break
			}
		}
		if used == 0 || 1+used+size > len(sample) {
			return nil, 0, fmt.Errorf("an OBU runs past its sample")
		}
		if typ := int(sample[0]>>3) & 0xf; typ == 6 {
			p := sample[1+used : 1+used+size]
			if len(p) == 0 {
				return nil, 0, fmt.Errorf("an empty frame OBU")
			}
			if frameType := p[0] >> 5 & 3; frameType == 0 {
				return p, 8, nil
			}
			return p, 18, nil
		}
		sample = sample[1+used+size:]
	}
	return nil, 0, fmt.Errorf("no frame OBU in the sample")
}

// filmTileCount is how many tiles the first frame of a film is cut into, read
// from the file - for the guards that have to know they asked about a film of
// tiles rather than a film of one.
func filmTileCount(t *testing.T, film []byte) int {
	t.Helper()
	f, err := walkWebM(film)
	if err != nil || len(f.blocks) == 0 {
		t.Fatalf("reading the film back: %v", err)
	}
	frame, before, err := frameOBU(f.blocks[0].data)
	if err != nil {
		t.Fatal(err)
	}
	tiles, err := readFilmTiles(frame, before, f.width, f.height)
	if err != nil {
		t.Fatal(err)
	}
	return len(tiles.rects)
}

// Every tile of a film is gav1d's own coding of the picture's pixels there,
// and decodes in the film exactly as it does alone.
//
// The first half holds the tile cache to the pixels: each tile in the file is
// the tail of what gav1d makes, here, of that tile cut from the picture
// painted whole (video.WholePicture) - so a key that missed something the
// tile's pixels depend on, a tile taken from the wrong place or coded from the
// wrong stride is a different tile. The pictures asked are the ones where a
// kept tile comes back: every step of the square, the clock's minute, and ten
// minutes in, where the seconds' tile shows 0:00 again. The second half holds
// the frame to the tiles: the film's frame, decoded by gav1d, is in each tile
// the picture gav1d decodes from that tile alone - tile_info, the sizes and
// the header bits the tiles share all have to be right for that.
//
// gav1d's decoder agreed with libaom and dav1d to the byte on frames of tiles
// (docs/WEBM-WYDAJNOSC-2026-10-06.md section 8.3), and it needs no tool
// installed, so this runs everywhere.
func TestEveryTileOfAFilmIsGav1dsCodingOfItsPixelsAndDecodesAsItDoesAlone(t *testing.T) {
	cases := []struct {
		props   map[string]string
		bytes   int64
		changes []int64
	}{
		{map[string]string{"width": "320", "height": "180", "duration": "11m"}, 8 << 20, []int64{0, 1, 2, 3, 9, 10, 11, 59, 60, 61, 599, 600, 601, 659}},
		{map[string]string{"width": "640", "height": "360", "duration": "3s", "change_interval": "100ms"}, 4 << 20, rangeOf(30)},
		{map[string]string{"width": "1001", "height": "563", "duration": "12s"}, 4 << 20, rangeOf(12)},
		{map[string]string{"width": "1920", "height": "1080", "duration": "2s"}, 4 << 20, rangeOf(2)},
		// Wider than a tile can be, so the rest of the width is cut in two,
		// and a row cut for the area tile_info leaves a tile. Then wider and
		// only a superblock and a pixel tall, where the columns are narrowed so
		// a row of tiles can be two superblocks (grid.go, cutToFit).
		{map[string]string{"width": "4240", "height": "1000", "duration": "2s"}, 4 << 20, rangeOf(2)},
		{map[string]string{"width": "4097", "height": "65", "duration": "3s"}, 4 << 20, rangeOf(3)},
	}
	tilesAsked, mostTiles := 0, 0
	for _, c := range cases {
		n, most := tilesOfFilmHold(t, c.props, c.bytes, c.changes)
		tilesAsked += n
		mostTiles = max(mostTiles, most)
	}
	if tilesAsked < 300 || mostTiles < 8 {
		t.Fatalf("%d tiles asked about, at most %d in one frame - the guard did not see films of tiles", tilesAsked, mostTiles)
	}
}

func rangeOf(n int64) []int64 {
	out := make([]int64, n)
	for i := range out {
		out[i] = int64(i)
	}
	return out
}

// tilesOfFilmHold makes one film and asks both halves about the pictures of
// the given changes, giving back how many tiles it asked about and the most in
// one frame.
func tilesOfFilmHold(t *testing.T, props map[string]string, size int64, changes []int64) (int, int) {
	t.Helper()
	b, facts, seed := filmWithSeed(t, filmTarget(size, props))
	f, err := walkWebM(b)
	if err != nil {
		t.Fatalf("%v: reading the film back: %v", props, err)
	}
	durationMs, _ := facts["duration_ms"].(int64)
	changeMs, _ := facts["change_interval_ms"].(int64)
	fps, _ := facts["frame_rate"].(int)
	quality, _ := facts["quality"].(int)
	tl, err := video.NewTimeline("webm", durationMs, 60_000, changeMs, fps)
	if err != nil {
		t.Fatal(err)
	}
	label := core.Label("webm", size, seed)
	var dec av1.Decoder
	if _, err := dec.DecodeOBUs(append([]byte{0x12, 0x00}, f.blocks[0].data...)); err != nil {
		t.Fatalf("%v: gav1d does not decode the film's first frame: %v", props, err)
	}
	asked, most := 0, 0
	for _, c := range changes {
		block := f.blocks[c*changeMs*int64(fps)/1000].data
		frame, before, err := frameOBU(block)
		if err != nil {
			t.Fatal(err)
		}
		tiles, err := readFilmTiles(frame, before, f.width, f.height)
		if err != nil {
			t.Fatalf("%v, picture %d: %v", props, c, err)
		}
		pics, err := dec.DecodeOBUs(append([]byte{0x12, 0x00}, block...))
		if err != nil || len(pics) != 1 {
			t.Fatalf("%v, picture %d: gav1d decoded %d pictures from its frame: %v", props, c, len(pics), err)
		}
		whole := video.WholePicture(f.width, f.height, seed, label, tl, c)
		for i, r := range tiles.rects {
			if why := tileHolds(whole, r, tiles.data[i], pics[0], (100-quality)*255/100); why != "" {
				t.Errorf("%v, picture %d, tile %d at %v: %s", props, c, i, r, why)
			}
		}
		asked, most = asked+len(tiles.rects), max(most, len(tiles.rects))
	}
	return asked, most
}

// tileHolds codes the tile r of whole through gav1d as a picture of its own
// and says what is wrong with the film's tile and with the film's decoded
// picture there, or nothing.
func tileHolds(whole video.Planes, r [4]int, inFilm []byte, decoded *av1.Picture, qindex int) string {
	w, h := r[2]-r[0], r[3]-r[1]
	cw, ch, pcw := (w+1)/2, (h+1)/2, (whole.Width+1)/2
	y, u, v := make([]uint8, w*h), make([]uint8, cw*ch), make([]uint8, cw*ch)
	for row := range h {
		copy(y[row*w:], whole.Y[(r[1]+row)*whole.Width+r[0]:][:w])
	}
	for row := range ch {
		copy(u[row*cw:], whole.U[(r[1]/2+row)*pcw+r[0]/2:][:cw])
		copy(v[row*cw:], whole.V[(r[1]/2+row)*pcw+r[0]/2:][:cw])
	}
	alone := av1.Encode(av1.EncodeConfig{Width: w, Height: h, BitDepth: 8, QIndex: qindex, Speed: 10,
		Src: y, SrcStride: w, SrcU: u, SrcV: v, SrcUVStride: cw})
	if alone == nil {
		return "gav1d refused the tile"
	}
	// Temporal delimiter (2 bytes), sequence header, then the frame: the
	// tile's data is the end of it, after a header of a few bytes.
	if !bytes.HasSuffix(alone, inFilm) || len(alone)-len(inFilm) > 64 {
		return fmt.Sprintf("the film's tile is %d B and is not the tail of gav1d's %d B coding of these pixels", len(inFilm), len(alone))
	}
	var d av1.Decoder
	pics, err := d.DecodeOBUs(alone)
	if err != nil || len(pics) != 1 {
		return fmt.Sprintf("gav1d does not decode its own tile: %v", err)
	}
	for plane, sub := range []int{0, 1, 1} {
		for row := range (h + sub) >> sub {
			got := decoded.Data[plane][(r[1]>>sub+row)*decoded.Stride[plane]+r[0]>>sub:][:(w+sub)>>sub]
			want := pics[0].Data[plane][row*pics[0].Stride[plane]:][:(w+sub)>>sub]
			if !bytes.Equal(got, want) {
				return fmt.Sprintf("plane %d row %d of the film's decoded picture differs from the tile decoded alone", plane, row)
			}
		}
	}
	return ""
}

// filmWithSeed is filmOne with the seed the file was made with, which its
// pictures are painted from.
func filmWithSeed(t *testing.T, target engine.Target) ([]byte, map[string]any, uint64) {
	t.Helper()
	dir := t.TempDir()
	opt := engine.Options{OutDir: dir, Seed: goldenSeed, Command: "test"}
	planned, err := engine.Plan([]engine.Target{target}, opt)
	if err != nil {
		t.Fatalf("planning %s: %v", target.Format, err)
	}
	if _, err := engine.Run(context.Background(), planned, opt); err != nil {
		t.Fatalf("running %s: %v", target.Format, err)
	}
	b, err := os.ReadFile(filepath.Join(dir, planned[0].Name))
	if err != nil {
		t.Fatal(err)
	}
	return b, planned[0].Plan.Properties, planned[0].Seed
}
