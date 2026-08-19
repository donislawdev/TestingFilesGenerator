package parts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

// revealMargin is how much of the form is kept above a control brought into
// view, so it arrives looking like part of a form rather than pinned to the top
// edge with its own label cut off above it.
const revealMargin = 24

// Reveal scrolls a control into view and puts the keyboard in it.
//
// Pressing Generate with a bad value in a box did nothing anybody could see.
// The box was already red and the sentence was already under it, both of them
// possibly several hundred pixels off the bottom of a form that does not fit
// its window - so from where the person is looking, a button they just pressed
// had no effect, and the obvious next move is to press it again (O107).
//
// The mark saying the keyboard is here IS drawn for this, and that is a
// decision rather than an oversight of PointerFocus. The mark is withheld from
// the pointer because a mouse press is not somebody using the keyboard. This is
// neither: it is the form moving the keyboard by itself, and saying where it
// went is the entire purpose.
//
// The scrolling area is handed in rather than looked for. Finding it would mean
// walking down through the widgets between a form and a field, and this package
// cannot see inside a widget - only the screen that built the scroll knows
// which one holds its form.
//
// It reports whether it found somewhere to put the keyboard, so a caller can
// tell "nothing to reveal" from "revealed".
func Reveal(scroll *container.Scroll, control fyne.CanvasObject) bool {
	if control == nil {
		return false
	}
	app := fyne.CurrentApp()
	if app == nil {
		return false
	}
	driver := app.Driver()
	if driver == nil {
		return false
	}
	canvas := driver.CanvasForObject(control)
	if canvas == nil {
		return false
	}

	if scroll != nil {
		// Where the control sits inside the scrolled content: its distance
		// from the viewport plus how far the viewport has already moved. Asked
		// of the driver rather than added up from parents, because a position
		// is relative to a parent and the chain from a form down to a field is
		// several containers deep.
		top := driver.AbsolutePositionForObject(control).Y -
			driver.AbsolutePositionForObject(scroll).Y +
			scroll.Offset.Y - revealMargin
		if top < 0 {
			top = 0
		}
		scroll.Offset = fyne.NewPos(scroll.Offset.X, top)
		scroll.Refresh()
	}

	found := inside(control, func(o fyne.CanvasObject) bool {
		_, ok := o.(fyne.Focusable)
		return ok
	})
	if found == nil {
		return false
	}
	canvas.Focus(found.(fyne.Focusable))
	return true
}

// inside is the first object at or under o that the question says yes to.
//
// A registered control is not always the widget itself: several are wrapped in
// a container that fixes their width, so asking the registered object whether
// it can hold the keyboard - or be disabled - answers about the wrapper. That
// cost two guards their first run, both silently: the wrapper is neither, so
// both operations did nothing at all and reported nothing (O106, O107).
//
// Only containers are opened. This package cannot see inside a widget, and does
// not need to: every wrapper it puts round a control is a container it built.
func inside(o fyne.CanvasObject, want func(fyne.CanvasObject) bool) fyne.CanvasObject {
	if o == nil {
		return nil
	}
	if want(o) {
		return o
	}
	box, ok := o.(*fyne.Container)
	if !ok {
		return nil
	}
	for _, child := range box.Objects {
		if found := inside(child, want); found != nil {
			return found
		}
	}
	return nil
}
