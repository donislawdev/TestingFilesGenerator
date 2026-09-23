package guard

import (
	"testing"

	"fyne.io/fyne/v2"

	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// Nothing a person can operate stands on the form without a name over it.
//
// The switch between the three ways of stating a size was the one that did,
// from the day it replaced three radio circles until 2026-09-23. Every box on
// that screen carried a name in the column and this stood between two of them
// with nothing, so what it was about came from what happened to sit next to
// it - which is guessing, and which is the one control the rest of the screen
// teaches somebody not to expect.

// TestEveryControlOnTheFormStandsUnderAName walks the batch screen and asks
// every switch, box and menu on it whether the thing above it is a name.
//
// Read off the FORM rather than from a list of what should be named, which is
// the difference between this and a guard that would have stayed green: a
// control added tomorrow with no name is caught by being on the screen, and no
// list has to be remembered. The shape asked about is the one the window
// builds - a field is a container laid out as one, and its first thing is the
// name (parts.IsField) - so a control that went on some other way is not
// named by definition and is reported by position.
func TestEveryControlOnTheFormStandsUnderAName(t *testing.T) {
	batches, _, _ := screenInAWindowWithHost(t, text.TabRecipe())

	// The one this is really about is on the screen, or the walk below proves
	// nothing by finding nothing.
	switchOnIt := sizeWaySwitch(t, batches)

	named := map[fyne.CanvasObject]string{}
	walk(batches, func(obj fyne.CanvasObject) {
		field, ok := obj.(*fyne.Container)
		if !ok || !parts.IsField(field) || len(field.Objects) < 2 {
			return
		}
		head, is := headingOf(field.Objects[0])
		if !is || head == "" {
			return
		}
		walk(field.Objects[1], func(inner fyne.CanvasObject) { named[inner] = head })
	})
	// A box to tick carries its name BESIDE the square since 2026-09-23 - the
	// line is the square, then the name (parts.ToggleSaying) - so for a switch
	// the name is asked of what stands after it rather than over it.
	walk(batches, func(obj fyne.CanvasObject) {
		line, ok := obj.(*fyne.Container)
		if !ok || len(line.Objects) < 2 {
			return
		}
		if check, isToggle := line.Objects[0].(*parts.Toggle); isToggle {
			if head, is := headingOf(line.Objects[1]); is && head != "" {
				named[check] = head
			}
		}
	})

	if got, is := named[switchOnIt]; !is {
		t.Errorf("the switch that chooses between %q, %q and %q stands on the form with no name over it, "+
			"so the only thing saying what it is about is what happens to be beside it",
			text.SizeWayExact(), text.SizeWayRange(), text.SizeWayBoundary())
	} else if got != text.FieldSizeWay() {
		t.Errorf("the switch between the three ways of stating a size stands under %q and the window calls it %q",
			got, text.FieldSizeWay())
	}

	// And it is the whole class rather than the one case. Anything a person
	// can type into, choose from or press on the form is asked the same
	// question - see the doc above for why this is walked and not listed.
	walk(batches, func(obj fyne.CanvasObject) {
		if !operable(obj) || !obj.Visible() {
			return
		}
		if _, is := named[obj]; !is {
			t.Errorf("a %T is on the form with no name over it", obj)
		}
	})
}

// operable is a control somebody works with, as opposed to the words, the
// rules and the surfaces around one.
//
// Named by type rather than by "anything focusable", because a button is
// focusable and buttons say what they do on their own face - the bar at the
// foot of the screen is full of them and none belongs to the column of names.
func operable(o fyne.CanvasObject) bool {
	switch o.(type) {
	case *parts.Entry, *parts.Chooser, *parts.Toggle, *parts.Segments:
		return true
	}
	return false
}
