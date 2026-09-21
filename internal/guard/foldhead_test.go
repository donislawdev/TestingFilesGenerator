package guard

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// The whole head row of a fold opens it and shuts it: the title, the arrow,
// the line said while it is shut and the room to the right of them - and the
// buttons a batch keeps in its head do their own job without touching the
// fold.
//
// Measured on 2026-09-16 (O221) with test.TapCanvas on the recipe screen:
// two presses on the title "Notes for the manifest" and the section stayed
// shut, a press on the arrow opened it. The arrow was the control and the
// title was words beside it. The owner's decision is that the whole row is
// one target, and this guard presses the row where a person would - through
// the canvas, at positions read off the laid out screen, so a target that
// is not where it looks is a red guard rather than a green one.
//
// Each press is asserted to have CHANGED the fold, not only to have landed:
// a press that toggles twice - the row and something inside it both
// answering - leaves the fold as it was, and "as it was" is what a guard
// asking only "did it open" would read as open already (O118).
func TestTheWholeHeadRowOfAFoldOpensAndShutsIt(t *testing.T) {
	ourTheme(t)
	content, c := laidOutWindow(t)
	screen := tabContent(t, content, text.TabRecipe())
	head := foldTitled(t, screen, "", text.BatchHeading(1))
	if !head.Open() {
		t.Fatal("batch 1 is built shut, so this guard is not asking what it thinks")
	}

	at := fyne.CurrentApp().Driver().AbsolutePositionForObject(head)
	size := head.Size()
	if size.Width < 200 || size.Height < 20 {
		t.Fatalf("the head of batch 1 is %v, which is not a row anybody could press", size)
	}
	arrow := arrowIn(t, screen, head)
	arrowAt := fyne.CurrentApp().Driver().AbsolutePositionForObject(arrow)
	middle := at.Y + size.Height/2

	for _, press := range []struct {
		label string
		where fyne.Position
	}{
		{"the title's first letters", fyne.NewPos(at.X+parts.TabInset+4, middle)},
		{"the arrow", arrowAt.Add(fyne.NewPos(arrow.Size().Width/2, arrow.Size().Height/2))},
		{"the empty room right of the words", fyne.NewPos(at.X+size.Width-4, middle)},
		{"the row's top edge", fyne.NewPos(at.X+size.Width/2, at.Y+1)},
	} {
		before := head.Open()
		test.TapCanvas(c, press.where)
		if head.Open() == before {
			t.Errorf("a press on %s (at %v) left the fold as it was, open=%v - the whole head row is one target (O221)",
				press.label, press.where, before)
		}
	}

	// The pointer puts the keyboard on the row quietly: the next Space works,
	// and no mark is drawn for somebody using the mouse (PointerFocus).
	if c.Focused() != head {
		t.Errorf("after a press the keyboard is on %T, not on the head row", c.Focused())
	}
	if head.Marked() {
		t.Error("the row was pressed with the pointer and drew the keyboard mark")
	}

	// A button in the head does its own job and leaves the fold alone. A
	// press on it goes to the button, which is on top of the row - and if the
	// row answered as well, pressing Duplicate would also shut the batch.
	duplicate := buttonNamed(screen, text.ButtonDuplicateBatch())
	if duplicate == nil {
		t.Fatalf("batch 1 has no %q button", text.ButtonDuplicateBatch())
	}
	before, batches := head.Open(), batchFolds(screen)
	buttonAt := fyne.CurrentApp().Driver().AbsolutePositionForObject(duplicate)
	test.TapCanvas(c, buttonAt.Add(fyne.NewPos(duplicate.Size().Width/2, duplicate.Size().Height/2)))
	if got := batchFolds(screen); got != batches+1 {
		t.Fatalf("pressing %q left %d batch(es) where %d were expected, so the press did not land on the button",
			text.ButtonDuplicateBatch(), got, batches+1)
	}
	head = foldTitled(t, screen, "", text.BatchHeading(1))
	if head.Open() != before {
		t.Errorf("pressing %q in the head of batch 1 also toggled the fold (open %v -> %v)", text.ButtonDuplicateBatch(), before, head.Open())
	}
}

