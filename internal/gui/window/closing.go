package window

import (
	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// Closing the window, which during work is a cancellation and not a kill - G7.
//
// The invariant that the output directory never holds a half written file
// rests on the signal handler in cmd/tfg, and closing a window is not a signal.
// So the window asks every screen to stop and waits for them before it lets
// go, and that has not changed. What changed on 2026-11-03 (R06 in
// docs/STABILITY-REVIEW-2026-10-26.md) is where it waits: on a goroutine of
// its own rather than on the interface thread, with the window saying what it
// is waiting for. Waiting on the interface thread was a window that stopped
// drawing for as long as the disk held on to a write, and Windows offers to end
// a window that has not drawn for five seconds - which is ending the process in
// the middle of a file, the one thing the wait is there to prevent. There is no
// way to close without waiting (the owner's decision of 2026-11-03): leaving
// half a file and no manifest behind is the defect, not a faster way out.

// closing is the window's way out, taken once.
type closing struct {
	host Host
	// screens are asked to stop and waited for - every one rather than the one
	// on show, because a run keeps going while somebody moves to another
	// screen, so the busy one is not necessarily the visible one. Stop does
	// nothing on a screen that is idle.
	screens []interface{ Stop() }
	// busy is the state of the same screens, asked whether there is anything
	// to wait for at all.
	busy []*busy
	// quiet is the window's wait for quiet (tidy.go), called off with the rest.
	quiet interface{ Stop() }

	// working and showing are where the directory to write down comes from:
	// the screen somebody was last on. The three carry it between themselves,
	// so the screen being looked at is the one holding the answer, and showing
	// is a pointer because it changes as somebody moves.
	working map[string]destination
	showing *string
	// looking is the tab the keyboard acts on - see wireKeyboard.
	looking *string

	// begun is whether the way out has been taken, so a second press of the
	// close button while the window waits does nothing.
	begun bool
	// gone is closed once the window has been let go after a wait, for
	// Settled. Nil until a wait begins.
	gone chan struct{}
}

// tabClosing is what is looked at while the window waits to close: the notice,
// which no key acts on.
const tabClosing = "closing"

// closeCleanly puts the way out on the window's close button, for the window
// it was made for.
//
// Handed back as well, because closing the window from inside it - Restart now
// on the Preferences screen - has to go this same way. The toolkit's Close does
// not pass through the intercept (driver/glfw window.go, read on 2026-09-29),
// so calling it alone would skip the runs and the folder.
func closeCleanly(h Host, c *closing) *closing {
	c.host = h
	h.SetCloseIntercept(c.leave)
	return c
}

// leave writes down where the files were going, and lets the window go - at
// once when nothing is working, and once the work has stopped when something
// is.
//
// The directory is written down first, from the screen somebody was on when
// they closed: one moment rather than "whenever it changes", so what comes back
// is describable in a sentence - the window opens where you left it. Writing on
// every keystroke would also put a half typed path on somebody's disk.
//
// While it waits, the window shows a notice in place of the screens and the
// keyboard acts on nothing. Both for the same reason: a screen whose work has
// stopped while another is still winding down is a screen whose Generate works
// again, and a run started there after the wait had passed it would be ended
// in the middle of a file when the window goes.
func (c *closing) leave() {
	if c.begun {
		return
	}
	c.begun = true
	c.quiet.Stop()
	if screen, ok := c.working[*c.showing]; ok {
		c.host.Remembered().RememberDirectory(screen.OutDir())
		c.host.Remembered().RememberNumbersIfTaken(screen.NumbersIfTaken())
	}
	if !anyBusy(c.busy) {
		c.host.Close()
		return
	}
	*c.looking = tabClosing
	c.host.SetContent(parts.Screen(parts.Titled(text.HeadingClosing(), text.SubtitleClosing())))
	gone := make(chan struct{})
	c.gone = gone
	go func() {
		for _, screen := range c.screens {
			screen.Stop()
		}
		fyne.Do(func() { c.host.Close() })
		close(gone)
	}()
}

// Settled waits for a window that is closing to have closed, for a guard - the
// same seam as runner.Settled. Returns at once when no wait has begun.
func (c *closing) Settled() {
	if c.gone != nil {
		<-c.gone
	}
}
