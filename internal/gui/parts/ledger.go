package parts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
)

// Ledger is a table of three columns of words - on the About screen, what a
// piece of carried code is, under which licence, and whose it is.
//
// The first two columns are as wide as their widest entry, so a module path
// or a licence identifier stays on one line and the three line up down the
// table. The last takes what is left and wraps.
//
// Unless what is left would be less than a third of the row. Measured on the
// prototype of 2026-09-23: the one entry shipped beside the program on
// Windows has a name of 41 characters and a licence of "MIT AND Apache-2.0
// WITH LLVM-exception AND BSL-1.0", and the copyright beside them broke into
// seven lines of one or two words. Then the first two give up width in
// proportion to what they asked for, and wrap as well.
//
// Prototype of 2026-09-23. It replaced one line per entry with the three
// parts told apart by two spaces, which in the real window read as a wall of
// text in which nothing lined up.
func Ledger(rows [][3]string) fyne.CanvasObject {
	var first, second float32
	for _, r := range rows {
		first = fyne.Max(first, measured(r[0]))
		second = fyne.Max(second, measured(r[1]))
	}
	shape := &ledgerShape{first: first, second: second}
	lines := make([]fyne.CanvasObject, 0, len(rows))
	for _, r := range rows {
		lines = append(lines, container.New(ledgerRow{shape}, Prose(r[0]), Prose(r[1]), Prose(r[2])))
	}
	return Column(GapLabel, lines...)
}

// measured is how wide a column has to be to hold one entry on one line: the
// words themselves, which is all a Prose label draws round its ink, and
// GapInline on top so that rounding a fraction of a pixel cannot push the
// last word onto a second line.
func measured(s string) float32 {
	return fyne.MeasureText(s, TextBody, fyne.TextStyle{}).Width + GapInline
}

// ledgerShape is the widths the columns of one table asked for, shared by
// every row so the columns line up.
type ledgerShape struct{ first, second float32 }

// widths is how wide each column is in a row this wide.
func (l *ledgerShape) widths(width float32) [3]float32 {
	room := fyne.Max(0, width-GapColumns*2)
	first, second := l.first, l.second
	if least := room / 3; first+second > room-least {
		scale := (room - least) / (first + second)
		first, second = first*scale, second*scale
	}
	return [3]float32{first, second, fyne.Max(0, room-first-second)}
}

// ledgerRow lays one row out in the table's columns.
type ledgerRow struct{ shape *ledgerShape }

func (l ledgerRow) MinSize(objects []fyne.CanvasObject) fyne.Size {
	height := float32(0)
	for _, o := range objects {
		height = fyne.Max(height, o.MinSize().Height)
	}
	return fyne.NewSize(NumericWidth*3+GapColumns*2, height)
}

func (l ledgerRow) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) < 3 {
		return
	}
	widths := l.shape.widths(size.Width)
	x := float32(0)
	for i, o := range objects[:3] {
		o.Resize(fyne.NewSize(widths[i], o.MinSize().Height))
		o.Move(fyne.NewPos(x, 0))
		x += widths[i] + GapColumns
	}
}
