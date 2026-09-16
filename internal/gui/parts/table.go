package parts

import "fyne.io/fyne/v2"

// Table is the one shape in which fields stand side by side: cells under
// names that are said once, over the first row, for the lists whose rows all
// have the same columns - the files inside an archive.
//
// Its own part rather than three more methods on Fields, and GUI rule 9 is
// the reason before the method ceiling is: a table has its own set of
// controls and its own rule about where a name goes, so it is a component from
// the start, and a screen that needs one asks for it by name. A field built
// with Fields.Add is a row of the form and stands under the one before it. A
// cell built here has no name of its own on the screen - the name is in the
// registry, where a refusal finds it, and over the column, where a person
// reads it once.
//
// Until 2026-09-16 every cell drew its own name over its control, so the
// second row of a table repeated the first row's names a control's height
// below them, and three files inside an archive read as nine names for three
// kinds of value. The comment on CellSaying had described the header-once
// shape since 2026-08-20 without anything drawing it.
type Table struct{ fields *Fields }

// Cell builds a field as a cell of a table - the control alone. Everything
// else about it is a field: the refusal, the edge, the registry, the name a
// refusal calls it by.
func (t *Table) Cell(setting, label, hint string, detail Detail, control fyne.CanvasObject) fyne.CanvasObject {
	return t.fields.register(setting, label, alsoSaying(hint, detail), control, CellSaying(control))
}

// Header is the row of names over the table, said once.
//
// It is DERIVED from a row of the table rather than declared beside it: given
// the objects of one row - the same objects Row is given - it draws, over each
// cell that is a field, the name that field was registered with, its star if
// it has to be filled in and the button to its explanation, and over anything
// else a blank of the same column. The header and the rows share one layout
// and one set of columns, so a name stands over the box it names.
//
// Derived, because two declarations of one table are how the header of a
// column and the refusal about a cell under it come to call the same thing by
// two names.
func (t *Table) Header(row ...fyne.CanvasObject) fyne.CanvasObject {
	heads := make([]fyne.CanvasObject, 0, len(row))
	for _, o := range row {
		f := t.fields.holding(o)
		if f == nil {
			heads = append(heads, Clear())
			continue
		}
		heads = append(heads, headingRow(f.Label, f.detail, t.fields.required[f.Setting]))
	}
	return Row(heads...)
}

// Row puts cells side by side and gives their refusals the whole width.
//
// A refusal in this tool has four parts - what happened, why, what is allowed,
// what to do instead - so it is a sentence and not a word. Inside a column of
// a row it gets a quarter of the form to say that in. Measured off a render on
// 2026-08-20: a size below what BMP can make wrapped onto four lines in the
// left column while the right half of the panel was empty, and the four lines
// pushed everything under them down by three.
//
// So the controls share the row and the messages do not. The message still
// belongs to its own field - it is the same area, marked and cleared with the
// same box - it is just laid out where there is room to read it.
//
// Given something that is not a field it puts it in the row whole, because a
// row of cells and one plain object - the button that removes the row - is a
// shape this screen builds.
func (t *Table) Row(objects ...fyne.CanvasObject) fyne.CanvasObject {
	bodies := make([]fyne.CanvasObject, 0, len(objects))
	areas := make([]fyne.CanvasObject, 0, len(objects))
	for _, o := range objects {
		f := t.fields.holding(o)
		if f == nil || f.body == nil {
			bodies = append(bodies, o)
			continue
		}
		bodies = append(bodies, f.body)
		areas = append(areas, f.area.Object())
	}
	return Column(GapTight, append([]fyne.CanvasObject{Row(bodies...)}, areas...)...)
}
