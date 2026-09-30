package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// The hands a guard works a screen with: fill a box, press a button, flip a
// switch, pick from a menu. Moved out of screenpixels_test.go on 2026-09-30,
// when that file went past the ceiling on the length of a test file - they
// are used by guards about every screen, not only by the one comparing
// pictures.

func fillField(t *testing.T, o fyne.CanvasObject, label, value string) {
	t.Helper()
	entryUnder(t, o, label).SetText(value)
}

// pressNamed calls the handler rather than tapping through the canvas, and that
// is deliberate here. Whether a button can really be reached is asked by
// TestEveryButtonAPersonCanSeeIsReallyPressable, which taps for real. This one
// is asking what the screen LOOKS like afterwards, so it wants the state and
// not the hit test - and a tap that missed would leave this guard comparing a
// screen in the wrong state against a picture, which reads as a rendering
// defect and is not one.
func pressNamed(t *testing.T, o fyne.CanvasObject, name string) {
	t.Helper()
	b := buttonNamed(o, name)
	if b == nil {
		t.Fatalf("there is no %q button on this screen", name)
	}
	if b.Disabled() {
		t.Fatalf("the %q button is disabled, so this state cannot be reached", name)
	}
	b.OnTapped()
}

// flipSwitch turns a switch off the way a person does, through the canvas.
//
// Tapping rather than calling SetChecked, and the difference is visible: a tap
// also takes focus - widget.Check.Tapped calls focusIfNotMobile - so a switch
// set in code draws no focus ring and a switch pressed by somebody does. The
// picture would differ from the screen, which is the one thing this guard must
// never do.
//
// The point is inside MinSize rather than the middle of the widget, because
// Check.Tapped ignores anything past its MinSize width and our switches are
// stretched to the width of the column. tools/probes/guirender presses the same
// way for the same reason.
func flipSwitch(t *testing.T, c fyne.Canvas, o fyne.CanvasObject, label string) {
	t.Helper()
	box := checkNamed(o, label)
	if box == nil {
		t.Fatalf("there is no switch labelled %q on this screen", label)
	}
	before := box.Checked

	at := fyne.CurrentApp().Driver().AbsolutePositionForObject(box)
	active := box.MinSize()
	test.TapCanvas(c, at.Add(fyne.NewPos(active.Width/2, box.Size().Height/2)))

	if box.Checked == before {
		t.Fatalf("a press on the switch %q did not change it, so this state was never built.\n"+
			"Reason: it is %gx%g at %v and the press reached something else.\n"+
			"What to do: check whether the switch moved behind another control or off the laid out area.",
			label, box.Size().Width, box.Size().Height, at)
	}
}

func chooserFor(t *testing.T, o fyne.CanvasObject) *parts.Chooser {
	t.Helper()
	chooser, ok := controlUnder(o, text.FieldFormat()).(*parts.Chooser)
	if !ok {
		t.Fatal("there is no format menu on this screen")
	}
	return chooser
}

func chooseFormat(t *testing.T, o fyne.CanvasObject, format string) {
	t.Helper()
	chooserFor(t, o).SetSelected(format)
}

// menuUnder is the list under any labelled field, on any screen.
//
// chooserFor asks for the format menu by name and only one screen has one. This
// is what the preset screen's own two lists needed, and not having it is why
// neither had ever been photographed.
func menuUnder(t *testing.T, o fyne.CanvasObject, label string) *parts.Chooser {
	t.Helper()
	chooser, ok := controlUnder(o, label).(*parts.Chooser)
	if !ok {
		t.Fatalf("there is no menu under %q on this screen", label)
	}
	return chooser
}

// chooseWithThePointer picks a value the way somebody with a mouse does.
//
// The press is what matters and not the value. Since 2026-08-18 a press moves
// the keyboard into the control WITHOUT drawing the mark that says so, and
// calling SetSelected on its own never presses anything - so a scene built that
// way photographs the keyboard path and says nothing about the one that was
// reported twice from the screen.
//
// The list is taken away afterwards because that is what happens: the item that
// sets the value is inside the popup, and pressing it closes the popup behind
// itself.
func chooseWithThePointer(t *testing.T, c fyne.Canvas, o fyne.CanvasObject, format string) {
	t.Helper()
	chooser := chooserFor(t, o)
	chooser.Tapped(&fyne.PointEvent{})
	chooser.SetSelected(format)
	if top := c.Overlays().Top(); top != nil {
		c.Overlays().Remove(top)
	}
}

// explanationBeside finds the question mark button that sits next to a field.
func explanationBeside(t *testing.T, o fyne.CanvasObject, label string) *parts.DetailButton {
	t.Helper()
	var found *parts.DetailButton
	walk(o, func(obj fyne.CanvasObject) {
		row, ok := obj.(*fyne.Container)
		if !ok || len(row.Objects) < 2 || found != nil {
			return
		}
		head, named := wordsOf(row.Objects[0])
		if !named || head != label {
			return
		}
		// Searched, not indexed - see detailButtonIn. A field that has to be
		// filled in carries a star between its name and this button.
		if b := detailButtonIn(row); b != nil {
			found = b
		}
	})
	if found == nil {
		t.Fatalf("there is no explanation button beside %q", label)
	}
	return found
}
