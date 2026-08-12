package guard

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"

	"fyne.io/fyne/v2/widget"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// A refusal about one setting appears under that setting's box.
//
// O73, measured on 2026-08-11: asking for 0 files put the sentence about it
// 748 px below the field it named, under every other field on the screen - and
// that distance grows with each field added rather than with the size of the
// window. UX8 asks for the message near where the error came from, and in a
// window that means beside the field.
//
// The guard checks both halves, because moving a message is easy to do badly:
// the sentence has to arrive under the right field AND stop arriving at the
// foot of the form. A run that shows it in both places looks fixed in a
// screenshot of the top of the screen.
//
// It is the engine that says which setting a refusal is about, in recipe keys.
// The alternative was matching the wording in the window, which is a second
// copy of rules the engine owns - and the copy that drifts.
func TestARefusalAboutOneSettingAppearsUnderIt(t *testing.T) {
	_, content := screen(t)
	fill(t, content, text.FieldCount, "0")
	press(t, content, "Preview")

	const refusal = "asks for 0 files"

	field := fieldBox(content, text.FieldCount)
	if field == nil {
		t.Fatal(`there is no field for the count, so this guard read the wrong tree`)
	}
	if !strings.Contains(allText(field), refusal) {
		t.Errorf("the refusal is not under the field it is about. That field shows:\n%s", allText(field))
	}

	// Everything outside that field must be silent about it. Written as "the
	// rest of the screen" rather than as "the area at the bottom", because a
	// second copy anywhere is the same defect.
	rest := allTextExcept(content, field)
	if strings.Contains(rest, refusal) {
		t.Errorf("the refusal also appears away from its field, so it was added rather than moved:\n%s", rest)
	}
}

// The refusal every format can produce lands under the box that caused it.
//
// A separate test from the count above because it comes from somewhere else
// entirely. The count is refused by the engine, which puts a recipe key on its
// error, and a size below a format's minimum is refused by the format package -
// a layer down, and one that had no way to say which field it was about.
//
// That gap was the defect, found by looking at the refused screen on
// 2026-08-12. Of the four fields on those two rows, the size was the only one
// with nowhere for its answer to land, and it is the one most likely to get an
// answer: every format has a minimum and this is the box that asks for less
// than it. The message appeared at the foot of the form, about 900 px under
// the box, while the message about the count next to it appeared under the
// count.
func TestARefusalAboutTheSizeAppearsUnderIt(t *testing.T) {
	_, content := screen(t)
	fill(t, content, text.FieldSize, "1")
	press(t, content, "Preview")

	// The wording is the format's and this guard does not own it, so it asks
	// for the part that says what happened rather than for the sentence.
	const refusal = "cannot be smaller than"

	field := fieldBox(content, text.FieldSize)
	if field == nil {
		t.Fatal("there is no field for the size, so this guard read the wrong tree")
	}
	if !strings.Contains(allText(field), refusal) {
		t.Errorf("the refusal is not under the size it is about. That field shows:\n%s", allText(field))
	}
	if rest := allTextExcept(content, field); strings.Contains(rest, refusal) {
		t.Errorf("the refusal also appears away from its field, so it was added rather than moved:\n%s", rest)
	}
}

// The same holds on the preset screen, where the fields are the preset's own.
//
// A separate test rather than a loop, because the two screens name their
// settings from different places: the generate screen from the recipe keys the
// engine uses, and this one from the parameters the preset declares. The
// mechanism is shared and the vocabulary is not, so proving one proves nothing
// about the other - and the preset screen was the half left undone when the
// generate screen's refusals were moved on 2026-08-11.
func TestARefusalAboutAPresetSettingAppearsUnderIt(t *testing.T) {
	host, _ := screen(t)
	content := selectTab(t, host.content, text.TabPresets)

	fill(t, content, "limit", "512")
	press(t, content, "Preview")

	const refusal = "cannot build this set"

	field := fieldBox(content, "limit")
	if field == nil {
		t.Fatal(`there is no field called "limit", so this guard read the wrong tree`)
	}
	if !strings.Contains(allText(field), refusal) {
		t.Errorf("the refusal is not under the limit it is about. That field shows:\n%s", allText(field))
	}
	if rest := allTextExcept(content, field); strings.Contains(rest, refusal) {
		t.Errorf("the refusal also appears away from its field, so it was added rather than moved:\n%s", rest)
	}
}

// fieldBox is the container of a labelled field - heading, control, hint and
// the place a refusal about it goes.
//
// It reads the heading through headingOf and steps over heading rows for the
// same reason controlUnder does: a field with a longer explanation carries the
// button that opens it on the same line as its name, so the name is inside a
// row rather than first in the field.
func fieldBox(o fyne.CanvasObject, label string) *fyne.Container {
	var found *fyne.Container
	walk(o, func(obj fyne.CanvasObject) {
		box, ok := obj.(*fyne.Container)
		if !ok || len(box.Objects) < 2 || isDetailButton(box.Objects[1]) {
			return
		}
		if head := headingOf(box.Objects[0]); head != nil && head.Text == label {
			found = box
		}
	})
	return found
}

func allText(o fyne.CanvasObject) string {
	var out []string
	walk(o, func(obj fyne.CanvasObject) {
		if l, ok := obj.(*widget.Label); ok && l.Text != "" {
			out = append(out, l.Text)
		}
	})
	return strings.Join(out, "\n")
}

// allTextExcept is every label on the screen that is not inside skip.
func allTextExcept(o fyne.CanvasObject, skip fyne.CanvasObject) string {
	inside := map[fyne.CanvasObject]bool{}
	walk(skip, func(obj fyne.CanvasObject) { inside[obj] = true })

	var out []string
	walk(o, func(obj fyne.CanvasObject) {
		if inside[obj] {
			return
		}
		if l, ok := obj.(*widget.Label); ok && l.Text != "" {
			out = append(out, l.Text)
		}
	})
	return strings.Join(out, "\n")
}
