package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// The busy face waits for work that lasts, and never arrives for work that
// does not.
//
// Reported by the owner from the running window on 2026-09-21: the whole
// window shook when Preview was pressed. Measured there: a preview of one
// file is done in about 50 ms, and in that time every box on the form was
// frozen - the toolkit draws a frozen box with a bright edge and grey words -
// a Cancel button came into the row and pushed the other two aside, a bar
// stood at nought, and then all of it was taken back. A flash, not a state.
//
// So the STATE is immediate - a second press of Generate inside the first
// moment of the first is still refused - and the FACE follows after
// BusyFaceAfter if the work is still going. Asked through the clock the host
// hands the window, held here so the moment before the face can be read:
// under the test driver a real timer would be a second writer to the widgets
// this guard is reading. Three things are held, and each is a way the delay
// could be wrong on its own: that the face is not on before the clock fires,
// that it IS on after, and that the clock was asked for the delay this
// package names rather than for nought.
func TestTheBusyFaceWaitsForWorkThatLasts(t *testing.T) {
	host, content, hold := heldScreen(t)
	clock := host.holdTheClock()
	w := test.NewWindow(host.content)
	t.Cleanup(w.Close)
	w.Resize(window.LargestOpening)

	fill(t, content, text.FieldOutputDir(), t.TempDir())
	fill(t, content, text.FieldCount(), "20000")
	box := entryUnder(t, content, text.FieldSize())
	if box == nil {
		t.Fatal("there is no size box, so this guard read the wrong tree")
	}
	press(t, content, text.ButtonPreview())

	// The moment after the press: nothing on the screen has changed yet.
	if box.Disabled() {
		t.Error("the size box froze the moment Preview was pressed - the flash the owner reported, for work that may be over in 50 ms")
	}
	if cancel := buttonNamed(content, text.ButtonCancel()); cancel != nil && cancel.Visible() {
		t.Error("Cancel came into the row the moment Preview was pressed, pushing the other buttons aside for work that may be over in 50 ms")
	}
	if clock.then == nil {
		t.Fatal("the window asked for nothing later, so the face either never arrives or arrived at once")
	}
	if clock.after != window.BusyFaceAfter || clock.after <= 0 {
		t.Errorf("the face was asked for after %v, and the delay this package names is %v", clock.after, window.BusyFaceAfter)
	}
	// And the state is there all the same: a second press starts nothing.
	// The button is still live, so pressIfLive would press it - the refusal
	// has to come from the state, see runner.onPreview. A second preview
	// would ask the clock a second time.
	press(t, content, text.ButtonPreview())
	if clock.asked != 1 {
		t.Errorf("Preview was pressed twice inside the first moment and the clock was asked %d times, so the second press started a second preview", clock.asked)
	}

	hold.look(func() {
		// The work is still going and the clock fires: now the face goes on.
		clock.fire()
		if !box.Disabled() {
			t.Error("the clock fired while the preview was still going and the form is not frozen")
		}
		cancel := buttonNamed(content, text.ButtonCancel())
		if cancel == nil || !cancel.Visible() || cancel.Disabled() {
			t.Error("the clock fired while the preview was still going and there is no live Cancel to stop it")
		}
		if bar, _ := runMessages(content); bar != nil && bar.Visible() {
			t.Error("a preview shows the progress bar, which stands at nought for as long as the preview takes - what a stuck run looks like")
		}
	})
	join(host)

	// Over: the face comes off at once, whether or not it was ever on.
	if box.Disabled() {
		t.Error("the preview is over and the form is still frozen")
	}
	if cancel := buttonNamed(content, text.ButtonCancel()); cancel != nil && cancel.Visible() {
		t.Error("the preview is over and Cancel is still offered")
	}
}

