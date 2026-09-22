package catalogue

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// Screen is the catalogue as the window shows it: one section per entry, and
// under each entry its states in order, each under its caption, in the same
// column and at the same widths as the fields of a form - so a control here
// is the size it is on a screen and not the size it would take if left alone.
//
// Two sections at the end say what the catalogue does not draw and why. A
// catalogue that is silent about what it leaves out looks complete, which is
// rule 6 of the untouchables from the other side.
func Screen() fyne.CanvasObject {
	return container.NewVScroll(Page())
}

// Page is the catalogue before it is put in a scroll: the whole height of
// it, for a guard that stores a picture of all of it rather than of the
// first screenful.
func Page() fyne.CanvasObject {
	entries := Entries()
	sections := make([]fyne.CanvasObject, 0, len(entries)+2)
	for _, e := range entries {
		sections = append(sections, section(e))
	}
	// The palette stands before the two sections that say what is left out,
	// because it is what every control above it is painted with rather than
	// another thing in the list.
	sections = append(sections, paletteSections()...)
	sections = append(sections,
		parts.Section("Not drawn, and why", parts.Bullets(sentences(NotDrawn()))),
		parts.Section("Layout only, and why", parts.Bullets(sentences(LayoutOnly()))),
	)
	return parts.Screen(parts.Titled("Catalogue", "Every part of the window, in every state it has."), sections...)
}

// section is one entry as the screen shows it: its name over its states,
// each under its caption.
func section(e Entry) fyne.CanvasObject {
	rows := make([]fyne.CanvasObject, 0, len(e.States))
	for _, s := range e.States {
		built := s.Build()
		if e.Natural {
			built = container.NewHBox(built)
		}
		rows = append(rows, parts.Column(parts.GapInline, parts.Caption(s.Caption), built))
	}
	return parts.Section(e.Name, rows...)
}

func sentences(reasons []Reason) []string {
	out := make([]string, 0, len(reasons))
	for _, r := range reasons {
		out = append(out, r.Name+" - "+r.Why)
	}
	return out
}
