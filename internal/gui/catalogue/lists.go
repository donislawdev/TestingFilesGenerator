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
// a value already chosen, the kind of thing each value is, and the room a
// window leaves it - the list has no ceiling of its own, the window it opens
// in sets one (parts.ListCeiling), so the cut face is built by telling the
// list about a short window.
//
// Nor has it a width of its own. On a form the list is as wide as the box it
// drops from, and the toolkit's list measures its width off an empty template
// row, so a list stood here bare was a 42 px strip with the first letter of
// each value on it - seen on the render of 2026-09-15, and accepted with the
// rest of the catalogue before anybody read it at that height. Each state
// is drawn as wide as the box a menu of the same values would be, which is
// the rule the form follows.
func openList() Entry {
	few := []string{"png", "jpg", "avif"}
	many := []string{"avif", "bmp", "csv", "docx", "gif", "html", "ico", "jpg", "json", "jxl", "log", "md"}
	// shortWindow is the height of the window the cut state stands in - a
	// value the state is given, like the words above, not a look. Tall enough
	// for a few rows of the list and not for all twelve.
	const shortWindow = 240
	return Entry{Name: "OpenList", Covers: []string{"ListRow", "KindOfFile"}, Natural: true, States: []State{
		{"a few values, one chosen", func() fyne.CanvasObject {
			return asWideAsItsBox(few, parts.NewOpenList(few, "jpg", func(string, bool) {}, func(bool) {}))
		}},
		{"more values than a short window shows at once", func() fyne.CanvasObject {
			l := parts.NewOpenList(many, "json", func(string, bool) {}, func(bool) {})
			l.LimitTo(parts.ListCeiling(shortWindow))
			return asWideAsItsBox(many, l)
		}},
		{"every value with the kind of thing it is", func() fyne.CanvasObject {
			l := parts.NewOpenList(many, "docx", func(string, bool) {}, func(bool) {})
			l.KindOf = parts.KindOfFile
			return asWideAsItsBox(many, l)
		}},
		{"a long value", func() fyne.CanvasObject {
			values := []string{longText, "png"}
			return asWideAsItsBox(values, parts.NewOpenList(values, "png", func(string, bool) {}, func(bool) {}))
		}},
	}}
}

// asWideAsItsBox gives an open list the width of the menu it would drop from:
// the box a Chooser of the same values is given by parts.Menu, which is as
// wide as the widest value plus the arrow.
func asWideAsItsBox(values []string, list *parts.OpenList) fyne.CanvasObject {
	box := parts.Menu(parts.NewChooser(values, func(string) {}))
	return parts.Sized(box.MinSize().Width, list)
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
