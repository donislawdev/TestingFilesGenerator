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
// freed is asked for rather than handed over as a number, and the order is the
// whole reason. A zip entry's compressed bytes do not reach the writer until
// the entry is CLOSED, and the only thing that closes one is creating the next
// header - Flush does not. Measured while writing this: reading the total
// before the filler's header went down gave nought, and the archive overshot
// by exactly the compressed length of its entries.
func writeFillerEntry(ctx context.Context, zw *stdzip.Writer, m memo, withContents bool, freed func() int64) error {
	if !m.withFiller {
		return nil
	}
	if m.squeeze.On() {
		return writeCompressedFiller(ctx, zw, m, withContents, freed)
	}

	// The stored path, unchanged. It settles the size before the header
	// because a locked entry states its length there, and a locked archive
	// takes this path.
	size := m.fillerSize
	// The filler is locked with everything else. An archive where one entry
	// opens without the password and the rest do not is a file nobody
	// asked for, and the arithmetic is the same either way.
	crc, err := plaintextCRC(ctx, m, withContents, func(w io.Writer) error {
		return writeFiller(ctx, w, m.seed, size)
	})
	if err != nil {
		return err
	}
	entry, shut, err := openEntry(zw, m, entryPlan{
		name: fillerName, plain: size, index: len(m.children), stored: true,
		withContents: withContents, crc: crc,
	})
	if err != nil {
		return err
	}
	if !withContents {
		return nil
	}
	if err := writeFiller(ctx, entry, m.seed, size); err != nil {
		return err
	}
	return shut()
}

// writeCompressedFiller is the same entry for an archive whose others are
// squeezed, with the header written before the length is known.
//
// It carries no checksum and takes no lock, and neither is an omission:
// ReadCompression refuses compression together with a password, so a squeezed
// archive is never a locked one.
func writeCompressedFiller(ctx context.Context, zw *stdzip.Writer, m memo, withContents bool, freed func() int64) error {
	entry, shut, err := openEntry(zw, m, entryPlan{
		name: fillerName, index: len(m.children), stored: true,
		withContents: withContents,
	})
	if err != nil {
		return err
	}
	if !withContents {
		// The counting pass adds the filler's length arithmetically, so
		// writing its bytes here would count them twice.
		return nil
	}
	if err := writeFiller(ctx, entry, m.seed, m.fillerSize+freed()); err != nil {
		return err
	}
	return shut()
}
