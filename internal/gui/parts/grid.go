package parts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

// Grid lays the fields of a section in GridColumns columns, in the order
// given, each field taking as many columns as its value needs.
//
// Why, measured on 2026-09-23 in the real window at the size the owner's
// window opens at: the controls of the single batch screen used 37 per cent
// of their section's width, in three widths (140, 296 and 788 px), and the
// screen was taller than its window. A first prototype put two fields to a
// row, each filling its half - and the owner's verdict from the running
// window was the other half of the same defect: a box holding "1" or a list
// of "avif" drawn 386 px wide. So a field now takes the fewest columns that
// hold it and fills exactly those (see fixedWidth), and the right edge of a
// row is still one line.
//
// Anything handed over through Wide takes the whole row, and so does anything
// that is not a field: a sentence, a list, a fold, a path.
//
// A thing with no height takes no row. The boxes a screen refills at run time
// - the settings a format declares, the parameters of a damage - are empty
// until something is chosen, and an empty row would still cost a gap.
//
// A narrower grid gives a field more columns rather than cutting it, down to
// one field a row, so the window can be as narrow as its widest field.
func Grid(items ...fyne.CanvasObject) *fyne.Container {
	return container.New(&formGrid{}, items...)
}

// Wide marks something that takes the whole row of a Grid.
func Wide(o fyne.CanvasObject) fyne.CanvasObject {
	return container.New(wideCell{}, o)
}

// IsWide says whether a grid item was marked to take the whole row.
func IsWide(o fyne.CanvasObject) bool {
	c, ok := o.(*fyne.Container)
	if !ok {
		return false
	}
	_, wide := c.Layout.(wideCell)
	return wide
}

// wideCell is the mark Wide leaves. It lays its one child out whole, so it
// is invisible anywhere but in a grid.
type wideCell struct{}

func (wideCell) MinSize(objects []fyne.CanvasObject) fyne.Size {
	size := fyne.NewSize(0, 0)
	for _, o := range objects {
		if o.Visible() {
			size = size.Max(o.MinSize())
		}
	}
	return size
}

func (wideCell) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objects {
		o.Resize(size)
		o.Move(fyne.NewPos(0, 0))
	}
}

// fieldCell is the layout of one whole field - its body and the room under it
// for a refusal - and the mark a Grid reads to put it in a column. Anywhere
// else it is a column like any other.
//
// In a grid the refusal is not drawn in the column. Measured on the prototype
// of 2026-09-23: "Size "abc" has no number: write something like 10mb or
// 1048576" broke into three lines in a column 185 px wide and pushed the whole
// form down. So the grid tells the cell where its refusal goes - under the
// row, across the whole of it, and under any refusal about a field before it
// in the same row - and the cell draws it there. The red edge stays on the box
// it is about, and every refusal names its field.
//
// From the row's left edge since 2026-09-24. Until then a refusal started at
// its own field's left edge, so two refusals in one row stood as a staircase -
// the second one column in and a line down - which the review of that day
// found reading as an accident (UI-011). Wrapping each in its own column was
// the other way out, and it is the three lines above.
type fieldCell struct {
	column
	inGrid    bool
	areaLeft  float32
	areaTop   float32
	areaWidth float32
}

func (f *fieldCell) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if !f.inGrid || len(objects) < 2 {
		f.column.Layout(objects, size)
		return
	}
	body, area := objects[0], objects[1]
	body.Resize(fyne.NewSize(size.Width, body.MinSize().Height))
	body.Move(fyne.NewPos(0, 0))
	area.Resize(fyne.NewSize(f.areaWidth, area.MinSize().Height))
	area.Move(fyne.NewPos(f.areaLeft, f.areaTop))
}

// cellOf stacks a field's pieces - its body, then its refusal - as one grid
// cell.
func cellOf(pieces ...fyne.CanvasObject) fyne.CanvasObject {
	return container.New(&fieldCell{column: column{gap: GapTight}}, pieces...)
}

