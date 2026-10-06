package video

import (
	"fmt"
	"image"

	"github.com/gen2brain/gav1d/av1"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// encodeSpeed is gav1d's least searching setting, the one AVIF uses and for
// the same reason: the picture is a gradient with a label, and a run of ten
// thousand files should not spend its time on it.
const encodeSpeed = 10

// tileCoded is one tile through gav1d, coded as a picture of its own and
// taken apart into what this package wraps in headers of its own: the frame
// header bits after tile_info - the quantizer to reduced_tx_set - and the
// tile data after them. tile_info itself is this package's, because it
// describes the frame the tile goes into rather than the picture gav1d coded.
//
// The bits are read where gav1d wrote them, from and to in frame, rather than
// copied out: a film codes a tile at every change, and what a file allocates
// is counted.
type tileCoded struct {
	frame    []byte
	from, to uint
	data     []byte
}

// sameHeader says whether two tiles were coded under the same header bits,
// which every tile of one frame has to be: gav1d writes them from the
// quantizer alone (gav1d v0.2.5, av1/encode_hdr.go, writeFrameHdr - the loop
// filter off, no delta, no tx_mode_select), measured on 2026-10-06 as one
// suffix per quantizer across 27 pictures of 9 sizes. A later gav1d that chose
// them from the picture would break that, and the frame is refused rather
// than written with one tile's header over another's data.
func (t tileCoded) sameHeader(o tileCoded) bool {
	if t.to-t.from != o.to-o.from {
		return false
	}
	a, b := bitReader{b: t.frame, pos: t.from}, bitReader{b: o.frame, pos: o.from}
	for range t.to - t.from {
		if a.bit() != b.bit() {
			return false
		}
	}
	return true
}

func (t tileCoded) writeRest(w *bitWriter) {
	r := bitReader{b: t.frame, pos: t.from}
	for r.pos < t.to {
		w.bit(r.bit())
	}
}

// encodeTile codes the part r of a picture as a picture of its own. gav1d
// reads its source a row at a time by the stride and never past Width and
// Height (av1/encode_intra.go, residualEdge, the edge clamped to sw-1 and
// sh-1), so the tile is read where it lies in the picture's planes rather
// than copied out of them. r starts on an even row and column, as a tile does,
// so its chroma is the picture's.
//
// gav1d's output is read here by the specification rather than by what
// gav1d's writer happens to do, and anything other than the one shape this
// package knows how to re-wrap is refused as a defect. That refusal is the
// point: a gav1d raised underneath us that changed its header would otherwise
// go on producing files, and every one of them would carry a header this
// package wrote for a different encoder.
func encodeTile(p Planes, r image.Rectangle, qindex int) (tileCoded, error) {
	cw := (p.Width + 1) / 2
	luma, chroma := r.Min.Y*p.Width+r.Min.X, (r.Min.Y/2)*cw+r.Min.X/2
	out := av1.Encode(av1.EncodeConfig{
		Width: r.Dx(), Height: r.Dy(), BitDepth: 8, QIndex: qindex, Speed: encodeSpeed,
		Src: p.Y[luma:], SrcStride: p.Width, SrcU: p.U[chroma:], SrcV: p.V[chroma:], SrcUVStride: cw,
	})
	if out == nil {
		return tileCoded{}, core.Defect(fmt.Errorf("video: the encoder refused a %dx%d picture at quantizer %d", r.Dx(), r.Dy(), qindex))
	}
	var room [4]unit
	units, err := splitUnits(out, room[:0])
	if err != nil {
		return tileCoded{}, core.Defect(fmt.Errorf("video: the encoder's output is not a sequence of OBUs: %w", err))
	}
	if len(units) != 3 || units[0].typ != obuTemporalDelimiter || units[1].typ != obuSequenceHeader || units[2].typ != obuFrame {
		return tileCoded{}, core.Defect(fmt.Errorf("video: the encoder wrote %s where a temporal delimiter, a sequence header and one frame were expected", describeUnits(units)))
	}
	t, err := splitFrame(units[2].payload, r.Dx(), r.Dy())
	if err != nil {
		return tileCoded{}, core.Defect(fmt.Errorf("video: the encoder's frame header is not the one this package re-wraps: %w", err))
	}
	return t, nil
}

type unit struct {
	typ     int
	payload []byte
}

// splitUnits appends the OBUs of b to out, which a caller gives room in so
// that taking apart the few OBUs of one tile allocates nothing.
func splitUnits(b []byte, out []unit) ([]unit, error) {
	for len(b) > 0 {
		h := b[0]
		if h&0x80 != 0 || h&0x04 != 0 || h&0x02 == 0 {
			return nil, core.Defect(fmt.Errorf("an OBU header %#02x with the forbidden bit, an extension or no size", h))
		}
		size, used := readLeb128(b[1:])
		if used == 0 || 1+used+size > len(b) {
			return nil, core.Defect(fmt.Errorf("an OBU whose size runs past the end"))
		}
		out = append(out, unit{typ: int(h>>3) & 0xf, payload: b[1+used : 1+used+size]})
		b = b[1+used+size:]
	}
	return out, nil
}

func describeUnits(units []unit) string {
	s := fmt.Sprintf("%d OBUs of types", len(units))
	for _, u := range units {
		s += fmt.Sprintf(" %d", u.typ)
	}
	return s
}

// splitFrame reads a still picture's frame header - reduced still picture
// header, key frame, shown - and keeps everything after tile_info.
//
// The three bits before tile_info are disable_cdf_update,
// allow_screen_content_tools and render_and_frame_size_different, and all
// three have to be zero, because the headers this package writes say exactly
// that. tile_info has to say one tile, the picture gav1d was given. After it,
// to reduced_tx_set, the fields are the ones every intra frame carries, so
// they are copied as they are. Anything this reader does not expect - more
// than one tile, a quantizer matrix, segmentation, a delta quantizer, loop
// filter deltas - is refused, because none of it is something the copied bits
// would still mean once they sit under another header.
func splitFrame(frame []byte, width, height int) (tileCoded, error) {
	r := bitReader{b: frame}
	if r.bits(3) != 0 {
		return tileCoded{}, core.Defect(fmt.Errorf("disable_cdf_update, allow_screen_content_tools or render_and_frame_size_different is set"))
	}
	if err := readTileInfo(&r, width, height); err != nil {
		return tileCoded{}, err
	}
	start := r.pos
	if err := readQuantizerToTxSet(&r); err != nil {
		return tileCoded{}, err
	}
	end := r.pos
	for r.pos%8 != 0 {
		if r.bit() != 0 {
			return tileCoded{}, core.Defect(fmt.Errorf("byte_alignment after the header is not zero"))
		}
	}
	if r.overrun || r.pos/8 >= uint(len(frame)) {
		return tileCoded{}, core.Defect(fmt.Errorf("the header runs to the end of the frame and leaves no tile"))
	}
	return tileCoded{frame: frame, from: start, to: end, data: frame[r.pos/8:]}, nil
}

// tileBounds is what tile_info derives from a frame's size before it reads a
// bit, 06.bitstream.syntax.md lines 1176-1192: the frame in superblocks, the
// fewest columns and tiles it may be cut into, and the most columns and rows.
// The reader below and the writer in grid.go both start from it.
type tileBounds struct {
	sbCols, sbRows            int
	minLog2Cols, maxLog2Cols  int
	maxLog2Rows, minLog2Tiles int
}

func boundsOf(width, height int) tileBounds {
	b := tileBounds{sbCols: sbOf(width), sbRows: sbOf(height)}
	b.minLog2Cols = tileLog2(maxTileWidth/superblock, b.sbCols)
	b.maxLog2Cols = tileLog2(1, min(b.sbCols, maxTileCols))
	b.maxLog2Rows = tileLog2(1, min(b.sbRows, maxTileRows))
	b.minLog2Tiles = max(b.minLog2Cols, tileLog2(maxTileArea/(superblock*superblock), b.sbRows*b.sbCols))
	return b
}

// MAX_TILE_WIDTH, MAX_TILE_AREA, MAX_TILE_ROWS and MAX_TILE_COLS, from the
// specification's table of symbols (03.symbols.md lines 41-44).
const (
	maxTileWidth = 4096
	maxTileArea  = 4096 * 2304
	maxTileRows  = 64
	maxTileCols  = 64
)

// readTileInfo walks tile_info with uniform spacing, section 5.9.15, and
// accepts one tile only - gav1d was given one tile's worth of picture, and a
// frame it cut into more is one whose data this package would misread.
func readTileInfo(r *bitReader, width, height int) error {
	b := boundsOf(width, height)
	if r.bit() != 1 {
		return core.Defect(fmt.Errorf("uniform_tile_spacing_flag is not set"))
	}
	cols := b.minLog2Cols
	for cols < b.maxLog2Cols && r.bit() == 1 {
		cols++
	}
	rows := max(b.minLog2Tiles-cols, 0)
	for rows < b.maxLog2Rows && r.bit() == 1 {
		rows++
	}
	if cols > 0 || rows > 0 {
		return core.Defect(fmt.Errorf("a %dx%d frame is coded as more than one tile", width, height))
	}
	return nil
}

func tileLog2(blk, target int) int {
	k := 0
	for blk<<uint(k) < target {
		k++
	}
	return k
}

// readQuantizerToTxSet walks the header from quantization_params to
// reduced_tx_set for a 4:2:0 intra frame with CDEF and loop restoration off,
// sections 5.9.12 to 5.9.24.
func readQuantizerToTxSet(r *bitReader) error {
	baseQ := r.bits(8)
	lossless := baseQ == 0
	for range 3 { // DeltaQYDc, DeltaQUDc, DeltaQUAc
		if r.bit() == 1 {
			return core.Defect(fmt.Errorf("a quantizer delta is coded"))
		}
	}
	if r.bit() == 1 {
		return core.Defect(fmt.Errorf("using_qmatrix is set"))
	}
	if r.bit() == 1 {
		return core.Defect(fmt.Errorf("segmentation_enabled is set"))
	}
	if baseQ > 0 && r.bit() == 1 {
		return core.Defect(fmt.Errorf("delta_q_present is set"))
	}
	if !lossless {
		level0, level1 := r.bits(6), r.bits(6)
		if level0 != 0 || level1 != 0 {
			r.bits(12) // loop_filter_level[2] and [3]
		}
		r.bits(3) // loop_filter_sharpness
		if r.bit() == 1 {
			return core.Defect(fmt.Errorf("loop_filter_delta_enabled is set"))
		}
		r.bit() // tx_mode_select
	}
	r.bit() // reduced_tx_set
	if r.overrun {
		return core.Defect(fmt.Errorf("the frame ends inside its header"))
	}
	return nil
}
