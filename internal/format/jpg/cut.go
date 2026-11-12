package jpg

import (
	"bytes"
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
// With all of the padding in front of the picture, as an uncut file has it, a
// cut of any share took the whole picture. Behind the picture JPEG has one
// place for bytes that mean nothing: fill bytes, X'FF', which ITU T.81 B.1.1.2
// lets stand in front of any marker - here the end of image marker. A comment
// behind the last scan is outside the syntax (B.2.1), and bytes after the end
// marker are outside the picture. Pillow, FFmpeg, ImageMagick, Go's own
// decoder, the Windows Imaging Component and GDI+ return the same pixels
// either way (docs/TRUNCATE-2026-11-11.md section 8.3).
//
// The fill is in no file anyone is handed: the cut falls in the picture, and
// the generator stops there. Only a whole file written for a guard has it.
func writeForACut(ctx context.Context, w io.Writer, m memo, whole int64) error {
	pad := whole - m.body
	front := frontPadding(pad, m.body-soiSize-eoiSize, m.cutAt, whole)
	markAroundThePicture(w, m.body, front, pad-front)

	if _, err := w.Write([]byte{0xFF, 0xD8}); err != nil {
		return err
	}
	if front > 0 {
		segments, payload := segmentsFor(front)
		if err := writeComments(ctx, w, m.seed, segments, payload); err != nil {
			return err
		}
	}

	// The encoder's own start marker is dropped, as in Write, and its end
	// marker held back, so the fill goes in front of it.
	holder := &format.TailHolder{W: w, Keep: eoiSize}
	skip := &headSkipper{w: holder, keep: soiSize}
	if err := encode(skip, m); err != nil {
		return err
	}
	if err := skip.check(); err != nil {
		return err
	}
	if skip.written+soiSize != m.body {
		return core.Defect(fmt.Errorf("jpg: the picture encoded to %d B where planning said %d B", skip.written+soiSize, m.body))
	}
	if !bytes.Equal(holder.Tail, []byte{0xFF, 0xD9}) {
		return core.Defect(fmt.Errorf("jpg: the encoded stream ends with % x, not the end of image marker", holder.Tail))
	}
	if err := writeFill(ctx, w, pad-front); err != nil {
		return err
	}
	_, err := w.Write(holder.Tail)
	return err
}

// frontPadding is how much of the padding, comment headers included, goes in
// front of the picture, so that a cut after the first cutAt bytes of a whole
// file keeps content*cutAt/whole bytes of the picture's content.
//
// A comment costs four bytes before any padding, so the front is nothing or at
// least that. Where it cannot land exactly it grows, which moves the picture
// later and cuts at most three bytes more of it - the "at least" missing
// promises. The fill behind the picture has no header, so any length is
// possible there. The one way the cut can keep more is padding too small for
// the front it should have: all of it goes in front, and the cut keeps at most
// two bytes more than the share, because the end marker counts in the file and
// not in the content (docs/TRUNCATE-2026-11-11.md section 8.5). The manifest's
// cut reports what was kept either way.
func frontPadding(pad, content, cutAt, whole int64) int64 {
	front := format.PaddingInFront(soiSize, content, cutAt, whole)
	switch {
	case front <= 0:
		return 0
	case front < comOverhead:
		front = comOverhead
	}
	return min(front, pad)
}

// markAroundThePicture marks the map of a file whose padding is split around
// its picture: the start marker, the comments in front, the picture, the fill
// behind and the end marker. A side with no padding has no region.
func markAroundThePicture(w io.Writer, body, front, back int64) {
	format.Mark(w, format.RoleStructure, 0, soiSize)
	at := int64(soiSize)
	if front > 0 {
		format.Mark(w, format.RolePadding, at, front)
		at += front
	}
	format.Mark(w, format.RoleContent, at, body-soiSize-eoiSize)
	at += body - soiSize - eoiSize
	if back > 0 {
		format.Mark(w, format.RolePadding, at, back)
		at += back
	}
	format.Mark(w, format.RoleEnd, at, eoiSize)
}

// writeFill writes n fill bytes, X'FF', in pieces of padChunkSize so that
// cancellation is noticed.
func writeFill(ctx context.Context, w io.Writer, n int64) error {
	if n <= 0 {
		return nil
	}
	buf := bytes.Repeat([]byte{0xFF}, int(min(n, padChunkSize)))
	for remaining := n; remaining > 0; {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		k := min(remaining, int64(len(buf)))
		if _, err := w.Write(buf[:k]); err != nil {
			return err
		}
		remaining -= k
	}
	return nil
}
