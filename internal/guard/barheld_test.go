package guard

import (
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// TestTheFormDoesNotMoveWhenARunStartsAfterOneThatOfferedSomething is the run
// after a run whose offers stood on a line of their own under the buttons.
//
// The offers are taken away the moment the next run starts, and until
// 2026-11-26 they could not be on a line of their own. Since then they can
// (parts.ActionRow), and their line going with them would move the form at the
// press of Generate - the movement TestTheFormDoesNotMoveWhenARunStarts was
// written against, by a way into it that guard does not take. The batch screen,
// because there the offers do not fit beside the buttons in English, which is
// asserted rather than assumed.
func TestTheFormDoesNotMoveWhenARunStartsAfterOneThatOfferedSomething(t *testing.T) {
	newTestApp().Settings().SetTheme(parts.Theme())
	t.Cleanup(func() { newTestApp() })
	host := newFakeHost(t)
	screen := window.NewRecipe(host)
	hold := newHold()
	screen.HoldBeforeFinishing(hold.enter)
	t.Cleanup(screen.Settled)
	t.Cleanup(hold.free)

	content := screen.Object()
	w := test.NewWindow(content)
	t.Cleanup(w.Close)
	w.Resize(window.LargestOpening)
	settle(content, w)

	chooser, ok := controlUnder(content, text.FieldFormat()).(*parts.Chooser)
	if !ok {
		t.Fatal("the batch screen has no format menu, so this guard read the wrong tree")
	}
	chooser.SetSelected("txt")
	fill(t, content, text.FieldSize(), "1kb")
	fill(t, content, text.FieldOutputDir(), t.TempDir())
	press(t, content, text.ButtonGenerate())
	hold.look(func() {})
	screen.Settled()
	settle(content, w)

	preview, folder := buttonNamed(content, text.ButtonPreview()), buttonNamed(content, text.ButtonOpenFolder())
	if preview == nil || folder == nil || !folder.Visible() {
		t.Fatalf("after the first run the screen offers no %q, so there is no line of offers to lose. It says: %s",
			text.ButtonOpenFolder(), textIn(content))
	}
	drv := fyne.CurrentApp().Driver()
	if drv.AbsolutePositionForObject(folder).Y <= drv.AbsolutePositionForObject(preview).Y {
		t.Fatalf("%q stands on the line of %q, so the offers have no line of their own here and this guard "+
			"asks nothing - a longer offer or a narrower bar is what puts them under it", text.ButtonOpenFolder(), text.ButtonPreview())
	}
	scroll := scrollIn(content)
	if scroll == nil {
		t.Fatal("the batch screen has no scrolling area, so this guard read the wrong tree")
	}
	atRest := scroll.Size().Height
	previewAt := drv.AbsolutePositionForObject(preview)

	hold.again()
	press(t, content, text.ButtonGenerate())
	hold.look(func() {
		settle(content, w)
		if during := scroll.Size().Height; during != atRest {
			t.Errorf("after a run whose offers stood under the buttons the form has %.0f px, and once the next run "+
				"starts it has %.0f px - it moves %.0f px under the pointer that pressed Generate.\n"+
				"What to do: the run holds the row of buttons at its height until it is over (parts.HoldRows, "+
				"busy.keepRowHeight), before the offers are taken away.", atRest, during, during-atRest)
		}
		if at := drv.AbsolutePositionForObject(preview); at != previewAt {
			t.Errorf("%q stood at %v and moved to %v when the next run started", text.ButtonPreview(), previewAt, at)
		}
	})
	screen.Settled()
}

// TestCopyCommandLeavesTheWindowFreeWhileItLooksForTfg holds the search for tfg
// behind Copy command and asks that the press has already returned.
//
// The search looks at every folder on PATH, and a folder on a network host that
// does not answer took 21.0 s on 2026-11-26 (docs/REVIEW-264-2026-11-25.md,
// remark 4) - on the thread that draws the window, which stood still for all of
// it. A press that waits for the search is caught here as a press that has not
// returned while the search is held.
func TestCopyCommandLeavesTheWindowFreeWhileItLooksForTfg(t *testing.T) {
	host := newFakeHost(t)
	gen := window.NewGenerate(host)
	hold := newHold()
	gen.HoldBeforeFinishing(hold.enter)
	t.Cleanup(gen.Settled)
	t.Cleanup(hold.free)
	content := gen.Object()
	entryUnder(t, content, text.FieldOutputDir()).SetText(t.TempDir())

	copyBtn := buttonNamed(content, text.ButtonCopyCommand())
	if copyBtn == nil {
		t.Fatalf("the Single batch screen has no %q button", text.ButtonCopyCommand())
	}
	pressed := make(chan struct{})
	go func() {
		copyBtn.OnTapped()
		close(pressed)
	}()
	select {
	case <-pressed:
	case <-time.After(10 * time.Second):
		t.Fatal("Copy command has not returned while the search for tfg is held - the press waits for the " +
			"search, so a folder on PATH that does not answer stops the window for as long as it takes")
	}
	hold.look(func() {
		if host.copied != "" {
			t.Errorf("the clipboard holds %q while the search for tfg is still held - something other than "+
				"the search wrote it", host.copied)
		}
	})
	gen.Settled()
	if host.copied == "" {
		t.Fatal("the search answered and nothing reached the clipboard, so the press above proves nothing")
	}
}
