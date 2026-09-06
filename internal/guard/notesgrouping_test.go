package guard

import (
	"strings"
	"testing"

	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// A note is reported once per thing it says, not once per file.
//
// Measured on 2026-09-06: a run of 25 000 one byte text files emitted 25 001
// "note:" lines on stderr, one per file, every one of them the same sentence
// about the label not fitting. Two consequences, and the second is why this is
// a stability question rather than a cosmetic one.
//
// The advice was per file for a decision that is per target. And the one line
// that matters was buried under them: a run whose manifest will be too big for
// this build to read back says so first, in a "note:" line typographically
// identical to the 25 000 that follow it, and that line is the only thing
// standing between somebody and a directory neither verify nor cleanup can
// ever read. TestARunSaysWhenItsManifestWillBeTooBigToReadBack proves the
// sentence is printed. It cannot prove anybody can see it.
//
// The same reasoning was already applied to the progress bar - throttled and
// silent when stderr is not a terminal, because "thousands of redrawn lines in
// a CI log are worse than no bar". Notes had not had it applied to them.
func TestNotesAreGroupedByWhatTheySayRatherThanOneLinePerFile(t *testing.T) {
	const files = 400

	said := runCLI(t, "generate", "--format", "txt", "--size", "1b",
		"--count", itoa(files), "--dry-run", "--out", t.TempDir())

	notes := 0
	for _, line := range strings.Split(said, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "note:") {
			notes++
		}
	}

	// Asserted rather than assumed. Nought notes would pass every check below
	// by saying nothing at all, which is the shape this project keeps meeting:
	// a guard that stopped reaching the state it guards.
	if notes == 0 {
		t.Fatal("the run printed no notes at all, so this guard checked nothing. A one byte " +
			"text file cannot carry a label, and saying so is the note this is about.")
	}
	if notes >= files {
		t.Errorf("%d files produced %d note lines, which is one per file:\n%s\n"+
			"What to do: group them by what the note says. A reader cannot find the line "+
			"that matters in a list this long.", files, notes, said)
	}

	// The count has to be there, or grouping has thrown away how many files
	// this is about and the reader is worse off than with one line each.
	if !strings.Contains(said, itoa(files)+" files:") {
		t.Errorf("the grouped note does not say how many files it covers:\n%s", said)
	}
	// And some names, because "400 files carry no label" with no name at all
	// gives nobody a place to start looking.
	if !strings.Contains(said, "files_0001.txt") {
		t.Errorf("the grouped note names no file at all:\n%s", said)
	}
	// And what it is not showing.
	if !strings.Contains(said, "not named here") {
		t.Errorf("the grouped note does not say that most of the files are unnamed, so the "+
			"three it lists read as the only ones:\n%s", said)
	}
}

// One file keeps its name in front, the way it always had it.
//
// The sharp half of the pair. Grouping that dropped the file name would pass
// every check above - the counts and the examples would all be there - while
// making the common case, a single file with something to say about it, worse
// than it was before.
func TestANoteAboutOneFileStillNamesThatFileFirst(t *testing.T) {
	said := runCLI(t, "generate", "--format", "txt", "--size", "1b",
		"--count", "1", "--dry-run", "--out", t.TempDir())

	if !strings.Contains(said, "note:") {
		t.Fatal("the run printed no note, so this guard checked nothing")
	}
	if !strings.Contains(said, "note: files_0001.txt: ") {
		t.Errorf("a note about one file no longer leads with that file's name:\n%s", said)
	}
	if strings.Contains(said, "1 files") {
		t.Errorf("the note says \"1 files\":\n%s", said)
	}
	if strings.Contains(said, "not named here") {
		t.Errorf("a note about one file claims to be hiding others:\n%s", said)
	}
}

// The number carries the right noun at the boundary where it changes.
//
// Four files with three named leaves exactly one unnamed, which is the only
// count at which the plural is wrong in a way a reader notices. core.Count
// exists for this and the surrounding sentence is a participle, so nothing in
// it agrees with the number.
func TestTheUnnamedRemainderCarriesTheRightNoun(t *testing.T) {
	said := runCLI(t, "generate", "--format", "txt", "--size", "1b",
		"--count", "4", "--dry-run", "--out", t.TempDir())

	if !strings.Contains(said, "1 file not named here") {
		t.Errorf("four files with three named should leave \"1 file not named here\":\n%s", said)
	}
	if strings.Contains(said, "1 files not named") {
		t.Errorf("the remainder says \"1 files\":\n%s", said)
	}
}
