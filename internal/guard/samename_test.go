package guard

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

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

	for _, tab := range allTabs() {
		t.Run(tab, func(t *testing.T) {
			screen := tabContent(t, content, tab)

			sections := map[string]bool{}
			fields := map[string]bool{}
			atAbsolute(screen, func(o fyne.CanvasObject, _ fyne.Position) {
				label, is := o.(*widget.Label)
				if !is || label.Text == "" {
					return
				}
				// A section title is heavy and at the heading rank. A field
				// name is light and at the caption rank, which it became on
				// 2026-09-08 - and until this was changed with it, this guard
				// looked for a BOLD label and found no field names at all. It
				// went green on every screen while asking nothing, which is
				// O118 exactly: it had stopped reaching the state it guards.
				//
				// The line explaining a field is drawn at the same rank and in
				// the same colour, and what tells the two apart is that an
				// explanation wraps and a name does not.
				switch {
				case label.TextStyle.Bold && label.SizeName == theme.SizeNameHeadingText:
					sections[label.Text] = true
				case !label.TextStyle.Bold && label.SizeName == theme.SizeNameCaptionText &&
					label.Wrapping == fyne.TextWrapOff:
					fields[label.Text] = true
				}
			})

			if len(sections) == 0 {
				t.Skipf("%s draws no sections, so there is nothing to compare", tab)
			}
			for name := range sections {
				if fields[name] {
					t.Errorf("%q is the name of a section and the name of a field on the same screen."+
						" A group named after its own member says nothing the member does not", name)
				}
			}
		})
	}
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
