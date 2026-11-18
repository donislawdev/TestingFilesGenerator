package parts

import "fyne.io/fyne/v2"

// What a grid asks of its items: every part of every item, once a pass.
//
// The toolkit asks every shown object for its smallest size before each
// frame it draws after a change, and a notch of the wheel is a change. An
// object's answer asks every object inside it again, and nothing is
// remembered between the two (fyne v2.8.2, EnsureMinSize in
// internal/driver/common/canvas.go, read 2026-11-18). So whatever one
// layout asks of a child twice, every level above it pays twice as well.
//
// The grid asked a field for its size five times a pass - for the width, to
// see whether it had a height, for its columns, for its row and for its body
// - and a group of settings three times, and a batch is a grid holding
// groups holding grids. Measured on 2026-11-18 (O356, docs/PLAN-AND-WHEEL-
// 2026-11-18.md section 2): with forty batches on the screen a notch cost the
// window 19.5 ms, past the 16.7 ms of a frame at 60 Hz.
//
// So a grid asks each part of an item once and works the item's own size
// out from those answers by the item's own rule - stacked for the column a
// field is, toggleCell.over for a box to tick - rather than asking the item
// too.

// asked is what one shown item of a grid needs.
type asked struct {
	// whole is the item's own smallest size. An item with no height takes
	// no row.
	whole fyne.Size
	// across is the width it needs in a column. A field asks with its body
	// alone: its refusal is laid under the row, across it, so a long
	// sentence or the button under it must not widen the field's column -
	// measured on the prototype of 2026-09-23, the button offering the
	// smallest size pushed the size box to two columns and the field after
	// it to the next row.
	across float32
	// tall is its height in its row: a field's body without its refusal,
	// and a box to tick with the room its row gives it (levelToggles).
	tall float32
	// refusal is the height of a field's refusal, while one is shown.
	refusal float32
	// column is a box to tick's column before that room, which is what
	// levelToggles adds it to once the packing says which row it is in.
	column fyne.Size
}

// askAll asks every shown item of a grid what it needs, in order.
func askAll(objects []fyne.CanvasObject) []placed {
	shown := make([]placed, 0, len(objects))
	for _, o := range objects {
		if o.Visible() {
			shown = append(shown, placed{o: o, asked: ask(o)})
		}
	}
	return shown
}

// ask is what one item needs.
func ask(o fyne.CanvasObject) asked {
	if cell, _ := refusalOf(o); cell != nil {
		return askField(cell.column, o.(*fyne.Container).Objects)
	}
	if t, toggle := layoutOf(o).(*toggleCell); toggle {
		col := t.stack(o.(*fyne.Container).Objects)
		whole := t.over(col)
		return asked{whole: whole, across: whole.Width, tall: whole.Height, column: col}
	}
	whole := o.MinSize()
	return asked{whole: whole, across: whole.Width, tall: whole.Height}
}

// askField is what a field with a refusal under it needs: its body asked
// once whether it is shown or not, since its width and height are what the
// grid lays out by, the refusal and anything after it asked once while
// shown, and the field's own size stacked from those the way its column
// stacks them.
func askField(c column, objects []fyne.CanvasObject) asked {
	body := objects[0].MinSize()
	var s stacked
	var refusal float32
	for i, o := range objects {
		if !o.Visible() {
			continue
		}
		size := body
		if i > 0 {
			size = o.MinSize()
		}
		if i == 1 {
			refusal = size.Height
		}
		s.add(size)
	}
	return asked{whole: s.with(c.gap), across: body.Width, tall: body.Height, refusal: refusal}
}
