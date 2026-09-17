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
		parts.Indented(parts.Prose(text.AboutTagline())),
	}
	// Under the tagline and only when it is true: the window is drawn by the
	// software renderer shipped beside it, not by a driver. Said here rather
	// than in a title or a dialog because it is a fact about this window for
	// as long as it is open, and this is the screen that says what the
	// program is. Untouchable rule 6 - a window drawn in software that did
	// not say so would be a slow window with no explanation.
	if h.SoftwareRendering() {
		sections = append(sections, parts.Indented(parts.Prose(text.DrawingWithSoftwareRenderer())))
	}
	sections = append(sections,
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
	)
	sections = append(sections, carried()...)
	page := parts.Screen(parts.Title(text.HeadingAbout(version.Version)), sections...)

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
		nil, parts.ActionBar(rail(donateButton(h))), nil, nil, container.NewVScroll(page))
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