// The keyboard opens and shuts the row once per press, and the mark is
// drawn for the keyboard alone.
//
// One press of the space bar reaches a focused control twice from the
// desktop driver - as the key and as the character - which is how one press
// of Space added two batches on 2026-09-16. Both arrivals are delivered here
// the way the driver delivers them, and the fold has to move exactly once.
func TestTheKeyboardOpensAndShutsAFoldOncePerPress(t *testing.T) {
	ourTheme(t)
	content, c := laidOutWindow(t)
	screen := tabContent(t, content, text.TabRecipe())
	head := foldTitled(t, screen, "", text.BatchHeading(1))

	c.Focus(head)
	if c.Focused() != head {
		t.Fatalf("the head row cannot hold the keyboard: the canvas put it on %T", c.Focused())
	}
	if !head.Marked() {
		t.Error("the keyboard arrived on the head row and no mark was drawn")
	}
	for _, key := range []fyne.KeyName{fyne.KeySpace, fyne.KeyReturn, fyne.KeyEnter} {
		before := head.Open()
		head.TypedKey(&fyne.KeyEvent{Name: key})
		if key == fyne.KeySpace {
			head.TypedRune(' ')
		}
		if head.Open() == before {
			t.Errorf("%s left the fold as it was", key)
		}
	}
	c.Unfocus()
	if head.Marked() {
		t.Error("the keyboard left the head row and the mark stayed")
	}
}

// Reaching for the keyboard after pressing the head row draws the ring on
// it, the way the first key after a press draws the mark on every other
// control of the family. The family guard,
// TestReachingForTheKeyboardTurnsTheMarkOn, asks the menu, and the row was
// the one control that took the keyboard on a press and then drew nothing on
// the first key: until 2026-09-17 a fold pressed with the mouse and opened
// with Space held the keyboard with no ring. Found by asking the row's
// siblings while an outside review of #110 asked the opposite question -
// whether a press should take the ring OFF a row the keyboard had marked -
// which is a rule of the family, not of the row. Read off the drawn ring,
// not off the flag (GUI rule 10), and both halves of the press are asked:
// the ring and the fold, because a key that drew the ring and did nothing
// else would pass the first alone.
func TestAKeyAfterAPressOnTheHeadRowDrawsTheRing(t *testing.T) {
	ourTheme(t)
	fold := parts.NewFolding("Notes for the manifest", nil, parts.Prose("inside"))
	w := test.NewWindow(fold.Object())
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(600, 200))
	head := fold.Head()
	_, ring := headRectangles(t, head)

	head.Tapped(&fyne.PointEvent{})
	if w.Canvas().Focused() != head {
		t.Fatalf("after a press the keyboard is on %T, not on the head row", w.Canvas().Focused())
	}
	if ring.StrokeWidth != 0 {
		t.Fatal("the press already drew the ring, so this guard cannot tell what the key did")
	}
	before := head.Open()
	head.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	head.TypedRune(' ')
	if ring.StrokeWidth == 0 {
		t.Error("a key was pressed on the head row and it still draws no ring to say the keyboard is in it")
	}
	if head.Open() == before {
		t.Error("the same press of Space left the fold as it was")
	}
}

// The row says it is under the pointer with a fill, says it holds the
// keyboard with a ring, and says neither at rest - read off the drawn
// rectangles rather than off a flag, because a state set and not painted
// looks exactly like one painted (GUI rule 10). The arrow follows: it is the
// head's mark, inked like the words at rest and brighter under the pointer,
// and it points down at an open fold and right at a shut one.
func TestTheHeadRowDrawsItsStatesAndTheArrowFollows(t *testing.T) {
	ourTheme(t)
	fold := parts.NewFolding("Notes for the manifest", nil, parts.Prose("inside"))
	w := test.NewWindow(fold.Object())
	t.Cleanup(w.Close)
	w.Resize(fyne.NewSize(600, 200))
	head := fold.Head()
	arrow := arrowIn(t, fold.Object(), head)

	back, ring := headRectangles(t, head)
	if back.FillColor != color.Transparent || ring.StrokeWidth != 0 {
		t.Errorf("at rest the row draws fill %v and ring %v, and it has to draw nothing", back.FillColor, ring.StrokeWidth)
	}
	restingArrow := arrow.Resource.Name()

	head.MouseIn(&desktop.MouseEvent{})
	if want := parts.PaletteColour(theme.ColorNameHover, theme.VariantDark); back.FillColor != want {
		t.Errorf("under the pointer the row's fill is %v, not the hover colour %v", back.FillColor, want)
	}
	// The fill is as wide as the words and no wider, since 2026-09-21: the
	// owner's report from the running window was a hover the width of the
	// form, which is enormous. The row is still the target - the head is as
	// wide as the row - so the two widths are asked for apart: the head wide,
	// its fill narrow.
	if row, fill := head.Size().Width, back.Size().Width; fill >= row || fill < 1 {
		t.Errorf("under the pointer the fill is %.0f px wide on a head %.0f px wide - the fill has to cover the words and not the row", fill, row)
	}
	if arrow.Resource.Name() == restingArrow {
		t.Error("the arrow is inked the same under the pointer as at rest, so it does not follow the row")
	}
	head.MouseOut()
	if back.FillColor != color.Transparent {
		t.Errorf("the pointer left and the fill stayed at %v", back.FillColor)
	}
	if arrow.Resource.Name() != restingArrow {
		t.Error("the pointer left and the arrow stayed inked as under the pointer")
	}

	head.FocusGained()
	if ring.StrokeWidth == 0 {
		t.Error("the keyboard is on the row and no ring is drawn")
	}
	if back.FillColor != color.Transparent {
		t.Errorf("the keyboard mark is a fill (%v), and it has to be a line - see Ring", back.FillColor)
	}
	head.FocusLost()
	if ring.StrokeWidth != 0 {
		t.Error("the keyboard left and the ring stayed")
	}

	openArrow := arrow.Resource.Name()
	fold.Set(false)
	if arrow.Resource.Name() == openArrow {
		t.Error("the fold shut and the arrow still points the way it did open")
	}
	fold.Set(true)
	if arrow.Resource.Name() != openArrow {
		t.Error("the fold opened again and the arrow does not point the way it did before")
	}
}

