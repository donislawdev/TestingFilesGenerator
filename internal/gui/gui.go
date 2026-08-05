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
func Run(errOut io.Writer) int {
	return run(errOut)
}
