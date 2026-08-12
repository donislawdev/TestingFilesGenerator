//go:build cgo

package gui

import (
	"io"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/dialog"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
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

// desktop is a real window, told how to answer the one thing the screens
// cannot do for themselves.
//
// The toolkit's folder picker lives here rather than beside the screens for the
// same reason its app package does: it is the part that needs a real window,
// and keeping it out of internal/gui/window is what lets the whole screen tree
// build and render with no C compiler.
//
// It is the only place this build touches fyne.io/fyne/v2/dialog, and that
// import has a cost worth recording. The dialog package pulls in
// github.com/FyshOS/fancyfs, which decorates folder icons and is reached from
// exactly one line of it. Checked on 2026-08-05 before it was accepted: BSD-3,
// one way compatible with GPL-3.0 like the eleven other BSD-3 modules already
// here, 129 lines of code, written by the author of the toolkit itself. It was
// already named in our module graph because the toolkit requires it - what
// changed is that it is now downloaded, checksummed and compiled in.
type desktop struct {
	fyne.Window
}

func (d desktop) ChooseDirectory(chosen func(string)) {
	dialog.ShowFolderOpen(func(dir fyne.ListableURI, err error) {
		// Nothing chosen and nothing to say. Cancelling a picker is an
		// ordinary answer rather than a failure, and an error here is the
		// toolkit failing to read a directory - which the field the person
		// can still type into makes survivable.
		if err != nil || dir == nil {
			chosen("")
			return
		}
		chosen(dir.Path())
	}, d.Window)
}

func run(errOut io.Writer) int {
	// Said out loud rather than left to be inferred: everything that touches a
	// widget from the worker goes through fyne.Do, and a static guard checks
	// it, but the toolkit had no way to know that. Without this it printed
	// three lines at every start telling the person running it that this
	// application has not been migrated - a warning about our code, on their
	// terminal, that was not true.
	//
	// Set here rather than passed as a build tag, because a tag has to be
	// remembered at every build and this cannot be forgotten.
	app.SetMetadata(fyne.AppMetadata{
		ID:         appID,
		Name:       "Testing Files Generator",
		Version:    version.Version,
		Migrations: map[string]bool{"fyneDo": true},
	})

	a := app.NewWithID(appID)
	// The palette of docs/UX.md sections 8.2 and 8.3, computed before the first
	// widget and describing nothing until it was installed here - O70. It
	// answers dark whatever the desktop is set to, by the owner's decision.
	a.Settings().SetTheme(parts.Theme())
	w := a.NewWindow(text.WindowTitle(version.Version))
	window.Open(desktop{w})
	w.Resize(window.OpenSize)
	w.CenterOnScreen()
	w.ShowAndRun()
	return 0
}
