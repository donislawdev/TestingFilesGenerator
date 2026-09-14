package parts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"image/color"
)

// A field is one row: its name in a column of names, its control in a column
// of controls, so every name lines up with every other name and every value
// starts on one edge. GUI rule 13 of CLAUDE.md, in the owner's words: a form
// is a grid with a column of labels, not a centred stack of controls.
//
// Until 2026-09-14 a field was the name ABOVE the control, which cost two
// rows a field and put the name of a box 32 px above the box because the
// button beside the name was 32 px tall. In the row the name and the control
// share a line, so the height of a field is the height of its control.

// fieldRow lays a field out as cells: the first is the name and gets the
// column of names, the last gets whatever width is left, and anything between
// stands at its own width. GapColumns separates them.
//
// The last cell takes the rest rather than every cell sharing it out, so a
// box holding a path fills the row and a box holding a number, which sizes
// itself, does not - the row does not decide how wide a control is, the
// control does (see Numeric and Sized).
//
// Every cell keeps its own height and is centred on the row's, so a name
// stands level with the middle of its box rather than with the top of it, and
// the line drawn round a refused box goes round the box and not round the
// row - the stack holding a control and its mark is as tall as the control.
type fieldRow struct{ names float32 }

// FieldRow puts objects into one row of the form: the name cell first, the
// control next, anything else after. For the screens that need a row the
// field builders do not make - a switch that belongs to three boxes, a
// button that belongs to no field.
func FieldRow(names float32, cells ...fyne.CanvasObject) fyne.CanvasObject {
	return container.New(fieldRow{names: names}, cells...)
}

func (f fieldRow) MinSize(objects []fyne.CanvasObject) fyne.Size {
	size := fyne.NewSize(0, 0)
	shown := 0
	for i, o := range objects {
		if !o.Visible() {
			continue
		}
		min := o.MinSize()
		if i == 0 {
			size.Width += fyne.Max(min.Width, f.names)
		} else {
			size.Width += min.Width
		}
		size.Height = fyne.Max(size.Height, min.Height)
		shown++
	}
	if shown > 1 {
		size.Width += GapColumns * float32(shown-1)
	}
	return size
}

func (f fieldRow) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	last := -1
	for i, o := range objects {
		if o.Visible() {
			last = i
		}
	}
	x := float32(0)
	for i, o := range objects {
		if !o.Visible() {
			continue
		}
		width := o.MinSize().Width
		switch {
		case i == 0:
			width = fyne.Max(width, f.names)
		case i == last:
			width = fyne.Max(width, size.Width-x)
		}
		height := fyne.Min(o.MinSize().Height, size.Height)
		o.Resize(fyne.NewSize(width, height))
		o.Move(fyne.NewPos(x, (size.Height-height)/2))
		x += width + GapColumns
	}
}

// Clear is a shape that takes room and draws nothing.
//
// For the name cell of a row whose control carries its own name - a switch
// says what it is on the part somebody clicks - so the control still stands
// in the column of controls. Transparent rather than the colour of the panel,
// because a rectangle in the colour of the thing under it is right at the call
// site and wrong on the first screen that puts the row on something else.
func Clear() fyne.CanvasObject {
	return canvas.NewRectangle(color.Transparent)
}

// WidestName is how wide a column of names has to be to hold every name given,
// with the star and the button that can stand beside one.
//
// Measured from the same objects a heading is drawn with, so the answer and
// the drawing cannot come apart - a width worked out from a copy of the
// arithmetic would be a second copy of the drawing (the sibling project paid
// four pixels for exactly that on 2026-09-09).
func WidestName(names ...string) float32 {
	widest := float32(0)
	for _, name := range names {
		if w := Heading(name).MinSize().Width; w > widest {
			widest = w
		}
	}
	return widest + GapInline + newRequiredMark().MinSize().Width + GapLabel + GlyphButton
}

// IsFieldRow says whether a container is one row of the form, for a guard
// reading the names off a screen. Asked by the layout rather than by shape,
// because a section is also a container whose first thing is words and whose
// second is not.
func IsFieldRow(c *fyne.Container) bool {
	_, is := c.Layout.(fieldRow)
	return is
}
