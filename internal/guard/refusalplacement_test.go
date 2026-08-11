package guard

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
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
	fill(t, content, "how many", "0")
	press(t, content, "Preview")

	const refusal = "asks for 0 files"

	field := fieldBox(content, "how many")
	if field == nil {
		t.Fatal(`there is no field called "how many", so this guard read the wrong tree`)
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

// The same holds on the preset screen, where the fields are the preset's own.
//
// A separate test rather than a loop, because the two screens name their
// settings from different places: the generate screen from the recipe keys the
// engine uses, and this one from the parameters the preset declares. The
// mechanism is shared and the vocabulary is not, so proving one proves nothing
// about the other - and the preset screen was the half left undone when the
// generate screen's refusals were moved on 2026-08-11.
func TestARefusalAboutAPresetSettingAppearsUnderIt(t *testing.T) {
	host, generate := screen(t)
	press(t, generate, "Presets")
	content := host.content

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
func fieldBox(o fyne.CanvasObject, label string) *fyne.Container {
	var found *fyne.Container
	walk(o, func(obj fyne.CanvasObject) {
		box, ok := obj.(*fyne.Container)
		if !ok || len(box.Objects) < 2 {
			return
		}
		if head, ok := box.Objects[0].(*widget.Label); ok && head.Text == label {
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
