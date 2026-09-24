package guard

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
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
// It moves the pointer through the canvas rather than calling MouseIn, and
// that was corrected on 2026-09-16. Until then it called button.MouseIn
// directly - a guard that names the hover and does not do it - so it could
// not tell a button the pointer reaches from one it does not, and it could
// not tell a visible screen from a hidden one: the search for a button runs
// over the whole tree and a hidden screen's button answers to a call just as
// well. Through the canvas both questions are asked at once, because a hover
// aimed at a button that is not where the canvas draws it opens nothing.
func TestTheLongerExplanationOpensWhenAsked(t *testing.T) {
	app := test.NewApp()
	defer test.NewApp()
	app.Settings().SetTheme(parts.Theme())

	content, c := laidOutWindow(t)
	screen := tabNamed(t, content, text.TabOneTarget())

	// Every field that has one, rather than a sample. These are the sentences
	// nobody can see until they press something, so an unwired button is
	// invisible in exactly the way the permanent text was not.
	for _, field := range []struct{ label, detail string }{
		// The format field's sentence is the line under its label, which lives
		// behind this button like every longer explanation since 2026-08-24.
		// It had a second sentence until 2026-08-27 and the button outlived it,
		// which is the point: an empty explanation would leave the line with
		// nowhere to be read.
		{text.FieldFormat(), text.HintFormat()},
		{text.FieldSize(), text.DetailSize()},
		{text.FieldTargetID(), text.DetailTargetID()},
		{text.FieldNameTemplate(), text.DetailNameTemplate()},
		{text.FieldOutputDir(), text.DetailOutputDir()},
		{text.FieldSeed(), text.DetailSeed()},
		{text.FieldLabel(), text.DetailLabel()},
	} {
		button := detailButtonBeside(screen, field.label)
		if button == nil {
			t.Errorf("%q has a longer explanation and no button that opens it", field.label)
			continue
		}
		centre := drawnCentre(t, button, field.label)

		// Pointing at it, which is what somebody meeting a small letter i
		// actually does. Reported on 2026-08-12: it opened on a click and
		// people hovered and waited. The pointer arrives from nowhere, the way
		// it does in a window just opened, because that is the arrival that
		// showed nothing on 2026-09-16 (O217) - though what was wrong that day
		// is invisible here and held by canvastold_test.go instead.
		test.MoveMouse(c, centre)
		if shown := allText(screen); !strings.Contains(shown, field.detail) {
			t.Errorf("hovering the button beside %q did not show its explanation.\nWanted: %q\nShown: %q",
				field.label, field.detail, shown)
		}

		// And gone when the pointer leaves. Without this half the guard passes
		// on a tooltip that opens and never closes, which is worse than one
		// that never opens: it covers the field underneath it. Left along the
		// row, onto the field's name, which answers to no pointer.
		test.MoveMouse(c, centre.SubtractXY(button.Size().Width*2, 0))
		if shown := allText(screen); strings.Contains(shown, field.detail) {
			t.Errorf("the explanation for %q stayed on screen after the pointer left", field.label)
		}

		// A press still works, and that is not a nicety. Hovering is not
		// something a keyboard can do, and UX9 says whatever the mouse can
		// reach the keyboard can too - so the tap has to survive the hover
		// being added.
		test.TapCanvas(c, centre)
		if shown := allText(screen); !strings.Contains(shown, field.detail) {
			t.Errorf("pressing the button beside %q did not show its explanation.\nWanted: %q\nShown: %q",
				field.label, field.detail, shown)
		}
		// And a second press takes it away again, because somebody who opened
		// it from a keyboard has no pointer to move off it.
		test.TapCanvas(c, centre)
		if shown := allText(screen); strings.Contains(shown, field.detail) {
			t.Errorf("pressing the button beside %q a second time did not take the explanation away", field.label)
		}
	}
}

