package engine

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
	"github.com/donislawdev/TestingFilesGenerator/internal/version"
)

// How large the record of a run will be, worked out from the plan before a
// byte is written (docs/STABILITY-REVIEW-2026-10-26.md, R03 and R08).
//
// Two things ask it. The free space check: until 2026-10-26 it held the free
// space against the files alone, so a disk with room for the files and not
// for the manifest took the files and refused the manifest - files nothing can
// remove, because the manifest is the only authority over that (untouchable
// rule 7). And the warning that a manifest will be too large to read back,
// which came from a constant per entry that was low for every format.

// RecordSize is the record a run will write beside its files.
type RecordSize struct {
	// Manifest is the length of the manifest.
	Manifest int64
	// TooLarge says this build would refuse to read that manifest back.
	TooLarge bool
	// Instructions is the length of the instructions, nought when no file was
	// given a purpose and none are written.
	Instructions int64
}

// Bytes is the whole record.
func (r RecordSize) Bytes() int64 { return r.Manifest + r.Instructions }

// hashPlaceholder stands in for a hash not worked out yet. Every hash a run
// records is this long, so the document is as long as the one the run writes.
var hashPlaceholder = strings.Repeat("0", 64)

// PredictRecord is the record a run of these files writes when every file is
// produced. A file that fails gains a note in its entry, so a run with
// failures writes a little more than this says - which is why the surfaces say
// again, from what was written, when the two disagree.
//
// Entries are counted one at a time and not kept (manifest.Projection). The
// instructions are the exception: what they say depends on every file at once,
// so for a run that gives files a purpose the document is built whole to write
// them. Runs that do are presets and recipes written by hand, not the runs of
// a million files that a whole document would weigh on.
//
// Ctrl+C is asked as it goes. It runs before the first byte, where stopping is
// expected to be immediate, and over a million files it is a pass worth
// stopping - the shape R05 of the review of 2026-10-21 found in the drawing of
// sizes, which this must not repeat.
func PredictRecord(ctx context.Context, files []PlannedFile, opt Options) (RecordSize, error) {
	scratch := &Result{}
	header := newManifest(opt)
	header.Run.Complete = true
	counted := header.Projection()
	explains := false
	for i, f := range files {
		if i%askEvery == 0 {
			if err := ctx.Err(); err != nil {
				return RecordSize{}, err
			}
		}
		e := scratch.entryFor(f, hashPlaceholder, true, nil)
		explains = explains || e.Purpose != ""
		counted.Add(e)
	}
	var out RecordSize
	if explains {
		header.Run.Instructions = filepath.Base(InstructionsPath(opt))
		whole := newManifest(opt)
		for i, f := range files {
			if i%askEvery == 0 {
				if err := ctx.Err(); err != nil {
					return RecordSize{}, err
				}
			}
			whole.Add(scratch.entryFor(f, hashPlaceholder, true, nil))
		}
		out.Instructions = int64(len(whole.Instructions(filepath.Base(ManifestPath(opt)))))
	}
	var err error
	out.Manifest, out.TooLarge, err = counted.Bytes()
	return out, err
}

// forecast is the record the caller handed in, or the one worked out here
// when it handed in none.
func (o Options) forecast(ctx context.Context, files []PlannedFile) (RecordSize, error) {
	if o.Record != nil {
		return *o.Record, nil
	}
	return PredictRecord(ctx, files, o)
}

// askEvery is how many entries go by between two questions about Ctrl+C. An
// entry costs a few microseconds, so this answers within a few milliseconds
// and asks a thousand times less often than there are entries.
const askEvery = 1024

// newManifest is the empty manifest of a run with these options. One place,
// so the record a run writes and the record PredictRecord measures start from
// the same header.
func newManifest(opt Options) *manifest.Manifest {
	m := manifest.New(
		"testing-files-generator", version.Version,
		runID(opt.Seed), opt.Command, opt.Seed,
		runtime.GOOS, runtime.GOARCH,
	)
	m.Run.RecipeHash = opt.RecipeHash
	m.Run.Overrides = opt.Overrides
	m.Run.Preset = opt.Preset
	return m
}
