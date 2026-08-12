package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// Cancel is not on the screen while there is nothing to cancel.
//
// Asked for on 2026-08-11 after looking at the window. It had been there and
// greyed, which is a question the screen keeps asking and answering itself -
// and it sat beside the two buttons that do work, so a row of two choices read
// as three.
//
// Both halves, because hiding it and never bringing it back is the easy way to
// get this wrong and looks right in a screenshot of an idle window.
func TestCancelIsOnlyThereWhenThereIsSomethingToCancel(t *testing.T) {
	dir := t.TempDir()
	host, content := screen(t)

	cancel := buttonNamed(content, "Cancel")
	if cancel == nil {
		t.Fatal("there is no Cancel button at all, so this guard read the wrong tree")
	}
	if cancel.Visible() {
		t.Error("Cancel is on the screen before anything has been started")
	}

	fill(t, content, text.FieldOutputDir, dir)
	fill(t, content, text.FieldSize, "1kb")
	fill(t, content, text.FieldCount, "200")
	press(t, content, "Generate")
	if !cancel.Visible() {
		t.Error("Cancel is not on the screen during a run, so a run cannot be stopped")
	}

	join(host)
	waitForManifest(t, dir)

	// And gone again when the run is over. Without this the guard passes on the
	// constructor alone - it hides the button once at the start, so a screen
	// that never hid it again would look right until the first run finished.
	// Proven by mutation: taking the hiding out of the end of a run left this
	// green until the line below was added.
	if cancel.Visible() {
		t.Error("Cancel is still on the screen after the run finished, with nothing left to cancel")
	}
}

// Cancel is drawn as a button, not as words.
//
// Reported from the running screen on 2026-08-12. It had been given the lowest
// importance so it would recede beside Preview and Generate, and the toolkit
// draws that rank with no fill and no border at all - so the way to stop a run
// arrived as bare text between two filled buttons, which reads as a disabled
// label rather than as a control. It is the one thing on the screen somebody
// reaches for in a hurry.
//
// The rank is what this asks about rather than the pixels, and that is a proxy
// stated as one: whether a surface is painted is the toolkit's decision, and
// the rank is the whole of what we tell it. Measured before writing this - the
// lowest rank puts no fill behind the words, and the default rank does.
func TestCancelIsDrawnAsAButton(t *testing.T) {
	_, content := screen(t)

	cancel := buttonNamed(content, "Cancel")
	if cancel == nil {
		t.Fatal("there is no Cancel button at all, so this guard read the wrong tree")
	}
	if cancel.Importance == widget.LowImportance {
		t.Error("Cancel is drawn at the lowest importance, which paints no surface - so it arrives as bare words beside two filled buttons and reads as disabled")
	}
}

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
