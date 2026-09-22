// Package catalogue is every part of the window drawn in every state it has,
// from one registry.
//
// GUI rule 4 of the owner's asks for a hidden screen in the application that
// shows each component in all its states, with very long text and extreme
// values, and that is closed in both directions: it may not show components
// the application does not have, and may not miss ones it does. Until
// 2026-09-15 that screen was a probe outside the repository
// (tools/probes/partscatalog), and it stopped compiling at step 5 of the
// rework without anything saying so for two steps - a catalogue nothing
// builds is a catalogue that does not exist. This package is in the tree, the
// window opens it with --catalog, and guards hold it to the package it draws.
//
// The registry DRIVES the screen. The screen is built from Entries, the guard
// compares Entries with the exported surface of internal/gui/parts, and a
// second guard renders every state and refuses two that draw the same. If the
// drawing came from one list and the checking from another, the catalogue
// would be a third copy of the same list and would rot like the first two.
//
// What counts as a component was decided on 2026-09-15 (O209) and is written
// where each name lands: a type that draws and has states is an Entry or a
// state of one (Covers), a value or a helper is in NotDrawn with its reason,
// and a function that only arranges is in LayoutOnly. The lists of reasons
// are shorter than the list of entries, on purpose.
//
// The words here are English and do not come from the text package. This is a
// screen for whoever builds the window, reached by a flag nobody is told
// about, and translating sixty captions of states would be work with no
// reader - decision of the owner, 2026-09-15. The guard that keeps every word
// of the window in the text package names this package as the exception.
package catalogue

import "fyne.io/fyne/v2"

// Entry is one part of the window and the states worth seeing it in.
type Entry struct {
	// Name is the part's name in internal/gui/parts: a constructor without
	// its New, or a function. The guard matches it against that surface.
	Name string

	// Covers names everything else in parts this entry draws: the types that
	// are states of it (a ListRow is a state of the OpenList), the second
	// constructor of the same control (NewGlyphButton), the functions its
	// states call. Every name here has to exist, and nothing may be named by
	// two entries - one part, one place to look at it.
	Covers []string

	// States are drawn in this order, under this caption.
	States []State

	// Natural says the states are drawn at the width they ask for and not
	// the width of the column: a button, a menu, a switch. A row of a form,
	// a list, a section fill the column the way they fill a screen.
	Natural bool
}

// State is one way a part can look, with the caption written above it.
type State struct {
	Caption string
	Build   func() fyne.CanvasObject
}

// Reason is a name from parts that the catalogue does not draw, and why.
type Reason struct {
	Name string
	Why  string
}

// Entries is the registry, in the order the screen shows it: the controls a
// person operates first, then the lists and the strip, then the rows of a
// form, then what is only read.
func Entries() []Entry {
	return []Entry{
		button(), chooser(), entry(), toggle(), segments(),
		openList(), tabs(),
		fields(), propertyField(), byteCount(), tips(), errorArea(), progress(), folding(),
		textRanks(), structure(), swatch(),
	}
}

// NotDrawn is every exported type of parts that is not a component: a value
// something returns, the model a control draws from, a piece inside one.
// Named with the reason rather than left off the guard's list, because a name
// on no list is the way a component goes uncatalogued.
//
// What the guard asks about is written there: every exported type, every
// constructor, and every exported function that returns something drawable.
// A function returning a number or a yes is not a component and is not asked
// about, which is why Theme, WidestName and their kind are not here.
func NotDrawn() []Reason {
	return []Reason{
		{"Choice", "a value: one row of an open list, read back by a guard"},
		{"Detail", "data: the longer explanation of a field and the sheet it opens on"},
		{"Field", "the model of one row of a form, drawn by Fields"},
		{"Built", "a value: what building a field comes to, handed to the registry in one piece"},
		{"Tab", "data: one screen and the word that leads to it, drawn by Tabs"},
		{"Look", "an enum: which face a Button wears, every value drawn under Button"},
		{"PointerFocus", "a piece inside a control, knowing what put the keyboard there - no picture of its own"},
		{"Returnable", "an interface: a control the window can tell it is coming back to the front, nothing of its own to draw"},
		{"Shortcuts", "a keyboard map: nothing to draw, and saying so is the point of this row"},
	}
}

// LayoutOnly is every exported function of parts that returns something
// drawable and arranges rather than draws: it has no colour, no words and no
// state of its own, so a picture of it is a picture of what it was given.
func LayoutOnly() []Reason {
	return []Reason{
		{"Column", "stacks children at one gap of the scale"},
		{"Row", "puts fields side by side"},
		{"FieldColumn", "the column a screen refills at run time, stacked like a section"},
		{"Indented", "the same left edge as the fields inside a panel"},
		{"Sized", "one width, given"},
		{"Numeric", "the width a number needs"},
		{"Text", "the width a short piece of text needs - a name, a template, a file name"},
		{"Padded", "one distance of the scale round its content"},
		{"Stacked", "panels one under another"},
		{"Screen", "the readable width round a head and its sections"},
		{"BesideFields", "the room to the right of the column of names"},
		{"WithRoomForARun", "the room under a form for the bar to speak into"},
		{"Flush", "a label kept by a screen, on the edge every other word stands on"},
		{"Clear", "a shape that takes room and draws nothing"},
	}
}

// longText is the caption every control is offered once, because a catalogue
// that only shows short text hides the defect that shows up with real text.
const longText = "Write a label inside each generated file, including the ones that are far too small to hold it"
