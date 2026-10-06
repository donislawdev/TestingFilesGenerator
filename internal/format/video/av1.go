package video

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
	out := []byte{byte(typ<<3) | 0x02}
	out = append(out, leb128(len(payload))...)
	return append(out, payload...)
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
	var w bitWriter
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

// keyFrame is a shown KEY_FRAME around a coded picture. Shown, it is a place
// to start playing, and the specification makes it impossible to show again.
func keyFrame(c Coded) []byte {
	var w bitWriter
	w.bit(0)     // show_existing_frame
	w.bits(0, 2) // frame_type: KEY_FRAME
	w.bit(1)     // show_frame
	// error_resilient_mode is 1 by rule for a shown key frame, not written
	w.bit(0) // disable_cdf_update
	w.bit(0) // frame_size_override_flag
	// refresh_frame_flags is every slot by rule, not written
	w.bit(0) // render_and_frame_size_different
	w.bit(1) // disable_frame_end_update_cdf
	c.writeSuffix(&w)
	w.align()
	return obu(obuFrame, append(w.bytes(), c.tile...))
}

// hiddenCopy is the same picture as an INTRA_ONLY frame that is not shown now
// and is showable later, kept in copySlot.
func hiddenCopy(c Coded) []byte {
	var w bitWriter
	w.bit(0)     // show_existing_frame
	w.bits(2, 2) // frame_type: INTRA_ONLY_FRAME
	w.bit(0)     // show_frame
	w.bit(1)     // showable_frame
	w.bit(0)     // error_resilient_mode
	w.bit(0)     // disable_cdf_update
	w.bit(0)     // frame_size_override_flag
	w.bits(1<<copySlot, 8)
	w.bit(0) // render_and_frame_size_different
	w.bit(1) // disable_frame_end_update_cdf
	c.writeSuffix(&w)
	w.align()
	return obu(obuFrame, append(w.bytes(), c.tile...))
}

// showCopy shows what copySlot holds, and is three bytes.
func showCopy() []byte {
	var w bitWriter
	w.bit(1) // show_existing_frame
	w.bits(copySlot, 3)
	w.trailing()
	return obu(obuFrameHeader, w.bytes())
}
