package window

import (
	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// Where a work screen sends its files: the output directory, and whether a
// run into one that already holds an earlier run is added beside it under a
// number (engine.Options.NumberIfTaken, docs/RUN-FOLDERS-2026-11-14.md).
//
// One type for the three work screens since 2026-11-14. Each of them held its
// own box and its own copy of OutDir and SetOutDir, word for word, and the
// switch would have been a third pair of copies on each - so the box, the
// switch and what is carried between the screens live here once.

// settingNumberIfTaken is the switch's place among a screen's fields. It is
// not a key of the recipe - where the files go is the window's question, as
// the command line's --number-if-taken is a flag and not a key - so it is
// named the way the flag is.
const settingNumberIfTaken = "number-if-taken"

// destination is what the window carries from the screen somebody leaves to
// the one they arrive at, and writes down when it closes.
type destination interface {
	OutDir() string
	SetOutDir(string)
	NumbersIfTaken() bool
	SetNumbersIfTaken(bool)
}

// outputFolder is the box and the switch, held by a work screen and promoted
// to it, so the screen answers destination without a word of its own.
type outputFolder struct {
	outDir *parts.Entry
	// numbers is whether a run into a directory holding an earlier one is
	// added beside it under a number rather than refused. On unless somebody
	// turned it off - the owner's decision of 2026-11-14, section 9.
	numbers *parts.Toggle
}

// newOutputFolder is the box, holding the directory the window offers, and
// the switch, on.
func newOutputFolder() outputFolder {
	o := outputFolder{outDir: parts.NewEntry(), numbers: parts.NewToggle(nil)}
	o.outDir.SetText(startingDirectory())
	o.numbers.SetChecked(true)
	return o
}

// OutDir and SetOutDir are how the screens keep one answer to "where do the
// files go" between them. The window carries it across on the way from one to
// the other - see Open.
func (o outputFolder) OutDir() string { return o.outDir.Text }

// SetOutDir leaves an empty value alone. Emptying the box is somebody clearing
// it to type, and copying that over would wipe the other screen's answer.
func (o outputFolder) SetOutDir(dir string) {
	if dir != "" {
		o.outDir.SetText(dir)
	}
}

// NumbersIfTaken and SetNumbersIfTaken are the switch, carried with the
// directory because it is the other half of the same answer.
func (o outputFolder) NumbersIfTaken() bool      { return o.numbers.Checked }
func (o outputFolder) SetNumbersIfTaken(on bool) { o.numbers.SetChecked(on) }

// numberField is the switch as a field of the screen's form, under the box.
func (o outputFolder) numberField(fields *parts.Fields, tips *parts.Tips) fyne.CanvasObject {
	return fields.AddToggle(settingNumberIfTaken, text.FieldNumberIfTaken(), "",
		tips.Say(text.DetailNumberIfTaken()), o.numbers)
}

// carryDestination moves the answer from the screen somebody left to the one
// they arrived at.
func carryDestination(from, to destination) {
	to.SetOutDir(from.OutDir())
	to.SetNumbersIfTaken(from.NumbersIfTaken())
}
