package parts

import (
	"unicode"

	"fyne.io/fyne/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// FilterBox is the box at the top of an open list of formats that narrows it
// to the formats holding what was typed.
//
// Inside the open list and never in place of the closed box, and that is the
// answer to the objection that deferred it on 2026-08-25: a filter is a box to
// type in, and the closed menu had just been made to look unlike one. The
// closed menu still looks like a menu. What changed is the list it opens,
// twenty six formats long on 2026-09-23 and eighteen of them in sight - see
// docs/FORMAT-MENU-2026-09-23.md.
//
// It follows the ARIA pattern for a combobox whose list is filtered by an
// editable box: the keyboard stays in the box, the arrows move through the
// list, Enter takes the value the list is on and Escape closes. Home and End
// stay the box's own, because in a box to type in they move the caret.
type FilterBox struct {
	Entry

	list *OpenList
}

func newFilterBox(l *OpenList) *FilterBox {
	f := &FilterBox{list: l}
	f.PlaceHolder = text.PlaceholderFilter()
	f.OnChanged = l.narrowTo
	f.ExtendBaseWidget(f)
	return f
}

// TypedKey hands the list the keys that move through it, take and close, and
// keeps the rest - letters arrive through TypedRune and never come here.
func (f *FilterBox) TypedKey(event *fyne.KeyEvent) {
	if event == nil {
		return
	}
	switch event.Name {
	case fyne.KeyUp, fyne.KeyDown, fyne.KeyReturn, fyne.KeyEnter, fyne.KeyEscape:
		f.list.TypedKey(event)
		return
	}
	f.Entry.Entry.TypedKey(event)
}

// TypedRune drops white space typed into an empty box, and takes everything
// else as the box it is.
//
// The case it is for is the Space that OPENS the list. The driver hands the
// key to whatever has the keyboard and then the character to whatever has it
// after that - and the key is what moved the keyboard here (Chooser.TypedKey
// opens the list, the list hands the keyboard to this box). So the character
// of the same press landed in the box: no placeholder, the caret one space in,
// and nothing on screen saying why. Reported by an outside review of #127,
// seen in the real window, held by
// TestTheSpaceThatOpensTheFormatListIsNotTypedIntoItsFilter.
//
// Only while the box is empty, because a space after a word is somebody
// typing - and a leading one means nothing to the list anyway, which trims
// what it narrows by (narrowTo).
func (f *FilterBox) TypedRune(r rune) {
	if f.Text == "" && unicode.IsSpace(r) {
		return
	}
	f.Entry.TypedRune(r)
}
