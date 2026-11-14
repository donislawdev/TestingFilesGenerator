package guard

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// Every control of a field is off while a run owns the screen, the button
// beside a box as well as the box.
//
// The form freezes so that nothing on it can change while files go into the
// directory it names (O106). Until 2026-11-14 the freeze took the FIRST control
// of a field it could switch off, and the output directory is a box with
// Choose... beside it - so the box went grey and the button stayed live. A
// folder picked during a run went into the grey box, and the screen named a
// directory the run was not writing into, which is the confusion the freeze
// exists to prevent.
//
// Asked of the button by its label, and only after the guard has found it:
// a screen that lost the button would otherwise pass by having nothing to ask.
func TestEveryControlOfAFieldIsOffWhileARunOwnsTheScreen(t *testing.T) {
	// The canary first: the check has to see a live button as live, or a
	// green answer below says nothing.
	if live := liveControls([]fyne.CanvasObject{parts.NewButton(parts.Secondary, "live", func() {})}); len(live) != 1 {
		t.Fatalf("the check found %d live controls in a list holding one live button, so it cannot see the defect it is for", len(live))
	}

	host, content, hold := heldScreen(t)
	clock := host.holdTheClock()
	w := test.NewWindow(host.content)
	t.Cleanup(w.Close)
	w.Resize(window.LargestOpening)

	fill(t, content, text.FieldOutputDir(), t.TempDir())
	fill(t, content, text.FieldCount(), "20000")
	choose := buttonsNamed(content, text.ButtonChoose())
	if len(choose) == 0 {
		t.Fatalf("there is no %q button on the screen, so this guard read the wrong tree", text.ButtonChoose())
	}
	press(t, content, text.ButtonPreview())

	hold.look(func() {
		clock.fire()
		box := entryUnder(t, content, text.FieldOutputDir())
		if !box.Disabled() {
			t.Fatal("the clock fired during the preview and the output directory box is not frozen, so this guard is not looking at a frozen form")
		}
		objects := make([]fyne.CanvasObject, 0, len(choose))
		for _, b := range choose {
			objects = append(objects, b)
		}
		if live := liveControls(objects); len(live) > 0 {
			t.Errorf("the form is frozen and %s is still live beside the output directory, so a folder picked now changes a box the run is not reading", strings.Join(live, ", "))
		}
	})
}

// liveControls names the controls in the list that can still be pressed.
func liveControls(objects []fyne.CanvasObject) []string {
	var live []string
	for _, o := range objects {
		if d, ok := o.(fyne.Disableable); ok && !d.Disabled() {
			name := "a control"
			if b, isButton := o.(*parts.Button); isButton {
				name = "the " + b.Text + " button"
			}
			live = append(live, name)
		}
	}
	return live
}
