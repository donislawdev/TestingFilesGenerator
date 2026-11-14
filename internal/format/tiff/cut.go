package tiff

import (
	"context"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// layOut says where a TIFF's directory and padding go. A TIFF a damage will cut
// is laid out for the cut (splitForACut). One of an odd size puts its directory
// right behind the header, with all of its padding still in front of the
// pixels: TIFF 6.0 starts a directory on a word boundary, and last, where the
// 172 B of the directory and its values end the file, it would start on an odd
// byte (O342). Nine decoders give the same pixels from that layout
// (docs/TRUNCATE-2026-11-11.md section 15.3). The rest keep the directory last.
func layOut(m *memo, total, cutAt int64) {
	switch {
	case cutAt > 0:
		splitForACut(m, total, cutAt)
	case total%2 == 1:
		m.directoryFirst = true
	}
}

// splitForACut lays a TIFF out for a cut after its first cutAt bytes
// (format.Request.CutAt), so that the cut keeps the share of the pixels it
// keeps of the whole file.
//
// The directory and the values it points at move up to right behind the
// header. TIFF 6.0 lets the directory stand anywhere after the header on a
// word boundary, and byte 8 is one. Left last, it is the end of the file: 172
// B that a cut of 1% of a file of a few kilobytes falls in. The padding then
// goes around the pixels - in front of them as much as the share wants, the
// rest behind them, where no field points. It is raw bytes on either side, so
// every length lands. Nine decoders give the pixels of a TIFF nothing cuts
// from the directory first with the padding in front, split and behind, and
// exiftool reads it (docs/TRUNCATE-2026-11-11.md section 15.3).
//
// A TIFF nothing cuts keeps its directory last and its padding in front, byte
// for byte what it was (D11).
func splitForACut(m *memo, total, cutAt int64) {
	pad := m.gap
	m.directoryFirst = true
	m.gap = format.SplitFront(pad, header+directory+heap, pixelBytes(m.width, m.height), cutAt, total, 1)
	m.back = pad - m.gap
}

// writeDirectoryFirst writes a TIFF whose directory follows its header - one
// laid out for a cut, or one of an odd size: the header, the directory and its
// values, the padding in front, the pixels and the padding behind.
func writeDirectoryFirst(ctx context.Context, w io.Writer, m memo) error {
	head := int64(header + directory + heap)
	pixels := pixelBytes(m.width, m.height)
	format.Mark(w, format.RoleStructure, 0, head)
	format.Mark(w, format.RolePadding, head, m.gap)
	format.Mark(w, format.RoleContent, head+m.gap, pixels)
	format.Mark(w, format.RolePadding, head+m.gap+pixels, m.back)

	if err := writeHeader(w, m); err != nil {
		return err
	}
	if err := writeDirectory(w, m); err != nil {
		return err
	}
	if err := writeGap(ctx, w, m.seed, m.gap); err != nil {
		return err
	}
	if err := writePixels(ctx, w, m); err != nil {
		return err
	}
	return writeGap(ctx, w, m.seed+1, m.back)
}
