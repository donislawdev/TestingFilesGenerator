package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// What this defends. A table on a screen names its columns ONCE, over the
// first row, and every cell under a name is the control alone.
//
// Why it needed a guard. Until 2026-09-16 the table of files inside an
// archive drew each cell as a name over a control, so the second row of the
// table repeated the first row's names a control's height below them - three
// files read as nine names for three kinds of value. The rework of the window
// (docs/GUI-REWORK-2026-09-14.md) put it on the list of what was left, and the
// comment on parts.CellSaying had described the header-once shape since
// 2026-08-20 without anything drawing it - a written intent that no guard
// compared against the screen.
//
// How it is asked. On the batch screen with TWO rows of contents, because one
// row cannot tell a header from a name over a cell: with one row the picture
// is the same either way. For every column of the table, the name has to
// appear exactly once between the table's heading and its first row, in the
// column's own x, and no cell may carry the name inside itself.
func TestATableNamesItsColumnsOnceOverTheFirstRow(t *testing.T) {
	ourTheme(t)
	screen, body, w := twoRowsOfContents(t)
	fields := screen.Fields()

	heading := textAt(t, body, text.ContentsHeading())
	firstRow := controlOf(t, fields, body, recipe.ContentAddress(1, 1, recipe.KeyFormat))

	for _, column := range []struct{ key, name string }{
		{recipe.KeyFormat, text.FieldFormat()},
		{recipe.KeyCount, text.FieldCount()},
		{recipe.KeySize, text.FieldSize()},
	} {
		control := controlOf(t, fields, body, recipe.ContentAddress(1, 1, column.key))
		// The name, once, over the table: below its heading, above the first
		// row, starting where the column starts. Counted rather than found,
		// because "found" is satisfied by a table that names its column over
		// every row.
		heads := 0
		atAbsolute(body, func(o fyne.CanvasObject, at fyne.Position) {
			words, ok := o.(*canvas.Text)
			if !ok || words.Text != column.name {
				return
			}
			if at.Y > heading.Y && at.Y < firstRow.Y && within(at.X, control.X, 1) {
				heads++
			}
		})
		if heads != 1 {
			t.Errorf("the name %q stands %d times over the table of files inside an archive, and a table"+
				" names a column once. Below its heading at y=%.0f, above the first row at y=%.0f, at x=%.0f.",
				column.name, heads, heading.Y, firstRow.Y, control.X)
		}
		// And no cell carries the name itself. Asked of both rows, since the
		// defect this closes was a name in EVERY cell.
		for row := 1; row <= 2; row++ {
			f := fields.Lookup(recipe.ContentAddress(1, row, column.key))
			if f == nil {
				t.Fatalf("no field is registered for row %d, column %q", row, column.key)
			}
			inCell := 0
			walk(f.Object(), func(o fyne.CanvasObject) {
				if words, ok := o.(*canvas.Text); ok && words.Text == column.name {
					inCell++
				}
			})
			if inCell != 0 {
				t.Errorf("the cell in row %d under %q carries its own name %d time(s), so the table names"+
					" the column once over the header and again in every row", row, column.name, inCell)
			}
		}
	}
	w.Close()
}

// And every row stands under the same names: a column's control in every row
// starts where the column's name starts, and is as wide as it is in every
// other row.
//
// The second half is a defect that was found by looking at the two row table
// rendered for this change, not by reading: the format menu in the first row
// was 152 px and in the second 140, because parts.menuWidth measured the
// placeholder the toolkit had put into the menu that had been drawn once and
// not into the one that had not. The same setting was two widths on one
// screen, one row apart.
func TestEveryRowOfATableStandsUnderTheSameNames(t *testing.T) {
	ourTheme(t)
	screen, body, w := twoRowsOfContents(t)
	fields := screen.Fields()
	for _, key := range []string{recipe.KeyFormat, recipe.KeyCount, recipe.KeySize} {
		first := controlOf(t, fields, body, recipe.ContentAddress(1, 1, key))
		second := controlOf(t, fields, body, recipe.ContentAddress(1, 2, key))
		if !within(first.X, second.X, 1) {
			t.Errorf("the %q column starts at x=%.2f in the first row and x=%.2f in the second,"+
				" so the rows do not stand under the same names", key, first.X, second.X)
		}
		if first.Width != second.Width {
			t.Errorf("the %q control is %.2f px wide in the first row and %.2f in the second - one"+
				" setting, two widths, one row apart", key, first.Width, second.Width)
		}
	}
	w.Close()
}