// The title of a fold stands on the edge its fields stand on, with the row
// reaching TabInset to the left of it for the fill and the ring to draw in -
// asked here of the batch on the recipe screen, and of every title on every
// screen by TestEverythingAPersonReadsStartsOnOneLeftEdge.
func TestTheHeadRowOverhangsTheColumnAndTheTitleDoesNot(t *testing.T) {
	ourTheme(t)
	content, _ := laidOutWindow(t)
	screen := tabContent(t, content, text.TabRecipe())
	head := foldTitled(t, screen, "", text.BatchHeading(1))
	title, ok := labelBox(screen, text.BatchHeading(1))
	if !ok {
		t.Fatalf("the recipe screen has no title reading %q", text.BatchHeading(1))
	}
	field, ok := labelBox(screen, text.FieldFormat())
	if !ok {
		t.Fatalf("the recipe screen has no field named %q", text.FieldFormat())
	}
	if off := title.X - field.X; off > 1 || off < -1 {
		t.Errorf("the title of batch 1 starts at %.1f px and the name of its first field at %.1f px - one edge for everything a person reads", title.X, field.X)
	}
	headAt, ok := absoluteOf(screen, head)
	if !ok {
		t.Fatal("the head row is not on the screen it was found in")
	}
	if got := title.X - headAt.X; got < parts.TabInset-1 || got > parts.TabInset+1 {
		t.Errorf("the row starts %.1f px left of its title, and it has to start TabInset (%v) left of it - the room the fill and the ring draw in", got, parts.TabInset)
	}
}

// arrowIn is the arrow of the fold whose head is given: the icon standing
// inside the head row's bounds.
func arrowIn(t *testing.T, root fyne.CanvasObject, head *parts.FoldHead) *widget.Icon {
	t.Helper()
	headAt, ok := absoluteOf(root, head)
	if !ok {
		t.Fatal("the head row is not on the screen it was found in")
	}
	var found *widget.Icon
	atAbsolute(root, func(o fyne.CanvasObject, pos fyne.Position) {
		icon, ok := o.(*widget.Icon)
		if !ok || found != nil {
			return
		}
		inside := pos.X >= headAt.X && pos.Y >= headAt.Y &&
			pos.X < headAt.X+head.Size().Width && pos.Y < headAt.Y+head.Size().Height
		if inside {
			found = icon
		}
	})
	if found == nil {
		t.Fatalf("the head row %q has no arrow in it", head.Title())
	}
	return found
}

// headRectangles are the fill and the ring a head row draws, in that order,
// read from the renderer the way the canvas reads them.
func headRectangles(t *testing.T, head *parts.FoldHead) (back, ring *canvas.Rectangle) {
	t.Helper()
	objects := test.WidgetRenderer(head).Objects()
	if len(objects) != 2 {
		t.Fatalf("the head row draws %d object(s), and this guard knows a fill and a ring", len(objects))
	}
	back, okBack := objects[0].(*canvas.Rectangle)
	ring, okRing := objects[1].(*canvas.Rectangle)
	if !okBack || !okRing {
		t.Fatalf("the head row draws %T and %T, and this guard knows two rectangles", objects[0], objects[1])
	}
	return back, ring
}
