package window

import (
	"time"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// The screen while a run or a preview owns it, and the face it wears for that.
//
// Split out of the runner on 2026-09-21, when the face stopped arriving at
// once. The runner stands at its ceiling of fields and of methods, so the
// state and the four controls that show it moved here together rather than
// a timer being squeezed in beside them.

// BusyFaceAfter is how long a run or a preview may take before the screen
// shows that it is busy - the form frozen, Cancel offered, the bar for a run.
//
// A delay rather than at once, on the owner's report from the running window
// on 2026-09-21 that the whole window shook when Preview was pressed.
// Measured on that window: a preview of one file is done in about 50 ms, and
// in that time the screen froze every box - the toolkit draws a frozen box
// with a bright edge and grey words - put a Cancel button into the row, which
// moved Preview and Generate half a button to the left, showed a bar
// standing at nought, and then took all of it back. A flash, not a state.
//
// Work that is over before anybody could have read the busy face never
// needs one, and work that lasts gets the face in time to matter. The number
// is the one a desktop waits before it changes the pointer to an hourglass
// and the web before it shows a spinner: long enough that a fast answer
// shows nothing at all, short enough that a slow one is not mistaken for a
// button that did nothing.
const BusyFaceAfter = 300 * time.Millisecond

// later is what a host offers for running something on the interface thread
// after a while: it hands back the way to call it off. See Host.Later.
type later func(after time.Duration, then func()) (callOff func())

// busy is whether a run or a preview owns the screen, and what the screen
// wears while one does.
//
// The state and the face are two things and they come apart on purpose. The
// state is set the moment work starts, so that a second press of Generate
// during the first moment of the first is refused - see runner.onGenerate.
// The face follows after BusyFaceAfter, if the work is still going, and comes
// off at once when it ends.
type busy struct {
	// occupied is the state: whether work owns the screen. It exists because
	// the runner's stop cannot answer that - stop is set on the first press
	// and never cleared. Only ever touched on the interface thread, which is
	// what makes a plain bool enough.
	occupied bool
	// worn is whether the face is on, so it is taken off exactly when it
	// was put on and never twice.
	worn bool

	fields   *parts.Fields
	preview  *parts.Button
	generate *parts.Button
	cancel   *parts.Button
	bar      *parts.Progress
	// also are controls that are neither fields nor run buttons and still
	// have no business being pressed while work is going. The batch screen's
	// "add a batch" is one: pressing it rebuilds the form under a run.
	also []fyne.Disableable
	// row is the row the buttons stand in, laid out again whenever one of
	// them comes or goes - see relay.
	row *fyne.Container

	later   later
	callOff func()
	// epoch counts the pieces of work that have owned the screen, so that a
	// face asked for by one of them can never dress the next. Calling the
	// clock off is not enough: the real window's clock hands the face to
	// the toolkit's queue, and a face already queued when the work ends
	// still runs - after the next work has started, if the next press comes
	// in that gap. An outside review of the pull request named it.
	epoch int
}

// busyFace is which parts of the face a piece of work earns: whether there
// is anything to stop, and whether there is progress to show. A preview can
// be stopped since 2026-08-26 and reports no progress - a dry run returns
// before the writing loop, so a bar for it would stand at nought for as long
// as the preview took, which is what a stuck run looks like.
type busyFace struct{ stoppable, progressing bool }

// set puts the screen into the state, and puts the face on or off. The face
// goes on later - BusyFaceAfter from now, on the interface thread, if the
// work is still going - and comes off at once.
func (b *busy) set(occupied bool, face busyFace) {
	b.occupied = occupied
	if b.callOff != nil {
		b.callOff()
		b.callOff = nil
	}
	if !occupied {
		b.undress()
		return
	}
	b.epoch++
	mine := b.epoch
	b.callOff = b.later(BusyFaceAfter, func() {
		// The same work still going, and not already worn: a face asked for
		// by earlier work is a face for a screen that has moved on, and a
		// face put on twice would be taken off once.
		if b.epoch == mine && b.occupied && !b.worn {
			b.wear(face)
		}
	})
}

// wear is the face: the form frozen, the buttons that start work off, and
// whatever the work earned on top.
//
// The form goes with the buttons. It stayed editable through a run, so
// somebody could change the output directory while files were going into
// the old one and nothing said which run that applied to - almost certainly
// none of them, which is exactly the answer a person cannot reach from
// looking (O106).
func (b *busy) wear(face busyFace) {
	b.worn = true
	b.fields.Freeze(true)
	for _, control := range b.also {
		control.Disable()
	}
	b.preview.Disable()
	b.generate.Disable()
	// Cancel is hidden rather than greyed when there is nothing to cancel,
	// asked for on 2026-08-11 after looking at the window. A permanently dead
	// control is a question the screen keeps asking and answering itself, and
	// it sat beside the two buttons that do work - so the row read as three
	// choices where there were two. It appears with the work and goes with
	// it.
	if face.stoppable {
		b.cancel.Enable()
		b.cancel.Show()
	}
	if face.progressing {
		b.bar.Show()
	}
	b.relay()
}

// undress takes the face off, whether or not it was ever put on: the
// buttons are enabled and the form thawed either way, because a face that
// never arrived costs nothing to take off and a state that lies costs a run.
func (b *busy) undress() {
	b.worn = false
	b.fields.Freeze(false)
	for _, control := range b.also {
		control.Enable()
	}
	b.preview.Enable()
	b.generate.Enable()
	b.cancel.Disable()
	b.cancel.Hide()
	b.bar.Hide()
	b.relay()
}

// relay lays the row of buttons out again, because the toolkit does not.
//
// Measured in the pinned toolkit on 2026-09-21, from the owner's report that
// Preview and Generate stood off centre after a preview: hiding a child
// changes what the row asks for, so the canvas lays out the row's PARENT
// (internal/driver/common/canvas.go, EnsureMinSize, parentNeedingUpdate) -
// and the parent hands the row the same size it had, which a container
// answers by doing nothing (Container.Resize returns on an unchanged size).
// So the row kept the positions it had worked out with Cancel in it, and the
// two buttons stayed half a Cancel to the left until something else moved
// them. Showing a child goes through the same path and happens to work,
// because the child's own size changes and the row is the parent then. One
// call here for both directions, so the row never depends on which way the
// toolkit happened to walk.
func (b *busy) relay() {
	if b.row != nil {
		b.row.Refresh()
	}
}
