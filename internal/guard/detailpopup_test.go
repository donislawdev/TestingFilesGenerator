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
		if shown := allText(content); !strings.Contains(shown, field.detail) {
			t.Errorf("hovering the button beside %q did not show its explanation.\nWanted: %q\nShown: %q",
				field.label, field.detail, shown)
		}

		// And gone when the pointer leaves. Without this half the guard passes
		// on a tooltip that opens and never closes, which is worse than one
		// that never opens: it covers the field underneath it.
		button.MouseOut()
		if shown := allText(content); strings.Contains(shown, field.detail) {
			t.Errorf("the explanation for %q stayed on screen after the pointer left", field.label)
		}

		// A click still works, and that is not a nicety. Hovering is not
		// something a keyboard can do, and UX9 says whatever the mouse can
		// reach the keyboard can too - so the tap has to survive the hover
		// being added.
		button.OnTapped()
		if shown := allText(content); !strings.Contains(shown, field.detail) {
			t.Errorf("pressing the button beside %q did not show its explanation.\nWanted: %q\nShown: %q",
				field.label, field.detail, shown)
		}
		// And a second press takes it away again, because somebody who opened
		// it from a keyboard has no pointer to move off it.
		button.OnTapped()
		if shown := allText(content); strings.Contains(shown, field.detail) {
			t.Errorf("pressing the button beside %q a second time did not take the explanation away", field.label)
		}
		button.MouseOut()
	}
}

// The explanation never uses the overlay layer.
//
// This is the flicker, written as the rule that actually governs it, and it
// took two goes to find. Reported on 2026-08-12: the explanation strobed as
// soon as the pointer moved on the icon, then after the first fix it vanished
// instead. Both are the same cause.
//
// internal/driver/util.go, FindObjectAtPositionMatching: when an overlay is
// present the driver walks THE OVERLAY AND NOTHING ELSE looking for whatever
// is under the pointer. It replaces the roots rather than being tried before
// them. So while any overlay is up the button cannot be found at all - it is
// told the pointer left, it closes the explanation, the overlay goes, and it
// is found again.
//
// The first fix put a container answering nothing into the overlay, which
// removed the competing hover target and left the real problem untouched: an
// overlay that matches nothing still stops the search from reaching the
// button. That is why this guard asks about the LAYER rather than about what
// is on it. "Nothing up there is hoverable" was true of the broken version.
//
// The guard beside this one cannot see any of it. It opens the explanation and
// reads it, which is exactly what happens on the first frame of the loop - so
// it stayed green through both versions, and both were reported by somebody
// moving a mouse.
func TestTheExplanationNeverUsesTheOverlayLayer(t *testing.T) {
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

	// It has to be up before its absence anywhere else means anything. A guard
	// that checked for an empty overlay without opening the explanation would
	// pass on a button that does nothing.
	if shown := allText(content); !strings.Contains(shown, text.DetailSize) {
		t.Fatal("hovering showed nothing, so this guard is measuring a button that does not work")
	}

	if overlays := w.Canvas().Overlays().List(); len(overlays) != 0 {
		t.Errorf("the explanation put %d thing(s) in the overlay layer, the first a %T.\n"+
			"While an overlay is up the driver looks for the pointer in the overlay and nowhere else, "+
			"so the button underneath cannot be found - it is told the pointer left and closes the "+
			"explanation, which is the flicker.", len(overlays), overlays[0])
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
	switch v := unringed(o).(type) {
	case *widget.Label:
		return v.Text
	case *widget.Check:
		return v.Text
	}
	return ""
}
