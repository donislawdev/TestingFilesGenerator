package guard

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// The box a refusal is about is marked, and only that box.
//
// Asked for from the screen on 2026-08-12: a value the run will not take should
// look wrong where it was typed. The sentence had moved under the right field
// the day before - O73 - and the field itself still looked like every other one,
// so on a form of eight settings the reader has to find the paragraph before
// they can find the box.
//
// Two marks rather than one, and that is not belt and braces. A colour on its
// own says nothing to a reader who cannot tell it from the others, and a
// sentence on its own leaves them counting boxes. Neither replaces the other,
// which is why this guard checks the edge and the refusal guards check the
// words.
//
// It asks what the edge IS rather than whether one exists. A rectangle that is
// present and drawn in the background colour passes any check for presence, and
// this project has already paid for that lesson twice - a guard that opens
// something and reads it cannot see a loop, and a colour measured without the
// thing behind it is a colour nobody sees.
func TestTheBoxARefusalIsAboutIsMarked(t *testing.T) {
	_, content := screen(t)
	fill(t, content, text.FieldSize, "1")
	press(t, content, "Preview")

	marked := edgeOf(t, content, text.FieldSize)
	if marked.StrokeWidth <= 0 {
		t.Error("the size was refused and its box draws no edge, so the only sign is a paragraph further down")
	}
	if want := parts.PaletteColour(theme.ColorNameError, theme.VariantDark); !sameColour(marked.StrokeColor, want) {
		t.Errorf("the edge round the refused box is %v rather than the error colour %v", marked.StrokeColor, want)
	}

	// The field beside it was not refused and must look untouched. Without this
	// the guard passes for a window that marks everything, which is a window
	// that marks nothing.
	if quiet := edgeOf(t, content, text.FieldCount); quiet.StrokeWidth != 0 {
		t.Errorf("the count was not refused and its box draws an edge %.1f px wide", quiet.StrokeWidth)
	}
}

// The mark goes when the reason for it goes.
//
// The half that is easy to leave out, because nothing on the screen looks wrong
// while it is missing: a red edge left behind after the value was fixed
// describes a state that is no longer true, and the next run is refused by a
// screen that was already showing a refusal.
func TestTheMarkGoesWhenTheValueIsFixed(t *testing.T) {
	host, content := screen(t)
	fill(t, content, text.FieldSize, "1")
	press(t, content, "Preview")
	if edgeOf(t, content, text.FieldSize).StrokeWidth <= 0 {
		t.Fatal("the refused box was never marked, so this guard cannot tell whether the mark goes")
	}

	fill(t, content, text.FieldSize, "1mb")
	press(t, content, "Preview")
	// This press is the one that is ACCEPTED, so unlike the one above it starts
	// a worker. Joined before the tree is read - otherwise this goroutine and
	// that one are both in the font shaper, and the panic lands in whichever
	// test happens to be running when it goes off.
	join(host)
	if got := edgeOf(t, content, text.FieldSize).StrokeWidth; got != 0 {
		t.Errorf("the size is acceptable now and its box still draws a %.1f px edge", got)
	}
}

// The same on the preset screen, where the settings are the preset's own.
//
// A separate test rather than a loop over both, for the reason the refusal
// guards give: the two screens name their settings from different places, so
// proving one proves nothing about the other. The preset screen was the half
// left undone the last time a refusal moved.
func TestTheBoxARefusalIsAboutIsMarkedOnThePresetScreenToo(t *testing.T) {
	_, content := presetScreen(t)
	fill(t, content, text.SettingLabel("limit"), "512")
	press(t, content, "Preview")

	marked := edgeOf(t, content, text.SettingLabel("limit"))
	if marked.StrokeWidth <= 0 {
		t.Error("the limit was refused and its box draws no edge")
	}
	if want := parts.PaletteColour(theme.ColorNameError, theme.VariantDark); !sameColour(marked.StrokeColor, want) {
		t.Errorf("the edge round the refused limit is %v rather than the error colour %v", marked.StrokeColor, want)
	}
}

