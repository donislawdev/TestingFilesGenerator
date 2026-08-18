package parts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// PointerFocus is the part of a control that knows what put the keyboard in it.
//
// Every focusable control in this toolkit draws its "the keyboard is here" mark
// as a FILL in the focus colour, and every one of them takes the keyboard when
// it is pressed with the mouse - widget/select.go tapped, widget/check.go
// Tapped, both by way of focusIfNotMobile. Neither gives it back. So a value
// chosen with the mouse leaves the menu painted blue for as long as the screen
// is open, and a switch flipped with the mouse keeps a blue disc behind it. Both
// were reported from the screen, the menu twice - 2026-08-12 and 2026-08-18.
//
// The mark is not the problem and neither is its shape. The problem is that a
// mark meaning "the keyboard is here" is drawn for somebody who is not using the
// keyboard. So the pointer moves the focus silently and the keyboard draws it,
// which is what every desktop platform does and what the web calls focus-visible.
//
// It is a type rather than a bool in each control because the next focusable
// control has to get the same answer without anybody remembering. See
// docs/UX.md section 7.0 gate 2: a fix that has to be repeated is not a fix of
// the class.
type PointerFocus struct{ silent bool }

// Quietly moves the focus the way a press does: the control gets the keyboard
// and nothing is drawn to say so.
func (p *PointerFocus) Quietly(focus func()) {
	p.silent = true
	defer func() { p.silent = false }()
	focus()
}

// Quiet reports whether the focus arriving right now came from the pointer.
func (p *PointerFocus) Quiet() bool { return p.silent }

// Toggle is a switch that shows the keyboard mark only when the keyboard put it
// there.
//
// The toolkit draws that mark as a disc behind the square, filled with the focus
// colour - widget/check.go updateFocusIndicator - and its shape cannot be
// changed from a theme, because the geometry is fixed in checkRenderer.Layout
// and the theme supplies only the colour. That is why this is not a third shape:
// there is no shape to choose. The disc is right for the keyboard and wrong for
// the mouse, so it is drawn for one and not the other.
//
// A ring is not the answer either and that is written down rather than guessed.
// One was built on 2026-08-12 and withdrawn the same hour, because a ring round
// a checkbox goes round its words as well as its square and reads as a box drawn
// around a sentence. See the note at the foot of ring.go.
type Toggle struct {
	widget.Check

	from   PointerFocus
	marked bool
}

// NewToggle makes a switch carrying its own name.
func NewToggle(name string, changed func(bool)) *Toggle {
	t := &Toggle{}
	t.Text = name
	t.OnChanged = changed
	t.ExtendBaseWidget(t)
	return t
}

// Tapped flips the switch and takes the keyboard without drawing the mark.
//
// The value is flipped by the toolkit rather than here, on purpose: widget.Check
// changes Checked inside Tapped, so a switch that set its own value would set it
// twice or disagree about which press did it.
func (t *Toggle) Tapped(event *fyne.PointEvent) {
	t.from.Quietly(func() { t.Check.Tapped(event) })
}

// FocusGained draws the mark only for the keyboard. See PointerFocus.
func (t *Toggle) FocusGained() {
	if t.from.Quiet() {
		return
	}
	t.mark()
}

func (t *Toggle) mark() {
	t.marked = true
	t.Check.FocusGained()
}

func (t *Toggle) FocusLost() {
	t.marked = false
	t.Check.FocusLost()
}

// TypedKey turns the mark on, because somebody has now used the keyboard. Space
// flips the switch and the toolkit does that part.
func (t *Toggle) TypedKey(event *fyne.KeyEvent) {
	if !t.marked {
		t.mark()
	}
	t.Check.TypedKey(event)
}

// Marked says whether the keyboard mark is drawn, for a guard. The toolkit keeps
// this in an unexported field, so the alternative is reading a colour off the
// canvas and deciding what it meant.
func (t *Toggle) Marked() bool { return t.marked }
