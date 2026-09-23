package parts

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// ToggleSaying is a switch drawn as a box to tick with its name beside it, on
// one line: the square, the name, and the button behind which the longer
// explanation waits.
//
// Beside rather than over, which is the one exception to a name standing over
// its control, and it is the prototype of 2026-09-23 on the owner's choice
// from the running window. A square alone on a line under its name read as an
// empty box rather than as something to tick. The name is part of the target:
// pressing it ticks the box, the way every box to tick on a desktop answers.
//
// A cell of a Grid like any field. Beside a field with a name over its box,
// the square stands level with that box rather than with the name - see
// toggleCell.
func ToggleSaying(label string, detail Detail, check *Toggle) Built {
	line := []fyne.CanvasObject{check, container.NewStack(Heading(label), newNameTap(check))}
	if detail.Text != "" && detail.on != nil {
		line = append(line, newDetailButton(detail))
	}
	area := NewErrorArea()
	body := FieldStack(container.New(headingLine{}, line...))
	return Built{Object: container.New(&toggleCell{}, body, area.Object()), Body: body, Area: area}
}

// toggleCell is the layout of a box to tick in a grid: a column like any
// field's, pushed down by the height of a name line when the grid says the
// row it stands in has a named field in it. A row of boxes to tick alone,
// or a box alone under a sentence, keeps no such room - measured on the
// prototype, the room alone read as a hole.
//
// The height of a name line is the token headingLine.MinSize answers with,
// not a measurement.
type toggleCell struct{ level bool }

func (t *toggleCell) drop() float32 {
	if t.level {
		return GlyphButton + GapLabel
	}
	return 0
}

func (t *toggleCell) MinSize(objects []fyne.CanvasObject) fyne.Size {
	size := column{gap: GapTight}.MinSize(objects)
	size.Height += t.drop()
	return size
}

func (t *toggleCell) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	y := t.drop()
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		height := o.MinSize().Height
		o.Resize(fyne.NewSize(size.Width, height))
		o.Move(fyne.NewPos(0, y))
		y += height + GapTight
	}
}

// levelToggles tells every box to tick in a packing whether it shares its
// row with a named field.
func levelToggles(items []placed) {
	named := map[int]bool{}
	for _, p := range items {
		if _, field := layoutOf(p.o).(*fieldCell); field {
			named[p.row] = true
		}
	}
	for _, p := range items {
		if t, toggle := layoutOf(p.o).(*toggleCell); toggle {
			t.level = named[p.row]
		}
	}
}

// layoutOf is the layout of a container, or nil for anything else - so a
// question about what kind of cell something is takes one step, not two.
func layoutOf(o fyne.CanvasObject) fyne.Layout {
	if c, ok := o.(*fyne.Container); ok {
		return c.Layout
	}
	return nil
}

// NameTap lies over the name beside a box to tick and ticks the box when
// the name is pressed. It draws nothing: the name stays plain words, the same
// kind every other name on the form is, so whatever reads the names off a
// screen reads this one too. Exported so that a guard walking the tree can
// tell it from a control: the name and this over it are a stack of words
// followed by something, which is the shape of a field.
type NameTap struct {
	widget.BaseWidget
	check *Toggle
}

func newNameTap(check *Toggle) *NameTap {
	t := &NameTap{check: check}
	t.ExtendBaseWidget(t)
	return t
}

// Tapped ticks the box the name belongs to.
func (t *NameTap) Tapped(*fyne.PointEvent) { t.check.Tapped(nil) }

func (t *NameTap) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.Transparent))
}
