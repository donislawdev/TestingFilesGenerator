package guard

import (
	"testing"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// The buttons that are not about the run stand at the edge of the bar, and stay
// there when the window is made wider.
//
// Asked for by the owner on 2026-08-19, looking at the built window: Donate was
// a margin in from the left and he wanted it as far left as it goes. Until then
// the whole left hand group lived inside the form's column - laid over the row
// the run buttons are centred in - so it started where the form started, which
// at the opening size is 78 px in.
//
// Measured at two widths rather than compared against a number, and that is the
// whole design of this guard. A pixel it was written down against would be a
// copy of the layout's own arithmetic and would drift the first time a padding
// changed. The behaviour that actually differs between the two layouts is how
// the button answers a wider window: pinned to the edge it does not move, and
// held in a centred column it slides right by half of what the window gained.
// At 1600 px the old layout put it 300 px further right.
//
// Every screen, because the bar is on all four and Donate is the one control
// that is on all four - see TestTheDonateButtonIsOnEveryScreen.
func TestWhatIsNotAboutTheRunStandsAtTheEdgeOfTheBar(t *testing.T) {
	for _, tab := range []string{
		text.TabOneTarget(), text.TabPresets(), text.TabRecipe(), text.TabAbout(),
	} {
		t.Run(tab, func(t *testing.T) {
			content, w := screenInAWindow(t, tab)

			donate := buttonNamed(content, text.ButtonDonate())
			if donate == nil {
				t.Fatalf("this screen has no %q button, so this guard read the wrong tree",
					text.ButtonDonate())
			}

			atOpening := fyne.CurrentApp().Driver().AbsolutePositionForObject(donate).X

			wider := fyne.NewSize(window.OpenSize.Width+600, window.OpenSize.Height)
			w.Resize(wider)
			content.Refresh()
			w.Resize(wider)

			whenWider := fyne.CurrentApp().Driver().AbsolutePositionForObject(donate).X

			if whenWider != atOpening {
				t.Errorf("%q sits %.0f px from the left at %.0f px wide and %.0f px from the left "+
					"at %.0f px wide, so it is riding the middle of the window rather than "+
					"standing at the edge of the bar.\n"+
					"What to do: it belongs in the rail argument of parts.ActionBar, which is laid "+
					"outside the form's column. Put back inside that column it moves with it.",
					text.ButtonDonate(), atOpening, window.OpenSize.Width, whenWider, wider.Width)
			}

			// The bar's own padding is the only thing that should stand between
			// the button and the edge. Without this, a rail nailed to a fixed
			// offset far from the edge would pass the test above.
			if room := float32(parts.ColumnWidth) / 2; atOpening > room {
				t.Errorf("%q starts %.0f px from the left edge, which is further in than half a "+
					"form column (%.0f px) - it does not read as standing at the edge.",
					text.ButtonDonate(), atOpening, room)
			}
		})
	}
}
