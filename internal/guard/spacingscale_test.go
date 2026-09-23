package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// A name stands over its box, on one edge with it and with every other.
//
// Over since 2026-09-21, on the owner's decision from the running window
// ("that is prettier"). Beside from 2026-09-14 to that day, in a column of
// names, when this guard asked for the middle of the name to be the middle
// of the box. Over before 2026-09-14 too - so the question this asks is the
// one a stacked form has, written down this time so the third turn of the
// wheel can read it: a name belongs to the box UNDER it, which means the gap
// from a name down to its box has to be smaller than the gap from that box
// down to the next name. A form where those two are equal is a list of
// words and boxes with no pairing, and a form where the name is nearer the
// box above it pairs every name with the wrong box.
//
// Three things, all read off the laid out screen rather than off the
// constants, for the reason the section guards give below: a layout is free
// to put a name anywhere, and reading the numbers back out of the package that
// declares them proves they were declared, which is not the question.
//
// Over means the name ends above the box begins. One edge means the name and
// its box start on one X - a box indented under its name reads as a child of
// it rather than as the value it holds - and every box starts on the edge of
// one of the grid's columns (since 2026-09-23, parts.Grid - before that there
// was one column and one edge). Apart means two fields in one column have
// room between them, or the form is a list with no rhythm.
func TestANameStandsOverItsBoxOnOneEdgeWithEveryOther(t *testing.T) {
	ourTheme(t)
	content, _ := laidOutWindow(t)
	generate := tabContent(t, content, text.TabOneTarget())

	names := []string{text.FieldFormat(), text.FieldSize(), text.FieldCount(), text.FieldTargetID(),
		text.FieldNameTemplate(), text.FieldOutputDir(), text.FieldSeed()}
	edges := map[float32][]string{}
	var boxes []band
	var nameGaps []float32
	for _, label := range names {
		name, box := nameAndBox(t, generate, label)
		if name.Y+name.Height > box.Y {
			t.Errorf("%q ends at y=%.1f and its box begins at y=%.1f, so the name does not stand over the box",
				label, name.Y+name.Height, box.Y)
		}
		if off := name.X - box.X; off > 1 || off < -1 {
			t.Errorf("%q starts at x=%.1f and its box at x=%.1f - a box not on its name's edge reads as indented under it",
				label, name.X, box.X)
		}
		nameGaps = append(nameGaps, box.Y-(name.Y+name.Height))
		edges[box.X] = append(edges[box.X], label)
		boxes = append(boxes, box)
	}
	// Columns since 2026-09-23 (parts.Grid): the boxes start on the edges of
	// the grid's columns rather than on one, and the first of those is the
	// edge the first field stands on. More edges than the grid has columns
	// is a box standing between two of them.
	if len(edges) > parts.GridColumns {
		t.Errorf("the boxes on the generate screen start on %d different edges, more than the %d columns the form "+
			"is laid in, so some box stands off every column: %v", len(edges), parts.GridColumns, edges)
	}
	if first := boxes[0].X; len(edges[first]) == 0 || boxes[0].X > minEdge(edges)+1 {
		t.Errorf("the first box starts at x=%.1f and the leftmost at x=%.1f - the form's first column is where it begins",
			first, minEdge(edges))
	}
	// Apart, and the pairing, asked down each column: a field stands under
	// the one above it in its own column, while the one beside it shares its
	// row. The pairing - a name is nearer the box under it than that box is
	// to the next name - is asked for each pair down the screen rather than
	// once at the top, after the outside review of #116: measured once, a
	// later pair could drift while the first still held.
	for i := 0; i < len(names); i++ {
		j := nextInColumn(boxes, i)
		if j < 0 {
			continue
		}
		if boxes[j].Y <= boxes[i].Y+boxes[i].Height {
			t.Errorf("%q and %q are not apart: one box ends at %.1f and the next begins at %.1f",
				names[i], names[j], boxes[i].Y+boxes[i].Height, boxes[j].Y)
		}
		between := gapBelowField(t, generate, names[i], names[j])
		if nameGaps[i] >= between {
			t.Errorf("%q stands %.1f px over its box and %q stands %.1f px under that box, so nothing says which box the name belongs to",
				names[i], nameGaps[i], names[j], between)
		}
	}
}

// nextInColumn is the first field after i that starts on i's column, or -1.
func nextInColumn(boxes []band, i int) int {
	for j := i + 1; j < len(boxes); j++ {
		if off := boxes[j].X - boxes[i].X; off <= 1 && off >= -1 {
			return j
		}
	}
	return -1
}

// minEdge is the leftmost of the edges boxes start on.
func minEdge(edges map[float32][]string) float32 {
	first := true
	var least float32
	for x := range edges {
		if first || x < least {
			least, first = x, false
		}
	}
	return least
}

