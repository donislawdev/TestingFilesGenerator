package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// The buttons that run something sit in the middle of the form.
//
// The owner's decision of 2026-08-19, and it REVERSES the one of 2026-08-18
// that this guard used to hold. Both came from looking at the built window, and
// the earlier reasoning is kept rather than deleted because it was a choice and
// not a mistake - a reader who sees only the new rule cannot tell whether the
// old one was ever tried.
//
// What it said before: they stood at the left, the owner asked why, and the
// honest answer was that nobody had chosen it - measured from the stored tree
// that morning, the bar is 820 px wide and the two buttons were at x=0 and
// x=73, which is where a horizontal box puts things when nothing pushes them.
// It was then set to the right edge, as the end of the reading path.
//
// Written as a guard rather than left to the stored picture because the two
// answer different questions. The picture says the screen has not changed. This
// says which arrangement was chosen, so somebody reading the failure is told
// what the rule is rather than being handed two images to compare.
func TestTheButtonsThatRunSomethingSitInTheMiddle(t *testing.T) {
	_, content := screenOnACanvas(t)

	run := buttonNamed(content, text.ButtonGenerate)
	if run == nil {
		t.Fatalf("there is no %q button, so this guard read the wrong tree", text.ButtonGenerate)
	}
	row := rowHolding(content, run)
	if row == nil {
		t.Fatal("the Generate button is not inside a container, so its place cannot be read")
	}

	// Within one padding of the edge. An exact number would be a copy of the
	// layout's arithmetic rather than a statement about where the button is.
	const slack = 8

	first := buttonNamed(content, text.ButtonPreview)
	if first == nil {
		t.Fatalf("there is no %q button, so this guard read the wrong tree", text.ButtonPreview)
	}

	// The group is measured rather than one button, because being centred is a
	// property of the pair: either one alone can sit near the middle while the
	// two of them are plainly off to one side.
	before := first.Position().X
	after := row.Size().Width - (run.Position().X + run.Size().Width)

	if before <= slack || after <= slack {
		t.Errorf("the run buttons are hard against an edge: %.1f px before them and %.1f px "+
			"after, in a bar %.1f px wide. They were moved to the middle on 2026-08-19.",
			before, after, row.Size().Width)
	}
	if diff := before - after; diff > slack || diff < -slack {
		t.Errorf("the run buttons are not centred: %.1f px before them and %.1f px after, in a "+
			"bar %.1f px wide. What to do: keep a spacer at BOTH ends of the group in "+
			"runner.actions, because one spacer can only push the group to an end.",
			before, after, row.Size().Width)
	}
}

// rowHolding is the container one control is directly inside.
func rowHolding(o fyne.CanvasObject, want *widget.Button) *fyne.Container {
	var found *fyne.Container
	walk(o, func(obj fyne.CanvasObject) {
		box, ok := obj.(*fyne.Container)
		if !ok {
			return
		}
		for _, child := range box.Objects {
			if child == want {
				found = box
			}
		}
	})
	return found
}
