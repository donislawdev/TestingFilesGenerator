package guard

import (
	"testing"

	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// A finished run looks like one.
//
// Measured off a render on 2026-08-20: "3 files written." came out at
// #E6E6E6, which is the ordinary text colour, at the ordinary size, in the
// ordinary weight - the same drawing as the neutral line naming the output
// folder. A refusal on the same screen is four lines of red. The window
// shouted about a mistake and whispered about the thing somebody pressed the
// button for.
//
// Three outcomes rather than two. "Written with failures" is not success and
// is not a refusal either, and untouchable rule 6 bans hiding it - so a run
// that skipped files has to be told apart from one that did not, before it is
// read rather than after.
//
// Colour is never the only carrier - UX1 - and it is not one here: the three
// sentences already differ. What the colour adds is which of the three this is.
func TestAFinishedRunIsColouredLikeAFinishedRun(t *testing.T) {
	dir := t.TempDir()
	host, content := screen(t)

	fill(t, content, text.FieldOutputDir(), dir)
	fill(t, content, text.FieldSize(), "2kb")
	fill(t, content, text.FieldCount(), "3")
	press(t, content, text.ButtonGenerate())
	waitForManifest(t, dir)
	join(host)

	_, status := runMessages(content)
	if status == nil {
		t.Fatal("the generate screen has no status line, so this guard read the wrong tree")
	}
	if status.Importance != widget.SuccessImportance {
		t.Errorf("a run that wrote every file it promised said %q at importance %v rather than %v."+
			" The strongest moment this program has was drawn more weakly than anything else on the screen",
			status.Text, status.Importance, widget.SuccessImportance)
	}
}

// What a run says goes back to the ordinary colour when the next thing is said.
//
// Anything coloured here is coloured about one run. Left set, the green from a
// finished run would still be on the line while the run after it reports its
// progress - a screen saying "done" about something that is not done yet.
//
// A refusal is what this presses next, because it is the deterministic way to
// make the line speak again: it goes through the same sentence-on-the-line
// path a run's progress does, and it needs no stopwatch.
//
// The first version of this pressed Preview into the folder the run had just
// filled and asserted the colour had been cleared. It failed, and it was the
// guard that was wrong rather than the code: that refusal lands on a box and
// the line at the foot is never touched, so the green it was reading was a
// true statement about the run that really had finished. Worth writing down,
// because the guard read exactly like one that works.
func TestTheColourOfAnOutcomeDoesNotOutliveIt(t *testing.T) {
	dir := t.TempDir()
	host, content := screen(t)

	fill(t, content, text.FieldOutputDir(), dir)
	fill(t, content, text.FieldSize(), "2kb")
	fill(t, content, text.FieldCount(), "2")
	press(t, content, text.ButtonGenerate())
	waitForManifest(t, dir)
	join(host)

	_, status := runMessages(content)
	if status.Importance != widget.SuccessImportance {
		t.Fatalf("the first run did not finish green, so this guard never reaches its question")
	}

	// A size no format can make. The run is refused before a byte is written
	// and the foot of the form says so.
	fill(t, content, text.FieldSize(), "1")
	press(t, content, text.ButtonGenerate())

	if status.Text == text.Written(2) {
		t.Fatalf("the line still reads %q, so the refusal never reached it and this guard is asking nothing", status.Text)
	}
	if status.Importance == widget.SuccessImportance {
		t.Errorf("the line still carries the colour of a finished run while saying %q."+
			" A colour about one run has to be cleared by the next thing said", status.Text)
	}
}

// The colours an outcome uses are colours, and they are ours.
//
// The guards above compare an enum, and an enum proves the field was set. What
// a person sees is whether that changes the drawing at all - so this asks the
// palette the toolkit will read whether the three names it maps to are three
// different colours. Both of these were declared in the palette when it was
// written and reached the screen for the first time on 2026-08-20: checked
// across the whole tree, the only semantic colour in use until then was the
// error red.
func TestTheOutcomeColoursAreThreeDifferentColours(t *testing.T) {
	plain := parts.PaletteColour(theme.ColorNameForeground, theme.VariantDark)
	good := parts.PaletteColour(theme.ColorNameSuccess, theme.VariantDark)
	partial := parts.PaletteColour(theme.ColorNameWarning, theme.VariantDark)

	same := func(a, b interface{}) bool { return a == b }
	if same(good, plain) {
		t.Error("the success colour is the ordinary text colour, so setting it changes nothing on the screen")
	}
	if same(partial, plain) {
		t.Error("the warning colour is the ordinary text colour, so setting it changes nothing on the screen")
	}
	if same(good, partial) {
		t.Error("success and partial success are the same colour, so a run that skipped files looks like one that did not")
	}
}
