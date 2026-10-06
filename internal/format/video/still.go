package video

import (
	"fmt"

	"github.com/gen2brain/gav1d/av1"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// encodeSpeed is gav1d's least searching setting, the one AVIF uses and for
// the same reason: the picture is a gradient with a label, and a run of ten
// thousand files should not spend its time on it.
const encodeSpeed = 10

// Coded is one picture through gav1d, taken apart into what this package
// wraps in headers of its own: the frame header bits from tile_info to the end
// of the header, and the tile data after them.
type Coded struct {
	suffix     []byte
	suffixBits uint
	tile       []byte
}

// Size is how many bytes the picture's tile data takes, which is what every
// frame that carries the picture costs above its few bytes of header.
func (c Coded) Size() int { return len(c.tile) }

func (c Coded) writeSuffix(w *bitWriter) {
	r := bitReader{b: c.suffix}
	for range c.suffixBits {
		w.bit(r.bit())
	}
}

// Encode codes a picture.
//
// gav1d's output is read here by the specification rather than by what
// gav1d's writer happens to do, and anything other than the one shape this
// package knows how to re-wrap is refused as a defect. That refusal is the
// point: a gav1d raised underneath us that changed its header would otherwise
// go on producing files, and every one of them would carry a header this
// package wrote for a different encoder.
func Encode(p Planes, qindex int) (Coded, error) {
	out := av1.Encode(av1.EncodeConfig{
		Width: p.Width, Height: p.Height, BitDepth: 8, QIndex: qindex, Speed: encodeSpeed,
		Src: p.Y, SrcStride: p.Width, SrcU: p.U, SrcV: p.V, SrcUVStride: (p.Width + 1) / 2,
	})
	if out == nil {
		return Coded{}, core.Defect(fmt.Errorf("video: the encoder refused a %dx%d picture at quantizer %d", p.Width, p.Height, qindex))
	}
	units, err := splitUnits(out)
	if err != nil {
		return Coded{}, core.Defect(fmt.Errorf("video: the encoder's output is not a sequence of OBUs: %w", err))
	}
	if len(units) != 3 || units[0].typ != obuTemporalDelimiter || units[1].typ != obuSequenceHeader || units[2].typ != obuFrame {
		return Coded{}, core.Defect(fmt.Errorf("video: the encoder wrote %s where a temporal delimiter, a sequence header and one frame were expected", describeUnits(units)))
	}
	c, err := splitFrame(units[2].payload, p.Width, p.Height)
	if err != nil {
		return Coded{}, core.Defect(fmt.Errorf("video: the encoder's frame header is not the one this package re-wraps: %w", err))
	}
	return c, nil
}

type unit struct {
	typ     int
	payload []byte
}

func splitUnits(b []byte) ([]unit, error) {
	var out []unit
	for len(b) > 0 {
		h := b[0]
		if h&0x80 != 0 || h&0x04 != 0 || h&0x02 == 0 {
			return nil, fmt.Errorf("an OBU header %#02x with the forbidden bit, an extension or no size", h)
		}
		size, used := readLeb128(b[1:])
		if used == 0 || 1+used+size > len(b) {
			return nil, fmt.Errorf("an OBU whose size runs past the end")
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
// header, key frame, shown - and keeps everything from tile_info on.
//
// The three bits before tile_info are disable_cdf_update,
// allow_screen_content_tools and render_and_frame_size_different, and all
// three have to be zero, because the headers this package writes say exactly
// that. From tile_info to reduced_tx_set the fields are the ones every intra
// frame carries, so they are copied as they are. Anything this reader does not
// expect - more than one tile, a quantizer matrix, segmentation, a delta
// quantizer, loop filter deltas - is refused, because none of it is something
// the copied bits would still mean once they sit under another header.
func splitFrame(frame []byte, width, height int) (Coded, error) {
	r := bitReader{b: frame}
	if r.bits(3) != 0 {
		return Coded{}, fmt.Errorf("disable_cdf_update, allow_screen_content_tools or render_and_frame_size_different is set")
	}
	start := r.pos
	if err := readTileInfo(&r, width, height); err != nil {
		return Coded{}, err
	}
	if err := readQuantizerToTxSet(&r); err != nil {
		return Coded{}, err
	}
	end := r.pos
	for r.pos%8 != 0 {
		if r.bit() != 0 {
			return Coded{}, fmt.Errorf("byte_alignment after the header is not zero")
		}
	}
	if r.overrun || r.pos/8 >= uint(len(frame)) {
		return Coded{}, fmt.Errorf("the header runs to the end of the frame and leaves no tile")
	}

	var w bitWriter
	copyBits := bitReader{b: frame, pos: start}
	for copyBits.pos < end {
		w.bit(copyBits.bit())
	}
	return Coded{suffix: w.bytes(), suffixBits: end - start, tile: frame[r.pos/8:]}, nil
}

// readTileInfo walks tile_info with uniform spacing, section 5.9.15, and
// accepts one tile only - the size of a frame this package codes stays within
// what one tile holds, and a frame that does not is one the copied header
// would describe wrongly.
func readTileInfo(r *bitReader, width, height int) error {
	miCols := 2 * ((width + 7) >> 3)
	miRows := 2 * ((height + 7) >> 3)
	sbCols := (miCols + 15) >> 4
	sbRows := (miRows + 15) >> 4
	minLog2TileCols := tileLog2(4096>>6, sbCols)
	maxLog2TileCols := tileLog2(1, min(sbCols, 64))
	maxLog2TileRows := tileLog2(1, min(sbRows, 64))
	minLog2Tiles := max(minLog2TileCols, tileLog2((4096*2304)>>12, sbRows*sbCols))

	if r.bit() != 1 {
		return fmt.Errorf("uniform_tile_spacing_flag is not set")
	}
	cols := minLog2TileCols
	for cols < maxLog2TileCols && r.bit() == 1 {
		cols++
	}
	rows := max(minLog2Tiles-cols, 0)
	for rows < maxLog2TileRows && r.bit() == 1 {
		rows++
	}
	if cols > 0 || rows > 0 {
		return fmt.Errorf("a %dx%d frame is coded as more than one tile", width, height)
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
			return fmt.Errorf("a quantizer delta is coded")
		}
	}
	if r.bit() == 1 {
		return fmt.Errorf("using_qmatrix is set")
	}
	if r.bit() == 1 {
		return fmt.Errorf("segmentation_enabled is set")
	}
	if baseQ > 0 && r.bit() == 1 {
		return fmt.Errorf("delta_q_present is set")
	}
	if !lossless {
		level0, level1 := r.bits(6), r.bits(6)
		if level0 != 0 || level1 != 0 {
			r.bits(12) // loop_filter_level[2] and [3]
		}
		r.bits(3) // loop_filter_sharpness
		if r.bit() == 1 {
			return fmt.Errorf("loop_filter_delta_enabled is set")
		}
		r.bit() // tx_mode_select
	}
	r.bit() // reduced_tx_set
	if r.overrun {
		return fmt.Errorf("the frame ends inside its header")
	}
	return nil
}