// Two sections are the same distance apart wherever they are.
//
// The defect this was written for, measured on 2026-08-20: the generate screen
// put 7 px between its two panels and the preset screen put 23 px between
// each of its three. One relationship, two answers, and nobody had compared
// them because each screen looked settled on its own.
//
// Worse than uneven. A panel carries padding inside it, so at 7 px the space
// between two SEPARATE sections was smaller than the space inside one - which
// inverts what proximity says and made Output read as glued to the panel above
// it.
//
// The cause was not a number anybody chose. Screen puts GapSection between the
// panels it is handed, and the generate screen handed it one box holding two
// panels, so the gap landed around the box instead of between them. That is
// why this measures every screen and compares them with each other rather than
// against a constant: a screen can be built in a way that never reaches the
// spacing, and a guard reading the constant would not notice.
func TestTwoSectionsAreTheSameDistanceApartOnEveryScreen(t *testing.T) {
	ourTheme(t)
	content, _ := laidOutWindow(t)

	seen := map[string][]float32{}
	for _, tab := range allTabs() {
		gaps := sectionGaps(t, tabContent(t, content, tab))
		if len(gaps) > 0 {
			seen[tab] = gaps
		}
	}
	if len(seen) < 2 {
		t.Fatalf("only %d screen holds two sections, so nothing is being compared", len(seen))
	}

	var first float32
	var from string
	for _, tab := range allTabs() {
		for _, gap := range seen[tab] {
			if from == "" {
				first, from = gap, tab
				continue
			}
			// A pixel of slack, because a panel's own rounding is not the
			// subject and two screens can land half a pixel apart.
			if diff := gap - first; diff > 1 || diff < -1 {
				t.Errorf("%s puts %.1f px between two sections and %s puts %.1f px."+
					" One relationship drawn two ways is what a person reads as two different screens",
					from, first, tab, gap)
			}
		}
	}
}

// The space between two sections is bigger than the space inside one.
//
// The half of the defect above that survives even when every screen agrees:
// sections 7 px apart were uniform on the generate screen and still wrong,
// because a boundary drawn more weakly than the padding inside a panel groups
// the wrong things.
func TestTheGapBetweenSectionsIsWiderThanTheGapBetweenFields(t *testing.T) {
	ourTheme(t)
	content, _ := laidOutWindow(t)
	generate := tabContent(t, content, text.TabOneTarget())

	gaps := sectionGaps(t, generate)
	if len(gaps) == 0 {
		t.Fatal("the generate screen shows fewer than two sections, so there is no gap to measure")
	}
	between := gapBelowField(t, generate, text.FieldFormat(), text.FieldSize())
	for _, gap := range gaps {
		if gap <= between {
			t.Errorf("two sections are %.1f px apart and two fields inside one are %.1f px apart."+
				" A section boundary has to be the wider of the two or it groups nothing", gap, between)
		}
	}
}

// tabContent is one screen of the window, laid out, by the name on its tab.
//
// It selects rather than reaching straight for the content, and that is what
// this guard got wrong first: a tab nobody has opened is never laid out, so
// every widget on it sits at the origin with no size. Read without selecting,
// three of the four screens answered that they hold no sections at all - which
// would have made this pass by looking at one screen.
func tabContent(t *testing.T, window fyne.CanvasObject, tab string) fyne.CanvasObject {
	t.Helper()
	content := selectTab(t, window, tab)
	if content == nil {
		t.Fatalf("the window has no tab called %q", tab)
	}
	return content
}

// gapBelowField is the room between the bottom of one field and the name of
// the next.
//
// It measures from the CONTROL rather than from the line under it, and that is
// the change of 2026-08-24 rather than a different question: the line under a
// field moved behind the button beside its name, so the last thing a field
// draws is the box itself. Measured the old way this reported the distance from
// a label that is no longer there.
func gapBelowField(t *testing.T, screen fyne.CanvasObject, above, below string) float32 {
	t.Helper()
	control := controlUnder(screen, above)
	if control == nil {
		t.Fatalf("no control under %q", above)
	}
	top, ok := objectBox(screen, control)
	if !ok {
		t.Fatalf("the control under %q is not laid out", above)
	}
	bottom, ok := labelBox(screen, below)
	if !ok {
		t.Fatalf("no label reading %q on this screen", below)
	}
	return bottom.Y - (top.Y + top.Height)
}

