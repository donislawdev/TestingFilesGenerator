package parts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

// rowView is the rows of an open list on the screen: one ListRow for every
// position of the arrangement, against each other in a scroll.
//
// Ours since 2026-09-23. It was widget.List until then, under a theme
// override that took out the room the toolkit puts between rows - and Fyne
// 2.8.1 gives an override a new scope at construction, at CreateRenderer and
// at every Refresh, and parses the fonts again for every scope a new string is
// drawn in. Every opening of a list was a new override, and the letters a
// filter makes bold are new strings: ten openings of the format list with a
// new letter each kept 158 MB that nothing gave back, measured in the real
// window (docs/GUI-MEMORY-2026-09-23.md section 4e). Rows we lay out ourselves
// are spaced the way we say, and no override is needed.
//
// Every position gets a row, where widget.List built only the rows in sight.
// The longest list in the window is the formats under their headings, about
// thirty rows, and building them is not what an opening costs (section 4f of
// the same document). A menu of hundreds of values is where that stops being
// true, and the answer then is to build only the rows in sight.
type rowView struct {
	scroll *container.Scroll
	stack  *fyne.Container
	// built is every row made so far, kept for the next arrangement, and
	// shown how many of them the current one uses.
	built []*ListRow
	shown int
	// drawn says the list has a renderer. Rows are built only from then on,
	// as widget.List built them.
	drawn bool
}

func newRowView() *rowView {
	v := &rowView{stack: container.New(rowStack{})}
	v.scroll = container.NewVScroll(v.stack)
	return v
}

// show puts a row on the screen for each of n positions, filled by fill, and
// draws them once.
func (v *rowView) show(n int, fill func(at int, row *ListRow)) {
	if !v.drawn {
		return
	}
	for len(v.built) < n {
		v.built = append(v.built, newListRow())
	}
	objects := make([]fyne.CanvasObject, n)
	for at, row := range v.built[:n] {
		fill(at, row)
		objects[at] = row
	}
	v.stack.Objects = objects
	v.shown = n
	// The scroll first takes the new height of the rows, then refreshes them.
	v.scroll.Refresh()
}

// bringIntoView scrolls the least that shows the row at this position whole:
// up to it when it is above what is in sight, and until it is the last row in
// sight when it is below. widget.List's scrollTo, with no room between rows.
// Takes effect at the next show.
//
// Nothing happens before the list has a height, and that is a difference
// from widget.List. A list filtered before it is laid out - the catalogue
// does that - came up from widget.List 124 px down, with the row the keyboard
// was on above what was in sight (the stored catalogue tree until 2026-09-23).
// Here it comes up at its top, which is where typing puts a list on the
// screen.
func (v *rowView) bringIntoView(at int) {
	if v.scroll.Size().Height <= 0 {
		return
	}
	row := listRowHeight()
	top := float32(at) * row
	switch {
	case top < v.scroll.Offset.Y:
		v.scroll.Offset.Y = top
	case top+row > v.scroll.Offset.Y+v.scroll.Size().Height:
		v.scroll.Offset.Y = top + row - v.scroll.Size().Height
	}
}

// toTop scrolls back to the first row. Takes effect at the next show.
func (v *rowView) toTop() { v.scroll.Offset.Y = 0 }

// inSight says whether any of the row at this position is inside the room the
// list has, which is what widget.List built a row for.
func (v *rowView) inSight(at int) bool {
	row := listRowHeight()
	top := float32(at) * row
	return top+row > v.scroll.Offset.Y && top < v.scroll.Offset.Y+v.scroll.Size().Height
}

// rowStack puts the rows of an open list against each other: each one row
// tall and as wide as the list, in the order given.
//
// Against each other is the point. What separates two rows is the surface a
// row draws under the pointer or the keyboard, which is a thing somebody can
// see, rather than a gap, which is not - and no rule between them either:
// widget.List drew a hairline in its gap, and with the gap taken out it came
// out as a line every 28 px, so the list read as a ruled table rather than as
// a menu. Measured before any of this existed: thirteen formats made a list
// 476 px tall, 78 px of which was the gap between rows (OpenList).
type rowStack struct{}

// MinSize is every row, as wide as a row with no words - which is what
// widget.List measured its width off, an empty template row. The list is
// drawn at the width of the box it drops from, and the box is sized for the
// longest word (RowWidthFor).
func (rowStack) MinSize(objects []fyne.CanvasObject) fyne.Size {
	return fyne.NewSize(RowWidthFor(0, false), float32(len(objects))*listRowHeight())
}

func (rowStack) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	row := listRowHeight()
	for at, o := range objects {
		o.Move(fyne.NewPos(0, float32(at)*row))
		o.Resize(fyne.NewSize(size.Width, row))
	}
}
