package gif

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// loopExtensionSize is the application extension the encoder writes after the
// colour table of a picture that moves: introducer, label, block size, the
// eleven bytes of NETSCAPE2.0, a three byte sub block with the loop count, and
// the terminator.
const loopExtensionSize = 3 + 11 + 4 + 1

// loopExtensionStart is how that extension opens.
var loopExtensionStart = []byte("\x21\xff\x0bNETSCAPE2.0")

// writeForACut writes a file a damage will cut after its first m.cutAt bytes,
// with the padding split into a comment in front of the first frame and one in
// front of the trailer, so that the cut keeps the share of the frames it keeps
// of the whole file (format.Request.CutAt).
//
// With all of the padding behind the frames, as an uncut file has it, a large
// file cut at any share kept the whole picture and lost only padding. GIF89a
// lets a comment stand wherever a block can begin, the beginning of the data
// stream included (section 24), but not in front of the colour table, which
// belongs to the screen descriptor (appendix B). The comment in front goes after
// the head the encoder writes - the descriptor, the colour table and, for a
// picture that moves, the loop extension - where Pillow, FFmpeg, ImageMagick,
// Go, the Windows Imaging Component, GDI+, exiftool, Tk and Chrome return the
// same frames, the same loop count and the same pixels as for the file uncut
// (docs/TRUNCATE-2026-11-11.md section 9.3).
//
// The frames stream out as the encoder writes them, so the cut stops the
// encoder where it falls. The comment in front is sized from their length, so a
// plan that did not encode the picture encodes it here first, which is what
// every GIF below the ladder ceiling pays at planning today.
func writeForACut(ctx context.Context, w io.Writer, m memo, whole int64) error {
	if !m.bodyKnown {
		body, err := encodedBodySize(m)
		if err != nil {
			return err
		}
		m.body, m.bodyKnown = body, true
	}
	if err := settlePadding(&m, whole, m.body+trailerSize); err != nil {
		return core.Defect(fmt.Errorf("gif: %w - ladderCeiling is wrong", err))
	}
	head := headBytes(m)
	pad := whole - m.body - trailerSize
	front := frontPadding(pad, m.body-head, head, m.cutAt, whole)
	markAroundThePicture(w, m.body, head, front, pad-front)

	// Both comments draw from the file's seed. Nobody reads padding, and the
	// comment behind the frames lies past any cut.
	split := &format.AfterTheHead{W: w, Head: head, Insert: func(w io.Writer, got []byte) error {
		if err := checkHead(got, m); err != nil {
			return err
		}
		return writePadding(ctx, w, m.seed, front)
	}}
	holder := &format.TailHolder{W: split, Keep: trailerSize}
	if err := encode(holder, m); err != nil {
		return err
	}
	if holder.Written != m.body {
		return core.Defect(fmt.Errorf("gif: the picture encoded to %d B where planning said %d B", holder.Written, m.body))
	}
	if holder.Tail[0] != 0x3B {
		return core.Defect(fmt.Errorf("gif: the encoded stream does not end with the trailer"))
	}
	if err := writePadding(ctx, w, m.seed, pad-front); err != nil {
		return err
	}
	_, err := w.Write(holder.Tail)
	return err
}

// headBytes is how long the head the encoder writes in front of the first
// frame is: the signature and screen descriptor, the colour table, and the loop
// extension when the picture moves. The map counts it as structure - the colour
// table is a table the frames are read through, and the frames are what a
// reader hands back (docs/TRUNCATE-2026-11-11.md section 9.4).
func headBytes(m memo) int64 {
	n := int64(screenHeader + 3*paletteSize(m.width, m.height))
	if m.frames > 1 {
		n += loopExtensionSize
	}
	return n
}

// checkHead is whether the head that went out before the comment is the head
// the encoder wrote: the screen descriptor, the colour table its packed field
// announces, of the size planning counted, and for a picture that moves the
// loop extension right after it - nothing more and nothing less. A head of
// another length puts the comment inside the colour table or a frame.
func checkHead(got []byte, m memo) error {
	if len(got) < screenHeader {
		return core.Defect(fmt.Errorf("gif: %d B went out before the comment, less than the screen descriptor", len(got)))
	}
	table, packed := 0, got[10]
	if packed&0x80 != 0 {
		table = 3 << (packed&0x07 + 1)
	}
	if want := 3 * paletteSize(m.width, m.height); table != want {
		return core.Defect(fmt.Errorf("gif: the encoder's colour table is %d B where planning counted %d B", table, want))
	}
	head := screenHeader + table
	if m.frames > 1 {
		head += loopExtensionSize
	}
	if len(got) != head {
		return core.Defect(fmt.Errorf("gif: %d B went out before the comment where the encoder's head is %d B", len(got), head))
	}
	if m.frames > 1 {
		loop := got[screenHeader+table:]
		if !bytes.HasPrefix(loop, loopExtensionStart) || loop[len(loop)-1] != 0 {
			return core.Defect(fmt.Errorf("gif: the encoder did not write the loop extension right after the colour table"))
		}
	}
	return nil
}

// frontPadding is how much of the padding, as a comment, goes in front of the
// first frame, so that a cut after the first cutAt bytes of a whole file keeps
// frames*cutAt/whole bytes of the frames.
//
// A comment is nothing, three bytes or at least five, so both sides are
// rounded to that. Where they cannot land exactly, the front grows - at most
// four bytes - which moves the frames later and cuts more of them, the "at
// least" missing promises. The one way it can keep more is a padding too short
// for the frames the cut should take: there all of it goes in front, and the cut
// keeps at most one byte more than the share (docs/TRUNCATE-2026-11-11.md
// section 9.5). The manifest's cut reports what was kept either way.
func frontPadding(pad, frames, head, cutAt, whole int64) int64 {
	if pad == 0 || cutAt <= 0 || cutAt >= whole {
		return 0
	}
	front := format.PaddingInFront(head, frames, cutAt, whole)
	if front <= 0 {
		return 0
	}
	for ; front < pad; front++ {
		if isComment(front) && isComment(pad-front) {
			return front
		}
	}
	return pad
}

// isComment is whether n bytes can be spent as one comment extension, or as
// none at all.
func isComment(n int64) bool {
	return n == 0 || n == emptyComment || n >= smallestCarryingComment
}

// writePadding writes n bytes of padding as one comment, or nothing for n = 0.
func writePadding(ctx context.Context, w io.Writer, seed uint64, n int64) error {
	if n == 0 {
		return nil
	}
	blocks, payload := commentShape(n)
	return writeComment(ctx, w, seed, blocks, payload)
}

// markAroundThePicture marks the map of a file whose padding is split around
// its frames: the head, the comment in front, the frames, the comment behind
// and the trailer. A side with no padding has no comment and no region.
func markAroundThePicture(w io.Writer, body, head, front, back int64) {
	format.Mark(w, format.RoleStructure, 0, head)
	at := head
	if front > 0 {
		format.Mark(w, format.RolePadding, at, front)
		at += front
	}
	format.Mark(w, format.RoleContent, at, body-head)
	at += body - head
	if back > 0 {
		format.Mark(w, format.RolePadding, at, back)
		at += back
	}
	format.Mark(w, format.RoleEnd, at, trailerSize)
}
