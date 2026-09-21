package parts

import (
	"fyne.io/fyne/v2"
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
type PointerFocus struct {
	silent bool
	// returning is set by the window just before the toolkit hands the
	// keyboard back to the control that held it, because the WINDOW came to
	// the front again - see WindowReturning. wasMarked is whether the mark
	// was drawn when the keyboard last left, which is what a return draws.
	returning bool
	wasMarked bool
}

// Quietly moves the focus the way a press does: the control gets the keyboard
// and nothing is drawn to say so.
func (p *PointerFocus) Quietly(focus func()) {
	p.silent = true
	defer func() { p.silent = false }()
	focus()
}

// Draws reports whether the focus arriving right now is to be drawn: not
// when the pointer put it here, and - when the window is coming back to the
// front - only if it was drawn when the window went behind.
//
// The second half is the defect the owner reported on 2026-09-21 as one
// menu wearing a different colour from the others. Measured in the pinned
// toolkit: when the system gives the window the front, the driver calls
// FocusGained on whatever holds the keyboard as if the keyboard had just
// arrived (internal/driver/glfw/window.go processFocused, then
// internal/app/focus_manager.go FocusGained) - and it does so on the very
// first activation, right after the window has put the keyboard on the first
// field quietly. So the first menu on the first screen opened marked, alone
// among every control in the window, and marked again after every Alt-Tab.
// A control cannot tell that call from the keyboard moving to it, because
// both arrive as one FocusGained with the same state - only the window can,
// and it says so through WindowReturning before the call lands.
func (p *PointerFocus) Draws() bool {
	if p.returning {
		p.returning = false
		return p.wasMarked
	}
	return !p.silent
}

// Lost is what a control says as the keyboard leaves it: whether its mark
// was drawn. The window coming back draws exactly that again.
func (p *PointerFocus) Lost(marked bool) { p.wasMarked = marked }

// WindowReturning tells the control that the next FocusGained is the window
// coming back to the front, not the keyboard moving. The real window says it
// from the toolkit's foreground hook, which runs just before the driver's
// call - see Draws, and the wiring in internal/gui.
func (p *PointerFocus) WindowReturning() { p.returning = true }

// Returnable is a control that can be told the window is coming back, so
// that it does not mistake the toolkit's call for the keyboard arriving.
type Returnable interface{ WindowReturning() }

// Take is what a press does with the keyboard: it moves the focus to the
// control quietly, unless the control holds it already.
//
// The one place for it, because three controls had a copy by 2026-09-17 - the
// switch, the segmented switch and the head row of a fold - and the third
// copy is where a repeated shape becomes a part. It is also where the copies
// disagreed: the first two skipped a control that was focused already and
// the third did not, so a Space on a focused head row went through the
// focus manager's search for the object in the whole tree - Focus in
// internal/app/focus_manager.go walks the tree before its focus() returns
// on finding the same object - once per press, for nothing. The reason the
// first copy wrote down for the skip, that re-focusing would run FocusLost
// and FocusGained, was wrong: focus() returns before either. The skip is
// right for the cost of the walk.
func (p *PointerFocus) Take(control focusableObject) {
	app := fyne.CurrentApp()
	if app == nil || control == nil {
		return
	}
	canvas := app.Driver().CanvasForObject(control)
	if canvas == nil || canvas.Focused() == fyne.Focusable(control) {
		return
	}
	p.Quietly(func() { canvas.Focus(control) })
}

// focusableObject is a control on a canvas that can hold the keyboard - both
// halves, because the canvas is found through the one and asked about the
// other.
type focusableObject interface {
	fyne.CanvasObject
	fyne.Focusable
}

// FocusQuietly puts the keyboard on a control without drawing the mark that
// says it is there.
//
// For a focus the PROGRAM places - when a window opens, or when somebody moves
// to another screen. The mark means "the keyboard is here and you are using
// it", and nobody has pressed a key yet, so drawing it would be the window
// answering a question that was not asked. The first keystroke draws it.
//
// This is the opposite decision to Reveal, which draws the mark on purpose -
// there the form has MOVED the keyboard away from where somebody was working
// and saying where it went is the whole point.
//
// A control that does not know how to be quiet is focused plainly. Nothing in
// this window is in that case today, and a new one would draw its mark early
// rather than not be reachable.
func FocusQuietly(canvas fyne.Canvas, control fyne.Focusable) {
	if canvas == nil || control == nil {
		return
	}
	if quiet, ok := control.(interface{ Quietly(func()) }); ok {
		quiet.Quietly(func() { canvas.Focus(control) })
		return
	}
	canvas.Focus(control)
}
