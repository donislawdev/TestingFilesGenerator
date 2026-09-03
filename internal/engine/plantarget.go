package engine

import (
	"context"
	"fmt"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// planning is what one call to PlanContext carries from target to target.
//
// A struct rather than six parameters, and the reason is the running totals:
// the byte total and the claimed names are answers about the WHOLE run rather
// than about one target, and threading them through as arguments is how a
// ceiling on one target alone gets written by accident. That is not
// hypothetical here - the file count used to be bounded per target and the
// total was reachable by writing the number out in pieces.
//
// Split out of engine.go on 2026-08-26, when PlanContext went past the ceiling
// on how long one function may be. The ceiling is a ratchet, so the answer is a
// split and never a higher number.
type planning struct {
	out        []PlannedFile
	names      map[string]nameOwner
	totalBytes int64
	budget     *planMemory
}

// files plans every file of one target and adds them to the run.
//
// position is the target's place in the recipe, counted from one, and it is
// what every refusal below carries so a window can mark the right box.
func (pl *planning) files(ctx context.Context, t *Target, desc format.Descriptor, targetSeed uint64, position int) error {
	for idx, size := range t.Sizes {
		fileSeed := core.FileSeed(targetSeed, idx)

		p, err := planWithoutCrashing(desc, format.Request{
			Bytes:            size,
			SizeFromContents: t.SizeFromContents,
			Contains:         t.Contains,
			Seed:             fileSeed,
			Label:            t.Label,
			Properties:       t.Properties,
		})
		if err != nil {
			return atTarget(position, err)
		}

		name, err := renderName(t, desc, idx)
		if err != nil {
			return atTarget(position, err)
		}
		// Two files heading for one name means one of them would be
		// destroyed by the other, and the manifest would still describe
		// both. A manifest that quietly lost a file looks complete and
		// reaches the test suite as a false truth.
		//
		// "One name" is not "the same string", and reading it that way cost
		// exactly what this check exists to prevent. Most filesystems people
		// run this on keep the case somebody typed and match without it -
		// NTFS, APFS and exFAT do, ext4 does not - so "report.txt" and
		// "REPORT.TXT" are two files on one machine and one file on the
		// next. Measured on Windows, 2026-08-03: exit 0, two entries in the
		// manifest, one file on the disk, and "tfg verify" failing on the
		// tool's own output a second later.
		//
		// Case is not the only way to spell one name twice. An accented
		// letter is one code point, U+00E9, and it is also the plain letter
		// followed by a combining accent, U+0301. Both are valid UTF-8 and
		// they print identically. APFS normalises what it is given, so on
		// macOS the two spellings are one file, while NTFS and ext4 keep
		// them apart. Measured there 2026-08-04: exit 0, two entries in the
		// manifest, one file on the disk, and "tfg verify" failing on the
		// tool's own output a second later - the same defect the case rule
		// exists to stop, reached by spelling a letter the other way.
		//
		// So the key folds case and normalises, and neither alone is
		// enough: case folding does not bring the two spellings together
		// and normalising does not bring REPORT.TXT to report.txt.
		//
		// Refused everywhere rather than only where it bites, the same as a
		// path separator and for the same reason: a recipe travels between
		// machines by design, and one that quietly loses a file on somebody
		// else's is worse than one refused on both. Producing such a pair on
		// purpose belongs to the name laboratory and its archive mode, D10.
		// No address, deliberately, and this is the one refusal here that
		// keeps it. Two targets produce the pair, so naming one of them
		// would send somebody to a box that is not wrong on its own - and
		// which of the two to change is theirs to decide. The sentence
		// names both ids, which is what a person needs and what a window
		// cannot place either way.
		if err := claimFileName(pl.names, position, t.ID, name); err != nil {
			return err
		}

		if pl.totalBytes, err = core.AddSizes(pl.totalBytes, p.Bytes); err != nil {
			// The size of this target, for the same reason as the ceiling
			// above: the total belongs to the run, the box somebody can
			// change belongs to a target.
			return &RecipeError{
				Setting: core.TargetAddress(position, format.SettingSize),
				Detail:  fmt.Sprintf("target %q brings the run to a size that is too large to measure", t.ID),
				Because: err.Error()}
		}

		pl.out = append(pl.out, PlannedFile{
			ID:     fmt.Sprintf("f_%04d", len(pl.out)+1),
			Target: t,
			Index:  idx,
			Name:   name,
			Seed:   fileSeed,
			Desc:   desc,
			Plan:   p,
		})

		// What the plan costs, rather than how many files are in it. See
		// planmemory.go for why the count alone could not see this.
		if err := pl.budget.account(position, len(pl.out)); err != nil {
			return err
		}
		// Asked here rather than per target, because one target of ten
		// thousand pictures is the case this is for.
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	return nil
}
