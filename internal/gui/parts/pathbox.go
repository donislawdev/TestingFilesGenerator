package parts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// Takes is what a path box holds: a folder or a file.
//
// Declared once, on the box, and read by everything that puts a path into it:
// the picker beside it opens for that kind, and a folder or a file dropped onto
// the window lands only in a box that takes it. Two declarations - which picker
// and what a drop may bring - would be two answers to one question, and the
// copy is the one that drifts.
type Takes int

// The two kinds of path. Nought is a box that takes no path at all, which is
// every box that is not a path box.
const (
	TakesFolder Takes = iota + 1
	TakesFile
)

// NewPathBox is a box for a path with a button beside it that opens a picker.
//
// Typing a path is fine when you have one to hand and hopeless when you do not,
// so the box carries a way to browse to one. The box stays editable: a picker
// that replaced typing would take away pasting a path somebody sent you, which
// is how most of these get filled in.
//
// choose opens the picker for what the box takes and calls back with the path
// picked, or with nothing when the person changed their mind - which leaves the
// box as it was.
//
// One box and one button for every path on every screen, so a path field looks
// and behaves the same whatever it names (GUI rule 2). Moved here from the
// window on 2026-11-14 so that it is a part the catalogue draws and the box can
// say what it takes.
func NewPathBox(box *Entry, takes Takes, choose func(func(string))) *fyne.Container {
	box.takes = takes
	if box.PlaceHolder == "" {
		box.SetPlaceHolder(pathHint(takes))
	}
	pick := NewButton(Secondary, text.ButtonChoose(), func() {
		choose(func(path string) {
			if path != "" {
				box.SetText(path)
			}
		})
	})
	return container.NewBorder(nil, nil, nil, pick, box)
}

// pathHint is what an empty path box says, which is also the one place on the
// screen that tells somebody a folder or a file can be dropped there.
func pathHint(takes Takes) string {
	if takes == TakesFolder {
		return text.HintDropFolder()
	}
	return text.HintDropFile()
}

// PathTarget is a path box on a screen, with the field it stands in.
type PathTarget struct {
	Field *Field
	Box   *Entry
	Takes Takes
}

// PathTargets is every path box among the fields of a screen, in the order
// they were built - which is the order they stand on the screen.
//
// Read from the registry every time rather than kept, because the registry is
// what a screen rebuilds: the Tools tab draws the boxes of whichever tool is
// chosen, and a list kept from before would point at boxes no longer there.
// A function rather than a method, because Fields stands at its ceiling of
// methods.
func PathTargets(s *Fields) []PathTarget {
	var out []PathTarget
	for _, f := range s.list {
		out = append(out, pathBoxesOf(f)...)
	}
	return out
}

// pathBoxesOf is the path boxes under one field, which is one box or none on
// every screen today.
func pathBoxesOf(f *Field) []PathTarget {
	var out []PathTarget
	for _, box := range boxesIn(f.Control) {
		if box.takes != 0 {
			out = append(out, PathTarget{Field: f, Box: box, Takes: box.takes})
		}
	}
	return out
}
