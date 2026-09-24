package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/catalogue"
)

// TestNoWordsInTheCatalogueRunPastTheEdgeOfItsSections lays the catalogue out
// at the width its stored picture is taken at and reads where every line of
// words it draws ends.
//
// A caption in a row beside something else is handed no width of its own, so
// it has nothing to wrap to and draws its whole line from where it starts.
// The palette's sentence for menuBackground grew on #136 and ran 82.9 px past
// the edge of its section, onto the page - in the stored picture, where
// nobody looked, and found by an outside review (docs/REVIEW-136-2026-09-24.md).
// No guard read where the words of the catalogue end, only whether they were
// there.
//
// The edge is the column the sections stand in, so a line that runs past a
// narrower box inside a section and stops short of the section's own edge is
// not seen here. Words inside something that scrolls are left out: they are
// cut to its bounds, and a long value in a box is a state the catalogue
// shows on purpose.
func TestNoWordsInTheCatalogueRunPastTheEdgeOfItsSections(t *testing.T) {
	ourTheme(t)
	page := catalogue.Page()
	w := test.NewTempWindow(t, page)
	// Twice at each size for the reason renderScene gives: a wrapping line
	// knows its width only on the second pass.
	for _, size := range []fyne.Size{
		fyne.NewSize(referenceWidth, referenceHeight),
		fyne.NewSize(referenceWidth, page.MinSize().Height),
	} {
		w.Resize(size)
		w.Resize(size)
	}

	sections, ok := page.(*fyne.Container)
	if !ok || len(sections.Objects) != 1 {
		t.Fatalf("the catalogue page is a %T, not the one column parts.Screen stands its sections in", page)
	}
	column := sections.Objects[0]
	if column.Size().Width >= page.Size().Width {
		t.Fatalf("the column of sections is %.1f px wide on a page of %.1f, so its edge is the page's and "+
			"a line running past a section would not be seen", column.Size().Width, page.Size().Width)
	}
	// Counted from the same place the walk below counts from: the page's own
	// position in the window, which the walk adds first.
	edge := page.Position().X + column.Position().X + column.Size().Width

	lines := 0
	wordsEndingAt(page, 0, func(words *canvas.Text, ends float32) {
		lines++
		if ends > edge+0.5 {
			t.Errorf("%q ends at x=%.1f, %.1f px past the edge of the sections at x=%.1f",
				words.Text, ends, ends-edge, edge)
		}
	})
	// The catalogue draws every part in every state with a caption over each,
	// so a count this low is a walk that did not reach the words.
	if lines < 100 {
		t.Fatalf("only %d line(s) of words were found in the catalogue, so it was not read", lines)
	}
}

// wordsEndingAt walks what is drawn under o, which stands at x, and hands
// every visible line of words to visit with where on the page it ends. It
// does not go into anything that scrolls, since that cuts what it holds.
func wordsEndingAt(o fyne.CanvasObject, x float32, visit func(*canvas.Text, float32)) {
	if o == nil || !o.Visible() {
		return
	}
	x += o.Position().X
	switch v := o.(type) {
	case *canvas.Text:
		if v.Text != "" {
			visit(v, x+v.MinSize().Width)
		}
	case *container.Scroll:
	case *fyne.Container:
		for _, child := range v.Objects {
			wordsEndingAt(child, x, visit)
		}
	case fyne.Widget:
		for _, child := range test.WidgetRenderer(v).Objects() {
			wordsEndingAt(child, x, visit)
		}
	}
}
