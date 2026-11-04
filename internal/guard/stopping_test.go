package guard

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool/checksum"
)

// Stopping work does not hold the window (R06 in
// docs/STABILITY-REVIEW-2026-10-26.md, O262, the owner's decision of
// 2026-11-03).
//
// Cancel, Escape and closing the window all used to cancel and then wait for
// the worker, on the interface thread. The engine asks its context between
// writes, so on a disk that answers that was milliseconds, and on one that
// holds on to a write it was a window that did not draw for as long - five
// seconds of that and Windows offers to end it, which ends the process in the
// middle of a file. Every guard here parks the worker just before it reports,
// which is a write that has not come back as far as the window can tell, and
// asks the press to come back while it is parked.

// Close counts, and tells letGo the first time when a guard is listening.
// Closing during work lets the window go from a goroutine of its own, so a
// guard that has to see it NOT happen while a run is held waits on this rather
// than reading the count at a moment nothing orders. Here rather than beside
// the stand in, whose file is in the band below the ceiling on test files.
func (h *fakeHost) Close() {
	h.closed++
	if h.closed == 1 && h.letGo != nil {
		close(h.letGo)
	}
}

// writersHeld holds a run's writers at their first report of progress until
// the run is asked to stop, for a guard that stops a run and has to have
// stopped it in the middle.
//
// In place of watching the folder for its first entry and stopping the run
// from there, which bet that four hundred files would still be going a moment
// later - an outside review of the pull request named it. Held, the run cannot
// end before it is stopped, and what lets the writers go is the stopping
// itself. See runner.HoldWhileWriting.
type writersHeld struct {
	// reached is buffered for the reason holdDuringRun's is.
	reached chan struct{}
	// abandoned lets the writers go when a guard ended without stopping the
	// run, so a guard that failed early fails rather than waiting out the
	// bound.
	abandoned chan struct{}
	// first makes it one hold: the writers queued behind it, and every later
	// report, pass straight through once it has let go.
	first sync.Once
	// outlasted says the bound ran out with nothing asking the run to stop.
	outlasted atomic.Bool
}

// writersHeldAtMost bounds the hold, so a run that nothing stops is let go and
// the guard says so, rather than the package hanging on its join.
const writersHeldAtMost = 20 * time.Second

// enter is what a writer calls, with the run's context.
func (p *writersHeld) enter(ctx context.Context) {
	p.first.Do(func() {
		p.reached <- struct{}{}
		select {
		case <-ctx.Done():
		case <-p.abandoned:
		case <-time.After(writersHeldAtMost):
			p.outlasted.Store(true)
		}
	})
}

// HoldWhileWriting is the optional interface the window asks for that hold
// through. Nil unless a guard asked for one with writersHeldScreen, so every
// other guard runs exactly as before.
func (h *fakeHost) HoldWhileWriting() func(context.Context) {
	if h.writers == nil {
		return nil
	}
	return h.writers.enter
}

// writersHeldScreen is the one target screen with the writers held.
func writersHeldScreen(t *testing.T) (*fakeHost, fyne.CanvasObject, *writersHeld) {
	t.Helper()
	held := &writersHeld{reached: make(chan struct{}, 1), abandoned: make(chan struct{})}
	host := newFakeHost(t)
	host.writers = held
	window.Open(host)
	if host.content == nil {
		t.Fatal("opening the window put no screen in it")
	}
	t.Cleanup(func() {
		close(held.abandoned)
		join(host)
	})
	return host, tabNamed(t, host.content, text.TabOneTarget()), held
}

// inTheMiddle waits for a writer to be held: a run that has claimed its
// manifest, is writing files and cannot end before it is stopped.
func (p *writersHeld) inTheMiddle(t *testing.T) {
	t.Helper()
	select {
	case <-p.reached:
	case <-time.After(writersHeldAtMost):
		t.Fatal("no file was being written within twenty seconds, so there was never a run to stop")
	}
}

