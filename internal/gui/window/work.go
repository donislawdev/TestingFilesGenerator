package window

import (
	"context"
	"sync"
)

// One piece of work beside the window - a run, a preview, a tool - and the
// three ways the window ends it or waits for it.
//
// Out of the runner and the Tools tab on 2026-11-03, when Cancel stopped
// waiting (R06 in docs/STABILITY-REVIEW-2026-10-26.md, O262). Until then each
// held a stop that cancelled and then waited, and Cancel, Escape and closing
// the window all called it on the interface thread. The engine asks its context
// between writes, so on a disk that answers the wait was milliseconds - and on
// one that holds on to a write it was as long as the disk took, with a window
// that did not draw. Windows offers to end a window that has not drawn for five
// seconds, and ending the process in the middle of a file is the one thing G7
// is there to prevent.

// work is what a screen keeps of the work it started.
//
// Kept after the work has ended rather than cleared, for the reason the runner
// gave its stop before this: clearing it would be a write from the worker, and
// a handle that is safe to hold on to is the honest way out. Stopping work
// that has ended cancels a context that is done and receives from a closed
// channel, and both return at once.
type work struct {
	cancel context.CancelFunc
	// done is closed by the worker once it has handed its last word to the
	// interface thread - see end.
	done chan struct{}

	// talking is held by whoever changes what the screen says about this work:
	// a press of Cancel, and each piece of news the worker sends back.
	//
	// In the shipped program both happen on the interface thread, so it is
	// never contended. It is here for the test driver, where fyne.Do runs the
	// worker's news on the worker (O124) - there a press of Cancel and a line of
	// progress are two goroutines writing one status line, and the race
	// detector is right to say so. HoldBeforeFinishing cannot order the two,
	// because the progress comes from the writers part way through a run, and
	// a lock can: it keeps a guard free to press Cancel at any moment of a run.
	talking sync.Mutex
	// stopping is whether Cancel has been pressed. Only under talking.
	stopping bool
}

// begin starts a piece of work: the context it runs under, and its handle.
func begin() (context.Context, *work) {
	ctx, cancel := context.WithCancel(context.Background())
	return ctx, &work{cancel: cancel, done: make(chan struct{})}
}

// halt asks the work to stop and returns at once - Cancel, and Escape through
// it.
//
// say is what the screen says about it, run once under the same lock as the
// worker's news. Nothing asks whether the work has already ended: Cancel is
// hidden and switched off by the worker's last word, and Escape presses Cancel
// only while it is live, so no press reaches a finished piece of work.
func (w *work) halt(say func()) {
	if w == nil {
		return
	}
	w.talking.Lock()
	defer w.talking.Unlock()
	if w.stopping {
		return
	}
	w.stopping = true
	w.cancel()
	say()
}

// stop asks the work to stop and waits until it has - for closing the window,
// which does it on a goroutine of its own (see closing.go).
func (w *work) stop() {
	if w == nil {
		return
	}
	w.cancel()
	<-w.done
}

// settled waits for the work without asking it to stop - see runner.Settled.
func (w *work) settled() {
	if w == nil {
		return
	}
	<-w.done
}

// news is something the worker says part way through - the bar and the line
// of progress - and it runs on the interface thread. Dropped once Cancel has
// been pressed: a line of progress written over "Stopping" would say the press
// did nothing. In the shipped program fyne.Do queues the news, so a line
// queued just before the press arrives just after it.
func (w *work) news(say func()) {
	w.talking.Lock()
	defer w.talking.Unlock()
	if !w.stopping {
		say()
	}
}

// end is the worker's last word, on the interface thread, said whatever was
// pressed. The worker closes done after handing this to fyne.Do and never
// before, so whoever waits on done finds the last word at least queued.
func (w *work) end(say func()) {
	w.talking.Lock()
	defer w.talking.Unlock()
	say()
}
