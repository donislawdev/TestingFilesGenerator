package guard

import (
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// No format declares a minimum nobody could ever meet.
//
// Three formats worked their minimum out by measuring, and two of them answered
// 1<<62 if the measurement failed - 4611686018427387904 B, which is four
// exbibytes. The reasoning written beside it was that refusing every size is
// safer than declaring a wrong minimum. It is not safer, it is quieter: the
// format would refuse every request ever made, with a message naming a number
// no disk has ever held, and nothing would say why. That is rule 6 broken in
// the shape this project keeps finding.
//
// zip.minimumBytes already had the argument written out in full, having stopped
// doing it - while png and jpg still did. Review item N3, fixed on 2026-08-27
// by making both panic instead, which is what format.Register does for the same
// class of mistake and means a build that cannot state its own minimum fails at
// start rather than at every use.
//
// This asks the outcome rather than the mechanism, and it asks it of every
// registered format rather than of the two that had it. A minimum is a promise
// printed by "tfg formats" and used by every refusal below it, so the property
// worth holding is that the promise is a size somebody could actually ask for.
func TestNoFormatDeclaresAMinimumNobodyCouldMeet(t *testing.T) {
	// A gibibyte is far above every real minimum here - the largest is TAR.GZ,
	// in the low thousands of bytes - and far below the four exbibytes the
	// quiet answer produced. Anything landing between those two is a format
	// that has stopped being usable, whichever side of the line put it there.
	const absurd = int64(1) << 30

	for _, id := range format.IDs() {
		desc, err := format.Get(id)
		if err != nil {
			t.Errorf("%s is in the list of formats and cannot be fetched: %v", id, err)
			continue
		}
		// Nought is legal and several formats have it: a nought byte txt or md
		// is a real test case and this tool makes one on request. Only a
		// negative is nonsense, and it would come from arithmetic rather than
		// from a decision.
		if desc.MinBytes < 0 {
			t.Errorf("%s declares a minimum of %d B, which is not a size", id, desc.MinBytes)
			continue
		}
		if desc.MinBytes >= absurd {
			t.Errorf("%s declares a minimum of %d B (%s). No request can meet it, so this format refuses "+
				"every size anybody asks for and the refusal names a number instead of a reason",
				id, desc.MinBytes, core.HumanBytes(desc.MinBytes))
		}
	}
}
