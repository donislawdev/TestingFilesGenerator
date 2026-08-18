package parts

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
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

// WithRoomForItsName puts a switch on the screen with its words clear of its
// square.
//
// Measured off the stored tree on 2026-08-18, after the focus disc stopped
// being drawn for a press and stopped filling the space: the square spans x=4
// to x=24 and the words start at x=28, so four pixels separate a 20 px box from
// the sentence beside it and they read as touching. O95.
//
// The arithmetic is the toolkit's - checkRenderer.Layout puts the words at
// iconInline + innerPadding + 2 * inputBorder and the square at innerPadding/2 +
// inputBorder - so the gap is half the inner padding minus one border. There is
// no size named for it. Raising the inner padding FOR THIS SUBTREE moves the
// two apart without touching a single box or button on the form, which is the
// knob a note in theme.go said did not exist until 2026-08-18. See
// parts/openlist.go for where that was found.
func WithRoomForItsName(t *Toggle) fyne.CanvasObject {
	return container.NewThemeOverride(t, roomierToggle{})
}

// roomierToggle is our theme with more room inside a switch and nowhere else.
type roomierToggle struct{}

func (roomierToggle) Color(n fyne.ThemeColorName, v fyne.ThemeVariant) color.Color {
	return Theme().Color(n, v)
}
func (roomierToggle) Font(s fyne.TextStyle) fyne.Resource     { return Theme().Font(s) }
func (roomierToggle) Icon(n fyne.ThemeIconName) fyne.Resource { return Theme().Icon(n) }
func (roomierToggle) Size(n fyne.ThemeSizeName) float32 {
	if n == theme.SizeNameInnerPadding {
		// Ten rather than six takes the gap from four pixels to six, which is
		// what the rows of an open list use between their mark and their words.
		// One number for "a small gap beside a glyph" across the window.
		return 10
	}
	return Theme().Size(n)
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
