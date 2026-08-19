package guard

import (
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// The form cannot be edited while a run is going.
//
// The buttons that start a run were already dealt with - two of them looking
// pressable during a run invites a second run into the directory the first is
// still filling - but every box stayed live. Somebody could change the output
// directory while files were going into the old one, and nothing said which run
// that applied to. The answer is almost certainly "none of them", and that is
// precisely the answer a person cannot reach by looking (O106).
//
// It checks a box goes back afterwards as well, because a form frozen and never
// thawed is a worse defect than the one being fixed and would pass a guard that
// only looked at the middle of a run.
func TestTheFormCannotBeEditedWhileARunIsGoing(t *testing.T) {
	dir := t.TempDir()
	_, content := screen(t)

	fill(t, content, text.FieldOutputDir, dir)
	fill(t, content, text.FieldSize, "64kb")
	fill(t, content, text.FieldCount, "400")
	press(t, content, "Generate")

	box := entryUnder(t, content, text.FieldSize)
	if box == nil {
		t.Fatal("there is no size box, so this guard read the wrong tree")
	}
	if !box.Disabled() {
		t.Error("the size box can still be typed into while a run is writing files, and nothing " +
			"on the screen says whether changing it affects the run in progress.")
	}

	cancel := buttonNamed(content, "Cancel")
	if cancel == nil {
		t.Fatalf("there is no Cancel button. The screen has: %v", buttonNames(content))
	}
	cancel.OnTapped()

	if box.Disabled() {
		t.Error("the size box is still frozen after the run stopped, so the form never comes back.")
	}
}

// A refusal brings the box it is about into view, and says so where the button
// is.
//
// Every one of these forms is taller than the window it opens in. A press of
// Generate with a bad value marked the box and put the reason under it, both of
// which could be hundreds of pixels below the bottom edge - so from where the
// person is standing, a button they just pressed did nothing at all, and the
// obvious next move is to press it again (O107).
//
// Three things are asked, because any one of them alone can be true of a screen
// that still looks broken: the form moved, the keyboard landed in the box that
// is wrong, and the foot of the form said the press was turned down. The last
// one matters on its own - it is the only part visible without looking away
// from the button that was just pressed.
func TestARefusalBringsTheBoxItIsAboutIntoView(t *testing.T) {
	content, w := screenInAWindow(t, text.TabOneTarget)

	scroll := scrollIn(content)
	if scroll == nil {
		t.Fatal("this screen has no scrolling area, so this guard read the wrong tree")
	}
	if scroll.Offset.Y != 0 {
		t.Fatalf("the form did not start at the top, so a move cannot be told from where it began")
	}

	// The seed sits in the last section, which is the part off the bottom of
	// this window - the whole case this is about.
	seed := entryUnder(t, content, text.FieldSeed)
	if seed == nil {
		t.Fatal("there is no seed box, so this guard read the wrong tree")
	}
	seed.SetText("not a number")

	press(t, content, "Generate")

	if scroll.Offset.Y <= 0 {
		t.Errorf("the form did not move when the press was refused, so the box that is wrong is "+
			"still wherever it was - %.0f px of form in %.0f px of room.",
			scroll.Content.MinSize().Height, scroll.Size().Height)
	}
	if focused := w.Canvas().Focused(); focused != seed {
		t.Errorf("the keyboard went to %T rather than to the box the refusal is about", focused)
	}
	if shown := textIn(content); !strings.Contains(shown, text.RefusedBeforeWriting) {
		t.Errorf("the foot of the form does not say %q, so a press that was refused looks like a "+
			"press that did nothing.", text.RefusedBeforeWriting)
	}
}
