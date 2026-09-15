package catalogue

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// The open list draws its rows only once it has a canvas to draw them on, so
// the two states a row has under a person's hands - the pointer on it and the
// keyboard on it - cannot be reached from a builder that runs before the
// canvas exists. They are looked at as stored scenes of the window instead
// (generate-menu-hovered and generate-menu-keyed), which is where they arise.
// What CAN be built here is everything the list is given: few values, many,
// a value already chosen, and the kind of thing each value is.
func openList() Entry {
	few := []string{"png", "jpg", "avif"}
	many := []string{"avif", "bmp", "csv", "docx", "gif", "html", "ico", "jpg", "json", "jxl", "log", "md"}
	return Entry{Name: "OpenList", Covers: []string{"ListRow", "KindOfFile"}, Natural: true, States: []State{
		{"a few values, one chosen", func() fyne.CanvasObject {
			return parts.NewOpenList(few, "jpg", func(string, bool) {}, func(bool) {})
		}},
		{"more values than it shows at once", func() fyne.CanvasObject {
			return parts.NewOpenList(many, "json", func(string, bool) {}, func(bool) {})
		}},
		{"every value with the kind of thing it is", func() fyne.CanvasObject {
			l := parts.NewOpenList(many, "docx", func(string, bool) {}, func(bool) {})
			l.KindOf = parts.KindOfFile
			return l
		}},
		{"a long value", func() fyne.CanvasObject {
			return parts.NewOpenList([]string{longText, "png"}, "png", func(string, bool) {}, func(bool) {})
		}},
	}}
}

func tabs() Entry {
	four := func() *parts.Tabs {
		return parts.NewTabs(
			&parts.Tab{Text: "Single batch", Content: parts.Prose("the first screen")},
			&parts.Tab{Text: "Presets", Content: parts.Prose("the second screen")},
			&parts.Tab{Text: "Several batches", Content: parts.Prose("the third screen")},
			&parts.Tab{Text: "About", Content: parts.Prose("the fourth screen")},
		)
	}
	return Entry{Name: "Tabs", Covers: []string{"TabWord", "Tabbed"}, States: []State{
		{"the strip, first word chosen", func() fyne.CanvasObject { return four() }},
		{"a word under the pointer", func() fyne.CanvasObject {
			t := four()
			t.Words()[1].MouseIn(&desktop.MouseEvent{})
			return t
		}},
		{"a word holding the keyboard", func() fyne.CanvasObject {
			t := four()
			t.Words()[2].FocusGained()
			return t
		}},
		{"the strip with its screens under it", func() fyne.CanvasObject {
			return parts.Tabbed(four())
		}},
		{"long words", func() fyne.CanvasObject {
			return parts.NewTabs(
				&parts.Tab{Text: longText, Content: parts.Prose("one")},
				&parts.Tab{Text: "Short", Content: parts.Prose("two")},
			)
		}},
	}}
}
