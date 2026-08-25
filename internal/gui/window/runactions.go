package window

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
)

// The buttons that start, stop and follow a run.
//
// Split out of run.go on 2026-08-25, when that file went past three quarters of
// its ceiling after the keyboard and the folder button arrived. The ceiling is
// a ratchet: the answer is a split and never a higher number.

// actions puts Preview before Generate, and that order is G6.
//
// It is the one thing a window does better than a command line rather than
// merely as well: --dry-run has to be known about and remembered, and this is
// on the way to the button beside it. With presets running to several gigabytes
// and disks that are not always emptier than that, it is not decoration.
// actions is the bar at the foot of the form. The buttons that run something go
// at the RIGHT edge of the column, and anything else stays at the left.
//
// Measured from the stored tree on 2026-08-18 before this changed: the bar is
// 820 px wide and the two buttons stood at x=0 and x=73. Nobody had chosen that
// - it was where an HBox puts things - and the owner asked why they sat over
// there. Decided on 2026-08-18: the end of the reading path, which is where a
// form with a fixed action bar puts the thing it wants pressed last. Cancel is
// hidden until a run starts, so at rest the rightmost button is Generate.
//
// The spacer is what does it. An HBox gives every child its minimum width and
// leaves the rest empty at the end, so without something greedy in front the
// buttons cannot move.
// PressGenerate, PressPreview and PressCancel are the three buttons of this bar
// reached from the keyboard.
//
// They press the button rather than call what it does, and the difference is
// the whole point: a disabled button does nothing when it is pressed, so a
// shortcut cannot start a second run during the first one or cancel a run that
// is not going. Calling onGenerate directly would bypass every one of those
// states - which is the same reason the guards in this project press through
// the canvas rather than through the handler.
func (r *runner) PressGenerate() { pressIfLive(r.generateBtn) }

func (r *runner) PressPreview() { pressIfLive(r.previewBtn) }

func (r *runner) PressCancel() { pressIfLive(r.cancelBtn) }

// pressIfLive presses a button somebody could have pressed.
//
// Hidden as well as disabled, because Cancel is BOTH while nothing is running -
// and a shortcut that worked on a button nobody can see would be a way to reach
// a state the screen does not offer.
func pressIfLive(b *widget.Button) {
	if b == nil || b.Disabled() || !b.Visible() || b.OnTapped == nil {
		return
	}
	b.OnTapped()
}

func (r *runner) actions() fyne.CanvasObject {
	// Centred, on the owner's decision of 2026-08-19, which reverses the one of
	// 2026-08-18 that put them at the right edge. Both were reports from
	// looking at the built window, and the reasoning for the first is kept
	// above rather than deleted because it was not wrong - it was a choice, and
	// this is a different one.
	//
	// A spacer at each end rather than one, because a single greedy spacer only
	// pushes: it can put the group at one end or the other and never in the
	// middle.
	//
	// Everything that is not one of these buttons went to the bar's rail on
	// 2026-08-19 - see parts.ActionBar. It used to be laid over this row, which
	// kept it inside the form's column and so a margin away from the edge.
	return container.NewHBox(
		layout.NewSpacer(), r.previewBtn, r.generateBtn, r.cancelBtn, r.openBtn, layout.NewSpacer())
}

// offerTheFolder shows the way to the files, once there are some.
//
// Asked of the RESULT rather than of the box on the screen: a run that wrote
// nothing has nothing to show, and a run that was stopped after three files has
// three files somebody may well want to look at. The manifest is what knows.
func (r *runner) offerTheFolder(res *engine.Result) {
	if res == nil || res.Manifest == nil || len(res.Manifest.Files) == res.Failures {
		return
	}
	if r.wroteInto == "" {
		return
	}
	r.openBtn.Show()
}

// hideTheFolder takes the offer away when the next run starts, so the button
// never points at the results of the run before this one.
func (r *runner) hideTheFolder() {
	r.wroteInto = ""
	r.openBtn.Hide()
}
