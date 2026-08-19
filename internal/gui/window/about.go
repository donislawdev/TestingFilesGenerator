// Package window holds the screens, each built from internal/gui/parts.
//
// No file here reaches the toolkit's app package, so this package builds and
// tests with CGO_ENABLED=0 - which is what lets the whole CI matrix render a
// screen and compare it, on runners with no graphics environment.
package window

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

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
// before it can be read is a form nobody sees the shape of.
//
// The height is measured rather than chosen. tools/probes/formheight prints
// what the generate form needs against the room the scroll it sits in actually
// gets: 768 px needed, 786 px given at this size, so it clears with 18 to
// spare.
//
// The room is read off the laid out window rather than worked out from the
// window height, and that distinction cost an attempt. Subtracting the action
// bar and forgetting the tab strip above the form said "fits" for a screen
// whose last field was cut off in the render taken a minute later.
//
// The preset screen still scrolls, at 1019 px on 2026-08-18, and that is not
// a failure to fix here: it carries a list whose length is the preset's rather
// than ours, and a window sized for the longest one would be sized for nothing
// else. The number is reprinted by tools/probes/formheight, so it is worth
// re-reading rather than trusting - it said 1011 when it was written.
// Raised from 900 on 2026-08-19, on the owner's decision, because at 900 none
// of the three forms fitted the room it left them (O102).
//
// 1000 rather than more, and the ceiling is somebody else's screen rather than
// taste. A window taller than the screen it opens on cannot be reached at the
// bottom at all, which is worse than one that scrolls, and this toolkit offers
// no portable way to ask how big the screen is - checked in the driver
// interface on 2026-08-19, there is none - so the number has to be safe rather
// than clever. A 1080p screen leaves about 1040 px once the taskbar has taken
// its share, so 1000 fits it with room to spare and the owner's own screen,
// measured the same day at 3840x2088 of usable area, is not the constraint.
//
// It does not make the forms fit and is not meant to: Single batch needs 958 px
// of form and gets 826 px here. On a 1080p screen it would not fit even
// maximised, which is a fact about the form rather than about the window. What
// it does is take the shortfall from 232 px to 132 px, and the destination -
// the one field whose absence had a named cost - is on the status line now
// whatever the window is doing.
var OpenSize = fyne.NewSize(1000, 1000)

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
func About(h Host) fyne.CanvasObject {
	page := parts.Screen(
		text.HeadingAbout(version.Version),
		parts.Prose(text.AboutTagline),
		// In a card like every other block on every other screen, so this reads
		// as a page of the application rather than as the one screen that was
		// left as it was.
		parts.Section(text.SectionLicence, parts.Prose(version.LicenceNotice)),
	)

	// The same bar the work screens carry, holding only the Donate button.
	//
	// This screen starts no run and has nothing else to put there, so the bar is
	// almost empty - and it is here anyway, because the button moved into that
	// bar on 2026-08-19 and a button asking for money that is missing from one
	// screen in four is one people conclude they imagined. It is also the screen
	// somebody reads when deciding what this program costs them, which is the
	// worst one to leave it off.
	return container.NewBorder(
		nil, parts.ActionBar(rail(donateButton(h))), nil, nil, page)
}
