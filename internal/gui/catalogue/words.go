package catalogue

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// The ranks of text, from the title of a screen down to a note. A scale is a
// thing a person looks at as a whole, which is why every rank is here once,
// one under another, and once more with a long line to see where it wraps.
func textRanks() Entry {
	return Entry{Name: "Title", Covers: []string{"Subtitle", "Titled", "Heading", "Subheading", "Prose", "Note", "Caption", "Bullets"}, States: []State{
		{"the title of a screen", func() fyne.CanvasObject { return parts.Title("Single batch") }},
		{"the sentence under a title", func() fyne.CanvasObject {
			return parts.Subtitle("Files of one format and one size, as many as you need.")
		}},
		{"a title with its sentence", func() fyne.CanvasObject {
			return parts.Titled("Single batch", "Files of one format and one size, as many as you need.")
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
		{"a caption, the smallest rank", func() fyne.CanvasObject {
			return parts.Caption("10 485 760 B")
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
			// left edge, the buttons centred between two spacers.
			return parts.ActionBar(container.NewHBox(parts.NewButton(parts.Quiet, "Donate", func() {})),
				container.NewHBox(layout.NewSpacer(),
					parts.NewButton(parts.Secondary, "Preview", func() {}),
					parts.NewButton(parts.Primary, "Generate", func() {}),
					layout.NewSpacer()))
		}},
		{"a section with a long title", func() fyne.CanvasObject {
			return parts.Section(longText, twoRows()...)
		}},
	}}
}
