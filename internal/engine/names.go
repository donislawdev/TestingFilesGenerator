package engine

import (
	"fmt"
	"strings"

	"golang.org/x/text/unicode/norm"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// What makes two names one name, and who is holding a name already.
//
// Split out of engine.go on 2026-08-25, when the manifest started holding its
// own name and the size ceiling said the file had grown past what a person can
// follow. Cut here rather than anywhere else because these four know one thing
// together - the rules of a name - and the rest of engine.go does not need to
// know any of it to plan a run.

// nameOwner remembers who took a name and how it was spelled, so a refusal can
// show both names as they were written rather than as they were compared.
type nameOwner struct {
	id   string
	name string
	// Set for the one entry that is not a target. The two collisions read
	// differently and are addressed differently: two targets leave a person
	// with a choice nobody else can make, while a target sitting on the
	// manifest has one box that is certainly filled in - its own.
	manifest bool
}

// claimFileName takes a name for one target, or refuses because somebody has it.
//
// Position is one based, because that is what an address carries.
func claimFileName(names map[string]nameOwner, position int, id, name string) error {
	key := collisionKey(name)
	owner, clash := names[key]
	if !clash {
		names[key] = nameOwner{id: id, name: name}
		return nil
	}

	// The manifest is answered here rather than left to the sentence below, and
	// it carries an address the target collision has no right to. A run that
	// never wrote output.manifest still has a manifest, so pointing at that box
	// would point at an empty one - while the name template is a box somebody
	// filled in.
	if owner.manifest {
		return &RecipeError{
			Setting: core.TargetAddress(position, SettingName),
			Detail: fmt.Sprintf("target %q produces a file named %s, and that is the name this run gives its manifest",
				id, name),
			Because: "both are written into the output directory, so the file would take the name the manifest needs and the run would end with files and nothing to remove them by",
			Remedy:  "Give the target a name template containing " + indexToken + ", or name the manifest something else",
		}
	}
	// No address, deliberately, and this is the one refusal here that keeps it.
	// Two targets produce the pair, so naming one of them would send somebody to
	// a box that is not wrong on its own - and which of the two to change is
	// theirs to decide. The sentence names both ids, which is what a person
	// needs and what a window cannot place either way.
	return &RecipeError{
		Detail: collisionDetail(owner, id, name),
		Remedy: "Give one of them a name template containing " + indexToken}
}

// collisionKey is the spelling two names are compared under. Two names sharing
// a key are one file on some filesystem somebody runs this on.
//
// Lowercasing covers NTFS, APFS and exFAT, which keep the case that was typed
// and match without it. Normalising covers APFS again, which folds the two
// spellings of an accented letter into one name. Neither step covers the other:
// lowercasing leaves the two spellings apart, and normalising leaves REPORT.TXT
// apart from report.txt.
//
// Lowercasing rather than Unicode case folding, and that is a measurement
// rather than an oversight. This said "folding case" until 2026-08-25, when an
// outside review pointed out that the words and the code disagreed and offered
// cases.Fold as the fix, on the grounds that Windows reads ß and SS as one
// name. Measured here on NTFS, writing files of different lengths under both
// spellings:
//
//	straße.txt and STRASSE.txt   two files
//	k.txt and U+212A.txt         two files
//	report.txt and REPORT.TXT    one file
//
// So folding would refuse a pair this filesystem keeps apart, which is a false
// refusal of a recipe that would have worked. Lowercasing is per character and
// one to one, which is what NTFS does with its own table.
//
// What is NOT measured is APFS, and that is where the review's claim would have
// to be true for it to be worth anything. A Mac can answer it, and only that
// answer is a reason to open this again.
func collisionKey(name string) string {
	return strings.ToLower(norm.NFC.String(name))
}

// collisionDetail says what the two names have in common. Two names that
// collide can print identically on screen, so a refusal that only shows them is
// one the reader cannot act on. A refusal has to say what is wrong, what is
// allowed and what to do instead.
func collisionDetail(owner nameOwner, id, name string) string {
	switch {
	case owner.name == name:
		return fmt.Sprintf("targets %q and %q both produce a file named %s", owner.id, id, name)
	case strings.EqualFold(owner.name, name):
		return fmt.Sprintf(
			"targets %q and %q produce the names %s and %s, which differ only in case. Most filesystems treat those as one file, so one would be written over the other and the manifest would describe both",
			owner.id, id, owner.name, name)
	default:
		return fmt.Sprintf(
			"targets %q and %q produce the names %s and %s. Those print the same because they are one name spelled two ways, an accented letter against the plain letter with its accent as a separate character. macOS stores both under one name, so one file would be written over the other and the manifest would describe both",
			owner.id, id, owner.name, name)
	}
}
