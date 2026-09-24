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
	return container.New(&buttonRow{}, items...)
}

// buttonRow is the layout behind ButtonRow. clearLeft is room at its left the
// row keeps free of buttons - the rail laid over the action bar, see railOver -
// and nought everywhere else. A field of the layout rather than a move made
// from outside, because the row lays itself out again whenever a button in it
// comes or goes, and a move made once would be undone by the next of those.
type buttonRow struct{ clearLeft float32 }

func (*buttonRow) MinSize(objects []fyne.CanvasObject) fyne.Size {
	size := fyne.NewSize(0, 0)
	shown := 0
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		min := o.MinSize()
		size.Width += min.Width
		size.Height = fyne.Max(size.Height, min.Height)
		shown++
	}
	if shown > 1 {
		size.Width += GapButtons * float32(shown-1)
	}
	return size
}

func (b *buttonRow) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	// Centred, and moved right of centre only as far as the room kept clear
	// asks - so in a wide window nothing changes, and a narrow one does not
	// have to be wide enough to centre the row clear of the rail.
	x := fyne.Max((size.Width-b.MinSize(objects).Width)/2, b.clearLeft)
	for _, o := range objects {
		if !o.Visible() {
			continue
		}
		min := o.MinSize()
		o.Resize(min)
		o.Move(fyne.NewPos(x, (size.Height-min.Height)/2))
		x += min.Width + GapButtons
	}
}
