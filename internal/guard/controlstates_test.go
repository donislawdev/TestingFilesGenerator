package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// The buttons, the switch and the segmented control this window draws itself
// arrived on 2026-09-15, and these guards are for the behaviour that came with
// them - the states the toolkit's own controls either drew wrong or did not
// draw at all. The look of each state is held by the stored screens; what is
// here is the behaviour a picture cannot show.

// A button draws its focus ring for the keyboard and not for a press.
//
// This is O207 turned into a test. The toolkit blended the focus colour into
// the fill, which on the filled primary button was blue on blue at 1.11 - a
// mark drawn and not seen. Ours is a ring, and it is drawn only when the
// keyboard put the focus here: a press does not focus a button (the driver
// unfocuses on a tap), so a mouse user never lights the ring, and a keyboard
// user always does.
func TestAButtonDrawsItsRingForTheKeyboardNotThePointer(t *testing.T) {
	b := parts.NewButton(parts.Primary, "Generate", func() {})
	if b.Marked() {
		t.Fatal("a fresh button is already drawing its ring")
	}

	b.MouseIn(&desktop.MouseEvent{})
	if b.Marked() {
		t.Error("the pointer entered the button and it drew the keyboard ring, which says the keyboard is here when it is not")
	}

	b.FocusGained()
	if !b.Marked() {
		t.Error("the keyboard moved onto the button and nothing on it says so")
	}
	b.FocusLost()
	if b.Marked() {
		t.Error("the keyboard left the button and the ring stayed")
	}
}

// A button forgets a press when the pointer leaves it.
//
// The driver sends the release to whatever is under the pointer when the button
// is let go, so pressing a button, sliding off it and releasing never delivers
// MouseUp back - and a button that only cleared the press in MouseUp would
// stay drawn as pressed for the rest of its life. Measured in the toolkit
// source (the Fyne guide, section 3.27).
func TestAButtonForgetsAPressWhenThePointerLeaves(t *testing.T) {
	b := parts.NewButton(parts.Secondary, "Preview", func() {})
	b.MouseDown(&desktop.MouseEvent{})
	if !b.Pressed() {
		t.Fatal("pressing the button did not draw the pressed face")
	}
	b.MouseOut()
	if b.Pressed() {
		t.Error("the pointer left mid press and the button stayed drawn as pressed, with no way back")
	}
}

// A button is pressed by Enter as well as by the space bar.
//
// The toolkit answers the space bar alone, so Enter on a focused button did
// nothing - and silence in place of an answer is the one thing this window
// refuses everywhere else. Both names of the return key, because a keyboard has
// two.
func TestAButtonIsPressedByEnterAsWellAsSpace(t *testing.T) {
	for _, key := range []fyne.KeyName{fyne.KeyReturn, fyne.KeyEnter, fyne.KeySpace} {
		pressed := false
		b := parts.NewButton(parts.Primary, "Generate", func() { pressed = true })
		b.TypedKey(&fyne.KeyEvent{Name: key})
		if !pressed {
			t.Errorf("%s did not press the button", key)
		}
	}
	// And a disabled button answers no key, which is the half that keeps a
	// shortcut from starting a run the screen has turned off.
	pressed := false
	b := parts.NewButton(parts.Primary, "Generate", func() { pressed = true })
	b.Disable()
	b.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if pressed {
		t.Error("Enter pressed a disabled button")
	}
}

// A segmented switch ignores a value it does not hold.
//
// A switch of fixed choices is not a box: handed a word it does not offer it
// changes nothing and stays on what it had, so a copy of a batch that reads one
// switch and writes another cannot land it on a value that is not there.
func TestASegmentedSwitchIgnoresAValueItDoesNotHold(t *testing.T) {
	changed := 0
	s := parts.NewSegments([]string{"one", "two", "three"}, func(string) { changed++ })
	if s.Selected != "one" {
		t.Fatalf("a fresh switch opened on %q, not the first value", s.Selected)
	}
	s.SetSelected("four")
	if s.Selected != "one" {
		t.Errorf("the switch moved to %q, a value it does not offer", s.Selected)
	}
	if changed != 0 {
		t.Error("the switch called OnChanged for a value it did not move to")
	}
	s.SetSelected("two")
	if s.Selected != "two" || changed != 1 {
		t.Errorf("the switch did not move to a value it does offer: selected %q, changed %d times", s.Selected, changed)
	}
}

// A segmented switch moves the choice with the arrows.
//
// One control with the arrows walking the choice, rather than three circles Tab
// stops at one by one - and the arrows MOVE the choice rather than only the
// focus, because a row of exclusive options is chosen by arrowing through it.
func TestASegmentedSwitchMovesTheChoiceWithTheArrows(t *testing.T) {
	s := parts.NewSegments([]string{"one", "two", "three"}, nil)
	s.TypedKey(&fyne.KeyEvent{Name: fyne.KeyRight})
	if s.Selected != "two" {
		t.Errorf("right arrow left the switch on %q", s.Selected)
	}
	s.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEnd})
	if s.Selected != "three" {
		t.Errorf("End left the switch on %q", s.Selected)
	}
	s.TypedKey(&fyne.KeyEvent{Name: fyne.KeyRight})
	if s.Selected != "three" {
		t.Errorf("right arrow past the last value moved off it to %q", s.Selected)
	}
	s.TypedKey(&fyne.KeyEvent{Name: fyne.KeyHome})
	if s.Selected != "one" {
		t.Errorf("Home left the switch on %q", s.Selected)
	}
}