// Work that is over before the clock fires never wears the face at all, and
// takes its request back so the clock cannot dress a screen that is idle.
func TestWorkOverBeforeTheClockWearsNoFace(t *testing.T) {
	// No hold here, on purpose: the worker has to FINISH before the clock
	// fires, which is the case this guard is about. A parked worker and a
	// join is a guard that waits forever.
	host := newFakeHost(t)
	clock := host.holdTheClock()
	window.Open(host)
	if host.content == nil {
		t.Fatal("opening the window put no screen in it")
	}
	content := tabNamed(t, host.content, text.TabOneTarget())
	w := test.NewWindow(host.content)
	t.Cleanup(w.Close)
	w.Resize(window.LargestOpening)

	fill(t, content, text.FieldOutputDir(), t.TempDir())
	box := entryUnder(t, content, text.FieldSize())
	if box == nil {
		t.Fatal("there is no size box, so this guard read the wrong tree")
	}
	press(t, content, text.ButtonPreview())
	join(host)
	if !clock.calledOff {
		t.Error("the preview finished and the window did not call the face off, so a clock firing later would freeze an idle screen")
	}
	clock.fire()
	if box.Disabled() {
		t.Error("the clock fired after the preview was over and froze the form")
	}
	if cancel := buttonNamed(content, text.ButtonCancel()); cancel != nil && cancel.Visible() {
		t.Error("the clock fired after the preview was over and offered Cancel for nothing")
	}
}

// The row of buttons is laid out again when a button comes or goes, so
// Preview and Generate stand where they stood once the work is over.
//
// The other half of the owner's report of 2026-09-21, and the lasting half:
// after a preview the two buttons stood 32 px to the left of centre and
// stayed there. Measured in the pinned toolkit: hiding a button changes what
// the row asks for, so the canvas lays out the row's PARENT, which hands the
// row the size it already had - and a container given its own size does
// nothing (fyne.Container.Resize). The row kept the positions it had worked
// out with Cancel in it. See busy.relay.
//
// The test driver lays nothing out on its own, so what is asked is the
// mechanism from both sides: a Cancel that has come is laid out - it has a
// size and stands after Generate - and once it has gone the two buttons are
// back where they were. A row left to the toolkit shows Cancel at no size
// on the way in and leaves the two buttons shifted on the way out.
func TestTheButtonsStandWhereTheyStoodAfterAPreview(t *testing.T) {
	host, content, hold := heldScreen(t)
	w := test.NewWindow(host.content)
	t.Cleanup(w.Close)
	w.Resize(window.LargestOpening)
	settle(content, w)

	preview, generate := buttonNamed(content, text.ButtonPreview()), buttonNamed(content, text.ButtonGenerate())
	if preview == nil || generate == nil {
		t.Fatal("the screen has no Preview or no Generate, so this guard read the wrong tree")
	}
	atRest := [2]fyne.Position{preview.Position(), generate.Position()}
	if atRest[0].X <= 0 {
		t.Fatal("Preview stands at the left edge, so the row has not been laid out and there is nothing to compare")
	}

	fill(t, content, text.FieldOutputDir(), t.TempDir())
	fill(t, content, text.FieldCount(), "20000")
	press(t, content, text.ButtonPreview())
	hold.look(func() {
		cancel := buttonNamed(content, text.ButtonCancel())
		if cancel == nil || !cancel.Visible() {
			t.Fatal("there is no Cancel while the preview is going, so the row never changed and this guard is about nothing")
		}
		if cancel.Size().Width <= 0 || cancel.Position().X <= generate.Position().X {
			t.Errorf("Cancel came into the row and was never laid out: %v at %v, with Generate at %v",
				cancel.Size(), cancel.Position(), generate.Position())
		}
		if preview.Position() == atRest[0] {
			t.Error("Cancel is in the row and Preview has not moved to make room, so the row was not laid out with it")
		}
	})
	join(host)

	if got := [2]fyne.Position{preview.Position(), generate.Position()}; got != atRest {
		t.Errorf("after the preview Preview and Generate stand at %v and %v, and before it they stood at %v and %v - "+
			"the row kept the positions it worked out with Cancel in it", got[0], got[1], atRest[0], atRest[1])
	}
}
