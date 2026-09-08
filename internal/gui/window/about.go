// Package window holds the screens, each built from internal/gui/parts.
//
// No file here reaches the toolkit's app package, so this package builds and
// tests with CGO_ENABLED=0 - which is what lets the whole CI matrix render a
// screen and compare it, on runners with no graphics environment.
package window

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/legal"
	"github.com/donislawdev/TestingFilesGenerator/internal/version"
)

// OpenSize is what the window opens at. Wide enough for the generate form to
// hold its fields beside the report column, and for a refusal to wrap at its
// own line breaks rather than at the frame, which is G9 as a measurement
// rather than a wish.
//
// Both numbers are measured by tools/probes/formheight, which prints what each
// form needs against the room the scroll it sits in actually gets. Read off the
// laid out window rather than worked out from the window height, and that
// distinction cost an attempt: subtracting the action bar and forgetting the
// tab strip above the form said "fits" for a screen whose last field was cut
// off in the render taken a minute later.
//
// The height is the half worth explaining, because it was WRONG for three
// weeks in a way nothing could notice. It said 1000 and the reasoning under it
// was sound for the window it was written against: a one column window whose
// form carried Output, needed 958 px and got 826, so 1000 took a 232 px
// shortfall down to 132. Every word of that was true on 2026-08-19.
//
// Then the two column layout moved Output out of the form and the form lost
// 621 px, and nobody measured again. Measured on 2026-08-08 at 1000x1000:
// Single batch needs 337 px of form and the scroll is handed 900, so the window
// opened with 563 px of nothing in it, every time, before anybody clicked
// anything. Presets 594, Several batches 494. A height chosen to close a
// shortfall stayed on after the shortfall turned into a surplus - which is what
// a measured number does when the thing it measured moves.
//
// 1120x760 is the size the whole layout was drawn for, and the owner's decision
// of 2026-09-08. The ceiling on the height is somebody else's screen rather
// than taste: a window taller than the screen it opens on cannot be reached at
// the bottom at all, which is worse than one that scrolls, and this toolkit
// offers no portable way to ask how big the screen is - checked in the driver
// interface on 2026-08-19, and again in v2.8.1, there is none. So the number
// has to be safe rather than clever, and 760 clears a 1366x768 laptop.
//
// All three forms fit it now, which the one column window never managed.
var OpenSize = fyne.NewSize(1120, 760)

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
	sections := []fyne.CanvasObject{
		parts.Prose(text.AboutTagline()),
		// In a card like every other block on every other screen, so this reads
		// as a page of the application rather than as the one screen that was
		// left as it was.
		parts.Section(text.SectionLicence(), parts.Prose(version.LicenceNotice)),
		// The support address, in words, on the one screen somebody reads when
		// they are deciding what this program costs them.
		//
		// It is here because the Donate button cannot always work and says so:
		// handing an address to the desktop fails on a machine with no browser
		// registered, and that refusal is swallowed on purpose, because there is
		// no useful thing to say to somebody who pressed a button out of
		// curiosity. That decision only holds while the address is READABLE
		// somewhere, and until 2026-09-07 it was not - the constant had exactly
		// one use in the whole tree, as the button's destination, so a person
		// whose desktop did nothing had nowhere to go. The comment beside
		// OpenLink said this screen carried it. This is that screen carrying it.
		parts.Section(text.SectionSupport(),
			parts.Prose(text.DetailDonate()), parts.Prose(text.SupportURL)),
	}
	page := parts.Screen(text.HeadingAbout(version.Version), sections...)

	// What the binary carries moves into the column beside the page, on the
	// same layout the three work screens use.
	//
	// Measured on the stored screen before this changed: three labels of 355,
	// 622 and 164 px, so 1141 px of unbroken text in a window 1060 px tall,
	// and the whole of it inside one scroll under the licence. Reading who
	// wrote what meant scrolling past the licence every time.
	//
	// It is the same split as everywhere else, which is the point rather than a
	// coincidence: what this program IS on the left, and what it carries on the
	// right. Nothing here is a form, so this is the one screen where the right
	// hand column holds no boxes at all.
	// Scrolled, unlike the report panel on the work screens. What stands there
	// is five lines about one run and is meant to be taken in at a glance. This
	// is a list of everything the binary carries - thirty odd modules and seven
	// fonts - and a list that cannot be read to the end is the one kind of
	// notice that fails at its only job.
	beside := parts.ReportColumnFilling(container.NewVScroll(parts.Stacked(carried()...)))

	// The same bar the work screens carry, holding only the Donate button.
	//
	// This screen starts no run and has nothing else to put there, so the bar is
	// almost empty - and it is here anyway, because the button moved into that
	// bar on 2026-08-19 and a button asking for money that is missing from one
	// screen in four is one people conclude they imagined. It is also the screen
	// somebody reads when deciding what this program costs them, which is the
	// worst one to leave it off.
	//
	// The page is scrolled, which the other three screens have been from the
	// start and this one did not need while it held four paragraphs. It holds
	// the list of what the binary carries now, and a licence notice that cannot
	// be read to the end is the one kind of notice that fails at its only job.
	return container.NewBorder(
		nil, parts.ActionBar(rail(donateButton(h))), nil, beside,
		container.NewVScroll(parts.Inset(page, parts.GapColumn, 0, 0, 0)))
}

// carried is what this binary contains that somebody else wrote, read out of
// the build's own record.
//
// The reviewed list rather than this build's own record of itself, and the
// reason is in internal/legal beside Reviewed: the window links the whole
// registry, and a binary built by go test reports no dependencies at all, so
// a screen asking its own build would draw something no user ever sees. The
// command line does ask its own build, because there the answer is real.
//
// The values are names and licence identifiers, which are not translated. The
// two headings are, and they come from the text package like every other word
// on this screen.
//
// Two sections rather than one, because a library and a font are different
// questions to whoever is checking what they may ship. A section with nothing
// in it is not drawn at all: the command line binary carries no embedded files
// and a heading over an empty space would be a claim that it does.
func carried() []fyne.CanvasObject {
	items := legal.Reviewed()
	var out []fyne.CanvasObject
	if lines := carriedLines(items, false); lines != "" {
		out = append(out, parts.Section(text.SectionCarriedCode(), parts.Prose(lines)))
	}
	if lines := carriedLines(items, true); lines != "" {
		out = append(out, parts.Section(text.SectionCarriedFiles(), parts.Prose(lines)))
	}
	return out
}

// carriedLines writes one group as text, in the order internal/legal settled.
func carriedLines(items []legal.Item, embedded bool) string {
	var lines []string
	for _, item := range items {
		if item.Embedded == embedded {
			lines = append(lines, item.Line())
		}
	}
	return strings.Join(lines, "\n")
}