// labelBox is where the words sit on the screen and how tall they are.
//
// Found by the text on the label rather than by a position handed in, because
// a guard given coordinates would be a copy of the layout it is checking.
func labelBox(screen fyne.CanvasObject, words string) (band, bool) {
	found := band{}
	ok := false
	atAbsolute(screen, func(o fyne.CanvasObject, at fyne.Position) {
		if ok {
			return
		}
		shown, is := wordsOf(o)
		if !is || shown != words {
			return
		}
		found, ok = band{X: at.X, Y: at.Y, Width: o.Size().Width, Height: o.Size().Height}, true
	})
	return found, ok
}

// nameAndBox is where a field's name and its control ended up on the screen.
func nameAndBox(t *testing.T, screen fyne.CanvasObject, label string) (name, box band) {
	t.Helper()
	name, ok := labelBox(screen, label)
	if !ok {
		t.Fatalf("no label reading %q on this screen", label)
	}
	control := controlUnder(screen, label)
	if control == nil {
		t.Fatalf("no control under %q", label)
	}
	box, ok = objectBox(screen, control)
	if !ok {
		t.Fatalf("the control under %q is not laid out", label)
	}
	return name, box
}

// objectBox is where one object this test already holds ended up.
func objectBox(screen fyne.CanvasObject, target fyne.CanvasObject) (band, bool) {
	found := band{}
	ok := false
	atAbsolute(screen, func(o fyne.CanvasObject, at fyne.Position) {
		if ok || o != target {
			return
		}
		found, ok = band{X: at.X, Y: at.Y, Width: o.Size().Width, Height: o.Size().Height}, true
	})
	return found, ok
}

// band is one thing on the screen, reduced to what these guards ask about.
type band struct{ X, Y, Width, Height float32 }

// sectionGaps is the empty space between each pair of panels on one screen,
// top to bottom.
func sectionGaps(t *testing.T, screen fyne.CanvasObject) []float32 {
	t.Helper()
	type box struct{ top, bottom float32 }
	var panels []box
	// Only what is inside the scroll. The action bar stands on the same
	// surface a section does - deliberately, so the two read as one system -
	// and it is pinned below the form rather than being part of it. Told apart
	// by where it lives rather than by how far away it is: the first version
	// of this used a height cut off and reported the distance from the last
	// panel to the bar as a gap between two sections, which is 106 px on a
	// window taller than the form.
	form := scrollIn(screen)
	if form == nil {
		return nil
	}
	atAbsolute(form, func(o fyne.CanvasObject, at fyne.Position) {
		stack, is := o.(*fyne.Container)
		if !is || len(stack.Objects) == 0 {
			return
		}
		rect, is := stack.Objects[0].(*canvas.Rectangle)
		if !is || rect.FillColor != parts.PaletteColour(parts.ColorNamePanel, theme.VariantDark) {
			return
		}
		// The action bar stands on the same surface and is not a section of the
		// form. It is the only panel outside the scroll, so anything laid out
		// below the form's own area is left out by height rather than by name.
		panels = append(panels, box{top: at.Y, bottom: at.Y + o.Size().Height})
	})
	if len(panels) < 2 {
		return nil
	}
	// Top to bottom, because a walk visits in tree order and a screen is free
	// to build its panels in any order.
	for i := 1; i < len(panels); i++ {
		for j := i; j > 0 && panels[j].top < panels[j-1].top; j-- {
			panels[j], panels[j-1] = panels[j-1], panels[j]
		}
	}
	var gaps []float32
	for i := 1; i < len(panels); i++ {
		gap := panels[i].top - panels[i-1].bottom
		// A panel nested inside another is not two sections in a row.
		if gap > 0 {
			gaps = append(gaps, gap)
		}
	}
	return gaps
}

// atAbsolute walks the tree carrying where each object ended up on the screen.
//
// Fyne positions an object inside its parent, so the number on a widget says
// nothing about where a person sees it. Everything this file asks is about
// distance between two things in different parents.
func atAbsolute(root fyne.CanvasObject, visit func(fyne.CanvasObject, fyne.Position)) {
	var step func(o fyne.CanvasObject, origin fyne.Position)
	step = func(o fyne.CanvasObject, origin fyne.Position) {
		if o == nil || !o.Visible() {
			return
		}
		at := origin.Add(o.Position())
		visit(o, at)
		switch v := o.(type) {
		case *fyne.Container:
			for _, child := range v.Objects {
				step(child, at)
			}
		case *container.Scroll:
			step(v.Content, at)
		case *widget.Card:
			step(v.Content, at)
		case *container.ThemeOverride:
			// Every screen is wrapped in one of these since 2026-08-20. A walk
			// that stops here reports a screen with nothing on it, and three
			// guards using this helper failed with "no such field" rather than
			// with anything about what they were asking.
			step(v.Content, at)
		}
	}
	step(root, fyne.NewPos(0, 0))
}
