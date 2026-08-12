package guard

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// The longer explanation is reachable, not merely written down.
//
// Half of every field's help moved behind a button on 2026-08-12, because the
// permanent version had grown to outweigh the controls: eight grey paragraphs
// on a form of eight settings, taking more vertical room than the boxes they
// explained. What is left under a field says what it does, and the units, the
// example and the consequence went behind an icon.
//
// That trade is only worth making if the second half can still be got at, and
// this is the guard for the half that is now invisible. docs/CLAUDE.md names
// the failure exactly: text with no reader is not dead text, it is pressure on
// the text beside it. A button that opens nothing would leave every one of
// these sentences written, shipped, translated and unread.
//
// It presses the button rather than looking for one. A button in the tree
// proves the layout, and only tapping it proves the wiring - which is the
// difference the parity guard's own note calls the threshold for saying a
// capability is reachable.
func TestTheLongerExplanationOpensWhenAsked(t *testing.T) {
	app := test.NewApp()
	defer test.NewApp()
	app.Settings().SetTheme(parts.Theme())

	host := &fakeHost{}
	window.Open(host)
	content := tabNamed(t, host.content, text.TabOneTarget)

	// A real canvas, because the button asks the toolkit which canvas it is on
	// and opens the explanation there. Built without one it does nothing at
	// all, on purpose - several guards build a tree they never show.
	w := test.NewWindow(host.content)
	defer w.Close()
	w.Resize(window.OpenSize)
	host.content.Refresh()

	// Every field that has one, rather than a sample. These are the sentences
	// nobody can see until they press something, so an unwired button is
	// invisible in exactly the way the permanent text was not.
	for _, field := range []struct{ label, detail string }{
		{text.FieldFormat, text.DetailFormat},
		{text.FieldSize, text.DetailSize},
		{text.FieldTargetID, text.DetailTargetID},
		{text.FieldNameTemplate, text.DetailNameTemplate},
		{text.FieldOutputDir, text.DetailOutputDir},
		{text.FieldSeed, text.DetailSeed},
		{text.FieldLabel, text.DetailLabel},
	} {
		button := detailButtonBeside(content, field.label)
		if button == nil {
			t.Errorf("%q has a longer explanation and no button that opens it", field.label)
			continue
		}

		// Pointing at it, which is what somebody meeting a small letter i
		// actually does. Reported on 2026-08-12: it opened on a click and
		// people hovered and waited. The toolkit has no tooltip, so this is
		// three methods of desktop.Hoverable answered by hand - and a hook that
		// is answered but never wired looks identical from outside.
		button.MouseIn(&desktop.MouseEvent{})
		if shown := overlayText(w.Canvas()); !strings.Contains(shown, field.detail) {
			t.Errorf("hovering the button beside %q did not show its explanation.\nWanted: %q\nShown: %q",
				field.label, field.detail, shown)
		}

		// And gone when the pointer leaves. Without this half the guard passes
		// on a tooltip that opens and never closes, which is worse than one
		// that never opens: it covers the field underneath it.
		button.MouseOut()
		if shown := overlayText(w.Canvas()); strings.Contains(shown, field.detail) {
			t.Errorf("the explanation for %q stayed on screen after the pointer left", field.label)
		}

		// A click still works, and that is not a nicety. Hovering is not
		// something a keyboard can do, and UX9 says whatever the mouse can
		// reach the keyboard can too - so the tap has to survive the hover
		// being added.
		button.OnTapped()
		if shown := overlayText(w.Canvas()); !strings.Contains(shown, field.detail) {
			t.Errorf("pressing the button beside %q did not show its explanation.\nWanted: %q\nShown: %q",
				field.label, field.detail, shown)
		}
		// And a second press takes it away again, because somebody who opened
		// it from a keyboard has no pointer to move off it.
		button.OnTapped()
		if shown := overlayText(w.Canvas()); strings.Contains(shown, field.detail) {
			t.Errorf("pressing the button beside %q a second time did not take the explanation away", field.label)
		}
		button.MouseOut()
	}
}

