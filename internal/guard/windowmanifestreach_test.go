package guard

import (
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// The window says the same thing the command line says about a record too big
// to read back.
//
// Observation O184, measured on 2026-09-06: TooLargeToReadBack had two callers,
// both in internal/cli and internal/manifest, and NOT ONE in internal/gui. A
// person who generated 25 000 files from the window was told nothing at all,
// and was left with a directory that tfg verify and tfg cleanup both refuse -
// the manifest is the only authority over what may be removed, so a manifest
// that cannot be read is a set of files with no owner.
//
// It is the kind of parity gap D1 loses most easily. Not something the engine
// can do from one surface and not the other, which is what the parity guard
// looks for, but something one surface SAYS and the other does not.
//
// The note was written down as a question about the manifest schema, on the
// grounds that notes are per file and this one is per run. It is not. The
// command line does not read this off the manifest either - it works it out
// from the plan and prints it before the first byte - and manifest.TooLarge-
// ToReadBack was put where it is exactly so the two surfaces could not come to
// different conclusions about one run. What was missing was a caller.

// overTheCeiling is a file count whose manifest this build would refuse.
//
// Worked out from the estimate rather than written here, for the reason the
// command line guard beside it gives: a guard carrying its own copy of a limit
// goes stale the day somebody changes the real one, and says nothing while it
// does.
func overTheCeiling(t *testing.T) int {
	t.Helper()
	over := int(manifest.MaxBytes/manifest.BytesPerEntry) + 1000
	if _, tooBig := manifest.TooLargeToReadBack(over, 0); !tooBig {
		t.Fatalf("%d entries was not judged too large, so this guard would prove nothing", over)
	}
	return over
}

// previewOf presses Preview for a run of count files and gives back what the
// screen said.
//
// It REFUSES to return a refusal, and that is the whole reason it exists. The
// first version of the pair below set a size the default format will not take -
// the window opens on the first format in the registry, which is avif, and
// 200 B is far under what a picture needs. Both previews were turned down, so
// the negative half passed while proving nothing: a screen that says "check the
// settings marked above" says nothing about a manifest ceiling either.
func previewOf(t *testing.T, count string) string {
	t.Helper()
	content, w, host := screenInAWindowWithHost(t, text.TabOneTarget())

	// txt rather than whatever the window opens on, for two reasons. The size
	// below has to be one the format takes, and planning twenty two thousand
	// pictures would encode twenty two thousand pictures - png, jpg, gif and
	// avif all do that while planning.
	picker, ok := controlUnder(content, text.FieldFormat()).(*parts.Chooser)
	if !ok {
		t.Fatal("the format field is not a list to choose from, so this guard read the wrong tree")
	}
	picker.SetSelected("txt")

	// Small files, because what is being asked about is the number of ENTRIES
	// rather than the number of bytes. A preview writes nothing either way.
	fill(t, content, text.FieldSize(), "200b")
	fill(t, content, text.FieldCount(), count)

	press(t, content, text.ButtonPreview())
	// This preview is accepted, so it answers from a worker. Joined before the
	// status line is read - see join.
	join(host)
	settle(content, w)

	_, status := runMessages(content)
	if status == nil {
		t.Fatal("the screen has no status line, so this guard read the wrong tree")
	}
	// Matched on the tail of the preview's own sentence, the way the action bar
	// guard does it, so this cannot be satisfied by a refusal.
	marker := text.PreviewCost(1, nil, "1 B")
	tail := marker[strings.LastIndex(marker, " ")+1:]
	if !strings.Contains(status.Text, tail) {
		t.Fatalf("the preview of %s files was not accepted, so nothing here was asked about the manifest.\nIt said:\n%s",
			count, status.Text)
	}
	return status.Text
}

// runOf presses Generate rather than Preview, and gives back what the screen
// said when it finished.
//
// It exists because the preview is OPTIONAL. Somebody who presses Generate
// straight away never sees the preview's answer, and that person is exactly the
// one observation O184 is about - they end up with a directory nothing in this
// toolset can read or clean. The window cannot say anything in the middle of a
// run, so the end of the run is the only place left.
//
// It writes files, which is why this is the one guard here that does. Twenty
// two thousand of them at 200 B, into a directory that goes away with the test.
func runOf(t *testing.T, count string) string {
	t.Helper()
	content, w, host := screenInAWindowWithHost(t, text.TabOneTarget())

	picker, ok := controlUnder(content, text.FieldFormat()).(*parts.Chooser)
	if !ok {
		t.Fatal("the format field is not a list to choose from, so this guard read the wrong tree")
	}
	picker.SetSelected("txt")
	fill(t, content, text.FieldSize(), "200b")
	fill(t, content, text.FieldCount(), count)
	fill(t, content, text.FieldOutputDir(), t.TempDir())

	press(t, content, text.ButtonGenerate())
	join(host)
	settle(content, w)

	_, status := runMessages(content)
	if status == nil {
		t.Fatal("the screen has no status line, so this guard read the wrong tree")
	}
	// The run has to have HAPPENED. A refused run says nothing about a
	// manifest either, and a guard that cannot tell those apart is the shape
	// this project has recorded as passing without reaching the code.
	if !strings.Contains(status.Text, text.Written(0)[strings.LastIndex(text.Written(0), " ")+1:]) {
		t.Fatalf("the run of %s files did not finish, so nothing here was asked about the manifest.\nIt said:\n%s",
			count, status.Text)
	}
	return status.Text
}

// warningAbout is the fixed half of the sentence, without the two numbers.
//
// Taken from the text package rather than typed here, so a reworded warning
// does not quietly stop being checked.
func warningAbout(t *testing.T) string {
	t.Helper()
	marker := text.ManifestTooLargeToRead("SIZE", "LIMIT")
	at := strings.Index(marker, "SIZE")
	if at < 0 {
		t.Fatal("the warning does not carry the size it was given, so this guard cannot find its fixed half")
	}
	return marker[:at]
}

func TestTheWindowSaysWhenItsManifestWillBeTooBigToReadBack(t *testing.T) {
	said := previewOf(t, itoa(overTheCeiling(t)))
	if !strings.Contains(said, warningAbout(t)) {
		t.Errorf("a preview of %d files said nothing about the record being too big to read back.\n"+
			"The command line has said this since 2026-08-26. Somebody who does the same from the window "+
			"gets a directory that neither Verify nor Clean up can read, and no warning.\nIt said:\n%s",
			overTheCeiling(t), said)
	}
}

// And a run that was never previewed says it too, which is the case that
// matters most.
//
// The preview is a button somebody may not press. The warning has to reach the
// person who pressed Generate and nothing else, because they are the one left
// with the directory.
func TestAFinishedRunInTheWindowSaysItsManifestIsTooBigToReadBack(t *testing.T) {
	said := runOf(t, itoa(overTheCeiling(t)))
	if !strings.Contains(said, warningAbout(t)) {
		t.Errorf("a finished run of %d files said nothing about the record being too big to read back.\n"+
			"That directory now has a manifest neither Verify nor Clean up can read, and nobody was told.\nIt said:\n%s",
			overTheCeiling(t), said)
	}
	// The line somebody pressed the button for stays first. The room for these
	// messages is a ceiling and the message scrolls inside it.
	if first := strings.SplitN(said, "\n", 2)[0]; strings.Contains(first, warningAbout(t)) {
		t.Errorf("the warning took the first line from the outcome:\n%s", said)
	}
}

// A run this build CAN read back stays quiet.
//
// Without this the guard above passes on a window that warns about every run,
// which teaches somebody to stop reading the line - the same reason the command
// line has this pair rather than only the first half.
func TestAnOrdinaryPreviewSaysNothingAboutTheManifestCeiling(t *testing.T) {
	said := previewOf(t, "100")
	if strings.Contains(said, warningAbout(t)) {
		t.Errorf("a hundred files drew the warning about the record being too big:\n%s", said)
	}
}

// Both surfaces judge the same run the same way.
//
// The window reads the answer off the document a dry run builds, and the
// command line works it out from the plan before anything is written. Two paths
// to one number, and what makes two paths acceptable is that they go through
// one predicate. Asked at the boundary, which is the only place a disagreement
// would show.
func TestBothSurfacesJudgeTheSameRunTheSameWay(t *testing.T) {
	for _, entries := range []int{
		int(manifest.MaxBytes / manifest.BytesPerEntry),
		int(manifest.MaxBytes/manifest.BytesPerEntry) + 1,
	} {
		_, fromThePlan := manifest.TooLargeToReadBack(entries, 0)

		m := manifest.New("testing-files-generator", "0.0.0-dev", "run_x", "tfg generate", 1, "linux", "amd64")
		for i := 0; i < entries; i++ {
			m.Add(manifest.File{Path: "f.txt", Materialized: true})
		}
		_, fromTheDocument := m.ReadBackReach()

		if fromThePlan != fromTheDocument {
			t.Errorf("at %d entries the plan says %v and the document says %v.\n"+
				"The command line answers from the first and the window from the second, so one run "+
				"would be warned about on one surface and not on the other",
				entries, fromThePlan, fromTheDocument)
		}
	}
}
