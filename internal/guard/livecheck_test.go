package guard

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// Every box that is wrong is marked, not the first one.
//
// Reported from the screen on 2026-08-18: after Preview or Generate exactly one
// field stood in red however many were bad. It was not a marking defect - it
// was that everything between the screen and the field registry was singular by
// TYPE. settle returned at the first bad box, refuse took one error, Mark marks
// one field, so there was never a moment at which two refusals existed at once.
//
// The layer below has done this since the beginning. RC7 has a recipe refused
// with every problem it has rather than the first, and the reason given there
// is the reason here: fixing a file one error per run is the cheapest way to
// make somebody stop using the tool.
func TestARefusalAboutTwoBoxesMarksBothOfThem(t *testing.T) {
	_, content := screen(t)
	fill(t, content, text.FieldSize, "abc")
	fill(t, content, text.FieldCount, "many")
	press(t, content, "Preview")

	for _, label := range []string{text.FieldSize, text.FieldCount} {
		if edgeOf(t, content, label).StrokeWidth <= 0 {
			t.Errorf("%q holds a value the run cannot use and its box draws no edge", label)
		}
	}

	// A box nobody spoiled has to look untouched, or this passes for a window
	// that marks the whole form whenever anything is wrong.
	if quiet := edgeOf(t, content, text.FieldSeed).StrokeWidth; quiet != 0 {
		t.Errorf("the seed was not refused and its box draws an edge %.1f px wide", quiet)
	}
}

// A bad value says so as it is typed, with nothing pressed.
//
// Asked for from the screen on 2026-08-18. Until then the only OnChanged
// anywhere in the window was the one handed to a menu, so a box said nothing
// about what was in it until somebody pressed a button - and the reader who
// mistypes a size finds out after asking for a run rather than while writing it.
//
// It runs the same settle the buttons run, which is what makes it worth having
// rather than a second opinion: there is no separate rule here to keep in step
// with the one the run uses, and a box that can be refused by a press is
// refused here for the same reason and in the same words.
func TestABadValueMarksItsBoxWithNothingPressed(t *testing.T) {
	_, content := screen(t)
	fill(t, content, text.FieldSize, "abc")

	if edgeOf(t, content, text.FieldSize).StrokeWidth <= 0 {
		t.Error("a size that is not a size was typed and the box says nothing until a button is pressed")
	}
	if said := sayingUnder(t, content, text.FieldSize); said == "" {
		t.Error("the box was marked and given no reason, so the colour is the whole of the message")
	}

	// And it goes when the value is fixed, which is the half that looks like
	// nothing while it is missing.
	fill(t, content, text.FieldSize, "10mb")
	if left := edgeOf(t, content, text.FieldSize).StrokeWidth; left != 0 {
		t.Errorf("the size was corrected and its box still draws an edge %.1f px wide", left)
	}
}

// Typing in one box leaves what another box was told alone.
//
// Found by the stored widget tree on 2026-08-18 rather than by reading. The
// first version of the live check cleared every refusal on the screen at each
// keystroke, so a size the FORMAT had refused went unmarked the moment anything
// else was typed - and it was still refused. The scene that caught it pins the
// output directory after building its state, which is a keystroke in every way
// that matters here.
//
// The distinction it rests on: what a person is typing has a stale verdict and
// what they are not typing does not. The live check can only see what settle
// sees, and a format minimum, a name already taken and a disk that is too full
// are the engine's answers - so wiping them on a keystroke removes an answer
// nothing here is able to work out again.
func TestTypingInOneBoxLeavesAnotherBoxesRefusalAlone(t *testing.T) {
	_, content := screen(t)
	fill(t, content, text.FieldSize, "1")
	press(t, content, "Preview")
	if edgeOf(t, content, text.FieldSize).StrokeWidth <= 0 {
		t.Fatal("the refused size was never marked, so this guard cannot tell whether the mark survives")
	}

	fill(t, content, text.FieldTargetID, "other")

	if edgeOf(t, content, text.FieldSize).StrokeWidth <= 0 {
		t.Error("something was typed in another box and the size stopped being marked, " +
			"though the size is still the one the format refused")
	}
	if said := sayingUnder(t, content, text.FieldSize); said == "" {
		t.Error("the reason under the size went away when a different box was typed in")
	}
}

// An empty box is somebody about to type, not somebody who typed something bad.
//
// Emptying a box to retype it is the commonest thing anybody does in a form,
// and a form that turns red the moment the box is empty is worse than one that
// waits. The toolkit takes the same view about its own validation and says so
// in widget/entry_validation.go, where an error is held back while the box has
// the keyboard.
//
// Pressing a button marks it anyway, and that is the other half: by then the
// person has said they are done.
func TestAnEmptyBoxIsNotCalledWrongUntilSomethingIsPressed(t *testing.T) {
	_, content := screen(t)
	fill(t, content, text.FieldSize, "")

	if marked := edgeOf(t, content, text.FieldSize).StrokeWidth; marked > 0 {
		t.Errorf("the size box was emptied and immediately marked %.1f px wide, "+
			"so clearing a value to retype it looks like a mistake", marked)
	}

	press(t, content, "Preview")
	if edgeOf(t, content, text.FieldSize).StrokeWidth <= 0 {
		t.Error("a run was asked for with an empty size and the box was not marked")
	}
}

