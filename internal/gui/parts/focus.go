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