// refusalOf is the grid cell and the refusal area of a field, or nil.
func refusalOf(o fyne.CanvasObject) (*fieldCell, fyne.CanvasObject) {
	c, ok := o.(*fyne.Container)
	if !ok || len(c.Objects) < 2 {
		return nil, nil
	}
	cell, is := c.Layout.(*fieldCell)
	if !is {
		return nil, nil
	}
	return cell, c.Objects[1]
}

// hanging lays a list item out: the marker level with the first line of the
// words, and the words hanging beside it so a second line starts under the
// first rather than under the marker. Two objects, marker first.
//
// The first line's height is asked of the same kind of label the item is
// drawn with, rather than written down, so it follows the font (GUI rule 14).
type hanging struct{}

func (hanging) firstLine() float32 { return Prose("Ag").MinSize().Height }

func (h hanging) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) < 2 {
		return fyne.NewSize(0, 0)
	}
	marker, item := objects[0].MinSize(), objects[1].MinSize()
	return fyne.NewSize(marker.Width+GapLabel+item.Width, fyne.Max(marker.Height, item.Height))
}

func (h hanging) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 2 {
		return
	}
	marker := objects[0].MinSize()
	objects[0].Resize(marker)
	objects[0].Move(fyne.NewPos(0, (h.firstLine()-marker.Height)/2))
	indent := marker.Width + GapLabel
	objects[1].Resize(fyne.NewSize(size.Width-indent, size.Height))
	objects[1].Move(fyne.NewPos(indent, 0))
}

// isGridCell is whether something goes in a column rather than across the
// row: a field, and nothing else. A sentence or a fold handed over without
// Wide still takes the row, because half a sentence is never what was meant.
func isGridCell(o fyne.CanvasObject) bool {
	c, ok := o.(*fyne.Container)
	if !ok || len(c.Objects) == 0 {
		return false
	}
	switch c.Layout.(type) {
	case *fieldCell, *toggleCell:
		return true
	}
	return false
}

// formGrid is the layout behind Grid. A pointer, because it remembers the
// width it was last laid out at - see MinSize.
type formGrid struct{ width float32 }

// placed is one item of a grid: where it starts, how many columns it takes
// and which row it is in.
type placed struct {
	o           fyne.CanvasObject
	row, column int
	span        int
}

// columnWidth is the width of one of GridColumns columns in a grid this wide.
func columnWidth(width float32) float32 {
	return (width - GapColumns*float32(GridColumns-1)) / GridColumns
}

// spanOf is how many columns an item takes at a given column width: the
// fewest that hold what it needs, and all of them for anything that is not a
// field or was marked Wide. Worked out from the item, never written down per
// field - a translation that makes a name longer makes its field wider.
func spanOf(o fyne.CanvasObject, column float32) int {
	if IsWide(o) || !isGridCell(o) {
		return GridColumns
	}
	need := cellWidthNeed(o)
	for span := 1; span < GridColumns; span++ {
		if column*float32(span)+GapColumns*float32(span-1) >= need {
			return span
		}
	}
	return GridColumns
}

// pack lays the shown items into rows at one width, in the order given: an
// item that does not fit in what is left of a row starts the next one.
func pack(objects []fyne.CanvasObject, width float32) []placed {
	column := columnWidth(width)
	var out []placed
	row, used := 0, 0
	for _, o := range objects {
		if !o.Visible() || o.MinSize().Height == 0 {
			continue
		}
		span := spanOf(o, column)
		if used > 0 && used+span > GridColumns {
			row, used = row+1, 0
		}
		out = append(out, placed{o: o, row: row, column: used, span: span})
		used += span
		if used >= GridColumns {
			row, used = row+1, 0
		}
	}
	// Before any height is asked: a box to tick is taller beside a named
	// field than alone, and only the packing knows which it is.
	levelToggles(out)
	return out
}

