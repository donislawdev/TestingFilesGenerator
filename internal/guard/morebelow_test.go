package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// A form with more under it says so, and a form that fits does not.
//
// Two of the three screens cannot fit in the window and one of them never will:
// the batch screen is a list of batches, so at two batches it scrolls whatever
// anybody does to the spacing. Measured on 2026-08-20 after the spacing and the
// box widths were dealt with: 832 px of form in 837 px of room on the single
// batch screen, 1000 in 837 on presets, 1259 in 837 on the batch list.
//
// So the answer for those two is not to keep chasing the height. It is to stop
// the form ending in a way that looks like the end - a form cut off at the
// window edge mid-control reads as a screen that is broken, and one that fades
// reads as a screen with more on it.
//
// Both halves in one guard on purpose. A fade drawn always would satisfy the
// first half and say "there is more" on a screen where there is not, which is a
// mark that carries no information - and it is the cheaper of the two to write
// by accident.
func TestAFormWithMoreUnderItSaysSoAndOneThatFitsDoesNot(t *testing.T) {
	for _, screen := range []struct {
		tab      string
		wantFade bool
		why      string
	}{
		{text.TabOneTarget, false, "this form fits the window it opens at"},
		{text.TabPresets, true, "this form is taller than the window"},
		{text.TabRecipe, true, "this form is taller than the window"},
	} {
		t.Run(screen.tab, func(t *testing.T) {
			content, w := screenInAWindow(t, screen.tab)
			settle(content, w)

			scroll := scrollIn(content)
			if scroll == nil {
				t.Fatal("this screen has no scrolling area, so this guard read the wrong tree")
			}
			// The state is asserted rather than assumed, because the heights
			// move with every change to the form and a guard set up for the
			// wrong one passes without asking anything.
			fits := scroll.Content.MinSize().Height <= scroll.Size().Height
			if fits == screen.wantFade {
				t.Fatalf("%s: %.0f px of form in %.0f px of room, and this guard is set up for the other case",
					screen.why, scroll.Content.MinSize().Height, scroll.Size().Height)
			}

			shown := fadeShowing(content)
			if shown != screen.wantFade {
				if screen.wantFade {
					t.Errorf("%.0f px of form in %.0f px of room and nothing says so, so the form ends at the "+
						"window edge as though that were the end of it",
						scroll.Content.MinSize().Height, scroll.Size().Height)
					return
				}
				t.Errorf("%.0f px of form in %.0f px of room and the screen still says there is more below."+
					" A mark that is always on carries no information",
					scroll.Content.MinSize().Height, scroll.Size().Height)
			}
		})
	}
}

// fadeShowing says whether the soft edge is drawn anywhere on this screen.
func fadeShowing(o fyne.CanvasObject) bool {
	found := false
	walk(o, func(obj fyne.CanvasObject) {
		if fade, is := obj.(*canvas.LinearGradient); is && fade.Visible() {
			found = true
		}
	})
	return found
}

// The sign that there is more below is one somebody can see with no text in it.
//
// The half the guard above cannot ask. It proves the fade is SHOWN on the right
// screens, and a fade shown but invisible passes it - which is what was
// happening, measured on 2026-08-24 off a render of the preset screen.
//
// The band used to end at the page colour, so where the last thing above the
// edge was writing it was unmistakable - the text falls from 91 L* to the page
// in twelve pixels - and where the last thing was an empty stretch of panel the
// whole band moved 5.9 L*. This project's own yardstick for a difference
// somebody notices is 10, so the sign was strongest exactly where a reader
// least needed it and absent where a form quietly ran on past the edge.
//
// It is asked of the gradient ON THE SCREEN rather than of the constant behind
// it. A number in a palette is a claim about a colour and only the tree is a
// claim about what got drawn - which is the same distinction the section
// surface guard keeps for a one pixel line.
func TestTheSignThatThereIsMoreBelowCanBeSeenWithNoTextInIt(t *testing.T) {
	content, w := screenInAWindow(t, text.TabPresets)
	settle(content, w)

	fade := gradientIn(content)
	if fade == nil {
		t.Fatal("this screen draws no fade at its foot, so this guard read the wrong tree")
	}
	if !fade.Visible() {
		t.Fatal("the fade is hidden on a screen taller than its window, so there is nothing to measure")
	}

	// Against the PANEL, because that is what is under the band where there is
	// no text - and text is the case that was already covered.
	panel := parts.PaletteColour(parts.ColorNamePanel, theme.VariantDark)
	gap := lightnessGap(fade.EndColor, panel)
	if gap < 10 {
		t.Errorf("the foot of the form fades to %.1f L* from the panel it is drawn on, "+
			"and 10 is the least this palette calls noticeable.\n"+
			"Reason: where the bottom of a form is empty panel rather than text, this band is the whole sign.",
			gap)
	}

	// And the shape beside the colour, which is UX1: a band that darkens is a
	// colour and nothing else. Shown and hidden with the band, so it cannot
	// become a mark that is always on.
	if !markShowing(content) {
		t.Error("the form runs past the window and no arrow is drawn, so the only sign that " +
			"there is more is a change of colour")
	}
	t.Logf("the band ends %.1f L* from the panel", gap)
}

func gradientIn(o fyne.CanvasObject) *canvas.LinearGradient {
	var found *canvas.LinearGradient
	walk(o, func(obj fyne.CanvasObject) {
		if fade, is := obj.(*canvas.LinearGradient); is && found == nil {
			found = fade
		}
	})
	return found
}

// markShowing is whether the arrow at the foot of the form is drawn.
func markShowing(o fyne.CanvasObject) bool {
	found := false
	walk(o, func(obj fyne.CanvasObject) {
		if img, is := obj.(*canvas.Image); is && img.Visible() && img.Size().Width == parts.MarkSize {
			found = true
		}
	})
	return found
}
