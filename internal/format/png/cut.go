package png

import (
	"context"
	"fmt"
	"io"

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
// The picture streams out as the encoder writes it, as in Write, so the cut
// stops the encoder where it falls instead of after the whole picture. The
// padding in front of it is sized from its length, so a plan that did not
// measure the picture measures it here first. That is the fast path in
// chooseSize, which only ever takes the top rung of the ladder, a couple of
// milliseconds to encode.
func writeForACut(ctx context.Context, w io.Writer, m memo, whole int64) error {
	if !m.bodyKnown {
		body, err := encodedBodySize(m)
		if err != nil {
			return err
		}
		m.body, m.bodyKnown = body, true
	}
	pad := whole - m.body - iendSize
	if pad < chunkOverhead || pad-chunkOverhead > maxChunkData {
		return core.Defect(fmt.Errorf("png: the picture encodes to %d B, which leaves %d B of padding for a %d B file", m.body, pad, whole))
	}
	front := frontPadding(pad, m.body-headSize, m.cutAt, whole)
	markAroundThePicture(w, m.body, front, pad-front)

	// Both padding chunks draw from the file's seed. Nobody reads padding, and
	// the one behind the picture lies past any cut.
	split := &afterTheHead{ctx: ctx, w: w, seed: m.seed, head: headSize, front: front}
	holder := &format.TailHolder{W: split, Keep: iendSize}
	if err := encode(holder, m); err != nil {
		return err
	}
	if holder.Written != m.body {
		return core.Defect(fmt.Errorf("png: the picture encoded to %d B where planning said %d B", holder.Written, m.body))
	}
	if string(holder.Tail[4:8]) != "IEND" {
		return core.Defect(fmt.Errorf("png: the encoded stream does not end with IEND"))
	}
	if back := pad - front; back > 0 {
		if err := writePaddingChunk(ctx, w, paddingChunk, m.seed, back-chunkOverhead); err != nil {
			return err
		}
	}
	_, err := w.Write(holder.Tail)
	return err
}

// afterTheHead passes the encoded picture on, and writes the padding that goes
// in front of the picture's data once the first head bytes - the signature and
// the header chunk - are out.
type afterTheHead struct {
	ctx  context.Context
	w    io.Writer
	seed uint64
	// head is how many bytes are still to pass before the padding.
	head int64
	// front is the padding, its chunk's header and checksum included, and 0
	// once it is written or when there is none.
	front int64
}

func (a *afterTheHead) Write(p []byte) (int, error) {
	n := 0
	if a.head > 0 {
		k := min(a.head, int64(len(p)))
		written, err := a.w.Write(p[:k])
		n, a.head = written, a.head-int64(written)
		if err != nil || a.head > 0 {
			return n, err
		}
		p = p[k:]
	}
	if a.front > 0 {
		front := a.front
		a.front = 0
		if err := writePaddingChunk(a.ctx, a.w, paddingChunk, a.seed, front-chunkOverhead); err != nil {
			return n, err
		}
	}
	if len(p) == 0 {
		return n, nil
	}
	written, err := a.w.Write(p)
	return n + written, err
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
	front := format.PaddingInFront(headSize, content, cutAt, whole)
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
