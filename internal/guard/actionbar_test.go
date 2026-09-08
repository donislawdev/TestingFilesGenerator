package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// The buttons that run something sit at the right hand end of the bar.
//
// The owner's decision of 2026-09-08, and it is the THIRD answer this one
// question has had. All three came from looking at the built window and none
// was a mistake, so all three are kept: at the left because that is where a
// horizontal box puts things when nothing pushes them and nobody had chosen it
// (measured 2026-08-18, the bar 820 px wide with the buttons at x=0 and x=73),
// then at the right as the end of the reading path, then in the middle, and now
// at the right again.
//
// What settled it this time was the two column body arriving. The middle of a
// bar has no relationship to the column somebody has just finished typing in,
// and the right hand end is where every desktop this ships to puts the button
// it wants pressed last.
//
// Written as a guard rather than left to the stored picture because the two
// answer different questions. The picture says the screen has not changed. This
// says which arrangement was chosen, so somebody reading the failure is told
// what the rule is rather than being handed two images to compare.
func TestTheButtonsThatRunSomethingSitAtTheRightHandEnd(t *testing.T) {
	_, content := screenOnACanvas(t)

	run := buttonNamed(content, text.ButtonGenerate())
	if run == nil {
		t.Fatalf("there is no %q button, so this guard read the wrong tree", text.ButtonGenerate())
	}
	row := rowHolding(content, run)
	if row == nil {
		t.Fatal("the Generate button is not inside a container, so its place cannot be read")
	}

	// Within one padding of the edge. An exact number would be a copy of the
	// layout's arithmetic rather than a statement about where the button is.
	const slack = 8

	first := buttonNamed(content, text.ButtonPreview())
	if first == nil {
		t.Fatalf("there is no %q button, so this guard read the wrong tree", text.ButtonPreview())
	}

	// The group is measured rather than one button, because being at an end is
	// a property of the pair: either one alone can sit near the edge while the
	// two of them are plainly somewhere else.
	before := first.Position().X
	after := row.Size().Width - (run.Position().X + run.Size().Width)

	if before <= slack {
		t.Errorf("the run buttons are hard against the LEFT edge: %.1f px before them, in a "+
			"bar %.1f px wide. Nothing should be there - that end belongs to the rail.",
			before, row.Size().Width)
	}
	// Far more room before them than after. "After" is not zero because the key
	// name that presses Generate stands past it, which is the one thing allowed
	// to be further right than the last button.
	if before <= after {
		t.Errorf("the run buttons are not at the right hand end: %.1f px before them and %.1f px "+
			"after, in a bar %.1f px wide. What to do: keep ONE greedy spacer in front of the "+
			"group in runner.actions - a spacer pushes, so one in front puts the group at the far "+
			"end and a second one behind it would put it back in the middle.",
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
