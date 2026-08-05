// Package window holds the screens, each built from internal/gui/parts.
//
// No file here reaches the toolkit's app package, so this package builds and
// tests with CGO_ENABLED=0 - which is what lets the whole CI matrix render a
// screen and compare it, on runners with no graphics environment.
package window

import (
	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/version"
)

// StartSize is what the window opens at. Wide enough for the licence notice to
// wrap at its own line breaks rather than at the frame.
var StartSize = fyne.NewSize(620, 440)

// Start is the first screen.
//
// It shows what the tool is and what its licence means for the files it
// produces, and that choice is deliberate rather than a placeholder. The
// sentence "the files you generate are yours" reached people through "tfg
// license" from 2026-08-04 and reached nobody using the window, which is
// exactly the drift D1 exists to stop - only outside the reach of the parity
// guard, because a licence is not a capability of the engine. docs/GUI.md
// section 7 names it as the thing to do with the first window.
//
// The text is not repeated here. It comes from version.LicenceNotice, the same
// constant the command prints, so the two cannot come to say different things.
func Start() fyne.CanvasObject {
	return parts.Screen(
		"Testing Files Generator "+version.Version,
		parts.Prose("Generate test files, and know how the system under test should react to them."),
		parts.Prose(version.LicenceNotice),
	)
}
