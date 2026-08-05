package window

import "fyne.io/fyne/v2"

// Open fills the window with the first screen and wires the way between them.
//
// The window opens on the work rather than on a welcome, decided by the owner
// on 2026-08-05. The licence notice is a screen you go to and not one you are
// shown: docs/GUI.md section 7 asks that somebody using only the window be able
// to read what the licence means for the files they generate, which is a
// question about whether it can be reached rather than about greeting people
// with it at every start.
//
// The generate screen is built once and kept. Rebuilding it on the way back
// from the licence would lose whatever was typed, and would lose a run in
// progress along with the only handle on stopping it.
func Open(h Host) {
	var showGenerate func()

	gen := NewGenerate(h, func() {
		h.SetContent(About(func() { showGenerate() }))
	})

	showGenerate = func() { h.SetContent(gen.Object()) }
	showGenerate()
}

// FirstScreen is what the window shows when it opens, without a window to put
// it in. It is what a guard renders, and it is the same tree Open installs.
func FirstScreen(h Host) fyne.CanvasObject {
	return NewGenerate(h, func() {}).Object()
}