// stoppedByTheGuard says whether it was the guard's press that let the writers
// go, rather than the bound.
func (p *writersHeld) stoppedByTheGuard(t *testing.T) {
	t.Helper()
	if p.outlasted.Load() {
		t.Errorf("nothing asked the run to stop within %v of a file being written, so the writers were let go to finish", writersHeldAtMost)
	}
}

// comesBackWhileHeld presses something that must not wait for the work, with the
// worker parked, and says whether it came back. A press that waits would wait
// for ever - the worker is parked until the guard lets it go - so it is given
// a while and then the worker is let go, which lets the press come back and
// the guard fail rather than hang the package.
func comesBackWhileHeld(hold *holdDuringRun, press func()) bool {
	back := make(chan struct{})
	go func() {
		press()
		close(back)
	}()
	select {
	case <-back:
		return true
	case <-time.After(5 * time.Second):
		hold.free()
		<-back
		return false
	}
}

// Cancel comes back while the run is still going, switches itself off, and
// the line says what is being waited for - until the run's last word replaces
// it.
func TestCancelAnswersAtOnceAndSaysWhatItWaitsFor(t *testing.T) {
	host, content, hold := heldScreen(t)
	fill(t, content, text.FieldOutputDir(), t.TempDir())
	fill(t, content, text.FieldSize(), "1kb")
	fill(t, content, text.FieldCount(), "3")
	press(t, content, text.ButtonGenerate())

	var cancel *parts.Button
	hold.look(func() {
		cancel = buttonNamed(content, text.ButtonCancel())
		if cancel == nil || !cancel.Visible() || cancel.Disabled() {
			t.Fatal("there is no live Cancel while the run is held, so this guard presses nothing")
		}
		if !comesBackWhileHeld(hold, cancel.OnTapped) {
			t.Fatal("Cancel waited for the run, so the window stops drawing for as long as the disk holds on to a write")
		}
		if !cancel.Disabled() {
			t.Error("Cancel is still on after it was pressed, so a second press looks like it would do something")
		}
		if said := textIn(content); !strings.Contains(said, text.Stopping()) {
			t.Errorf("the screen does not say it is stopping, so a press that came back at once looks like a press that did nothing:\n%s", said)
		}
	})
	join(host)

	if said := textIn(content); strings.Contains(said, text.Stopping()) {
		t.Errorf("the run has ended and the screen still says it is stopping:\n%s", said)
	}
	if cancel.Visible() {
		t.Error("the run has ended and Cancel is still on the screen")
	}
}

// The same on the Tools tab, whose Cancel is a button of its own.
func TestCancelOnTheToolsTabAnswersAtOnce(t *testing.T) {
	host, _, hold := heldScreen(t)
	screen := selectTab(t, host.content, text.TabTools())
	chooseTool(t, screen, checksum.ID)
	fillField(t, screen, text.SettingLabel(checksum.InputFile), writeTemp(t, "stopped.txt", "abc"))
	pressNamed(t, screen, text.ButtonRunTool())

	hold.look(func() {
		cancel := buttonNamed(screen, text.ButtonCancel())
		if cancel == nil || !cancel.Visible() || cancel.Disabled() {
			t.Fatal("there is no live Cancel while the tool is held, so this guard presses nothing")
		}
		if !comesBackWhileHeld(hold, cancel.OnTapped) {
			t.Fatal("Cancel on the Tools tab waited for the tool")
		}
		if !cancel.Disabled() {
			t.Error("Cancel on the Tools tab is still on after it was pressed")
		}
		if said := textIn(screen); !strings.Contains(said, text.Stopping()) {
			t.Errorf("the Tools tab does not say it is stopping:\n%s", said)
		}
	})
	join(host)
	if said := textIn(screen); strings.Contains(said, text.Stopping()) {
		t.Errorf("the tool has ended and the Tools tab still says it is stopping:\n%s", said)
	}
}

