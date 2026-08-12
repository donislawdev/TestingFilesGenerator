// Package window holds the screens, each built from internal/gui/parts.
//
// No file here reaches the toolkit's app package, so this package builds and
// tests with CGO_ENABLED=0 - which is what lets the whole CI matrix render a
// screen and compare it, on runners with no graphics environment.
package window

import (
	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/version"
)

// OpenSize is what the window opens at. Wide enough for the generate form to
// hold its fields and for a refusal to wrap at its own line breaks rather than
// at the frame, which is G9 as a measurement rather than a wish.
//
// Widened on 2026-08-12, decision of the owner. At 720 the form was narrower
// than the 820 it is allowed, so the column cap did nothing and every screen
// scrolled from the moment it opened - and a form that has to be scrolled
// before it can be read is a form nobody sees the shape of. 1000 leaves the
// column its full width with a margin either side, and 760 is what the
// generate screen needs once its explanations stop taking three lines each.
var OpenSize = fyne.NewSize(1000, 760)

// About is what the licence screen says.
//
// It carries the sentence somebody deciding whether to put this tool into a
// closed source product cannot get anywhere else: the licence covers the tool
// and not the files it makes. "tfg license" has printed it since 2026-08-04 and
// somebody using only the window had no way to read it, which is the drift D1
// exists to stop - in the one place the parity guard cannot see, because a
// licence is not a capability of the engine. docs/GUI.md section 7 names it.
//
// It was the opening screen while the window had only one. It is a screen you
// go to now rather than one you are shown, because a notice shown at every
// start is a notice nobody reads twice - and the window opens on the work.
//
// The text is not repeated here. It comes from version.LicenceNotice, the same
// constant the command prints, so the two cannot come to say different things.
// It carries no way back, and has not needed one since 2026-08-11: it is a tab,
// so every other screen is one click away and always visible. As a screen that
// replaced the whole window it needed a door, and a door somebody could delete
// without noticing was the thing worth guarding.
func About() fyne.CanvasObject {
	return parts.Screen(
		text.HeadingAbout(version.Version),
		parts.Prose(text.AboutTagline),
		// In a card like every other block on every other screen, so this reads
		// as a page of the application rather than as the one screen that was
		// left as it was.
		parts.Section(text.SectionLicence, parts.Prose(version.LicenceNotice)),
	)
}
