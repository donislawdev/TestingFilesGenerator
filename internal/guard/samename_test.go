package guard

import (
	"testing"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// No section is named after a field inside it.
//
// The preset screen had a card called Preset whose first field was also called
// Preset - the same word twice within 40 px, in two different ranks of type.
// Measured off a render on 2026-08-20. It reads as naming that was not
// finished: a group named after its only member says nothing the member does
// not already say, and it costs a line of height plus the reader's second look.
//
// Guarded across every screen rather than fixed in one place, because the next
// section somebody adds around a single field will be tempting to name after
// it - and it is the kind of thing nobody notices in the diff that introduces
// it.
func TestNoSectionIsNamedAfterAFieldInsideIt(t *testing.T) {
	ourTheme(t)
	content, _ := laidOutWindow(t)

	// Counted so that the comparison cannot quietly happen nowhere: a screen
	// with sections and no readable field names is skipped, and if every
	// screen were, this guard would be green over nothing.
	compared := 0
	for _, tab := range allTabs() {
		t.Run(tab, func(t *testing.T) {
			screen := tabContent(t, content, tab)

			sections := map[string]bool{}
			atAbsolute(screen, func(o fyne.CanvasObject, _ fyne.Position) {
				words, bold, size, is := boldWordsAt(o)
				if is && words != "" && bold && size == parts.TextHeading {
					sections[words] = true
				}
			})
			// Read off the rows of the form rather than by weight. Until
			// 2026-09-16 a field was "bold words at the body size", which
			// every field's name was until the form became a grid on
			// 2026-09-14 and names went to regular weight - from that day
			// this map was empty on every screen, the comparison below was
			// over nothing, and the full mutation run found the guard green
			// with the preset card named after its field again.
			fields := map[string]bool{}
			for _, name := range fieldNamesOn(screen) {
				fields[name] = true
			}

			if len(sections) == 0 {
				t.Skipf("%s draws no sections, so there is nothing to compare", tab)
			}
			if len(fields) == 0 {
				t.Skipf("%s draws no field with a name, so there is nothing to compare", tab)
			}
			compared++
			for name := range sections {
				if fields[name] {
					t.Errorf("%q is the name of a section and the name of a field on the same screen."+
						" A group named after its own member says nothing the member does not", name)
				}
			}
		})
	}
	if compared == 0 {
		t.Fatal("no screen had both sections and named fields, so nothing was compared")
	}
}

// fieldNamesOn is every field name a screen draws, read off the fields
// themselves - a field is the one container laid out as a field, and its
// first thing is its name. Asked by the layout rather than by the weight of
// the words, for the reason the guard above gives.
func fieldNamesOn(screen fyne.CanvasObject) []string {
	var out []string
	walk(screen, func(obj fyne.CanvasObject) {
		box, ok := obj.(*fyne.Container)
		if !ok || !parts.IsField(box) || len(box.Objects) < 2 {
			return
		}
		if name, named := headingOf(box.Objects[0]); named && name != "" {
			out = append(out, name)
		}
	})
	return out
}

// And the preset card still says what it is for.
//
// The half that stops the guard above being satisfied by deleting the title.
func TestThePresetCardIsNamedForWhatItAsks(t *testing.T) {
	ourTheme(t)
	content, _ := laidOutWindow(t)
	presets := tabContent(t, content, text.TabPresets())

	if _, ok := labelBox(presets, text.SectionPreset()); !ok {
		t.Errorf("the preset screen has no section called %q", text.SectionPreset())
	}
	if text.SectionPreset() == text.FieldPreset() {
		t.Errorf("the section and the field are both called %q again", text.SectionPreset())
	}
}
