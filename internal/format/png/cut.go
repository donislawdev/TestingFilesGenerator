package png

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/bits"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// writeForACut writes a file a damage will cut after its first m.cutAt bytes,
// with the padding split around the picture so that the cut keeps the share of
// the picture it keeps of the whole file (format.Request.CutAt).
//
// With all of the padding behind the picture, as an uncut file has it, a large
// file cut at any share kept the whole picture and lost only padding. Before
// the picture and behind it are both places PNG allows an ancillary chunk -
// only the gap between two IDAT chunks is not. Pillow, FFmpeg and the Windows
// Imaging Component return the same pixels either way, and Tk and GDI+ read
// the picture (docs/TRUNCATE-2026-11-11.md section 7.3).
//
// The picture is encoded into memory first, because the padding in front of it
// is sized from its length, and the fast path in chooseSize does not know that
// while planning. It is the encoded picture, a gradient that compresses about
// 210 to 1, so it is small next to the picture the encoder reads (holdsPerPixel).
func writeForACut(ctx context.Context, w io.Writer, m memo, whole int64) error {
	var buf bytes.Buffer
	if err := encode(&buf, m); err != nil {
		return err
	}
	data := buf.Bytes()
	body := int64(len(data)) - iendSize
	if err := checkEncoded(data, body, m); err != nil {
		return err
	}
	pad := whole - body - iendSize
	if pad != 0 && (pad < chunkOverhead || pad-chunkOverhead > maxChunkData) {
		return core.Defect(fmt.Errorf("png: the picture encoded to %d B, which leaves %d B of padding for a %d B file", body, pad, whole))
	}
	front := frontPadding(pad, body-headSize, m.cutAt, whole)
	markAroundThePicture(w, body, front, pad-front)

	pieces := []struct {
		bytes   []byte
		padding int64
	}{
		{bytes: data[:headSize], padding: front},
		{bytes: data[headSize:body], padding: pad - front},
		{bytes: data[body:]},
	}
	for _, p := range pieces {
		if _, err := w.Write(p.bytes); err != nil {
			return err
		}
		if p.padding == 0 {
			continue
		}
		// Both chunks draw from the file's seed. Nobody reads padding, and the
		// one behind the picture lies past any cut.
		if err := writePaddingChunk(ctx, w, paddingChunk, m.seed, p.padding-chunkOverhead); err != nil {
			return err
		}
	}
	return nil
}

// checkEncoded asks the encoded picture what the writer that streams it asks:
// that it is the length planning measured, where planning measured one, and
// that it ends with the closing chunk.
func checkEncoded(data []byte, body int64, m memo) error {
	if m.bodyKnown && body != m.body {
		return core.Defect(fmt.Errorf("png: the picture encoded to %d B where planning said %d B", body, m.body))
	}
	if body < headSize || string(data[body+4:body+8]) != "IEND" {
		return core.Defect(fmt.Errorf("png: the encoded stream does not end with IEND"))
	}
	return nil
}

// frontPadding is how much of the padding, chunk headers included, goes in
// front of the picture, so that a cut after the first cutAt bytes of a whole
// file keeps content*cutAt/whole bytes of the picture's content.
//
// A chunk costs twelve bytes before any padding, so each side is nothing or at
// least that. Where the chunks cannot land exactly, the front grows, which
// moves the picture later and cuts more of it - the "at least" missing
// promises. The one way it can keep more is a tail too short for the content
// the cut should take: there all of the padding is in front, and the cut keeps
// up to a chunk's worth more than the share (docs/TRUNCATE-2026-11-11.md
// section 7.4). The manifest's cut reports what was kept either way.
func frontPadding(pad, content, cutAt, whole int64) int64 {
	if pad == 0 || cutAt <= 0 || cutAt >= whole {
		return 0
	}
	// content*cutAt in 128 bits, because a file near the top of what one chunk
	// carries times a cut near it passes an int64. The quotient fits, because
	// cutAt is less than whole.
	hi, lo := bits.Mul64(uint64(content), uint64(cutAt))
	kept, _ := bits.Div64(hi, lo, uint64(whole))
	front := cutAt - headSize - int64(kept)
	switch {
	case front <= 0:
		return 0
	case front < chunkOverhead:
		front = chunkOverhead
	}
	if back := pad - front; back < chunkOverhead {
		// Behind the picture there is room for nothing or for less than a
		// chunk, so the whole padding goes in front.
		return pad
	}
	return front
}

// markAroundThePicture marks the map of a file whose padding is split around
// its picture: the header, the padding in front, the picture, the padding
// behind and the closing chunk. A side with no padding has no chunk and no
// region.
func markAroundThePicture(w io.Writer, body, front, back int64) {
	format.Mark(w, format.RoleStructure, 0, headSize)
	at := int64(headSize)
	if front > 0 {
		format.Mark(w, format.RolePadding, at, front)
		at += front
	}
	format.Mark(w, format.RoleContent, at, body-headSize)
	at += body - headSize
	if back > 0 {
		format.Mark(w, format.RolePadding, at, back)
		at += back
	}
	format.Mark(w, format.RoleEnd, at, iendSize)
}