// Closing the window during a run comes back at once and shows what it waits
// for, the window goes only once the run has stopped, a second press of the
// close button in the meantime does nothing, and the folder is written down as
// a close at rest writes it.
func TestClosingDuringARunKeepsTheWindowDrawingUntilTheRunHasStopped(t *testing.T) {
	host, content, hold := heldScreen(t)
	host.letGo = make(chan struct{})
	dir := t.TempDir()
	fill(t, content, text.FieldOutputDir(), dir)
	fill(t, content, text.FieldSize(), "1kb")
	fill(t, content, text.FieldCount(), "3")
	press(t, content, text.ButtonGenerate())

	hold.look(func() {
		if host.intercept == nil {
			t.Fatal("the window has no close intercept, so closing it does not reach the run at all")
		}
		if !comesBackWhileHeld(hold, host.intercept) {
			t.Fatal("closing the window waited for the run on the interface thread, so the window stops drawing while it waits")
		}
		// The window is let go from a goroutine of its own, so "not yet" is
		// asked over a while rather than read once. A wait that is right
		// never ends while the run is held, and one that does not wait at all
		// lets the window go in microseconds.
		select {
		case <-host.letGo:
			t.Fatal("the window was let go while the run was still going, so the process can end in the middle of a file")
		case <-time.After(500 * time.Millisecond):
		}
		if said := textIn(host.content); !strings.Contains(said, text.SubtitleClosing()) {
			t.Errorf("the closing window does not say what it waits for. It shows:\n%s", said)
		}
		notice := host.content
		if !comesBackWhileHeld(hold, host.intercept) {
			t.Fatal("a second press of the close button waited for the run")
		}
		if host.content != notice {
			t.Error("a second press of the close button began a second wait, so the window would be let go twice")
		}
	})
	join(host)

	if host.closed != 1 {
		t.Errorf("the window was let go %d time(s) and it should be let go exactly once", host.closed)
	}
	if got := host.Remembered().Directory(); got != dir {
		t.Errorf("the folder written down on the way out is %q, not %q", got, dir)
	}
	if info, err := os.Stat(filepath.Join(dir, "manifest.json")); err != nil || info.Size() == 0 {
		t.Errorf("the run that the window waited for left no manifest (%v)", err)
	}
}

// No key starts work while the window waits to close.
//
// A screen whose work has stopped, or never started, has a live Generate - and
// the shortcuts are on the canvas, not on the screens the notice replaced. A
// run started there after the wait had passed that screen would be ended in
// the middle of a file when the window goes. Asked on a screen that is idle
// while another one winds down, which is where a person who closed the window
// and pressed Ctrl+Enter out of habit would be.
func TestNoKeyStartsWorkWhileTheWindowWaitsToClose(t *testing.T) {
	host, content, hold := heldScreen(t)
	fill(t, content, text.FieldOutputDir(), t.TempDir())
	fill(t, content, text.FieldSize(), "1kb")
	fill(t, content, text.FieldCount(), "3")
	press(t, content, text.ButtonGenerate())

	hold.look(func() {
		presets := selectTab(t, host.content, text.TabPresets())
		box := entryUnder(t, presets, text.FieldOutputDir())
		if live := buttonNamed(presets, text.ButtonGenerate()); live == nil || live.Disabled() || !live.Visible() {
			t.Fatal("the preset screen has no live Generate, so a key that reached it would start nothing and this guard asks about nothing")
		}
		if !comesBackWhileHeld(hold, host.intercept) {
			t.Fatal("closing the window waited for the run on the interface thread")
		}
		before := host.settles
		pressInABox(t, box, fyne.KeyReturn, fyne.KeyModifierControl)
		if host.settles != before {
			t.Error("Ctrl+Enter read the preset screen's form while the window waited to close, so a run can start that the window will end in the middle of a file")
		}
	})
}
