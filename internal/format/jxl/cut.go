package jxl

import (
	"context"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/isobmff"
)

// splitForACut is how much of the padding pad goes in front of the codestream
// box of a file a damage will cut after its first cutAt bytes, so that the cut
// keeps the share of the codestream it keeps of the whole file
// (format.Request.CutAt). The rest goes behind it, past the cut.
//
// With all of the padding in front of the codestream, as an uncut file has it,
// a cut of any share would take the codestream before any padding. The
// codestream knows nothing of where it sits in the file, so nothing else
// moves (docs/TRUNCATE-2026-11-11.md section 13). Zero puts all of the padding
// behind the codestream, where the share wants nothing in front.
func splitForACut(pad, codestream, cutAt, whole int64) int64 {
	return format.SplitFront(pad, int64(len(signature)+len(fileType)), codestream, cutAt, whole, boxHeader)
}

// writeAround writes the container around the codestream with front bytes of
// skip boxes in front of the codestream box and back bytes behind it, and
// marks its map: the signature and file type boxes, the padding in front, the
// codestream box, the padding behind. A side with no padding has no box and no
// region.
//
// Skip rather than free on both sides, for ffmpeg: see the package comment. A
// file nothing cuts has nothing behind its codestream, and the side behind it
// in a file a damage cuts lies past the cut, so no file on disk carries a box
// behind its codestream.
func writeAround(ctx context.Context, w io.Writer, m memo, coded []byte, front, back int64) error {
	head := int64(len(signature) + len(fileType))
	box := boxHeader + int64(len(coded))
	format.Mark(w, format.RoleStructure, 0, head)
	if front > 0 {
		format.Mark(w, format.RolePadding, head, front)
	}
	format.Mark(w, format.RoleContent, head+front, box)
	if back > 0 {
		format.Mark(w, format.RolePadding, head+front+box, back)
	}

	if _, err := w.Write(signature[:]); err != nil {
		return err
	}
	if _, err := w.Write(fileType[:]); err != nil {
		return err
	}
	// Both sides draw their filler from the file's seed. Nobody reads padding.
	if err := isobmff.WriteFreeSpace(ctx, w, "skip", m.seed, front); err != nil {
		return err
	}
	if err := isobmff.WriteBoxHeader(w, "jxlc", int64(len(coded))); err != nil {
		return err
	}
	if _, err := w.Write(coded); err != nil {
		return err
	}
	return isobmff.WriteFreeSpace(ctx, w, "skip", m.seed, back)
}
