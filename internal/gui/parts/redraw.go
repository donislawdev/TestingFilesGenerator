package parts

import "fyne.io/fyne/v2"

// redraw tells the canvas about the pieces of a face that changed. A renderer
// calls it at the end of Refresh, naming what it draws.
//
// It exists because of what a renderer must NOT do instead, and that was
// measured rather than read: canvas.Refresh(r.button). The driver files every
// object it paints under the value the tree holds, and a value is a type and a
// pointer together - so a type that embeds Button hands the tree ITSELF, the
// driver files it as that outer type, and a refresh asked for the inner Button
// finds no canvas at all. Nothing is marked dirty, nothing is painted, and no
// error says so. On 2026-09-16 that was the explanation button: MouseIn ran,
// the explanation was put on its sheet, and the window stayed as it was until
// something else happened to repaint it (O217).
//
// The pieces have no such double identity. A rectangle in the tree is that
// rectangle, so asking for each piece reaches the canvas whoever holds the
// renderer. It is also what the toolkit's own renderers do, with one addition
// they can make and this package cannot: they refresh the widget through a
// private accessor that knows the outer type.
//
// The test driver cannot see any of this. It answers CanvasForObject with the
// last window's canvas whatever the object, and its canvas ignores Refresh, so
// a refresh sent to the wrong value is indistinguishable there from one sent to
// the right one. That is why the rule is held by reading the renderers rather
// than by driving them: TestARendererRedrawsThePiecesItDrawsAndNeverTheWidget.
func redraw(pieces ...fyne.CanvasObject) {
	for _, piece := range pieces {
		piece.Refresh()
	}
}
