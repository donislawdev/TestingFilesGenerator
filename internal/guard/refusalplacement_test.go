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
	content, w := screenInAWindow(t, text.TabOneTarget)
	fill(t, content, text.FieldCount, "0")
	press(t, content, "Preview")
	settle(content, w)

	const refusal = "asks for 0 files"

	// Everything outside the block the field sits in must be silent about it.
	// Written as "the rest of the screen" rather than as "the area at the
	// bottom", because a second copy anywhere is the same defect.
	sawItOnce(t, content, text.FieldCount, refusal)
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
	content, w := screenInAWindow(t, text.TabOneTarget)
	fill(t, content, text.FieldSize, "1")
	press(t, content, "Preview")
	settle(content, w)

	// The wording is the format's and this guard does not own it, so it asks
	// for the part that says what happened rather than for the sentence.
	const refusal = "cannot be smaller than"

	sawItOnce(t, content, text.FieldSize, refusal)
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
	content, w := screenInAWindow(t, text.TabPresets)

	fill(t, content, "limit", "512")
	press(t, content, "Preview")
	settle(content, w)

	const refusal = "cannot build this set"

	sawItOnce(t, content, "limit", refusal)
}

// sawItOnce is the whole of what these three guards ask: the message is beside
// the box it is about, and it is nowhere else.
//
// Measured in pixels rather than by looking inside the field's own subtree, and
// that changed on 2026-08-20 with the layout. Two fields share a row, so a
// message confined to one column of it had half the form to say four parts in -
// four wrapped lines with the other half of the panel empty. The controls share
// the row now and the messages are laid out under it across the full width,
// which means a message is no longer a descendant of its field.
//
// The distance is what UX8 asks for anyway. The defect this was written against
// - O73, measured 2026-08-11 - was a sentence 748 px below the box it named,
// under every other field, growing with each field added. A guard on the
// distance catches that wherever the message is parented. The subtree version
// would have gone red for a message moved two pixels into a sibling and stayed
// green for one moved to the bottom of a short screen.
//
// The distance allowed is a control, the hint under it and the gaps around
// them, measured at 78 px on the preset screen where the limit carries a hint
// and no row. A hundred and ten is that with room to spare and two orders away
// from the 748 px this was written against. What tells one field from another
// at close range is not this number - it is the red edge on the box, which is
// the other half of every refusal and has its own guard.
func sawItOnce(t *testing.T, content fyne.CanvasObject, label, refusal string) {
	t.Helper()

	control := controlUnder(content, label)
	if control != nil {
		if box, ok := objectBox(content, control); ok && box.Height == 0 {
			t.Fatal("nothing on this screen has been laid out, so every distance here would be zero " +
				"and this guard would pass against anything - it needs a screen in a window")
		}
	}
	if control == nil {
		t.Fatalf("there is no field called %q, so this guard read the wrong tree", label)
	}
	box, ok := objectBox(content, control)
	if !ok {
		t.Fatalf("the field called %q is not laid out", label)
	}

	said := refusalLabelSaying(content, refusal)
	if said == nil {
		t.Fatalf("nothing on the screen says %q. It says:\n%s", refusal, allText(content))
	}
	where, ok := objectBox(content, said)
	if !ok {
		t.Fatalf("the refusal about %q is not laid out", label)
	}

	const near = 110
	if gap := where.Y - (box.Y + box.Height); gap < 0 || gap > near {
		t.Errorf("the refusal about %q is %.0f px from the box it names, and anything past %d px is under "+
			"somebody else's field. It reads: %q", label, gap, near, said.Text)
	}

	// Once. A run that shows the message in two places looks fixed in a
	// screenshot of the top of the screen.
	seen := 0
	walk(content, func(obj fyne.CanvasObject) {
		if l, is := obj.(*widget.Label); is && strings.Contains(l.Text, refusal) {
			seen++
		}
	})
	if seen != 1 {
		t.Errorf("%q is on the screen %d times, so it was added rather than moved", refusal, seen)
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
