package catalogue

import (
	"errors"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// The ranks of text, from the title of a screen down to a note. A scale is a
// thing a person looks at as a whole, which is why every rank is here once,
// one under another, and once more with a long line to see where it wraps.
func textRanks() Entry {
	return Entry{Name: "Title", Covers: []string{"Subtitle", "Titled", "Heading", "Subheading", "Prose", "Note", "Caption", "Bullets", "Ledger"}, States: []State{
		{"the title of a screen", func() fyne.CanvasObject { return parts.Title("Single batch") }},
		{"the sentence under a title", func() fyne.CanvasObject {
			return parts.Subtitle("Files of one format and one size, with a manifest that says how the system under test should react to them.")
		}},
		{"a title with its sentence", func() fyne.CanvasObject {
			return parts.Titled("Single batch", "Files of one format and one size, with a manifest that says how the system under test should react to them.")
		}},
		{"the name of a field", func() fyne.CanvasObject { return parts.Heading("Output directory") }},
		{"the name of a block inside a section", func() fyne.CanvasObject { return parts.Subheading("Typically finds") }},
		{"a paragraph", func() fyne.CanvasObject {
			return parts.Prose("Every file has exactly the size you ask for, to the byte, or the run refuses before writing anything.")
		}},
		{"a note that has to stay visible", func() fyne.CanvasObject {
			return parts.Note("The limit of 100 000 files is this tool's, not the disk's.")
		}},
		{"a list of short statements", func() fyne.CanvasObject {
			return parts.Bullets([]string{"Upload validators", "Size limits", "Archive handling"})
		}},
		{"a list whose items do not fit on one line", func() fyne.CanvasObject {
			// Here because the list above could not show it and the window
			// now draws one: the About screen's three steps, where the third
			// runs to two lines. Rule 4 of the owner's list asks every
			// component to be in this catalogue with a very long text in it,
			// and a marker beside a wrapped item is exactly the case that
			// list of short statements cannot ask about (O235).
			// Twice the long line rather than once: measured off the stored
			// tree, one of them is 504 px inside 777 px of room and does not
			// wrap at all, so a state built from it would have been a state
			// that cannot show the thing it is named after.
			return parts.Bullets([]string{"One line", longText + " " + longText})
		}},
		{"a caption, the smallest rank", func() fyne.CanvasObject {
			return parts.Caption("10 485 760 B")
		}},
		// A table of words - what is carried in the binary, under which
		// licence, and whose - and the row whose first two columns ask for
		// so much that the last would be left a sliver, which is the one the
		// About screen really has.
		{"a table of three columns, the last one wrapping", func() fyne.CanvasObject {
			return parts.Ledger([][3]string{
				{"fyne.io/fyne/v2", "BSD-3-Clause", "(C) 2018 Fyne.io developers (see AUTHORS)"},
				{"Mesa 3D, llvmpipe software renderer  26.2.0", "MIT AND Apache-2.0 WITH LLVM-exception AND BSL-1.0",
					"Copyright (C) 1999-2007 Brian Paul, Copyright (C) 2008 VMware, Inc."},
				{"golang.org/x/text", "BSD-3-Clause", longText},
			})
		}},
		{"a long line at every rank", func() fyne.CanvasObject {
			return parts.Column(parts.GapField,
				parts.Title(longText), parts.Heading(longText), parts.Subheading(longText),
				parts.Prose(longText), parts.Note(longText))
		}},
	}}
}

// What a screen is built from besides its fields: the panel a section is,
// the line between two things, and the bar a run is started from.
func structure() Entry {
	twoRows := func() []fyne.CanvasObject {
		s := form()
		one := s.Add("size", "Size", "10mb", parts.NoDetail, parts.NewEntry())
		two := s.Add("count", "How many files", "1", parts.NoDetail, parts.NewEntry())
		return []fyne.CanvasObject{one, two}
	}
	return Entry{Name: "Section", Covers: []string{"Divider", "ActionBar"}, States: []State{
		{"a section of two rows", func() fyne.CanvasObject {
			return parts.Section("File configuration", twoRows()...)
		}},
		{"a section with a line across it", func() fyne.CanvasObject {
			rows := twoRows()
			return parts.Section("Output", rows[0], parts.Divider(), rows[1])
		}},
		{"the bar a run starts from", func() fyne.CanvasObject {
			// Composed the way the work screens compose it: the rail at the
			// left edge, the buttons in a row of their own, centred.
			return parts.ActionBar(container.NewHBox(parts.NewButton(parts.Quiet, "Donate", func() {}).InTheBar()),
				parts.ButtonRow(
					parts.NewButton(parts.Secondary, "Preview", func() {}).InTheBar(),
					parts.NewButton(parts.Primary, "Generate", func() {}).InTheBar()))
		}},
		{"fields in columns, each as wide as its value needs", func() fyne.CanvasObject {
			s := form()
			return parts.Section("File configuration",
				s.Add("format", "Format", "", parts.NoDetail, parts.NewChooser([]string{"avif", "png"}, nil)),
				s.Add("size", "Size", "10mb", parts.NoDetail, parts.Numeric(parts.NewEntry())),
				s.Add("count", "How many files", "1", parts.NoDetail, parts.Numeric(parts.NewEntry())),
				s.Add("damage", "Damage", "", parts.NoDetail, parts.NewChooser([]string{"none"}, nil)),
				parts.Wide(s.Add("dir", "Output directory", "", parts.NoDetail, parts.NewEntry())))
		}},
		{"a row with a refusal under it", func() fyne.CanvasObject {
			s := form()
			grid := parts.Section("File configuration",
				s.Add("size", "Size", "10mb", parts.NoDetail, parts.Numeric(parts.NewEntry())),
				s.Add("count", "How many files", "1", parts.NoDetail, parts.Numeric(parts.NewEntry())),
				s.Add("seed", "Seed", "0", parts.NoDetail, parts.Numeric(parts.NewEntry())))
			s.Mark("size", errors.New("size 3 B is below the smallest png, which is 73 B"))
			s.Mark("count", errors.New(longText))
			return grid
		}},
		{"a section with a long title", func() fyne.CanvasObject {
			return parts.Section(longText, twoRows()...)
		}},
	}}
}
