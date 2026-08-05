//go:build cgo

package gui

import (
	"io"

	"fyne.io/fyne/v2/app"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/version"
)

// run opens a real window. The only file in this tree that reaches the app
// package, and therefore the only one that needs a C compiler.
// appID names this application to the desktop it runs on.
//
// Reverse domain form because that is what every desktop expects. Without an
// id the toolkit prints a complaint on every start and its preferences API
// refuses to work - and while nothing here stores preferences yet, a warning
// nobody can act on is noise that teaches people to ignore the log.
//
// What it costs, measured on 2026-08-05 rather than assumed: starting the
// application creates an empty directory under the user's application data,
// and it does so with or without an id - app.New() makes one too. No file is
// written into it, because nothing here stores a preference. An id only
// decides whether that directory has our name on it or a derived one.
//
// So this is not the decision docs/GUI.md section 4.1 is waiting for. Whether
// the window ever KEEPS state between runs is still open and belongs to the
// owner: a configuration file would be a new artefact on somebody's disk, and
// would come under D16 and untouchable rule 7 - nothing goes out, nothing
// deletes itself.
const appID = "dev.donislaw.tfg"

func run(errOut io.Writer) int {
	a := app.NewWithID(appID)
	w := a.NewWindow("Testing Files Generator " + version.Version)
	w.SetContent(window.Start())
	w.Resize(window.StartSize)
	w.CenterOnScreen()
	w.ShowAndRun()
	return 0
}
