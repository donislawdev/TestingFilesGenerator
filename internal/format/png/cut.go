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
	front := format.SplitFront(pad, headSize, m.body-headSize, m.cutAt, whole, chunkOverhead)
	markAroundThePicture(w, m.body, front, pad-front)

	// Both padding chunks draw from the file's seed. Nobody reads padding, and
	// the one behind the picture lies past any cut. The padding in front goes
	// after the signature and the header chunk.
	split := &format.AfterTheHead{W: w, Head: headSize, Insert: func(w io.Writer, _ []byte) error {
		if front == 0 {
			return nil
		}
		return writePaddingChunk(ctx, w, paddingChunk, m.seed, front-chunkOverhead)
	}}
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
