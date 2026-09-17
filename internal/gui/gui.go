// Package gui holds the desktop window.
//
// Nothing outside this package imports the graphics toolkit, and inside it the
// import is narrower still: only the file behind a cgo build tag reaches the
// toolkit's app package. Everything that builds a widget tree lives in
// internal/gui/window and internal/gui/parts, which build and test with
// CGO_ENABLED=0.
//
// That split is forced rather than chosen. Measured on 2026-08-05: importing
// fyne.io/fyne/v2/app makes "go build ./..." fail with CGO disabled, because
// the OpenGL binding excludes every Go file for that configuration. Without
// the split one import would stop the whole tree building wherever a C
// compiler is missing, which is most of a CI matrix - and that is the reason
// this package was kept at arm's length from the beginning.
package gui

import "io"

// Run opens the window and returns the exit code of the process.
//
// A window has no machine consumer, so there is no mapping onto the frozen
// table the way the command line has - docs/GUI.md section 5 says that is a
// fact to write down rather than a gap to fill. The code still exists because
// this is a process, and it answers one question: did the window come up.
//
// args are the process's arguments after its name. Two of them are read.
// --catalogue (or --catalog) opens the catalogue of the window's parts
// instead of the work screens - GUI rule 4's hidden screen, reached from the
// launch line. --software-gl draws the window with the software renderer
// shipped beside it instead of the graphics driver, which is what the window
// does by itself, in a second process, when the driver offers no OpenGL - see
// software.go. Every other argument is ignored, which is what happened to all
// of them before this and is written here rather than changed: a window
// started from a shortcut with odd arguments has always opened.
func Run(args []string, errOut io.Writer) int {
	return run(ReadLaunch(args), errOut)
}

// Launch is what the launch line asked for.
//
// The arguments are kept as they were given, because a window that could not
// open starts this program again with them - plus the one flag that says to
// draw without the driver - and the second process has to be asked for the
// same thing the first one was.
type Launch struct {
	Catalogue  bool
	SoftwareGL bool
	Args       []string
}

// SoftwareFlag asks for the software renderer. Public, because it is the way
// to see the window as a machine without a driver sees it - and the way the
// window asks for itself when it starts again.
const SoftwareFlag = "--software-gl"

// ReadLaunch reads the two flags the window knows out of the launch line.
//
// The catalogue has two spellings, because the tree writes the word one way
// and the flag was named the other on the day it was decided, and a flag
// typed the wrong way would open the ordinary window without a word.
func ReadLaunch(args []string) Launch {
	launch := Launch{Args: args}
	for _, arg := range args {
		switch arg {
		case "--catalogue", "--catalog":
			launch.Catalogue = true
		case SoftwareFlag:
			launch.SoftwareGL = true
		}
	}
	return launch
}