// Nothing the explanation puts on screen answers to the pointer.
//
// This is the flicker, written as a rule. Reported on 2026-08-12: the
// explanation strobed as soon as the pointer moved on the icon, and the cause
// was a loop rather than a redraw. widget.PopUp wraps itself in the toolkit's
// OverlayContainer, that container is stretched over the whole canvas, and it
// declares itself desktop.Hoverable with empty methods - internal/widget/
// overlay_container.go lines 59 to 70. So opening the explanation put a hover
// target over the button that had opened it: the button was told the pointer
// had left, it closed the explanation, the cover went with it, the button was
// hovered again. At the speed of mouse movement.
//
// The guard beside this one could not see any of that. It opens the
// explanation and reads it, which is exactly what happens on the first frame of
// the loop - so it was green throughout, and the defect was reported by
// somebody moving a mouse.
//
// What it asks is the property that makes hovering possible at all: whatever
// goes on top must be invisible to the pointer, so the toolkit looks through it
// and finds the button still underneath.
func TestTheExplanationDoesNotStealThePointerFromTheButton(t *testing.T) {
	app := test.NewApp()
	defer test.NewApp()
	app.Settings().SetTheme(parts.Theme())

	host := &fakeHost{}
	window.Open(host)
	content := tabNamed(t, host.content, text.TabOneTarget)

	w := test.NewWindow(host.content)
	defer w.Close()
	w.Resize(window.OpenSize)
	host.content.Refresh()

	button := detailButtonBeside(content, text.FieldSize)
	if button == nil {
		t.Fatal("there is no explanation button beside the size field, so this guard read the wrong tree")
	}
	button.MouseIn(&desktop.MouseEvent{})
	defer button.MouseOut()

	overlays := w.Canvas().Overlays().List()
	if len(overlays) == 0 {
		t.Fatal("hovering put nothing on screen, so this guard measured an empty overlay")
	}

	for _, overlay := range overlays {
		walk(overlay, func(obj fyne.CanvasObject) {
			if _, steals := obj.(desktop.Hoverable); steals {
				t.Errorf("the explanation puts a %T on screen and it answers to the pointer.\n"+
					"Anything hoverable above the button takes the hover that keeps the explanation open, "+
					"so it closes, uncovers the button, opens again - which is the flicker.", obj)
			}
		})
	}
}

// detailButtonBeside is the icon button sharing a line with a field's name.
//
// Found through the heading row rather than by walking the whole field,
// because the field also holds its control - and the output directory's
// control is a box with a Choose button beside it, which is a button on the
// same field and not this one.
//
// A switch is the one field whose name is not a label: it carries its words on
// the thing you click, so the row is the switch and its button. Reading only
// labels here reported the switch as having no way to open its explanation,
// which was this guard being wrong rather than the window.
func detailButtonBeside(o fyne.CanvasObject, label string) *parts.DetailButton {
	var found *parts.DetailButton
	walk(o, func(obj fyne.CanvasObject) {
		row, ok := obj.(*fyne.Container)
		if !ok || len(row.Objects) != 2 || namedOnScreen(row.Objects[0]) != label {
			return
		}
		if button, isButton := row.Objects[1].(*parts.DetailButton); isButton {
			found = button
		}
	})
	return found
}

// namedOnScreen is the words a heading shows, whether it is a label above a
// control or a switch carrying its own name.
func namedOnScreen(o fyne.CanvasObject) string {
	switch v := o.(type) {
	case *widget.Label:
		return v.Text
	case *widget.Check:
		return v.Text
	}
	return ""
}

// overlayText is every word currently floating above the screen.
func overlayText(canvas fyne.Canvas) string {
	var out []string
	for _, overlay := range canvas.Overlays().List() {
		walk(overlay, func(obj fyne.CanvasObject) {
			if label, ok := obj.(*widget.Label); ok && label.Text != "" {
				out = append(out, label.Text)
			}
		})
	}
	return strings.Join(out, "\n")
}
