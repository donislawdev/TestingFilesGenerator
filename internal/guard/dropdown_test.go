package guard

import (
	"testing"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// A letter typed at a list moves to the values starting with it.
//
// Reported from the screen on 2026-08-18 and it was missing in both halves,
// which is why this asks about both. The toolkit has none of it anywhere:
// widget.Select.TypedRune carries the comment "intentionally left blank" and
// widget.PopUpMenu.TypedRune is an empty method, so pressing p did nothing with
// the list shut and nothing with it open.
//
// The WAI-ARIA authoring practices for a combobox over a closed set put both
// halves under one expectation - a printable character moves to the values
// starting with it, collapsed or expanded - and NN/g lists typing a letter
// among the things a dropdown has to support.
func TestALetterTypedAtTheShutListMovesToThatValue(t *testing.T) {
	_, content := screenOnACanvas(t)
	menu := chooserUnder(t, content, text.FieldFormat())

	// csv is where a fresh screen starts, so p has to reach pdf and png rather
	// than the first value in the list.
	menu.TypedRune('p')
	if menu.Selected != "pdf" {
		t.Errorf("p was typed at a list showing csv and it holds %q, where pdf is the first value starting with p", menu.Selected)
	}

	// The same letter again walks on rather than sticking, which is what a
	// desktop menu does and the only way to reach the second of two values
	// sharing a letter.
	menu.TypedRune('p')
	if menu.Selected != "png" {
		t.Errorf("p was typed twice and the list holds %q rather than walking on to png", menu.Selected)
	}

	// And a letter no value starts with leaves the value alone rather than
	// clearing it.
	menu.TypedRune('q')
	if menu.Selected != "png" {
		t.Errorf("a letter no format starts with changed the value to %q", menu.Selected)
	}
}

// The same letter, with the list open.
//
// It moves the keyboard rather than the value: the list is still open and
// nothing is settled until Enter, which is what the ARIA practices ask for and
// what stops a held key from committing a value nobody looked at.
func TestALetterTypedAtTheOpenListMovesTheKeyboard(t *testing.T) {
	_, content := screenOnACanvas(t)
	menu := chooserUnder(t, content, text.FieldFormat())
	menu.Tapped(&fyne.PointEvent{})

	list := menu.Opened()
	if list == nil {
		t.Fatal("the press opened no list, so this guard has nothing to type at")
	}
	before := menu.Selected

	list.TypedRune('w')
	rows := list.Rows()
	if at := list.Active(); at < 0 || at >= len(rows) || rows[at].Label != "wav" {
		t.Errorf("w was typed at the open list and the keyboard is on row %d of %d, where wav is what starts with w", at, len(rows))
	}
	if menu.Selected != before {
		t.Errorf("typing a letter at an OPEN list settled the value on %q, and nothing is settled until Enter", menu.Selected)
	}
}

// A press opens the list without painting the keyboard's place in it.
//
// The same rule as everywhere else in this window, and it needs saying here
// because the list has to put the arrow keys SOMEWHERE when it opens - on the
// value in the box, so that one press of Down does not jump to the first value
// while the box shows the ninth. Seen on the first render of this control: that
// starting point was drawn as a bar across the row, which says the keyboard is
// here to somebody who has just used a mouse.
func TestOpeningAListWithAPressDrawsNoKeyboardBar(t *testing.T) {
	_, content := screenOnACanvas(t)
	menu := chooserUnder(t, content, text.FieldFormat())
	menu.Tapped(&fyne.PointEvent{})

	list := menu.Opened()
	if list == nil {
		t.Fatal("the press opened no list")
	}
	if list.Showing() {
		t.Error("the list was opened with a press and draws a bar on the row the arrow keys start from")
	}
	if list.Active() < 0 {
		t.Error("the list was opened and the arrow keys start nowhere, so one press of Down jumps to the first value")
	}

	// And the first key turns it on, because from then on somebody is using it.
	list.TypedKey(&fyne.KeyEvent{Name: fyne.KeyDown})
	if !list.Showing() {
		t.Error("an arrow key was pressed at the open list and nothing says where the keyboard is")
	}
}

// The list has a ceiling and scrolls under it.
//
// Measured off the stored tree before this control existed: thirteen formats
// made a list 476 px tall with no limit of any kind, covering the whole form.
// At the twenty-five formats T1 is heading for that is about 925 px, which does
// not fit in the window - so this is a defect that gets worse with every format
// added and would have arrived without a line of code changing.
//
// Asked as a count of rows rather than a number of pixels, because the pixels
// follow the text size and the rule does not.
func TestTheOpenListStopsAtEightRowsHoweverManyValuesThereAre(t *testing.T) {
	_, content := screenOnACanvas(t)
	menu := chooserUnder(t, content, text.FieldFormat())

	values := len(format.IDs())
	if values <= 8 {
		t.Skipf("this build has %d formats, so the ceiling cannot be reached from this screen", values)
	}
	menu.Tapped(&fyne.PointEvent{})
	list := menu.Opened()
	if list == nil {
		t.Fatal("the press opened no list")
	}

	row := parts.ListRowHeight()
	if row <= 0 {
		t.Fatal("a row measures nothing, so the height below says nothing either")
	}
	shown := list.MinSize().Height / row
	if shown > 8.5 {
		t.Errorf("the list shows %.1f of %d values at once, and eight is the ceiling.\n"+
			"Reason: an open list that covers the form takes the context away from the person reading it,\n"+
			"and the number of formats only goes up.\n"+
			"What to do: keep visibleRows in parts/openlist.go.", shown, values)
	}
	if shown < 7.5 {
		t.Errorf("the list shows only %.1f rows of %d, which is fewer than the eight decided on", shown, values)
	}
}

// Escape closes the list and gives the keyboard back to the box.
//
// Asked for by name in the ARIA practices, and it is what stops the keyboard
// being left on a control that is no longer on the screen - after which Tab
// starts from somewhere nobody can see.
func TestEscapeClosesTheListAndGivesTheKeyboardBack(t *testing.T) {
	c, content := screenOnACanvas(t)
	menu := chooserUnder(t, content, text.FieldFormat())
	menu.Tapped(&fyne.PointEvent{})

	list := menu.Opened()
	if list == nil {
		t.Fatal("the press opened no list")
	}
	list.TypedKey(&fyne.KeyEvent{Name: fyne.KeyEscape})

	if menu.Opened() != nil {
		t.Error("Escape was pressed and the list is still the one this menu has open")
	}
	if c.Focused() != fyne.Focusable(menu) {
		t.Errorf("Escape closed the list and the keyboard went to %T rather than back to the box", c.Focused())
	}
	// Out loud, because a key did it - the same rule the rest of the window
	// follows.
	if !menu.Marked() {
		t.Error("Escape gave the keyboard back to the box and nothing on the box says so")
	}
}
