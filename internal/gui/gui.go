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
// args are the process's arguments after its name. One of them is read, and
// it is the first argument this binary has ever read: --catalogue (or
// --catalog) opens the catalogue of the window's parts instead of the work
// screens - GUI rule 4's hidden screen, reached from the launch line. Every
// other argument is ignored, which is what happened to all of them before
// this and is written here rather than changed: a window started from a
// shortcut with odd arguments has always opened.
func Run(args []string, errOut io.Writer) int {
	return run(wantsCatalogue(args), errOut)
}

// wantsCatalogue says whether the launch line asked for the catalogue. Two
// spellings, because the tree writes the word one way and the flag was
// named the other on the day it was decided, and a flag typed the wrong way
// would open the ordinary window without a word.
func wantsCatalogue(args []string) bool {
	for _, arg := range args {
		if arg == "--catalogue" || arg == "--catalog" {
			return true
		}
	}
	return false
}