// A menu is one width whether it is measured before it has ever been drawn or
// after. The cause of the two widths above, asked at its source: the toolkit
// puts its own placeholder into an empty menu while the renderer is made, and
// parts.menuWidth used to measure whatever placeholder was there.
//
// The state is asserted rather than assumed - the placeholder has to have
// ARRIVED for the second measurement to be the warm one - because a guard
// that measures a menu twice before the toolkit touched it would agree with
// the old code.
func TestAMenuIsOneWidthBeforeAndAfterItIsDrawn(t *testing.T) {
	ourTheme(t)
	menu := parts.NewChooser(format.IDs(), nil)
	// Menu measures the menu as it builds the box, so the box built here is
	// the cold measurement - and asking the box for its size is what makes the
	// toolkit build the renderer and put the placeholder in.
	sizedCold := parts.Menu(menu)
	if menu.PlaceHolder != "" {
		t.Fatalf("a menu built and not yet drawn already has the placeholder %q, so there is no cold"+
			" measurement to take", menu.PlaceHolder)
	}
	cold := sizedCold.MinSize().Width
	if menu.PlaceHolder == "" {
		t.Fatal("asking the box for its size did not put the toolkit's placeholder into the menu, so the" +
			" warm measurement below is the cold one again and this guard would prove nothing")
	}
	warm := parts.Menu(menu).MinSize().Width
	if cold != warm {
		t.Errorf("a menu of the format ids is %.2f px measured before it is drawn and %.2f px after"+
			" the toolkit put %q into it, so the same menu is two widths depending on when it was"+
			" measured", cold, warm, menu.PlaceHolder)
	}
}

// twoRowsOfContents is the batch screen with an archive holding two files,
// laid out in a window, which is the one place in this program where fields
// stand in a table.
func twoRowsOfContents(t *testing.T) (*window.Recipe, fyne.CanvasObject, fyne.Window) {
	t.Helper()
	screen := window.NewRecipe(newFakeHost(t))
	body := screen.Object()
	w := test.NewWindow(body)
	fields := screen.Fields()
	chooserIn(t, fields, recipe.TargetAddress(1, recipe.KeyFormat)).SetSelected("zip")
	pressNamed(t, body, text.ButtonAddContents())
	pressNamed(t, body, text.ButtonAddContents())
	settle(body, w)
	if fields.Lookup(recipe.ContentAddress(1, 2, recipe.KeyFormat)) == nil {
		t.Fatal("pressing the button twice did not put a second row of contents on the screen")
	}
	return screen, body, w
}

// controlOf is where a registered field's control stands on the screen, with
// its size - the box the eye lands on, not the field with its refusal area.
func controlOf(t *testing.T, fields *parts.Fields, body fyne.CanvasObject, at string) (box struct {
	X, Y, Width float32
}) {
	t.Helper()
	f := fields.Lookup(at)
	if f == nil {
		t.Fatalf("no field is registered at %q", at)
	}
	pos, ok := absoluteOf(body, f.Control)
	if !ok {
		t.Fatalf("the control at %q is not on the screen", at)
	}
	if f.Control.Size().Width == 0 {
		t.Fatalf("the control at %q has no width, so it was never laid out", at)
	}
	box.X, box.Y, box.Width = pos.X, pos.Y, f.Control.Size().Width
	return box
}

// textAt is where one line of words stands on the screen - canvas words or a
// one line label, which is what a block's name has been since 2026-09-16.
func textAt(t *testing.T, body fyne.CanvasObject, words string) fyne.Position {
	t.Helper()
	var at fyne.Position
	found := 0
	atAbsolute(body, func(o fyne.CanvasObject, pos fyne.Position) {
		if shown, _, _, ok := boldWordsAt(o); ok && shown == words {
			at = pos
			found++
		}
	})
	if found != 1 {
		t.Fatalf("%q stands %d times on the screen, and this guard needs it exactly once", words, found)
	}
	return at
}

func within(a, b, tolerance float32) bool {
	d := a - b
	return d <= tolerance && d >= -tolerance
}
