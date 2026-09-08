package engine

import (
	"errors"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// Settling the size of every file of a range target.
//
// Taken out of engine.go on 2026-09-08, when snapping arrived and that file
// went past the size ceiling. The line is what a part does rather than how long
// it is: everything here answers "what size does each file of this range get",
// and nothing else in the engine asks that question.

// firstWritable is the smallest size at or above from that this format will
// really write, without going past limit.
//
// It JUMPS rather than scans, and that is the whole reason it is cheap enough
// to run for every file. A format refusing a size it cannot write already names
// the next one it can, in BelowMinimumError.Minimum, so one refusal is normally
// one step. Measured 2026-09-08 on the two shapes this project has: an odd size
// under UTF-16 answers with size+1, and a PNG size inside the band above its
// encoded picture answers with the top of that band - 143 B answers 154 B.
//
// When nothing in the span is writable it hands back the format's OWN refusal
// for the first size it tried. That refusal already carries the four parts a
// refusal owes and names a size that would work, so there is no second wording
// here to drift away from it.
//
// The judge is the generator itself rather than a second copy of its rules. A
// copy would be a place for the two to disagree, and the disagreement would
// surface as a size that drawing accepted and writing refused.
//
// Descriptor.SmallestAccepted is this function's sibling and walks the same
// jump, from zero and with no ceiling. They are NOT one function, and the reason
// is the crash guard: everything the engine asks of a generator goes through
// planWithoutCrashing, so a panic costs one file rather than the process, while
// SmallestAccepted is called from guards and from the window, where that wrapper
// is not in hand. Merging them would mean handing a planner in, which buys less
// than it costs.
func firstWritable(desc format.Descriptor, r format.Request, from, limit int64) (int64, error) {
	var first error
	for at := from; at <= limit; {
		r.Bytes = at
		_, err := planWithoutCrashing(desc, r)
		if err == nil {
			return at, nil
		}
		if first == nil {
			first = err
		}
		var below *format.BelowMinimumError
		if !errors.As(err, &below) || below.Minimum <= at {
			// Not a refusal about the size, or one that names no way forward.
			// Either way there is nothing to jump to.
			return 0, err
		}
		at = below.Minimum
	}
	return 0, first
}

// drawSizes settles the size of every file of a range target.
//
// Every size is settled here, before a single byte is written, and that order
// is the point rather than an optimisation. A tool whose whole promise is that
// the same seed gives the same run cannot have an error that appears and
// disappears with the count.
//
// A DRAWN SIZE IS SNAPPED to one the format can write, and that is what this
// function gained on 2026-09-08. Until then it drew from the interval and hoped:
// a size the format could not write was refused later, by the per file plan, and
// whether that happened depended on what came out of the seed. Two shapes cause
// it and neither is rare. Four formats have unreachable BANDS - PNG cannot use
// the eleven byte counts above a picture's encoded size, because the smallest
// padding chunk costs twelve, and the OPC three declare the same shape. Three
// more have unreachable PARITY - a UTF-16 file is a whole number of sixteen bit
// units, so half of every range is unwritable, which took `--size-range
// 1000-1010` down about half the time (O190).
//
// Snapping is not rounding, and rule 1 is untouched. A range is a request for
// SOME size between two ends, not for a number - so answering with a writable
// size inside it is the answer, while `--size 1001` still refuses because that
// one named a number.
//
// PER FILE, not once for the target, because writability moves with the SEED as
// well as with the size: the same 64x64 PNG recipe has a floor of 144, 143 and
// 144 B at seeds 1, 2 and 3. Judging file 0 says nothing about file 2, and this
// comment claimed otherwise until 2026-09-08.
//
// What was written here before, and is now obsolete: closing this "needs the
// format to declare its unreachable bands, which is a change to
// format.Descriptor and the owner's call". It needed no such thing. The refusal
// ALREADY carries the next writable size, so probing and jumping does it with
// no new surface - measured before the change rather than argued.
//
// Bytes do not move for any run that worked before. A run that succeeds today
// has every drawn size writable, or it would have failed, and snapping a
// writable size returns it unchanged.
func drawSizes(t *Target, desc format.Descriptor, targetSeed uint64) error {
	first := format.Request{
		Contains:   t.Contains,
		Seed:       core.FileSeed(targetSeed, 0),
		Label:      t.Label,
		Properties: t.Properties,
	}

	// The low end is judged against what this format can do AT ALL, and that
	// check is older than the snapping below it. The two answer different
	// questions and both are wanted.
	//
	// A range starting under the format's floor is a recipe somebody should
	// fix: asking PDF for 10 B to 8 kB says a spread was wanted and most of it
	// does not exist, so the honest answer is the format's own refusal naming
	// its floor, not forty files quietly piled on it. A range starting at or
	// above the floor whose SOME sizes are unwritable - odd numbers under
	// UTF-16, the band above a PNG's encoded picture - is a different thing:
	// nobody can be expected to enumerate those, and snapping inside the range
	// is the answer.
	floor, err := firstWritable(desc, first, 0, t.SizeMax)
	if err != nil {
		// The floor is above the whole range, so nothing in it is writable.
		return err
	}
	if t.SizeMin < floor {
		// Asked again at the low end so the format words its own refusal, with
		// the number the person actually wrote.
		first.Bytes = t.SizeMin
		if _, err := planWithoutCrashing(desc, first); err != nil {
			return err
		}
	}

	span := uint64(t.SizeMax - t.SizeMin)
	t.SizeMoved = make([]bool, len(t.Sizes))

	for i := range t.Sizes {
		want := t.SizeMin
		if span != 0 {
			// Per index, never from a running stream. Raising a count then
			// leaves the sizes of the earlier files alone, which is rule 2 and
			// the reason core.SizeSeed takes an index at all.
			r := core.NewRand(core.SizeSeed(targetSeed, i))
			want = t.SizeMin + int64(r.Uint64N(span+1))
		}

		req := format.Request{
			Contains:   t.Contains,
			Seed:       core.FileSeed(targetSeed, i),
			Label:      t.Label,
			Properties: t.Properties,
		}

		got, err := firstWritable(desc, req, want, t.SizeMax)
		if err != nil && want > t.SizeMin {
			// Nothing writable from the draw upwards. The bottom of the range
			// can still hold something - a draw landing on the last odd number
			// of a range has nowhere above it and plenty below - so the range is
			// only empty once THAT fails too.
			got, err = firstWritable(desc, req, t.SizeMin, t.SizeMax)
		}
		if err != nil {
			return err
		}
		t.SizeMoved[i] = got != want
		t.Sizes[i] = got
	}
	return nil
}