// A menu says where the keyboard is with a line, because it cannot say it with
// a fill.
//
// The measurement is in parts.Ring and it is the reason this guard exists at
// all: widget.Select paints its whole background with the focus colour and
// writes the chosen value across it, so the colour has to be seen at 3.0
// against an untouched box and read through at 4.5, and no colour is both.
// The window shipped the visible half - a chosen format sat on a solid
// #4C9DF0 with its name at 2.28:1 - which is what was reported as a field that
// stays lit after the choice.
//
// So the fill became a wash and the state moved to a line. This asks about the
// line, and the palette guard asks about the wash. Neither is worth anything
// without the other: a line nobody draws leaves the keyboard invisible, and a
// wash nobody can read through is where this started.
func TestTheMenuTheKeyboardIsInDrawsALine(t *testing.T) {
	_, content := screen(t)
	picker, ok := controlUnder(content, text.FieldFormat).(*parts.Chooser)
	if !ok {
		t.Fatalf("the format field is %T rather than a menu", controlUnder(content, text.FieldFormat))
	}

	quiet := edgeOf(t, content, text.FieldFormat)
	if quiet.StrokeWidth != 0 {
		t.Errorf("a menu nobody is using draws a %.1f px edge", quiet.StrokeWidth)
	}

	picker.FocusGained()
	lit := edgeOf(t, content, text.FieldFormat)
	if lit.StrokeWidth <= 0 {
		t.Error("the keyboard is in the format menu and nothing on the screen says so")
	}
	if want := parts.PaletteColour(theme.ColorNamePrimary, theme.VariantDark); !sameColour(lit.StrokeColor, want) {
		t.Errorf("the line round the focused menu is %v rather than the primary colour %v", lit.StrokeColor, want)
	}

	picker.FocusLost()
	if got := edgeOf(t, content, text.FieldFormat).StrokeWidth; got != 0 {
		t.Errorf("the keyboard has left the format menu and it still draws a %.1f px edge", got)
	}
}

// A refusal outranks the keyboard, because one of the two stops the run.
func TestARefusedBoxStaysRedWhileTheKeyboardIsInIt(t *testing.T) {
	_, content := presetScreen(t)
	fill(t, content, text.SettingLabel("limit"), "512")
	press(t, content, "Preview")

	picker, ok := controlUnder(content, text.SettingLabel("format")).(*parts.Chooser)
	if !ok {
		t.Fatalf("the format setting of the preset is %T rather than a menu", controlUnder(content, text.SettingLabel("format")))
	}
	// The keyboard arrives somewhere else entirely, so this is about the box
	// that was refused rather than about the one being used.
	picker.FocusGained()
	defer picker.FocusLost()

	marked := edgeOf(t, content, text.SettingLabel("limit"))
	if want := parts.PaletteColour(theme.ColorNameError, theme.VariantDark); !sameColour(marked.StrokeColor, want) {
		t.Errorf("the refused limit is drawn %v rather than in the error colour", marked.StrokeColor)
	}
}

// edgeOf is the line round the control of one labelled field.
//
// It looks inside the field rather than at the control, because a control that
// is a box with a button beside it - the output directory - puts the edge round
// the pair. There is exactly one in a field and this fails loudly rather than
// answering with the first of several, since a second edge would mean two marks
// on one box and only one of them being cleared.
func edgeOf(t *testing.T, o fyne.CanvasObject, label string) *canvas.Rectangle {
	t.Helper()
	field := fieldBox(o, label)
	if field == nil {
		t.Fatalf("there is no field labelled %q, so this guard read the wrong tree", label)
	}

	var found []*canvas.Rectangle
	walk(field, func(obj fyne.CanvasObject) {
		if rect, ok := obj.(*canvas.Rectangle); ok {
			found = append(found, rect)
		}
	})
	switch len(found) {
	case 0:
		t.Fatalf("the field %q has no edge to draw, so nothing can mark it", label)
	case 1:
	default:
		t.Fatalf("the field %q has %d edges, and a box with two marks has one that never clears", label, len(found))
	}
	return found[0]
}

// sameColour compares what is drawn rather than how it was written. A palette
// value and a colour read back off a widget are the same colour in two types.
func sameColour(a, b color.Color) bool {
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}
