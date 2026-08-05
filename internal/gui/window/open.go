package window

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// Open fills the window with the first screen and wires the way between them.
//
// The window opens on the work rather than on a welcome, decided by the owner
// on 2026-08-05. The licence notice is a screen you go to and not one you are
// shown: docs/GUI.md section 7 asks that somebody using only the window be able
// to read what the licence means for the files they generate, which is a
// question about whether it can be reached rather than about greeting people
// with it at every start.
//
// Every screen is built once and kept. Rebuilding one on the way back would
// lose whatever was typed, and would lose a run in progress along with the only
// handle on stopping it.
func Open(h Host) {
	var showGenerate, showPreset, showAbout func()

	gen := NewGenerate(
		widget.NewButton("Presets", func() { showPreset() }),
		widget.NewButton("About", func() { showAbout() }),
	)
	pre := NewPreset(
		widget.NewButton("One target", func() { showGenerate() }),
		widget.NewButton("About", func() { showAbout() }),
	)

	showGenerate = func() { h.SetContent(gen.Object()) }
	showPreset = func() { h.SetContent(pre.Object()) }
	showAbout = func() { h.SetContent(About(func() { showGenerate() })) }

	// Closing the window during a run is a cancellation and not a kill, G7. The
	// invariant that the output directory never holds a half written file rests
	// on the signal handler in cmd/tfg, and closing a window is not a signal -
	// so without this the run would carry on with nobody watching it, or die in
	// the middle of a file.
	//
	// It asks every screen rather than the one on show, and that is the part
	// worth writing down: a run keeps going while somebody moves to another
	// screen, so the busy one is not necessarily the visible one. Stop does
	// nothing on a screen that is idle.
	h.SetCloseIntercept(func() {
		gen.Stop()
		pre.Stop()
		h.Close()
	})

	showGenerate()
}

// FirstScreen is what the window shows when it opens, without a window to put
// it in. It is what a guard renders, and it is the same tree Open installs.
func FirstScreen() fyne.CanvasObject {
	return NewGenerate().Object()
}
