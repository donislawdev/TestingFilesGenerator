package zip

import (
	stdzip "archive/zip"
	"fmt"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// Everything about squeezing a zip, kept together and kept out of zip.go.
//
// The split is by subject rather than by size, but the size is what forced it:
// zip.go reached 464 lines of code against a crowding cap of 413, and the
// ceilings in this project only ever come down.
//
// The design in one line: every structural field of a zip is fixed width, so a
// compressed archive differs from a stored one by the entry DATA and nothing
// else. That is what lets the plan keep working the padding out for a stored
// archive - which it can do without generating anything - and lets the writer
// give back exactly what the compressor freed.

// padCompressed settles the padding for an archive whose entries will be
// squeezed.
//
// It works the padding out for the STORED archive, exactly as the rest of this
// file does, and leaves the writer to add back what the compressor frees. That
// works because every structural field of a zip is fixed width: a compressed
// archive differs from a stored one by the entry DATA and by nothing else, so
// the two differ by a number the writer can measure and the plan does not have
// to predict.
func padCompressed(m *memo, p *format.Plan, r format.Request, groups []format.Content) error {
	m.withFiller = true
	withFiller, err := archiveSize(*m)
	if err != nil {
		return err
	}
	m.fillerSize = r.Bytes - withFiller
	if m.fillerSize < 0 {
		return &format.BelowMinimumError{
			Format:    "ZIP",
			Requested: r.Bytes,
			Minimum:   withFiller,
			Reason: fmt.Sprintf("an archive holding %s needs that much before anything is squeezed, "+
				"and how far it squeezes is not known until it has been", describeGroups(groups)),
			Hint: fmt.Sprintf("Ask for %d B or more, or hold fewer or smaller files.", withFiller),
		}
	}
	p.Properties["padding_entry"] = fillerName
	return nil
}

// build writes the archive.
//
// withContents says whether the files inside are actually generated. The
// writing path passes true. Planning passes false and adds the sizes on
// afterwards, which is what keeps measuring an archive from costing as much as
// producing one - see archiveSize.
//
// One function with a mode rather than two, so the structure, the order of the
// entries and the comment cannot drift between what was measured and what is
// written. Only the data writes differ.
// method is how one entry of this archive is written.
//
// Two entries are never compressed however hard the archive is squeezed, and
// both exceptions were measured rather than reasoned about.
//
// The FILLER is stored. It is random, so deflate cannot shrink it and grows it
// instead - measured at 65 195 B in and 65 220 B out. Worse than the waste, a
// compressed filler is a filler whose length nobody can aim, which is the one
// job it has.
//
// The COUNTING pass stores everything. That pass writes no contents, so a
// deflate entry would emit an empty deflate stream - two bytes an entry that
// the real archive does not have in the same place. The plan's arithmetic
// models the STORED archive and the writer gives back the difference, so the
// counting pass has to be that stored archive exactly.
//
// A locked archive never reaches the compressed branch at all: ReadCompression
// refuses the pair, because a locked entry states its length before its data
// is written and a compressed one does not know it yet.
func (m memo) methodFor(e entryPlan) uint16 {
	if m.squeeze.On() && e.withContents && !e.stored {
		return stdzip.Deflate
	}
	return stdzip.Store
}

// tally counts the bytes a compressor emits, so the writer can learn exactly
// how much the compressor freed.
//
// Registering a compressor is the only place that number is visible without
// arithmetic over the zip format itself. Working it out from the file position
// instead would mean modelling local headers, data descriptors and the central
// directory - all of which differ between the locked and unlocked paths, and
// one of which cost this design 16 bytes a run before it was measured.
type tally struct {
	to io.Writer
	n  *int64
}

func (t *tally) Write(p []byte) (int, error) {
	n, err := t.to.Write(p)
	*t.n += int64(n)
	return n, err
}
