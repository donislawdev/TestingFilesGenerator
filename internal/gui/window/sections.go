package window

import (
	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// The keys a section is remembered under. Names of places, never shown - kept
// as constants so that nothing a person reads is a literal on a screen.
const (
	sectionConfiguration = "configuration"
	sectionOutput        = "output"
	sectionPreset        = "preset"
	sectionSettings      = "settings"
	sectionBase          = "base"
)

// sections are the panels of a work screen, each of which folds away - the
// prototype of 2026-09-23, on the owner's word from the running window that
// File configuration should fold like a batch does, and then that every
// section of a form should, so that they all look alike.
//
// Separate from the runner, because two things have to outlive the panel
// they are about and the runner is already at its ceiling. Whether a section
// is folded survives a rebuild - the batch screen builds its panels again
// whenever a batch is added or removed, and a section that sprang open each
// time would be the defect Recipe.wire was written against. And a refusal
// about a box inside a folded section opens it, the same promise every other
// fold on these screens keeps (see runner.unfold): a box that is marked and
// cannot be seen reads as a button that did nothing.
//
// A folded section says nothing on its head line. What it holds is said at
// the foot of the screen already - the format, the size, how many and where
// they go - and a second summary of the same things one panel up would be the
// same sentence twice.
type sections struct {
	folded map[string]bool
	folds  map[string]*parts.Folding
}

func newSections() *sections {
	return &sections{folded: map[string]bool{}, folds: map[string]*parts.Folding{}}
}

// section builds one panel under a key that stays the same across rebuilds.
func (s *sections) section(key, title string, content ...fyne.CanvasObject) fyne.CanvasObject {
	fold := parts.NewFolding(title, nil, content...)
	fold.OnChange = func(open bool) { s.folded[key] = !open }
	fold.Set(!s.folded[key])
	s.folds[key] = fold
	return fold.Object()
}

// openHolding opens every section a control stands in.
func (s *sections) openHolding(control fyne.CanvasObject) {
	for _, fold := range s.folds {
		if fold.Holds(control) {
			fold.Set(true)
		}
	}
}
