package video

import (
	"fmt"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// OBU types this package writes, AV1 specification section 6.2.2.
const (
	obuSequenceHeader    = 1
	obuTemporalDelimiter = 2
	obuFrameHeader       = 3
	obuFrame             = 6
)

// The colour every picture here is in, said in the sequence header so that a
// player does not have to guess: BT.709 primaries, transfer and matrix, studio
// range. That is what web video is, and an AV1 stream with no description is
// read as BT.709 by some players and as BT.601 by others - the same file in two
// colours. The numbers are the ones ISO/IEC 23091-4 gives, which the AV1
// specification uses as they are.
const (
	colourPrimariesBT709 = 1
	transferBT709        = 1
	matrixBT709          = 1
)

// copySlot is the reference slot the hidden copy is kept in. A shown key frame
// fills all eight slots and leaves none of them showable, so the copy can take
// any of them - the number only has to be the same in the frame that fills it
// and in every frame that shows it.
const copySlot = 1

// obu wraps a payload with the header both containers take: no extension, and
// the size carried in the header.
func obu(typ int, payload []byte) []byte {
	return obuOf(typ, payload, nil)
}

// obuOf is an OBU whose payload is a header followed by data, written into
// one allocation of the right size - the data is a picture's tile, and copying
// it twice to join it to its header is the cost this avoids.
func obuOf(typ int, header, data []byte) []byte {
	size := leb128(len(header) + len(data))
	out := make([]byte, 0, 1+len(size)+len(header)+len(data))
	out = append(out, byte(typ<<3)|0x02)
	out = append(out, size...)
	out = append(out, header...)
	return append(out, data...)
}

// sequenceHeader is the payload of a full sequence header, not the reduced
// still picture one gav1d writes, with the same coding tools gav1d's tile data
// was coded under: 64 by 64 superblocks, no filter intra, intra edge filter on,
// no superres, no CDEF, no loop restoration, 8 bit 4:2:0. Every inter tool is
// off and there is no order hint, because no frame here is predicted from
// another - repeating a picture is done by showing it again.
//
// The field order is the specification's, section 5.5, and the reason for
// each zero is written beside it.
func sequenceHeader(width, height, level int) []byte {
	w := bitWriter{buf: make([]byte, 0, 16)}
	w.bits(0, 3) // seq_profile: Main
	w.bit(0)     // still_picture
	w.bit(0)     // reduced_still_picture_header
	w.bit(0)     // timing_info_present_flag: the container keeps the time
	w.bit(0)     // initial_display_delay_present_flag
	w.bits(0, 5) // operating_points_cnt_minus_1: one operating point
	w.bits(0, 12)
	w.bits(uint32(level), 5)
	if level > 7 {
		w.bit(0) // seq_tier: Main
	}
	wn, hn := bitsFor(width-1), bitsFor(height-1)
	w.bits(uint32(wn-1), 4)
	w.bits(uint32(hn-1), 4)
	w.bits(uint32(width-1), wn)
	w.bits(uint32(height-1), hn)
	w.bit(0) // frame_id_numbers_present_flag
	w.bit(0) // use_128x128_superblock
	w.bit(0) // enable_filter_intra
	w.bit(1) // enable_intra_edge_filter
	w.bit(0) // enable_interintra_compound
	w.bit(0) // enable_masked_compound
	w.bit(0) // enable_warped_motion
	w.bit(0) // enable_dual_filter
	w.bit(0) // enable_order_hint
	w.bit(0) // seq_choose_screen_content_tools
	w.bit(0) // seq_force_screen_content_tools: off, so frames do not say it
	w.bit(0) // enable_superres
	w.bit(0) // enable_cdef
	w.bit(0) // enable_restoration
	// color_config
	w.bit(0) // high_bitdepth
	w.bit(0) // mono_chrome
	w.bit(1) // color_description_present_flag
	w.bits(colourPrimariesBT709, 8)
	w.bits(transferBT709, 8)
	w.bits(matrixBT709, 8)
	w.bit(0)     // color_range: studio swing
	w.bits(0, 2) // chroma_sample_position: unknown, as gav1d declares it
	w.bit(0)     // separate_uv_delta_q
	w.bit(0)     // film_grain_params_present
	w.trailing()
	return w.bytes()
}

func bitsFor(v int) int {
	n := 1
	for v>>uint(n) != 0 {
		n++
	}
	return n
}

// codecConfig is the AV1CodecConfigurationRecord both containers carry (the
// av1C box in MP4, CodecPrivate in Matroska), followed by the sequence header
// OBU it describes. Byte two is seq_tier 0, 8 bit, not 12 bit, not
// monochrome, subsampled in both directions, sample position unknown.
func codecConfig(level int, sequence []byte) []byte {
	return append([]byte{0x81, byte(level & 0x1f), 0x0C, 0x00}, sequence...)
}

// The two frames a picture is carried in.
const (
	// keyKind is a shown KEY_FRAME. Shown, it is a place to start playing,
	// and the specification makes it impossible to show again.
	keyKind = iota
	// copyKind is the same picture as an INTRA_ONLY frame that is not shown
	// now and is showable later, kept in copySlot.
	copyKind
)

// headerStart writes a frame's uncompressed header up to tile_info.
func headerStart(w *bitWriter, kind int) {
	w.bit(0) // show_existing_frame
	if kind == keyKind {
		w.bits(0, 2) // frame_type: KEY_FRAME
		w.bit(1)     // show_frame
		// error_resilient_mode is 1 by rule for a shown key frame, not written
		w.bit(0) // disable_cdf_update
		w.bit(0) // frame_size_override_flag
		// refresh_frame_flags is every slot by rule, not written
		w.bit(0) // render_and_frame_size_different
		w.bit(1) // disable_frame_end_update_cdf
		return
	}
	w.bits(2, 2) // frame_type: INTRA_ONLY_FRAME
	w.bit(0)     // show_frame
	w.bit(1)     // showable_frame
	w.bit(0)     // error_resilient_mode
	w.bit(0)     // disable_cdf_update
	w.bit(0)     // frame_size_override_flag
	w.bits(1<<copySlot, 8)
	w.bit(0) // render_and_frame_size_different
	w.bit(1) // disable_frame_end_update_cdf
}

// frameShape is what a frame of a film needs besides its tiles: the layout
// and how many bytes each tile's size takes.
type frameShape struct {
	grid          grid
	tileSizeBytes int
}

// sample is before, the frame OBU carrying the picture's tiles, then after,
// written over dst when it has room and into one allocation of the right size
// when it has not - a film's pictures are made per change, and what a file
// allocates is counted.
//
// The frame is 06.bitstream.syntax.md "Frame OBU syntax" (lines 1748-1764):
// the header to tile_info, the layout's tile_info, the tiles' common header
// bits, byte_alignment, and the tile group (lines 1771-1816). With several
// tiles the group opens with tile_start_and_end_present_flag 0, which an
// OBU_FRAME has to carry (07.bitstream.semantics.md lines 2439-2443), and its
// byte_alignment, and every tile but the last follows its size less one in
// tileSizeBytes little endian bytes. One tile is its data alone, as gav1d
// wrote it.
func (s frameShape) sample(dst, before []byte, kind int, tiles []tileCoded, after []byte) ([]byte, error) {
	var scratch [64]byte
	w := bitWriter{buf: scratch[:0]}
	headerStart(&w, kind)
	s.grid.writeTileInfo(&w, s.tileSizeBytes)
	tiles[0].writeRest(&w)
	w.align()
	group := 0
	if len(tiles) > 1 {
		group = 1 + (len(tiles)-1)*s.tileSizeBytes
	}
	payload := len(w.buf) + group
	for i, t := range tiles {
		if !t.sameHeader(tiles[0]) {
			return nil, core.Defect(fmt.Errorf("video: tile %d of a picture was coded under other header bits than tile 0, and one frame carries one header", i))
		}
		payload += len(t.data)
	}
	var sizeRoom [8]byte
	size := appendLeb128(sizeRoom[:0], payload)
	out := dst[:0]
	if total := len(before) + 1 + len(size) + payload + len(after); cap(out) < total {
		out = make([]byte, 0, total)
	}
	out = append(append(append(append(out, before...), byte(obuFrame<<3)|0x02), size...), w.buf...)
	if group > 0 {
		out = append(out, 0)
	}
	for i, t := range tiles {
		if i < len(tiles)-1 {
			out = appendLittleEndian(out, len(t.data)-1, s.tileSizeBytes)
		}
		out = append(out, t.data...)
	}
	return append(out, after...), nil
}

// appendLittleEndian is le(n), 04.conventions.md: v in n bytes, the lowest
// first.
func appendLittleEndian(out []byte, v, n int) []byte {
	for b := range n {
		out = append(out, byte(v>>(8*b)))
	}
	return out
}

// showCopy shows what copySlot holds, and is three bytes.
func showCopy() []byte {
	var w bitWriter
	w.bit(1) // show_existing_frame
	w.bits(copySlot, 3)
	w.trailing()
	return obu(obuFrameHeader, w.bytes())
}
