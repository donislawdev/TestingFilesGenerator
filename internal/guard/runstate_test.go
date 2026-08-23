package guard

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
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

	// A window short enough that the form certainly does not fit, and the
	// state is ASSERTED rather than assumed.
	//
	// It used to rely on the opening size, and on 2026-08-20 that stopped being
	// enough: the settings a format declares are drawn two to a row now, so the
	// generate form came down to 826 px in 837 px of room and the seed box this
	// scrolls to was already on the screen. Nothing was broken and this guard
	// went red honestly - it had stopped reaching the state it exists for,
	// which is the same shape as observation O118.
	w.Resize(fyne.NewSize(window.OpenSize.Width, 500))
	content.Refresh()
	w.Resize(fyne.NewSize(window.OpenSize.Width, 501))
	content.Refresh()
	if room, form := scroll.Size().Height, scroll.Content.MinSize().Height; form <= room {
		t.Fatalf("the form is %.0f px in %.0f px of room, so nothing has to scroll and this guard "+
			"would pass without asking its question", form, room)
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

// What this defends. A preview does its disk work somewhere other than the
// interface thread, and says the screen is busy while it does.
//
// The preflight a preview goes through asks how much room the disk has and
// whether any of the names it would write are taken. That is somebody else's
// disk, possibly a network share, and nobody can put a number on it. Run on
// the interface thread it is a window that stops drawing - with both buttons
// still looking pressable, because nothing had said the screen was busy.
//
// Two halves, checked two ways, and the split is deliberate:
//
//   - The end state is run for real, below.
//   - The handing-off itself is read out of the source, because racing it
//     would be a flaky guard. To see a preview mid flight the test would have
//     to catch it before it finished, and a preview of three small files
//     finishes in no time - so the assertion would pass or fail on timing.
//     O111 is what a guard like that costs.
func TestAPreviewDoesItsDiskWorkOffTheInterfaceThread(t *testing.T) {
	body := readFile(t, "../gui/window/run.go")

	const signature = "func (r *runner) onPreview("
	from := strings.Index(body, signature)
	if from < 0 {
		t.Fatalf("there is no %s in run.go, so this guard is looking at something that has been renamed", signature)
	}
	fn := body[from:]
	if end := strings.Index(fn, "\n}\n"); end > 0 {
		fn = fn[:end]
	}

	worker := strings.Index(fn, "go func()")
	call := strings.Index(fn, "engine.Run(")
	busy := strings.Index(fn, "r.setBusy(true")
	switch {
	case call < 0:
		t.Fatal("onPreview no longer goes through engine.Run, so a preview has stopped answering the same question the run does")
	case worker < 0:
		t.Error("onPreview calls engine.Run without handing it to a goroutine.\n" +
			"Reason: its preflight reads the disk, so on a large set or a network share the window stops drawing with no bar and no way out.\n" +
			"What to do: cross to a worker and come back through fyne.Do, the way startRun does.")
	case call < worker:
		t.Error("onPreview calls engine.Run before it starts its goroutine, so the disk work is still on the interface thread.")
	}

	// And that it says so. Checked here rather than by running a preview and
	// looking, for the same timing reason: the busy state exists only between
	// the press and the answer, and on three small files that is no time at
	// all. What a run CAN check is that the screen comes back, and that is the
	// guard below.
	switch {
	case busy < 0:
		t.Error("onPreview never puts the screen into its busy state.\n" +
			"Reason: both buttons stay looking pressable while a preview reads the disk, so a second press starts a second one.")
	case busy > worker:
		t.Error("onPreview marks the screen busy only after starting its worker, which is a gap where the preview is running and the screen says it is idle.")
	}
}

// And the end state, run for real: a preview leaves the screen idle and usable.
func TestAPreviewGivesTheScreenBackWhenItIsDone(t *testing.T) {
	dir := t.TempDir()
	host, content := screen(t)

	fill(t, content, text.FieldOutputDir, dir)
	fill(t, content, text.FieldSize, "4kb")
	fill(t, content, text.FieldCount, "3")
	press(t, content, "Preview")
	join(host)

	box := entryUnder(t, content, text.FieldSize)
	if box == nil {
		t.Fatal("there is no size box, so this guard read the wrong tree")
	}
	if box.Disabled() {
		t.Error("the form is still frozen after the preview finished, so the screen never comes back")
	}
	for _, name := range []string{"Preview", "Generate"} {
		if b := buttonNamed(content, name); b == nil || b.Disabled() {
			t.Errorf("%q is still disabled after the preview finished", name)
		}
	}
	// A preview has nothing to cancel - preflight takes no context - so the
	// button that offers a way out must not be sitting there afterwards either.
	if b := buttonNamed(content, "Cancel"); b != nil && b.Visible() {
		t.Error("Cancel is still on the screen after a preview, offering a way out of something that is not happening")
	}
}

// What this defends. A preview that is turned down leaves the screen exactly as
// usable as it found it.
//
// This is the edge the busy state introduced. A preview marks the screen
// occupied and hands the disk work to a worker, and the worker is the only
// thing that hands the screen back - so a refusal that reached the busy state
// without starting a worker would freeze the form for the rest of the session.
// Today the refusal returns first and nothing is frozen, and that ordering is
// the whole of the correctness here.
//
// Nothing else was watching it. The guards for refusals check that the right
// box is marked and the right sentence appears, and every one of them would
// still pass with the form dead underneath.
func TestARefusedPreviewLeavesTheScreenUsable(t *testing.T) {
	for _, c := range []struct{ name, field, value string }{
		// Refused while settling, which is where a bad value is caught.
		{"a size that is not a size", text.FieldSize, "abc"},
		// Refused deeper, by the engine, for a size no format can make.
		{"a size below every minimum", text.FieldSize, "1"},
		// Legal and degenerate: nothing to do at all.
		{"nothing to produce", text.FieldCount, "0"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, content := screen(t)
			fill(t, content, text.FieldOutputDir, t.TempDir())
			fill(t, content, c.field, c.value)
			press(t, content, text.ButtonPreview)

			// No worker was started, so there is nothing to join - and that is
			// exactly why this can freeze: the thing that gives the screen back
			// never runs.
			box := entryUnder(t, content, text.FieldSize)
			if box == nil {
				t.Fatal("there is no size box, so this guard read the wrong tree")
			}
			if box.Disabled() {
				t.Error("the preview was refused and the form is frozen.\n" +
					"Reason: the screen is marked busy before the refusal is known, and only a worker hands it back - so a refused press leaves the form dead for the rest of the session.\n" +
					"What to do: settle and plan before marking the screen busy.")
			}
			for _, name := range []string{text.ButtonPreview, text.ButtonGenerate} {
				if b := buttonNamed(content, name); b == nil || b.Disabled() {
					t.Errorf("%q is disabled after a refused preview, so nothing can be tried again", name)
				}
			}
		})
	}
}
