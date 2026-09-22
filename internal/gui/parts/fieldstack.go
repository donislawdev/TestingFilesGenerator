package parts

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
)

// A field is its name over its control: the name on the left edge, the control
// under it starting on the same edge, and under the control whatever the field
// has to say about its value - the count of bytes a size comes to.
//
// Over rather than beside since 2026-09-21, on the owner's decision from the
// running window ("that is prettier"). It was beside from 2026-09-14, in a
// column of names as wide as the widest name the window could show, and over
// before that - so this is the second time round, and what is different this
// time is written down: the column and everything that measured it (the
// widest name, the list of every name a screen can draw, the guard holding
// the list complete) went with the layout, because a width nothing draws is
// a number waiting to be wrong.
//
// A layout of its own rather than a plain column, so that a guard reading the
// names off a screen can tell a field from any other stack of words and boxes
// - IsField, the way IsFieldRow was asked before.

// fieldStack is the layout behind a field: a column at the name's distance.
type fieldStack struct{ column }

// FieldStack stacks a field's pieces: the name, the control, then whatever the
// field says under its control.
func FieldStack(pieces ...fyne.CanvasObject) fyne.CanvasObject {
	return container.New(fieldStack{column{gap: GapLabel}}, pieces...)
}

// IsField says whether a container is one field of the form, for a guard
// reading the names off a screen. Asked by the layout rather than by shape,
// because a section is also a container whose first thing is words.
func IsField(c *fyne.Container) bool {
	_, is := c.Layout.(fieldStack)
	return is
}

// Clear is an empty cell: what a table draws over a column whose control has
// no name. Transparent rather than the colour of the panel, because a
// rectangle in the colour of the thing under it is right at the call site and
// wrong on the first screen that puts the row on something else.
func Clear() fyne.CanvasObject {
	return canvas.NewRectangle(color.Transparent)
}
