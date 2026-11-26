package guard

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
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

// searchHeld is a hold for the search behind Copy command: the first search to
// reach it waits there until the guard lets it go, and every search that
// reaches it is counted. A second one passes straight through, so a build that
// starts a search per press is counted rather than hung.
type searchHeld struct {
	entered atomic.Int32
	reached chan struct{}
	release chan struct{}
	freed   sync.Once
}

func newSearchHeld() *searchHeld {
	return &searchHeld{reached: make(chan struct{}, 1), release: make(chan struct{})}
}

func (h *searchHeld) enter() {
	if h.entered.Add(1) != 1 {
		return
	}
	h.reached <- struct{}{}
	<-h.release
}

func (h *searchHeld) free() { h.freed.Do(func() { close(h.release) }) }

// copyScreenHeld is the Single batch screen with the search behind its Copy
// command held, and the press that started it already returned - or the guard
// failed saying the press waited.
func copyScreenHeld(t *testing.T) (*window.Generate, *fakeHost, *searchHeld, *parts.Button) {
	t.Helper()
	host := newFakeHost(t)
	gen := window.NewGenerate(host)
	held := newSearchHeld()
	gen.HoldBeforeFinishing(held.enter)
	t.Cleanup(gen.Settled)
	t.Cleanup(held.free)
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
		t.Fatal("Copy command has not returned while the search for tfg is held inside its first look at the " +
			"disk - the press waits for the search, so a folder on PATH that does not answer stops the window " +
			"for as long as it takes")
	}
	select {
	case <-held.reached:
	case <-time.After(10 * time.Second):
		t.Fatal("the press returned and no search for tfg ever reached its first look at the disk")
	}
	return gen, host, held, copyBtn
}

// TestCopyCommandLeavesTheWindowFreeWhileItLooksForTfg holds the search for tfg
// behind Copy command inside its first look at the disk, and asks that the
// press has returned all the same - and that pressing again meanwhile neither
// starts a second search nor loses what was pressed last.
//
// The search looks at every folder on PATH, and a folder on a network host that
// does not answer took 21.0 s on 2026-11-26 (docs/REVIEW-264-2026-11-25.md,
// remark 4) - on the thread that draws the window, which stood still for all of
// it. Held inside the search rather than after it since the review of #266: a
// hold after the search let a search moved back onto the press pass, as long as
// it finished inside the time this guard waits.
func TestCopyCommandLeavesTheWindowFreeWhileItLooksForTfg(t *testing.T) {
	gen, host, held, copyBtn := copyScreenHeld(t)
	fields := gen.Fields()
	setBox(t, fields, "size", "2kb")
	copyBtn.OnTapped()
	setBox(t, fields, "size", "3kb")
	copyBtn.OnTapped()
	if host.copied != "" {
		t.Errorf("the clipboard holds %q while the search for tfg is still held - something other than "+
			"the search wrote it", host.copied)
	}
	held.free()
	gen.Settled()
	if n := held.entered.Load(); n != 1 {
		t.Errorf("three presses while one search was under way started %d searches - each press at a folder "+
			"that does not answer would leave another worker waiting on it", n)
	}
	if !strings.Contains(host.copied, "3kb") {
		t.Fatalf("after the search the clipboard holds %q, which is not the line of the last press, with 3kb", host.copied)
	}
}

// TestARefusedPressCopiesNothingWhileAnEarlierSearchAnswers presses Copy
// command on a form it takes, empties the batch name while the search is held,
// presses again and lets the search go: the refusal is the answer to the
// newest press, so nothing may reach the clipboard. Found by the review of
// #266 - the earlier search answered, copied the line from before the name was
// emptied, and said so under the refusal.
func TestARefusedPressCopiesNothingWhileAnEarlierSearchAnswers(t *testing.T) {
	gen, host, held, copyBtn := copyScreenHeld(t)
	fields := gen.Fields()
	setBox(t, fields, engine.SettingID, "")
	copyBtn.OnTapped()
	if findField(fields, engine.SettingID).Saying() == "" {
		t.Fatal("the emptied batch name was not refused, so there is no refusal for the earlier search to answer over")
	}
	held.free()
	gen.Settled()
	if host.copied != "" {
		t.Errorf("the newest press was refused and the clipboard holds %q, the line of the press before it", host.copied)
	}
	if findField(fields, engine.SettingID).Saying() == "" {
		t.Error("the refusal under the batch name went when the earlier search answered")
	}
}
