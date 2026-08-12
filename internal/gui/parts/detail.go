package parts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// DetailWidth is how wide the longer explanation gets when it opens.
//
// Narrower than the form on purpose. The column is 820 px because that is what
// the form needs, and the same width for a paragraph of prose is about 112
// characters a line - well past the 45 to 75 that reads easily. A block of text
// with nothing beside it has no reason to be as wide as a row of fields.
const DetailWidth = 380

// Tips is the sheet a screen's explanations are drawn on.
//
// It exists because of where an explanation may NOT go, and that took two
// wrong answers to establish. The toolkit's overlay layer is the obvious home
// for anything that floats, and it cannot be used here: while any overlay is
// up, the driver searches the overlay and nothing else for whatever is under
// the pointer - internal/driver/util.go, FindObjectAtPositionMatching, where
// the overlay case replaces the roots rather than being tried before them.
//
// So an explanation opened as an overlay makes its own button impossible to
// find. The button is told the pointer left, it closes the explanation, the
// overlay goes, the button is found again. That is the flicker, and it is not
// fixed by making the overlay ignore the mouse - an overlay that matches
// nothing still stops the search from reaching the button.
//
// A sheet inside the screen's own tree has none of that. It is content, so the
// search walks it like everything else, and the pieces on it answer to nothing
// - the walk carries on past them and finds the button still underneath.
type Tips struct {
	sheet *fyne.Container
}

// NewTips makes an empty sheet. One per screen.
func NewTips() *Tips {
	return &Tips{sheet: container.NewWithoutLayout()}
}

// Over puts the sheet above a screen, where it covers everything and holds
// nothing until somebody asks a question.
//
// Above the whole screen rather than inside the scrolling part, which is not a
// detail: a scroll clips what it draws and what it hits, so an explanation
// opened near the foot of the form would be cut off at the edge of the
// viewport - and cut off at exactly the end of the sentence.
func (t *Tips) Over(body fyne.CanvasObject) fyne.CanvasObject {
	return container.NewStack(body, t.sheet)
}

// Say is one explanation, ready to be handed to a field.
func (t *Tips) Say(detail string) Detail {
	return Detail{Text: detail, on: t}
}

// Detail is the longer explanation of a field, and where to draw it.
//
// The sheet travels with the words rather than being looked up later, because
// a window holds every screen at once - so "the sheet" is not a thing that can
// be found from a button, only a thing that can be given to it. Looking it up
// would find whichever screen the walk reached first, and draw the explanation
// on a tab nobody is looking at.
type Detail struct {
	Text string
	on   *Tips
}

// NoDetail is a field with nothing held back, which is most of them. Every
// setting a format or a preset declares is described in one sentence built
// from its declaration, so there is no second half to put behind a button.
var NoDetail = Detail{}

// withDetail puts the button that opens the longer explanation beside a label.
//
// Nothing at all when there is nothing more to say, rather than a button that
// opens an empty box. A control that is always there and sometimes does nothing
// teaches people to stop pressing it.
func withDetail(head fyne.CanvasObject, detail Detail) fyne.CanvasObject {
	if detail.Text == "" || detail.on == nil {
		return head
	}
	return container.NewHBox(head, newDetailButton(detail))
}

// DetailButton is the small control that shows one field's explanation.
//
// Exported for the same reason ErrorArea is: a guard has to be able to tell it
// from the controls around it, and the alternative was recognising it by shape
// - an icon button with no words - which is a rule that holds until somebody
// adds a second one.
//
// An icon rather than a word, because it sits on the same line as the field
// name and a word there competes with it. Low importance so it recedes: it is
// the quietest thing on the row until somebody wants it.
//
// It opens on HOVER, which is what anybody meeting a small letter i expects,
// and a press toggles it. Both, deliberately: hovering is not something a
// keyboard can do, and UX9 says whatever the mouse can reach the keyboard can
// too. The press toggles rather than only opening because somebody who got
// there without a pointer has no pointer to move away.
//
// The toolkit has no tooltip to reach for. Measured in v2.8.0 rather than
// taken on trust: there is no file and no identifier named tooltip anywhere in
// widget, internal/widget or driver/desktop. What it does have is
// desktop.Hoverable, three methods a widget can answer, so this is built here
// and no third party package enters the graph for it.
type DetailButton struct {
	widget.Button

	detail Detail
	// shown is the box while it is on the sheet, and nil when it is not. Only
	// ever touched from the interface thread, like every other field here.
	shown fyne.CanvasObject
}