// rowHeights is how tall each row of a packing is at one width: its tallest
// item without refusals, then every refusal about a field in the row, one
// under another across the row. It tells each field cell where its refusal
// goes, which is why it takes the width.
func rowHeights(items []placed, width float32) []float32 {
	var heights []float32
	for _, p := range items {
		for len(heights) <= p.row {
			heights = append(heights, 0)
		}
		body := p.o.MinSize().Height
		if cell, _ := refusalOf(p.o); cell != nil {
			body = p.o.(*fyne.Container).Objects[0].MinSize().Height
		}
		heights[p.row] = fyne.Max(heights[p.row], body)
	}
	column := columnWidth(width)
	under := make([]float32, len(heights))
	for _, p := range items {
		cell, area := refusalOf(p.o)
		if cell == nil {
			continue
		}
		x := float32(p.column) * (column + GapColumns)
		cell.inGrid = true
		// From the row's left edge, which is x to the left of the cell.
		cell.areaLeft = -x
		cell.areaWidth = width
		cell.areaTop = heights[p.row] + GapTight + under[p.row]
		if area.Visible() {
			under[p.row] += area.MinSize().Height + GapTight
		}
	}
	for row := range heights {
		heights[row] += under[row]
	}
	return heights
}

// MinSize is as wide as the widest single item, because a narrower grid gives
// an item more columns rather than cutting it, and as tall as the packing at
// the width this grid was last given. Before its first layout that is the
// width of the form's column, which is what almost every grid is given.
//
// The height depends on the width, which a toolkit minimum cannot say
// directly - the same thing a wrapping label does, and answered the same way:
// by remembering the last width.
func (g *formGrid) MinSize(objects []fyne.CanvasObject) fyne.Size {
	width := g.width
	if width <= 0 {
		width = ColumnWidth - Inset*2
	}
	var minWidth, height float32
	for _, o := range objects {
		if o.Visible() {
			minWidth = fyne.Max(minWidth, cellWidthNeed(o))
		}
	}
	items := pack(objects, width)
	gaps := rowGaps(items)
	for i, h := range rowHeights(items, width) {
		height += gaps[i] + h
	}
	return fyne.NewSize(minWidth, height)
}

func (g *formGrid) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	g.width = size.Width
	column := columnWidth(size.Width)
	items := pack(objects, size.Width)
	heights := rowHeights(items, size.Width)
	gaps := rowGaps(items)
	tops := make([]float32, len(heights))
	for i := 1; i < len(heights); i++ {
		tops[i] = tops[i-1] + heights[i-1] + gaps[i]
	}
	for _, p := range items {
		width := column*float32(p.span) + GapColumns*float32(p.span-1)
		height := p.o.MinSize().Height
		if cell, _ := refusalOf(p.o); cell != nil {
			height = heights[p.row]
		}
		p.o.Resize(fyne.NewSize(width, height))
		p.o.Move(fyne.NewPos(float32(p.column)*(column+GapColumns), tops[p.row]))
	}
}

// rowGaps is the room above each row of a packing: none above the first,
// GapSection next to a group of settings and GapField between rows of fields.
func rowGaps(items []placed) []float32 {
	rows := 0
	grouped := map[int]bool{}
	for _, p := range items {
		rows = max(rows, p.row+1)
		if holdsGroup(p.o) {
			grouped[p.row] = true
		}
	}
	gaps := make([]float32, rows)
	for i := 1; i < rows; i++ {
		gaps[i] = GapField
		if grouped[i] || grouped[i-1] {
			gaps[i] = GapSection
		}
	}
	return gaps
}

// holdsGroup is whether an item is a group of settings, or a box a screen
// refills that holds one.
func holdsGroup(o fyne.CanvasObject) bool {
	if isGroup(o) {
		return true
	}
	c, ok := o.(*fyne.Container)
	if !ok {
		return false
	}
	for _, child := range c.Objects {
		if child.Visible() && isGroup(child) {
			return true
		}
	}
	return false
}

// isGroup is whether something is a group of settings itself.
func isGroup(o fyne.CanvasObject) bool {
	_, group := layoutOf(o).(groupCell)
	return group
}

// cellWidthNeed is how wide an item asks to be in a grid. A field asks with
// its body alone: its refusal is laid under the row, across it, so a long
// sentence or the button under it must not widen the field's column -
// measured on the prototype, the button offering the smallest size pushed the
// size box to two columns and the field after it to the next row.
func cellWidthNeed(o fyne.CanvasObject) float32 {
	if cell, _ := refusalOf(o); cell != nil {
		return o.(*fyne.Container).Objects[0].MinSize().Width
	}
	return o.MinSize().Width
}
