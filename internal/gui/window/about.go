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
	var sections []fyne.CanvasObject
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
		// What to do with the program, before what may be done with the source
		// of it. Counted on the stored screen of 2026-09-22: the thesis had one
		// sentence and everything under it was the licence and the list of what
		// the binary carries, so four fifths of this screen answered a question
		// about redistribution - a real question, and not the one somebody has
		// on the day they open this.
		//
		// Bullets rather than paragraphs, and the same ones the preset screen
		// draws under "Typically finds:" - three steps are read at a glance in
		// a list and read one by one in prose.
		parts.Section(text.SectionHowToUse(), parts.Bullets(text.HowToUseSteps())),
		// In a card like every other block on every other screen, so this reads
		// as a page of the application rather than as the one screen that was
		// left as it was.
		parts.Section(text.SectionLicence(), parts.Prose(paragraphs(version.LicenceNotice))),
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
		//
		// The Donate button stands here, in the card, since the prototype of
		// 2026-09-23 - the bar at the foot of this screen held nothing else, a
		// whole strip of the window for one quiet word, while this card named
		// the same address as words nobody could press. The address stays
		// under the button for the reason above.
		parts.Section(text.SectionSupport(),
			parts.Prose(text.DetailDonate()),
			container.NewHBox(donate(h, parts.Secondary)),
			parts.Prose(text.SupportURL)),
	)
	sections = append(sections, carried()...)
	// The title with its sentence under it, as on the other three screens.
	// The sentence stood as a section of its own until 2026-09-24 and sat
	// 16 px further from the title than every other screen's does, measured
	// on the stored screens (review UI-007).
	page := parts.Screen(parts.Titled(text.HeadingAbout(version.Version), text.AboutTagline()), sections...)

	// No bar at the foot, since the prototype of 2026-09-23. It held only the
	// Donate button, which is in the Support card now - so the button is still
	// on every screen, and this one gives the strip back to the page.
	//
	// The page is scrolled, which the other three screens have been from the
	// start and this one did not need while it held four paragraphs. It holds
	// the list of what the binary carries now, and a licence notice that cannot
	// be read to the end is the one kind of notice that fails at its only job.
	return container.NewVScroll(page)
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
	groups := []struct {
		heading string
		in      func(legal.Item) bool
	}{
		{text.SectionCarriedCode(), func(i legal.Item) bool { return !i.Embedded && !i.Beside }},
		{text.SectionCarriedFiles(), func(i legal.Item) bool { return i.Embedded }},
		// Named on every system, although one archive carries it: the
		// notices say the same everywhere, and a screen that named it only
		// where it was found would say nothing on the machine where it was
		// deleted, which is the machine whose person is asking.
		{text.SectionCarriedBeside(), func(i legal.Item) bool { return i.Beside }},
	}
	for _, g := range groups {
		if rows := carriedRows(items, g.in); len(rows) > 0 {
			out = append(out, parts.Section(g.heading, parts.Ledger(rows)))
		}
	}
	return out
}

// carriedRows is one group as rows of a table - what it is, under which
// licence, and whose - in the order internal/legal settled. A table since the
// prototype of 2026-09-23: the same three things as one line each, told apart
// by two spaces, read as a wall of text in the real window.
func carriedRows(items []legal.Item, in func(legal.Item) bool) [][3]string {
	var rows [][3]string
	for _, item := range items {
		if !in(item) {
			continue
		}
		rows = append(rows, [3]string{item.Title(), item.SPDX, item.Copyright})
	}
	return rows
}

// paragraphs joins the lines of each paragraph of a notice written for a
// terminal, so the window wraps it to its own width instead of drawing a
// narrow column of lines broken for eighty characters. A short line stays a
// line of its own - the name and the copyright at the head of the notice are
// two lines on purpose, and they are the only lines shorter than this.
//
// The command line prints the notice as it is written.
func paragraphs(notice string) string {
	const wrapped = 40
	var out []string
	for _, para := range strings.Split(strings.TrimSpace(notice), "\n\n") {
		out = append(out, unwrapped(strings.Split(para, "\n"), wrapped))
	}
	return strings.Join(out, "\n\n")
}

// unwrapped joins the lines of one paragraph, keeping a break after any line
// shorter than wrapped - see paragraphs.
func unwrapped(lines []string, wrapped int) string {
	joined := lines[0]
	for i := 1; i < len(lines); i++ {
		divider := "\n"
		if len(lines[i-1]) >= wrapped {
			divider = " "
		}
		joined += divider + lines[i]
	}
	return joined
}
