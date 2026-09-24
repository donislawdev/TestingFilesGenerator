package window

import (
	"runtime/debug"
	"time"
)

// Giving memory back to the system once the window has been left alone.
//
// After a spell of work - batches added and taken away, a base preset switched,
// a run - the process kept 189-206 MB for as long as the window stood idle,
// measured over five minutes on 2026-09-23 (docs/GUI-MEMORY-2026-09-23.md
// section 4j). Two things hold it, and neither is ours to change:
//
//   - The toolkit keeps the renderers of objects that have left the screen for
//     a minute, and destroys them only while drawing a frame (Fyne 2.8.1,
//     internal/cache/base.go Clean). A window nobody touches draws no frames,
//     so they stay.
//   - Go gives the pages a collection freed back to the system slowly, and runs
//     a collection in the quiet only every two minutes.
//
// So after tidyAfterQuiet of nothing, one frame is asked for - the root of the
// canvas marked for redrawing, which touches no widget and moves nothing: the
// scroll, the keyboard, what every box, menu and switch holds were compared
// either side and were the same. tidyAfterFrame later the memory is given
// back. Measured: 252-263 MB to 117-119 MB, in four runs of four.
//
// Given back beside the window rather than on its thread, and that is the
// owner's decision of 2026-09-24: one of ten calls of FreeOSMemory measured on
// the window's thread took 2491 ms against 4.6-16.2 ms for the rest, for a
// reason nobody found, and it would land on somebody coming back to the
// window. So this file starts a goroutine, and it is declared as concurrent
// for that reason. The goroutine touches nothing of ours.
//
// What it does not do: a minimised window draws no frames either, so there the
// renderers stay and only what Go holds goes back.

// tidyAfterQuiet is how long nothing has to happen. Past the minute the
// toolkit keeps a renderer for (Fyne cache.ValidDuration), so the ones that
// left the screen with the last change have expired by then.
const tidyAfterQuiet = 70 * time.Second

// tidyAfterFrame is how long after the frame the memory goes back. The toolkit
// cleans at most once in ten seconds and a clean that comes sooner waits for
// the next, so two seconds - what the first measurement used - would sometimes
// give back before the renderers were gone.
const tidyAfterFrame = 12 * time.Second

// tidyWhenLeftAlone gives the window its one wait for quiet and has every
// screen tell it at every reading of the form - which every change and every
// run makes.
func tidyWhenLeftAlone(h Host, screens ...*runner) *tidy {
	t := newTidy(h, func() bool { return anyBusy(screens) })
	for _, r := range screens {
		r.touched = t.touch
	}
	return t
}

// anyBusy says whether work owns any of these screens.
func anyBusy(screens []*runner) bool {
	for _, r := range screens {
		if r.busy.occupied {
			return true
		}
	}
	return false
}

// tidy is the one quiet period being waited out, for the whole window.
type tidy struct {
	host Host
	// busy says whether any screen has work going. Memory is not given back
	// under a run - the run is what is using it - and the wait starts again,
	// so a run longer than the wait is followed by a release once it ends.
	busy func() bool
	// later is the host's clock, or a clock of its own a guard hands in - see
	// newTidy.
	later   later
	callOff func()
}

// newTidy builds the wait for one window.
//
// On the host's clock unless the host has a separate one for this. A guard's
// host holds ONE pending request - the busy face's, which several guards hold
// and fire - and a quiet period asked for on every key would take its place.
// So the guards' host keeps these apart, and the program's host, whose clock
// is a timer per request, needs nothing of the kind.
func newTidy(h Host, busy func() bool) *tidy {
	t := &tidy{host: h, busy: busy, later: h.Later}
	if q, ok := h.(interface {
		QuietLater(after time.Duration, then func()) func()
	}); ok {
		t.later = q.QuietLater
	}
	return t
}

// touch is told that something happened, and starts the quiet over.
func (t *tidy) touch() {
	t.Stop()
	t.callOff = t.later(tidyAfterQuiet, t.frame)
}

// Stop calls off whatever is being waited for. Closing the window stops it,
// along with every screen.
func (t *tidy) Stop() {
	if t.callOff != nil {
		t.callOff()
		t.callOff = nil
	}
}

// frame asks the toolkit to draw, so that it lets go of what has expired. Even
// under a run, which costs one frame and nothing else - whether to give memory
// back is asked once, in release.
func (t *tidy) frame() {
	t.callOff = nil
	if c := t.host.Canvas(); c != nil && c.Content() != nil {
		c.Refresh(c.Content())
	}
	t.callOff = t.later(tidyAfterFrame, t.release)
}

// release gives the memory back, beside the window.
func (t *tidy) release() {
	t.callOff = nil
	if t.busy() {
		t.touch()
		return
	}
	if r, ok := t.host.(interface{ ReleasingMemory() }); ok {
		r.ReleasingMemory()
	}
	go debug.FreeOSMemory()
}
