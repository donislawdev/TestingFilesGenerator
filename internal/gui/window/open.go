package window

import (
	"os"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
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

	gen := NewGenerate(h,
		widget.NewButton(text.ButtonPresets, func() { showPreset() }),
		widget.NewButton(text.ButtonAbout, func() { showAbout() }),
	)
	pre := NewPreset(h,
		widget.NewButton(text.ButtonOneTarget, func() { showGenerate() }),
		widget.NewButton(text.ButtonAbout, func() { showAbout() }),
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
func FirstScreen(h Host) fyne.CanvasObject {
	return NewGenerate(h).Object()
}

// chooserFor is the output directory box with a way to browse to one.
//
// Typing a path is fine when you have one to hand and hopeless when you do not,
// and the box beside it is the only field on either screen whose value is not
// something a person carries in their head. The button asks the window, which
// is the only thing that can open a picker.
//
// The box stays editable. A picker that replaces typing takes away pasting a
// path somebody sent you, which is how most of these get filled in.
func chooserFor(host Host, box *widget.Entry) fyne.CanvasObject {
	choose := widget.NewButton(text.ButtonChoose, func() {
		host.ChooseDirectory(func(dir string) {
			if dir != "" {
				box.SetText(dir)
			}
		})
	})
	return container.NewBorder(nil, nil, nil, choose, box)
}

// startingDirectory is where the output field points before anybody changes it.
//
// Spelled out in full rather than left as a dot, and that is the one place a
// window has to differ from the command line rather than merely look different.
// In a terminal "." is the directory you are standing in and you know which one
// that is, because you typed your way there. A window started from a desktop
// has a working directory nobody chose and nobody can see - so the same dot
// means "somewhere", and this is the one part of this tool that writes into
// other people's directories.
//
// The destination itself is unchanged. What changes is that it is legible
// before the button is pressed rather than after the files have appeared.
//
// A working directory we cannot read leaves the dot, because a dot that means
// "here" is still better than a path that is wrong.
func startingDirectory() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}
