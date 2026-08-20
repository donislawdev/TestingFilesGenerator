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

// Space says what belongs together.
//
// Measured off a render on 2026-08-20, before the scale existed: every gap in
// the form came out of the theme's one padding value, so the distance from a
// label to its own control was 20 px and the distance from the end of one
// field to the start of the next was 23 px. Sixteen consecutive gaps down one
// card, all of them between 20 and 27. The form gave the same distance to
// "these three things are one field" and "that group has ended", which is the
// whole of what spacing does, and a picture of it reads as a wall of text.
//
// This asks the laid out screen rather than the constants. Reading the numbers
// back out of the package that declares them proves they were declared, which
// is not the question - what a person sees is where the widgets ended up, and
// a layout is free to add padding of its own on top of any gap it is given.
// The old code did exactly that, which is why a spacer could not make a gap
// smaller and the tight step needed a layout rather than a constant.
//
// The ratio is what is asserted rather than the values. Nothing here says a
// field's parts must be 15 px apart - that is a judgement to make with a
// picture. What has to hold is that the eye can tell the two apart without
// counting, and 1.5 is the weakest version of that claim: the pair this
// replaced was 1.15 apart.
func TestAFieldHoldsTogetherMoreTightlyThanTwoFieldsDo(t *testing.T) {
	ourTheme(t)
	content, _ := laidOutWindow(t)
	generate := tabContent(t, content, text.TabOneTarget)

	inside := gapBelowLabel(t, generate, text.FieldFormat)
	between := gapBetween(t, generate, text.HintFormat, text.FieldSize)

	if inside <= 0 || between <= 0 {
		t.Fatalf("measured %.1f px inside a field and %.1f px between two, and neither can be zero", inside, between)
	}
	if between < inside*1.5 {
		t.Errorf("a field's own parts are %.1f px apart and two fields are %.1f px apart, which is a ratio of %.2f."+
			" Below 1.5 the two distances read as one and the form has no grouping left",
			inside, between, between/inside)
	}
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
	generate := tabContent(t, content, text.TabOneTarget)

	gaps := sectionGaps(t, generate)
	if len(gaps) == 0 {
		t.Fatal("the generate screen shows fewer than two sections, so there is no gap to measure")
	}
	between := gapBetween(t, generate, text.HintFormat, text.FieldSize)
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

// gapBetween is the empty space between the bottom of one labelled thing and
// the top of the next, both named by the words on them.
func gapBetween(t *testing.T, screen fyne.CanvasObject, above, below string) float32 {
	t.Helper()
	top, topOK := labelBox(screen, above)
	bottom, bottomOK := labelBox(screen, below)
	if !topOK {
		t.Fatalf("no label reading %q on this screen", above)
	}
	if !bottomOK {
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
		label, is := o.(*widget.Label)
		if !is || label.Text != words {
			return
		}
		found, ok = band{X: at.X, Y: at.Y, Height: o.Size().Height}, true
	})
	return found, ok
}

// gapBelowLabel is the space between a field's name and the control under it,
// which is the tightest step the scale has.
func gapBelowLabel(t *testing.T, screen fyne.CanvasObject, label string) float32 {
	t.Helper()
	name, ok := labelBox(screen, label)
	if !ok {
		t.Fatalf("no label reading %q on this screen", label)
	}
	control := controlUnder(screen, label)
	if control == nil {
		t.Fatalf("no control under %q", label)
	}
	box, ok := objectBox(screen, control)
	if !ok {
		t.Fatalf("the control under %q is not laid out", label)
	}
	return box.Y - (name.Y + name.Height)
}

// objectBox is where one object this test already holds ended up.
func objectBox(screen fyne.CanvasObject, target fyne.CanvasObject) (band, bool) {
	found := band{}
	ok := false
	atAbsolute(screen, func(o fyne.CanvasObject, at fyne.Position) {
		if ok || o != target {
			return
		}
		found, ok = band{X: at.X, Y: at.Y, Height: o.Size().Height}, true
	})
	return found, ok
}

// band is one thing on the screen, reduced to what these guards ask about.
type band struct{ X, Y, Height float32 }

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
		case *container.AppTabs:
			for _, item := range v.Items {
				step(item.Content, at)
			}
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
