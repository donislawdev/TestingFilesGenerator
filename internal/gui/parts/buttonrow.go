package parts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

// ButtonRow is a row of buttons side by side, GapButtons apart and centred in
// the room it is given: the ones that run something in the bar at the foot of
// a screen, and the ones in the head of a batch.
//
// A layout of ours rather than a box, which is what both were until the
// prototype of 2026-09-23: a box puts the toolkit's padding between its
// children, about 4 px, and Preview and Generate - and Duplicate and Remove -
// read as one control. What is hidden takes no room and no gap, so Cancel and
// the two offers after a run come and go without leaving a hole.
func ButtonRow(items ...fyne.CanvasObject) *fyne.Container {
	return container.New(&buttonRow{lead: len(items)}, items...)
}

// ActionRow is the ButtonRow of the bar at the foot of a work screen: the
// buttons that run something, then the ones a finished run offers. The offers
// stand beside the others while there is room for them between the rails, and
// on a line of their own under the others when there is not.
//
// Measured on 2026-11-26 with guirender after the review of #264
// (docs/REVIEW-264-2026-11-25.md): one line was all the row had, and once Copy
// command stood at the right end, the offers of a finished run ran under it -
// Open manifest of the batch screen entirely, in English. The bar is never
// wider than the readable column, so no window was wide enough to undo it, and
// a translation or a third offer brings it back at any width. A second line is
// the answer that holds for all three.
func ActionRow(run []fyne.CanvasObject, offers ...fyne.CanvasObject) *fyne.Container {
	items := append(append([]fyne.CanvasObject{}, run...), offers...)
	return container.New(&buttonRow{lead: len(run), wraps: true}, items...)
}

// HoldRows keeps an ActionRow at the height it has now, or lets it go. A run
// holds it from the press to its last word, so the offers of the run before,
// taken away when the next one starts, do not take their line with them and
// move the form under the pointer that has just pressed Generate - the
// movement TestTheFormDoesNotMoveWhenARunStarts was written against. Any other
// row is left as it is.
func HoldRows(row *fyne.Container, hold bool) {
	b, ok := row.Layout.(*buttonRow)
	if !ok {
		return
	}
	b.held = 0
	if hold {
		b.held = row.MinSize().Height
	}
}

// buttonRow is the layout behind ButtonRow and ActionRow. clearLeft and
// clearRight are room at either end of the first line the row keeps free of
// buttons - the rails laid over the action bar, see railOver - and nought
// everywhere else. Fields of the layout rather than a move made from outside,
// because the row lays itself out again whenever a button in it comes or goes,
// and a move made once would be undone by the next of those.
type buttonRow struct {
	clearLeft, clearRight float32
	// lead is how many of the buttons run something. wraps says whether the
	// ones after them may go to a line of their own - ActionRow.
	lead  int
	wraps bool
	// width is the width the row was last laid out at. How many lines it
	// takes depends on it, which a toolkit minimum cannot say directly -
	// remembered for the reason formGrid remembers its own.
	width float32
	// held is a height the row keeps while a run owns the screen - HoldRows.
	held float32
}

// MinSize is as wide as the buttons that run something, or the widest offer if
// that is wider - the offers can always go under them, one to a line at the
// least - and as tall as the lines the row takes at the width it was last
// given, or as it is held.
func (b *buttonRow) MinSize(objects []fyne.CanvasObject) fyne.Size {
	width := lineWidth(shown(objects))
	if b.wraps {
		lead := min(b.lead, len(objects))
		width = lineWidth(shown(objects[:lead]))
		for _, o := range shown(objects[lead:]) {
			width = fyne.Max(width, o.MinSize().Width)
		}
	}
	size := fyne.NewSize(width, 0)
	tallest := lineHeight(shown(objects))
	for i, line := range b.lines(objects, b.width) {
		if i > 0 {
			size.Height += GapButtonRows
		}
		size.Height += heightOf(line, tallest)
	}
	size.Height = fyne.Max(size.Height, b.held)
	return size
}

