package engine

import (
	"fmt"
	"strings"

	"golang.org/x/text/cases"
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
// Full Unicode case folding rather than lowercasing, and the road here is worth
// keeping, because the answer changed once and the reasoning has to change with
// it rather than be quietly replaced.
//
// This said "folding case" until 2026-08-25 while the code lowercased. An
// outside review spotted the disagreement and offered cases.Fold, on the
// grounds that Windows reads ß and SS as one name. Measured on NTFS, writing
// files of different lengths under both spellings, that was wrong:
//
//	straße.txt and STRASSE.txt   two files
//	k.txt and U+212A.txt         two files
//	report.txt and REPORT.TXT    one file
//
// So the words were changed and the code left alone, with one condition written
// down: APFS was not measured, and an answer from there would be a new argument.
//
// It arrived on 2026-08-26, from a Mac Mini running macOS 26.6.2 on a default
// APFS volume, and it is the other way round. Nine pairs, each written twice
// with different lengths so that one file and two files could be told apart by
// content rather than by a listing that could also mean the second write
// failed - tools/probes/apfs-case.py:
//
//	straße.txt and STRASSE.txt   ONE file
//	ſ.txt and s.txt              ONE file
//	ﬀ.txt and ff.txt             ONE file
//	İstanbul.txt and istanbul.txt   two files
//	ı.txt and i.txt              two files
//
// A recipe producing the first pair therefore writes two files on Windows and
// one on macOS, which is D10: a name has to mean the same thing everywhere. The
// same argument refuses a path separator and the characters Windows will not
// store, on every system rather than only where they bite.
//
// Which spelling to compare under is measured too, not reasoned about.
// tools/probes/casefold puts both against those nine pairs and exits non-zero
// if folding is not the better predictor: cases.Fold is right about 8 of 8 and
// strings.ToLower about 4.
//
// The old justification was wrong in BOTH directions, and the second half is
// the one nobody had noticed. Lowercasing lets through three collisions APFS
// really makes - the sharp s, the long s and the ff ligature - and it invents
// one that no measured filesystem makes: Go maps a dotted capital I onto a
// plain i in a single rune, so İstanbul.txt and istanbul.txt share a key today
// and both filesystems keep them apart. The over-refusal at the Kelvin sign was
// known and accepted as erring safely. This one is not that: folding does not
// have it, so it was never the price of caution.
func collisionKey(name string) string {
	return cases.Fold().String(norm.NFC.String(name))
}

// collisionDetail says what the two names have in common. Two names that
// collide can print identically on screen, so a refusal that only shows them is
// one the reader cannot act on. A refusal has to say what is wrong, what is
// allowed and what to do instead.
func collisionDetail(owner nameOwner, id, name string) string {
	switch {
	case owner.name == name:
		return fmt.Sprintf("targets %q and %q both produce a file named %s", owner.id, id, name)
	// Spelling before case, because normalising does not touch case and so a
	// pair that survives this one really is a difference of case.
	case norm.NFC.String(owner.name) == norm.NFC.String(name):
		return fmt.Sprintf(
			"targets %q and %q produce the names %s and %s. Those print the same because they are one name spelled two ways, an accented letter against the plain letter with its accent as a separate character. macOS stores both under one name, so one file would be written over the other and the manifest would describe both",
			owner.id, id, owner.name, name)
	// Lowercasing rather than strings.EqualFold, and a guard caught the
	// difference on 2026-08-26. EqualFold folds simply, which puts the LONG s
	// in the same orbit as s - so "maſs.txt" against "mass.txt" was answered
	// with "they differ only in case", about two letters that plainly are not
	// one letter in two sizes. Lowercasing is the narrower question and the one
	// this sentence actually claims.
	//
	//lint:ignore SA6005 EqualFold is the faster comparison and the wrong one here. It folds simply, which is a wider question than the one this branch asks, and the sentence it leads to would then be false about two names that are not one name in two sizes.
	case strings.ToLower(norm.NFC.String(owner.name)) == strings.ToLower(norm.NFC.String(name)):
		return fmt.Sprintf(
			"targets %q and %q produce the names %s and %s, which differ only in case. Most filesystems treat those as one file, so one would be written over the other and the manifest would describe both",
			owner.id, id, owner.name, name)
	default:
		// The fourth kind, unreachable until collisionKey started folding on
		// 2026-08-26. It is not a difference of case and not a difference of
		// spelling, so both sentences above would have been false about it -
		// and the one it would have fallen into says the two names differ by an
		// accent, which is worse than saying nothing.
		return fmt.Sprintf(
			"targets %q and %q produce the names %s and %s. Those are different letters that mean the same one - the sharp s against ss, the long s against s, a ligature against the letters in it. macOS stores both under one name, so one file would be written over the other and the manifest would describe both",
			owner.id, id, owner.name, name)
	}
}