func newDetailButton(detail Detail) *DetailButton {
	b := &DetailButton{detail: detail}
	b.ExtendBaseWidget(b)
	b.Icon = theme.InfoIcon()
	b.Importance = widget.LowImportance
	b.OnTapped = b.toggle
	return b
}

// MouseIn shows the explanation when the pointer arrives.
func (b *DetailButton) MouseIn(e *desktop.MouseEvent) {
	b.Button.MouseIn(e)
	b.show()
}

// MouseMoved is required by the interface and has nothing to do. The
// explanation is already open by the time the pointer is moving inside the
// button, and reopening it on every movement would rebuild it hundreds of
// times crossing one icon.
func (b *DetailButton) MouseMoved(*desktop.MouseEvent) {}

// MouseOut takes the explanation away again. The pointer leaving is what
// closes it, which is what makes it behave like the tooltip people expect
// rather than like a dialog somebody has to dismiss.
func (b *DetailButton) MouseOut() {
	b.Button.MouseOut()
	b.hide()
}

func (b *DetailButton) show() {
	if b.shown != nil {
		return
	}
	b.shown = b.detail.on.open(b, b.detail.Text)
}

func (b *DetailButton) hide() {
	if b.shown == nil {
		return
	}
	b.detail.on.close(b.shown)
	b.shown = nil
}

func (b *DetailButton) toggle() {
	if b.shown != nil {
		b.hide()
		return
	}
	b.show()
}

// open draws one explanation under the button that asked for it and hands back
// the box, so it can be taken off again.
//
// A box drawn where the pointer is not: under the button, and above it when
// there is no room below. The buttons at the foot of a form are the ones whose
// explanation would otherwise open past the bottom of the window, where none
// of it can be read.
func (t *Tips) open(near fyne.CanvasObject, detail string) fyne.CanvasObject {
	app := fyne.CurrentApp()
	if app == nil {
		return nil
	}
	driver := app.Driver()

	box := container.NewStack(panelSurface(), container.NewPadded(Prose(detail)))

	// Sized twice, and this is the same finding the render probe records rather
	// than superstition. A wrapping label reports the height it needs for the
	// width it currently knows about, and before the first resize that is not
	// the width it ends up with - so a single pass gives a box one line tall
	// with the rest of the paragraph outside it.
	box.Resize(fyne.NewSize(DetailWidth, box.MinSize().Height))
	box.Resize(fyne.NewSize(DetailWidth, box.MinSize().Height))
	box.Move(t.place(driver, near, box.Size()))

	t.sheet.Add(box)
	return box
}

func (t *Tips) close(box fyne.CanvasObject) {
	t.sheet.Remove(box)
}

// place is where the box goes, in the sheet's own coordinates.
//
// Both positions are asked of the driver and subtracted, rather than the
// button's position being used as it stands. The sheet covers the screen and
// the screen is not the window - there is a tab strip above it - so a position
// measured from the window would put every explanation too low by the height
// of that strip.
func (t *Tips) place(driver fyne.Driver, near fyne.CanvasObject, box fyne.Size) fyne.Position {
	at := driver.AbsolutePositionForObject(near).Subtract(driver.AbsolutePositionForObject(t.sheet))
	below := at.Y + near.Size().Height

	if rightmost := t.sheet.Size().Width - box.Width; at.X > rightmost {
		at.X = fyne.Max(0, rightmost)
	}
	if below+box.Height > t.sheet.Size().Height && at.Y-box.Height >= 0 {
		return fyne.NewPos(at.X, at.Y-box.Height)
	}
	return fyne.NewPos(at.X, below)
}
