package zip

import (
	stdzip "archive/zip"
	"context"
	"fmt"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/archive"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
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
//
// zm marks where the entry starts, which is known only once its header is
// written: the entry before it ends as this one opens.
func writeFillerEntry(ctx context.Context, zw *stdzip.Writer, zm *archive.StreamMap, m memo, withContents bool, freed func() int64) error {
	if !m.withFiller {
		return nil
	}
	if err := zm.Hold(); err != nil {
		return err
	}
	if m.squeeze.On() {
		return writeCompressedFiller(ctx, zw, zm, m, withContents, freed)
	}
	return writeStoredFiller(ctx, zw, zm, m, withContents, storedFiller{
		name: m.fillerEntryName(), size: m.fillerSize, index: len(m.children),
	})
}

// storedFiller is one entry of padding whose length is settled before its
// header: the one behind the files of a stored archive, and the one in front of
// them in an archive laid out for a cut (writeFrontEntry).
type storedFiller struct {
	name  string
	size  int64
	index int
}

// writeStoredFiller writes f, with what follows its header held (zm.Hold)
// until it is marked. It settles the size before the header because a locked
// entry states its length there, and a locked archive takes this path.
func writeStoredFiller(ctx context.Context, zw *stdzip.Writer, zm *archive.StreamMap, m memo, withContents bool, f storedFiller) error {
	// The filler is locked with everything else. An archive where one entry
	// opens without the password and the rest do not is a file nobody
	// asked for, and the arithmetic is the same either way.
	crc, err := plaintextCRC(ctx, m, withContents, func(w io.Writer) error {
		return writeFiller(ctx, w, m.seed, f.size)
	})
	if err != nil {
		return err
	}
	entry, shut, err := openEntry(zw, m, entryPlan{
		name: f.name, plain: f.size, index: f.index, stored: true,
		withContents: withContents, crc: crc,
	})
	if err != nil {
		return err
	}
	if err := markFiller(zm, f.name); err != nil {
		return err
	}
	if !withContents {
		return nil
	}
	if err := writeFiller(ctx, entry, m.seed, f.size); err != nil {
		return err
	}
	return shut()
}

// markFiller marks the filler entry named name as padding and sends on what
// was held while its header was written.
func markFiller(zm *archive.StreamMap, name string) error {
	if err := zm.MarkEntry(format.RolePadding, name, -1); err != nil {
		return err
	}
	return zm.Release()
}

// fillerEntryName is the name of the entry of padding behind the files:
// fillerName, unless an entry in front of them has it (split).
func (m memo) fillerEntryName() string {
	if m.split.front {
		return fillerEndName
	}
	return fillerName
}

// writeCompressedFiller is the same entry for an archive whose others are
// squeezed, with the header written before the length is known.
//
// It carries no checksum and takes no lock, and neither is an omission:
// ReadCompression refuses compression together with a password, so a squeezed
// archive is never a locked one.
func writeCompressedFiller(ctx context.Context, zw *stdzip.Writer, zm *archive.StreamMap, m memo, withContents bool, freed func() int64) error {
	entry, shut, err := openEntry(zw, m, entryPlan{
		name: m.fillerEntryName(), index: len(m.children), stored: true,
		withContents: withContents,
	})
	if err != nil {
		return err
	}
	if err := markFiller(zm, m.fillerEntryName()); err != nil {
		return err
	}
	if !withContents {
		// The counting pass adds the filler's length arithmetically, so
		// writing its bytes here would count them twice.
		return nil
	}
	// An archive laid out for a cut was settled on what a pass that only
	// measured found the compressor frees, and its files were marked with the
	// length that gave. Squeezing the same files again has to free the same.
	got := freed()
	if m.split.sized && got != m.split.freed {
		return core.Defect(fmt.Errorf("zip: the files inside came to %d B less once squeezed while measuring and %d B less while writing", m.split.freed, got))
	}
	// What the compressor freed can be NEGATIVE, and that is the case this
	// arithmetic was written without. Deflate grows data it cannot shrink -
	// about five bytes for every 65535 - so an archive holding already
	// compressed files comes out of the compressor LARGER than it went in.
	// The filler then has to shrink to compensate, and below zero it cannot.
	//
	// Left alone, writeFiller's loop simply wrote nothing and the archive came
	// out too long with no error at all - measured 2026-09-02 as a band 50 B
	// wide holding two 1 MB docx entries, which the engine reported as this
	// generator disagreeing with its own plan. That reads as a broken tool
	// rather than as an impossible size, and a person cannot act on it.
	//
	// Refused here rather than while planning, which is where targz refuses
	// the same thing for the same reason: how far contents squeeze is not
	// knowable without squeezing them, and planning is not allowed to. The
	// floor reported is measured rather than derived - it is what this
	// archive came to with no padding at all.
	size := m.fillerSize + got
	if size < 0 {
		return &format.BelowMinimumError{
			Format:    "ZIP",
			Requested: m.target,
			Minimum:   m.target - size,
			Reason: core.Says("format.TheseContentsComeOutOfThe", "these contents come out of the compressor larger than they went in, "+
				"so the archive cannot be made this small once they are squeezed"),
			Hint: core.Says("format.AskForBOrMoreOr10", "Ask for %d B or more, or ask for compression: none.", core.A("Target", m.target-size)),
		}
	}
	if err := writeFiller(ctx, entry, m.seed, size); err != nil {
		return err
	}
	return shut()
}

// writeFiller emits the padding entry without holding it in memory.
func writeFiller(ctx context.Context, w io.Writer, seed uint64, n int64) error {
	rng := core.NewRand(seed)
	buf := make([]byte, writeChunk)
	for remaining := n; remaining > 0; {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		size := int64(len(buf))
		if remaining < size {
			size = remaining
		}
		chunk := buf[:size]
		core.FillRandomBE(chunk, rng)
		if _, err := w.Write(chunk); err != nil {
			return err
		}
		remaining -= size
	}
	return nil
}
