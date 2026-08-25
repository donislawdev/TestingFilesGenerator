package parts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// Entry is a box to type in that does not swallow this window's own shortcuts.
//
// It exists because of a measurement in the toolkit source rather than a
// preference, and the measurement is worth carrying: a keyboard shortcut is
// delivered to the FOCUSED widget if that widget can take shortcuts at all, and
// only reaches the canvas when nothing is focused - internal/driver/glfw
// window.go asks whether the focused object can take shortcuts and gives it the
// shortcut when it can.
// widget.Entry can take them, its handler holds Copy, Cut, Paste, Select all,
// Undo and Redo, and an unknown one returns without a word - fyne.ShortcutHandler
// TypedShortcut, "if !ok { return }".
//
// So on a screen that is a form, the keyboard is almost always in a box, and a
// shortcut registered on the canvas would never arrive. Ctrl+Enter would do
// nothing exactly when somebody has just finished typing, which is the moment
// they would press it.
//
// What this does NOT do is second guess the toolkit. Everything widget.Entry
// knows about is handed straight to it. Only a desktop.CustomShortcut is passed
// on, and an Entry never registers one of those - the toolkit builds them for
// key combinations it has no meaning for, which is exactly what ours are.
type Entry struct {
	widget.Entry

	// onOurs is where a shortcut goes when this box has no use for it. Set by
	// the screen, because the screen is what knows the canvas.
	onOurs func(fyne.Shortcut)
}

// NewEntry builds one.
func NewEntry() *Entry {
	e := &Entry{}
	e.ExtendBaseWidget(e)
	return e
}

// PassShortcutsTo says where a shortcut this box has no use for should go.
//
// Called with the canvas rather than resolved from the widget, because
// CanvasForObject answers nil until the box has been laid out - and a shortcut
// pressed before the first layout is a shortcut silently lost.
func (e *Entry) PassShortcutsTo(onOurs func(fyne.Shortcut)) { e.onOurs = onOurs }

// TypedShortcut hands the toolkit what the toolkit understands, and passes on
// what only this window does.
func (e *Entry) TypedShortcut(shortcut fyne.Shortcut) {
	if _, ours := shortcut.(*desktop.CustomShortcut); ours && e.onOurs != nil {
		e.onOurs(shortcut)
		return
	}
	e.Entry.TypedShortcut(shortcut)
}

// TypedKey passes Escape on and hands everything else to the box.
//
// Escape needs its own path because the toolkit never turns it into a shortcut:
// a key becomes one only when a modifier is held - internal/driver/glfw
// window.go, "shortcut == nil && modifier != 0" - so Escape alone arrives here
// as an ordinary key and would stop.
//
// It is passed on as a shortcut all the same, so that the window has ONE place
// where a key means something rather than a shortcut table and a key table that
// have to agree.
//
// Nothing is swallowed: widget.Entry has no use for Escape, measured rather
// than assumed - its TypedKey handles the arrows, Return, Backspace, Delete and
// the rest, and Escape is not among them.
func (e *Entry) TypedKey(event *fyne.KeyEvent) {
	if event != nil && event.Name == fyne.KeyEscape && e.onOurs != nil {
		e.onOurs(&desktop.CustomShortcut{KeyName: fyne.KeyEscape})
		return
	}
	e.Entry.TypedKey(event)
}