// drawnCentre is the middle of a control as the canvas draws it, and a
// refusal when the canvas does not draw it at all.
//
// A position of 0,0 is what the driver answers for anything outside the
// visible tree, and the search that finds these buttons walks hidden screens
// as readily as the shown one. The batch screen carries a Size field with a
// button of its own, so without this a guard can hover a button nobody can
// see, read the sentence it put on a sheet nobody can see, and pass.
func drawnCentre(t *testing.T, o fyne.CanvasObject, label string) fyne.Position {
	t.Helper()
	at := fyne.CurrentApp().Driver().AbsolutePositionForObject(o)
	if at.IsZero() {
		t.Fatalf("the button beside %q is not in the visible tree, so this guard is reading a hidden screen", label)
	}
	return at.Add(fyne.NewPos(o.Size().Width/2, o.Size().Height/2))
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

	content, c := laidOutWindow(t)
	screen := tabNamed(t, content, text.TabOneTarget())

	button := detailButtonBeside(screen, text.FieldSize())
	if button == nil {
		t.Fatal("there is no explanation button beside the size field, so this guard read the wrong tree")
	}
	centre := drawnCentre(t, button, text.FieldSize())
	test.MoveMouse(c, centre)
	defer test.MoveMouse(c, centre.SubtractXY(button.Size().Width*2, 0))

	// It has to be up before its absence anywhere else means anything. A guard
	// that checked for an empty overlay without opening the explanation would
	// pass on a button that does nothing.
	if shown := allText(screen); !strings.Contains(shown, text.DetailSize()) {
		t.Fatal("hovering showed nothing, so this guard is measuring a button that does not work")
	}

	if overlays := c.Overlays().List(); len(overlays) != 0 {
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
//
// It answers the LAST row it meets, from wherever it is asked to look. Asked
// of the whole window that is a row on a hidden screen, at position 0,0 - so
// a caller asks it of one screen and checks the answer is drawn (drawnCentre).
func detailButtonBeside(o fyne.CanvasObject, label string) *parts.DetailButton {
	var found *parts.DetailButton
	walk(o, func(obj fyne.CanvasObject) {
		row, ok := obj.(*fyne.Container)
		if !ok || len(row.Objects) < 2 || nameOfRow(row) != label {
			return
		}
		// Searched rather than taken from position one. A heading row grew a
		// third thing on 2026-08-24 - the star marking a field that has to be
		// filled in - which sits between the name and this button, so every
		// required field carrying an explanation would have answered "there is
		// none". Indexing into this row is what has now had to be corrected
		// three times.
		if button := detailButtonIn(row); button != nil {
			found = button
		}
	})
	return found
}

// nameOfRow is the name a heading row carries: its first thing, or - on the
// line of a box to tick, which is the square and then its name since
// 2026-09-23 (parts.ToggleSaying) - its second.
func nameOfRow(row *fyne.Container) string {
	if _, isToggle := row.Objects[0].(*parts.Toggle); isToggle {
		head, _ := headingOf(row.Objects[1])
		return head
	}
	return namedOnScreen(row.Objects[0])
}

// namedOnScreen is the words a heading shows. A switch's name is a heading in
// the column like every other field's since 2026-09-15, so there is no special
// case for it here any more - it carries no words of its own.
func namedOnScreen(o fyne.CanvasObject) string {
	if words, ok := wordsOf(unringed(o)); ok {
		return words
	}
	return ""
}

// The explanation floats on the card an open list does, not on a panel's
// surface.
//
// Reported by the owner from the running window on 2026-09-21: the tooltips
// are hard to read because of their background. Measured on the shot: the box
// was drawn in the panel colour, and it opens over a panel - so it had no
// edge anywhere, and the sentence lay straight over the form covering the row
// beneath it. Since then it stands on what the list a menu drops down stands
// on, and since 2026-09-24 that is the card the owner chose from three drawn
// side by side (floatsAsACard) - both had been reported as a plain grey block.
func TestTheExplanationFloatsOnTheSurfaceAnOpenListDoes(t *testing.T) {
	app := test.NewApp()
	defer test.NewApp()
	app.Settings().SetTheme(parts.Theme())

	content, c := laidOutWindow(t)
	screen := tabNamed(t, content, text.TabOneTarget())
	button := detailButtonBeside(screen, text.FieldSize())
	if button == nil {
		t.Fatalf("%q has no button that opens its explanation", text.FieldSize())
	}
	test.MoveMouse(c, drawnCentre(t, button, text.FieldSize()))
	box := button.Shown()
	if box == nil {
		t.Fatal("hovering the button put nothing on the sheet, so there is no box to measure")
	}

	floatsAsACard(t, box, "the explanation")
}

// floatsAsACard asks something drawn over the form for the card it stands on:
// a face in the surface of a box to type in, with a box's edge and a panel's
// corner, and a translucent shade that shows below the face. Each rectangle is
// asked for by what it is rather than by its place in the tree, and the edge
// and the shade are what tell it from the form now that the face is a box's
// colour - the owner's choice of 2026-09-24 over a lighter face with neither.
func floatsAsACard(t *testing.T, root fyne.CanvasObject, what string) {
	t.Helper()
	dark := theme.VariantDark
	var face, shade *canvas.Rectangle
	walk(root, func(o fyne.CanvasObject) {
		rect, is := o.(*canvas.Rectangle)
		if !is {
			return
		}
		if sameColour(rect.FillColor, parts.PaletteColour(theme.ColorNameInputBackground, dark)) && rect.StrokeWidth > 0 && face == nil {
			face = rect
		} else if _, _, _, a := rect.FillColor.RGBA(); a > 0 && a < 0xFFFF && shade == nil {
			shade = rect
		}
	})
	if face == nil {
		t.Fatalf("%s draws no face in the surface of a box to type in with an edge round it, so it stands on nothing that floats", what)
	}
	if !sameColour(face.StrokeColor, parts.PaletteColour(theme.ColorNameInputBorder, dark)) {
		t.Errorf("%s's edge is %v and a box's is %v", what, face.StrokeColor, parts.PaletteColour(theme.ColorNameInputBorder, dark))
	}
	if face.CornerRadius != parts.RadiusPanel {
		t.Errorf("%s's corner is %.0f and the card's is %d", what, face.CornerRadius, parts.RadiusPanel)
	}
	if shade == nil {
		t.Errorf("%s casts no shade, so nothing but a thin line says it lies over the form rather than in it", what)
	} else if shade.Position().Y <= face.Position().Y {
		t.Errorf("%s's shade sits at y=%.0f and its face at y=%.0f - a shade that is not below what casts it reads as a smudge",
			what, shade.Position().Y, face.Position().Y)
	}
}