// sayingUnder is the refusal shown under one field, read off the tree.
//
// From the tree rather than from the registry, and the difference has been paid
// for here before: a screen can hold a message in a field object and never draw
// it, which the registry cannot tell apart from a screen that draws it. The
// refusal is a label in the danger importance - parts.NewErrorArea - so this
// asks for exactly the thing somebody reads.
func sayingUnder(t *testing.T, o fyne.CanvasObject, label string) string {
	t.Helper()
	box := fieldBox(o, label)
	if box == nil {
		t.Fatalf("there is no field labelled %q, so this guard read the wrong tree", label)
	}
	// The smallest block holding both the field and something red, rather than
	// the field alone. Since 2026-08-20 two fields share a row and their
	// messages are laid out under it across the full width, so a message is no
	// longer a descendant of its own field - see Fields.Row and the reason
	// there.
	//
	// In a row this reads the messages of both fields in it. That is enough for
	// what these guards ask - whether anything is being said around this box -
	// and WHICH box a message is about is answered by the edge drawn on it,
	// which has guards of its own.
	var best *fyne.Container
	var said []string
	walk(o, func(obj fyne.CanvasObject) {
		block, ok := obj.(*fyne.Container)
		if !ok || !holds(block, box) {
			return
		}
		var red []string
		walk(block, func(inner fyne.CanvasObject) {
			if l, is := inner.(*widget.Label); is && l.Importance == widget.DangerImportance && l.Text != "" {
				red = append(red, l.Text)
			}
		})
		if len(red) == 0 {
			return
		}
		if best == nil || countObjects(block) < countObjects(best) {
			best, said = block, red
		}
	})
	return strings.Join(said, "\n")
}

// countObjects is how big a block is, for picking the smallest one.
func countObjects(o fyne.CanvasObject) int {
	n := 0
	walk(o, func(fyne.CanvasObject) { n++ })
	return n
}

// What this defends. A screen keeps checking what is typed into it after a run
// has finished, instead of quietly going deaf.
//
// This was a real defect and it is the shape worth remembering, not the line
// that fixed it. The live check asked "is a run going" by looking at the handle
// that stops one - and that handle is deliberately never cleared, for a
// threading reason that is good and is written down at its declaration. So from
// the first Generate onwards the check returned at its first line, for the rest
// of the session. Fields stopped being marked while being typed in and stopped
// being unmarked once corrected.
//
// Nothing about the screen looked different. Every guard for the live check
// still passed, because every one of them started from a screen that had never
// run anything. That is the gap this closes: the state was reachable only by
// doing something first.
func TestTypingIsStillCheckedAfterARunHasFinished(t *testing.T) {
	dir := t.TempDir()
	host, content := screen(t)

	fill(t, content, text.FieldOutputDir, dir)
	fill(t, content, text.FieldSize, "2kb")
	fill(t, content, text.FieldCount, "2")
	press(t, content, "Generate")
	waitForManifest(t, dir)
	join(host)

	// Asserted rather than assumed. A guard that reaches this line with the run
	// still going would be testing the frozen form and passing for the wrong
	// reason - O118, which has already happened twice in this package.
	box := entryUnder(t, content, text.FieldSize)
	if box == nil {
		t.Fatal("there is no size box, so this guard read the wrong tree")
	}
	if box.Disabled() {
		t.Fatal("the form is still frozen, so this guard never reached the state it is about: a run that has ENDED")
	}

	fill(t, content, text.FieldSize, "abc")
	if edgeOf(t, content, text.FieldSize).StrokeWidth <= 0 {
		t.Error("a size that is not a size was typed after a run had finished and the box says nothing.\n" +
			"The screen has stopped checking what is typed into it, and looks no different doing so.")
	}
	if said := sayingUnder(t, content, text.FieldSize); said == "" {
		t.Error("the box was marked after a finished run and given no reason, so the colour is the whole of the message")
	}

	// And the other half: it still comes back off. A screen that can mark but
	// not unmark leaves a corrected field looking wrong for the rest of the
	// session.
	fill(t, content, text.FieldSize, "10mb")
	if left := edgeOf(t, content, text.FieldSize).StrokeWidth; left != 0 {
		t.Errorf("the size was corrected after a finished run and its box still draws an edge %.1f px wide", left)
	}
}