func (b *buttonRow) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	b.width = size.Width
	lines := b.lines(objects, size.Width)
	tallest := lineHeight(shown(objects))
	var top float32
	for i, line := range lines {
		width, height := lineWidth(line), heightOf(line, tallest)
		// The first line is centred, and moved off centre only as far as the
		// room kept clear asks - so in a wide window nothing changes, and a
		// narrow one does not have to be wide enough to centre the row clear
		// of both rails. The left rail wins when both ask, and the bar is
		// never narrower than both need (railOver). The rails are one line
		// tall, so the lines under them are centred in the whole width.
		x := fyne.Max((size.Width-width)/2, 0)
		if i == 0 {
			x = fyne.Min((size.Width-width)/2, size.Width-b.clearRight-width)
			x = fyne.Max(x, b.clearLeft)
		}
		// One line stands in the middle of the height it is given, which is
		// the head of a batch taller than its buttons. Lines under one another,
		// or a row held taller than its one line, start at the top, so the
		// buttons that run something stay where the pointer left them.
		if len(lines) == 1 && b.held == 0 {
			top = (size.Height - height) / 2
		}
		for _, o := range line {
			min := o.MinSize()
			o.Resize(min)
			o.Move(fyne.NewPos(x, top+(height-min.Height)/2))
			x += min.Width + GapButtons
		}
		top += height + GapButtonRows
	}
}

// lines is how the shown buttons stand at a width: all on one line while they
// fit between the rails, and otherwise the buttons that run something on the
// first line and the offers under them, as many to a line as the whole width
// takes. When not even the buttons that run something fit between the rails -
// longer words in a column that cannot grow, which the pseudo language shows on
// the batch screen - the first line is left to the rails and every button goes
// under them. A row that does not wrap, or a width not known yet, is one line.
func (b *buttonRow) lines(objects []fyne.CanvasObject, width float32) [][]fyne.CanvasObject {
	lead := min(b.lead, len(objects))
	run, offers := shown(objects[:lead]), shown(objects[lead:])
	all := append(append([]fyne.CanvasObject{}, run...), offers...)
	between := width - b.clearLeft - b.clearRight
	if !b.wraps || width <= 0 || lineWidth(all) <= between {
		return [][]fyne.CanvasObject{all}
	}
	lines := [][]fyne.CanvasObject{run}
	under := offers
	if lineWidth(run) > between {
		lines, under = [][]fyne.CanvasObject{nil}, all
	}
	var line []fyne.CanvasObject
	var taken float32
	for _, o := range under {
		w := o.MinSize().Width
		if len(line) > 0 && taken+GapButtons+w > width {
			lines = append(lines, line)
			line, taken = nil, 0
		}
		if len(line) > 0 {
			taken += GapButtons
		}
		line = append(line, o)
		taken += w
	}
	if len(line) > 0 {
		lines = append(lines, line)
	}
	return lines
}

// heightOf is the height of a line: its tallest button, or for the first line
// left to the rails, the height of the bar's buttons, which is the rails'.
func heightOf(line []fyne.CanvasObject, tallest float32) float32 {
	if len(line) == 0 {
		return tallest
	}
	return lineHeight(line)
}

// shown is the buttons that are not hidden, in their order.
func shown(objects []fyne.CanvasObject) []fyne.CanvasObject {
	var out []fyne.CanvasObject
	for _, o := range objects {
		if o.Visible() {
			out = append(out, o)
		}
	}
	return out
}

// lineWidth is how wide buttons stand side by side, GapButtons apart.
func lineWidth(line []fyne.CanvasObject) float32 {
	var width float32
	for i, o := range line {
		if i > 0 {
			width += GapButtons
		}
		width += o.MinSize().Width
	}
	return width
}

// lineHeight is the height of the tallest button on a line.
func lineHeight(line []fyne.CanvasObject) float32 {
	var height float32
	for _, o := range line {
		height = fyne.Max(height, o.MinSize().Height)
	}
	return height
}
