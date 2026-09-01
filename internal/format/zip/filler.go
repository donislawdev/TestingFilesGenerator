package zip

import (
	stdzip "archive/zip"
	"context"
	"io"
)

// writeFillerEntry puts the padding entry into the archive.
//
// It lives beside build rather than inside it because build had grown past the
// length this project caps functions at, and this is the piece that comes out
// whole: everything here is about one entry that is not one of the files
// anybody ordered.
//
// The filler stays at the top of the archive even when the files inside are
// nested. Its path is then a constant, so the size arithmetic does not move
// with the depth - and it is not one of the ordered files, so a reader can
// always tell it apart from them.
func writeFillerEntry(ctx context.Context, zw *stdzip.Writer, m memo, withContents bool) error {
	if !m.withFiller {
		return nil
	}
	// The filler is locked with everything else. An archive where one entry
	// opens without the password and the rest do not is a file nobody
	// asked for, and the arithmetic is the same either way.
	crc, err := plaintextCRC(ctx, m, withContents, func(w io.Writer) error {
		return writeFiller(ctx, w, m.seed, m.fillerSize)
	})
	if err != nil {
		return err
	}
	entry, shut, err := openEntry(zw, m, entryPlan{
		name: fillerName, plain: m.fillerSize, index: len(m.children),
		withContents: withContents, crc: crc,
	})
	if err != nil {
		return err
	}
	if !withContents {
		return nil
	}
	if err := writeFiller(ctx, entry, m.seed, m.fillerSize); err != nil {
		return err
	}
	return shut()
}
