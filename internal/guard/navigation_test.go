package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// Moving between screens is tabs at the top, not buttons at the foot.
//
// Reported from use on 2026-08-11. The way to the other screen sat in the row
// of actions under the last field, so changing screen meant scrolling past the
// whole form to find it - and nobody scrolls to the bottom to navigate. It also
// read as an action taken on the form, sitting between Preview and Generate,
// which is what it is not.
//
// Both halves are checked, because a half done move is the likely one: tabs
// added at the top while the buttons stay where they were leaves two ways to do
// one thing, and the screenshot of the top looks fixed.
func TestMovingBetweenScreensIsTabsAndNotButtons(t *testing.T) {
	host := &fakeHost{}
	window.Open(host)

	want := []string{text.TabOneTarget, text.TabPresets, text.TabAbout}
	got := tabNames(host.content)
	if len(got) != len(want) {
		t.Fatalf("the window has tabs %v and %v was expected", got, want)
	}
	for i, name := range want {
		if got[i] != name {
			t.Errorf("tab %d is %q and %q was expected", i, got[i], name)
		}
	}

	// No screen may still carry a button that moves to another screen. Asked of
	// every tab rather than of the visible one, because the buttons were on
	// both work screens and removing one of the two would read as done.
	for _, name := range want {
		screen := tabNamed(t, host.content, name)
		walk(screen, func(obj fyne.CanvasObject) {
			button, ok := obj.(*widget.Button)
			if !ok {
				return
			}
			for _, nav := range want {
				if button.Text == nav {
					t.Errorf("the %s screen still has a %q button, so there are two ways to move between screens and one of them is at the bottom of a form",
						name, button.Text)
				}
			}
		})
	}
}
