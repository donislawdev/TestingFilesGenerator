package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// The buttons that run something sit at the right hand end of the form.
//
// Asked about from the screen on 2026-08-18: they stood at the left and the
// owner asked why. The honest answer was that nobody had chosen it - measured
// from the stored tree that morning, the bar is 820 px wide and the two buttons
// were at x=0 and x=73, which is where a horizontal box puts things when
// nothing pushes them. Decided the same day: the end of the reading path, which
// is where a form with a fixed action bar puts the thing it wants pressed last.
//
// Written as a guard rather than left to the stored picture because the two
// answer different questions. The picture says the screen has not changed. This
// says which arrangement was chosen, so somebody reading the failure is told
// what the rule is rather than being handed two images to compare.
func TestTheButtonsThatRunSomethingSitAtTheRightEdge(t *testing.T) {
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
	right := run.Position().X + run.Size().Width
	if gap := row.Size().Width - right; gap > slack {
		t.Errorf("the %q button ends %.1f px short of the right edge of a bar %.1f px wide.\n"+
			"Reason: the action bar was decided on 2026-08-18 to end at the right of the column,\n"+
			"so the last thing read is the thing to press.\n"+
			"What to do: keep the spacer in front of the buttons in runner.actions.",
			text.ButtonGenerate, gap, row.Size().Width)
	}

	// And the other half: a bar that is only as wide as its buttons would pass
	// the check above while looking exactly like the arrangement that was
	// reported.
	first := buttonNamed(content, text.ButtonPreview)
	if first == nil {
		t.Fatalf("there is no %q button, so this guard read the wrong tree", text.ButtonPreview)
	}
	if first.Position().X <= slack {
		t.Errorf("the %q button starts at x=%.1f in a bar %.1f px wide, which is the left hand "+
			"arrangement that was reported", text.ButtonPreview, first.Position().X, row.Size().Width)
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
