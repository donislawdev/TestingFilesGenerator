package guard

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// The buttons, the switch and the segmented control this window draws itself
// arrived on 2026-09-15, and these guards are for the behaviour that came with
// them - the states the toolkit's own controls either drew wrong or did not
// draw at all. The look of each state is held by the stored screens. What is
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

// One press of the space bar presses a button ONCE.
//
// The desktop driver delivers a press of space to a focused control twice:
// as the key, ending in TypedKey, and as the character, ending in TypedRune
// (internal/driver/glfw/window.go, processKeyPressed and processCharInput,
// read in the pinned module on 2026-09-16 after an outside review said so).
// The test driver delivers them as two separate calls, which is why a button
// answering both was green here and pressed twice in the window - one press
// of space on Add batch added two batches. The toolkit's own button leaves
// TypedRune empty for this reason. Delivered here the way the driver does it,
// both calls for one press.
func TestOnePressOfSpacePressesAButtonOnce(t *testing.T) {
	presses := 0
	b := parts.NewButton(parts.Secondary, "Add batch", func() { presses++ })
	b.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	b.TypedRune(' ')
	if presses != 1 {
		t.Errorf("one press of the space bar, delivered as the key and the character the way the "+
			"desktop driver delivers it, pressed the button %d times", presses)
	}
}

// And flips a switch once - the same two deliveries, and a switch that
// answered both flipped twice, back to where it started, which reads as a
// switch that ignores the space bar. Found by reading the switch beside the
// button on 2026-09-16, not by the review that named the button.
func TestOnePressOfSpaceFlipsASwitchOnce(t *testing.T) {
	flips := 0
	s := parts.NewToggle(func(bool) { flips++ })
	s.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	s.TypedRune(' ')
	if flips != 1 || !s.Checked {
		t.Errorf("one press of the space bar flipped the switch %d times and left it %v", flips, s.Checked)
	}
}

// A frozen control answers no key.
//
// A form is frozen for the length of a run (Fields.Freeze disables every
// control), and a control that had the keyboard keeps it: the focus manager
// asks Disabled only when it MOVES the focus (internal/app/focus_manager.go)
// and the driver hands every key to whatever is focused. So a disabled
// switch went on moving its choice under a form drawn as frozen, and a
// disabled menu went on changing its value on Left and Right through the
// toolkit's own Select.TypedKey, which asks nothing either. An outside review
// of the pull request named the switch on 2026-09-16. The menu came out of
// reading what the switch's fix had to cover.
func TestAFrozenControlAnswersNoKey(t *testing.T) {
	changed := 0
	segments := parts.NewSegments([]string{"one", "two", "three"}, func(string) { changed++ })
	segments.Disable()
	segments.TypedKey(&fyne.KeyEvent{Name: fyne.KeyRight})
	if segments.Selected != "one" || changed != 0 {
		t.Errorf("a frozen segmented switch moved to %q on an arrow and reported %d change(s)", segments.Selected, changed)
	}

	menu := parts.NewChooser([]string{"one", "two", "three"}, func(string) { changed++ })
	menu.SetSelected("one")
	changed = 0
	menu.Disable()
	menu.TypedKey(&fyne.KeyEvent{Name: fyne.KeyRight})
	if menu.Selected != "one" || changed != 0 {
		t.Errorf("a frozen menu moved to %q on an arrow and reported %d change(s)", menu.Selected, changed)
	}

	toggle := parts.NewToggle(func(bool) { changed++ })
	changed = 0
	toggle.Disable()
	toggle.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	if toggle.Checked || changed != 0 {
		t.Errorf("a frozen switch flipped on the space bar and reported %d change(s)", changed)
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

// The tick of a switch is drawn on the whole of its square.
//
// Reported by the owner from the running window on 2026-09-21: the mark in a
// checked box was too small to read as one. Measured: the picture was drawn a
// step inside the square on every side, 12 px in a square of 20, and the
// toolkit's glyph fills about half of its own picture - so the tick was 7 px
// across. The glyph's own margin is all the room it needs, and this holds the
// picture to the square's edges.
func TestTheTickOfASwitchFillsItsSquare(t *testing.T) {
	s := parts.NewToggle(func(bool) {})
	s.SetChecked(true)
	s.Resize(s.MinSize())
	var square *canvas.Rectangle
	var tick *canvas.Image
	for _, o := range test.WidgetRenderer(s).Objects() {
		switch drawn := o.(type) {
		case *canvas.Rectangle:
			if drawn.FillColor == parts.PaletteColour(theme.ColorNamePrimary, theme.VariantDark) {
				square = drawn
			}
		case *canvas.Image:
			tick = drawn
		}
	}
	if square == nil || tick == nil {
		t.Fatalf("a checked switch draws no filled square or no tick (square %v, tick %v)", square != nil, tick != nil)
	}
	if tick.Size() != square.Size() || tick.Position() != square.Position() {
		t.Errorf("the tick is %v at %v and the square %v at %v - a tick drawn inside a margin of the square is a mark too small to read",
			tick.Size(), tick.Position(), square.Size(), square.Position())
	}
}

// A secondary button wears a face at rest, and the pointer lifts it.
//
// Reported by the owner from the running window on 2026-09-21: Duplicate,
// Choose, Preview and Add a batch looked very weak. Measured on the shot: an
// outline one pixel wide round nothing, with bold words inside - a bordered
// word, not a thing to press. The face is the surface a box to type in has,
// and it lightens under the pointer and again under a press, so the three
// states are three colours and not one. Asked of the drawn rectangle, so a
// face computed and not painted goes red.
func TestASecondaryButtonWearsAFaceAtRest(t *testing.T) {
	b := parts.NewButton(parts.Secondary, "Preview", func() {})
	b.Resize(b.MinSize())
	faceOf := func() color.Color {
		for _, o := range test.WidgetRenderer(b).Objects() {
			if rect, is := o.(*canvas.Rectangle); is && rect.StrokeWidth > 0 {
				return rect.FillColor
			}
		}
		t.Fatal("the button draws no edged rectangle, so it has no face to measure")
		return nil
	}
	rest := faceOf()
	if want := parts.PaletteColour(theme.ColorNameInputBackground, theme.VariantDark); rest != want {
		t.Errorf("at rest the face is %v and should be the surface of a box to type in, %v - an outline round nothing reads as a bordered word", rest, want)
	}
	b.MouseIn(&desktop.MouseEvent{})
	hovered := faceOf()
	if hovered == rest {
		t.Error("the pointer arriving changed nothing about the face, so nobody can see the button noticed it")
	}
	b.MouseDown(&desktop.MouseEvent{})
	if pressed := faceOf(); pressed == hovered || pressed == rest {
		t.Errorf("a press draws %v, which is the hovered or the resting face - a press has to be told from the hover it follows", pressed)
	}
	b.MouseUp(&desktop.MouseEvent{})
	b.MouseOut()
	b.Disable()
	// No fill is nil or transparent, both of which the toolkit draws as nothing.
	if off := faceOf(); off != nil && off != color.Transparent {
		t.Errorf("a disabled button still wears a face (%v), and a face on a control that does nothing is a control that lies", off)
	}
}
