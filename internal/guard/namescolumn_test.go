package guard

import (
	"testing"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
)

// Every name a screen can show is in the list the column of names is worked
// out from.
//
// The column is one width for the whole window, taken from every name the
// window can ever draw - window.EveryFieldName - so that choosing a format
// with a long setting name does not move every box on the screen. That list
// is written by hand, and a list written by hand is missing something the day
// after it is written: a name that is not in it draws at its own width, wider
// than the column, and its box starts to the right of every other box.
//
// So the names are read off the screens, with every format, every damage and
// every preset chosen in turn, and each one is asked whether the list has it.
// The screens are the source and the list is what is checked, which is the
// direction that finds the missing entry rather than the one that confirms
// the entries there are.
func TestEveryNameOnAScreenIsInTheListTheColumnIsWorkedOutFrom(t *testing.T) {
	ourTheme(t)
	content, _ := laidOutWindow(t)

	listed := map[string]bool{}
	for _, name := range window.EveryFieldName() {
		listed[name] = true
	}
	if len(listed) == 0 {
		t.Fatal("the window lists no names at all, so there is nothing to compare")
	}

	seen := map[string]bool{}
	note := func(screen fyne.CanvasObject) {
		for _, name := range fieldNamesOn(screen) {
			seen[name] = true
		}
	}

	generate := tabContent(t, content, text.TabOneTarget())
	formats := menuUnder(t, generate, text.FieldFormat())
	for _, id := range format.IDs() {
		formats.SetSelected(id)
		note(generate)
	}
	damages := menuUnder(t, generate, text.FieldDamage())
	for _, id := range damage.Names() {
		damages.SetSelected(id)
		note(generate)
	}

	presets := tabContent(t, content, text.TabPresets())
	picker := menuUnder(t, presets, text.FieldPreset())
	for _, id := range preset.IDs() {
		picker.SetSelected(id)
		note(presets)
	}

	recipe := tabContent(t, content, text.TabRecipe())
	batches := menuUnder(t, recipe, text.FieldFormat())
	for _, id := range format.IDs() {
		batches.SetSelected(id)
		note(recipe)
	}

	if len(seen) < len(listed)/2 {
		t.Fatalf("only %d names were read off the screens against %d listed, so this guard did not reach the screens",
			len(seen), len(listed))
	}
	for name := range seen {
		if !listed[name] {
			t.Errorf("%q is the name of a field on a screen and is not in the list the column of names is worked out from,"+
				" so its box may start to the right of every other box", name)
		}
	}
}

// fieldNamesOn is the name of every field on a screen, read off the rows the
// form is made of. A switch carries its own name and stands in the column of
// controls, so its row has no name here, which is right - the column of names
// is not for it.
func fieldNamesOn(screen fyne.CanvasObject) []string {
	var out []string
	walk(screen, func(obj fyne.CanvasObject) {
		box, ok := obj.(*fyne.Container)
		if !ok || !parts.IsFieldRow(box) || len(box.Objects) < 2 {
			return
		}
		if name, named := headingOf(box.Objects[0]); named && name != "" {
			out = append(out, name)
		}
	})
	return out
}
