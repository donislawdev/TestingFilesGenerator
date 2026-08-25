package parts

import "fyne.io/fyne/v2"

// Shortcuts is this window's own table of what a key combination does.
//
// A table of ours rather than the canvas's, and the reason is a measurement
// rather than a preference. Two things about the toolkit make the canvas the
// wrong place to keep this:
//
// A shortcut is delivered to the FOCUSED widget when that widget can take
// shortcuts, and only reaches the canvas when nothing is focused - measured in
// internal/driver/glfw window.go on 2026-08-25. These screens are forms, so the
// keyboard is nearly always in a box, and a box drops what it does not know
// without a word. So a box has to pass ours on to somewhere, and that somewhere
// has to be reachable from the box.
//
// And the canvas cannot be that somewhere in a way anything can check.
// fyne.Canvas does not declare TypedShortcut, and driver/software's
// WindowlessCanvas - the one a guard renders on - does not add it, so reaching
// the table through the canvas works in a real window and is nil under test.
// A shortcut nothing can press is a shortcut nobody can prove, and this project
// has the rule about guards that pass without reaching the code.
//
// So the window keeps the table, the boxes deliver into it, and the canvas gets
// the same entries as well - for the case where nothing has the keyboard.
type Shortcuts struct {
	by map[string]func()
}

// NewShortcuts starts an empty table.
func NewShortcuts() *Shortcuts { return &Shortcuts{by: map[string]func(){}} }

// On says what a shortcut does. Registering the same one twice replaces it,
// which is what a window rebuilding its wiring would want.
func (s *Shortcuts) On(shortcut fyne.Shortcut, do func()) {
	if s == nil || shortcut == nil || do == nil {
		return
	}
	s.by[shortcut.ShortcutName()] = do
}

// Deliver runs what a shortcut does, and does nothing for one this window has
// no meaning for - which is most of them, since the toolkit builds a shortcut
// for every key combination it does not recognise itself.
func (s *Shortcuts) Deliver(shortcut fyne.Shortcut) {
	if s == nil || shortcut == nil {
		return
	}
	if do, ours := s.by[shortcut.ShortcutName()]; ours {
		do()
	}
}

// Len is how many this window answers to, for a guard that wants to know the
// table was filled in at all.
func (s *Shortcuts) Len() int {
	if s == nil {
		return 0
	}
	return len(s.by)
}
