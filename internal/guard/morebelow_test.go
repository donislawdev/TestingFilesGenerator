package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"

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
