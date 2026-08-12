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

// withDetail puts the button that opens the longer explanation beside a label.
//
// Nothing at all when there is nothing more to say, rather than a button that
// opens an empty box. A control that is always there and sometimes does nothing
// teaches people to stop pressing it.
func withDetail(head fyne.CanvasObject, detail string) fyne.CanvasObject {
	if detail == "" {
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
// and it also opens on a click. Both, deliberately:
//
// Hover alone would be unreachable from a keyboard, and UX9 says whatever can
// be done with a mouse can be done without one. Click alone is what this was
// on 2026-08-12, and it was reported the same day as the wrong behaviour by
// somebody who simply pointed at it and waited.
//
// The toolkit has no tooltip to reach for. Measured in v2.8.0 rather than
// taken from anybody's word for it: there is no file and no identifier named
// tooltip anywhere in widget, internal/widget or driver/desktop. What it does
// have is desktop.Hoverable, which is three methods a widget can answer - so
// the behaviour is built here rather than depended on, and no third party
// package enters the graph for it.
type DetailButton struct {
	widget.Button

	detail string
	// open is the explanation while it is on screen, and nil when it is not.
	// Only ever touched from the interface thread, like every other field of a
	// widget here.
	open *widget.PopUp
}

func newDetailButton(detail string) *DetailButton {
	b := &DetailButton{detail: detail}
	b.ExtendBaseWidget(b)
	b.Icon = theme.InfoIcon()
	b.Importance = widget.LowImportance
	b.OnTapped = b.show
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

// MouseOut takes the explanation away again.
//
// The pointer leaving is the only thing that closes it, which is what makes it
// behave like the tooltip people expect rather than like a dialog somebody has
// to dismiss.
func (b *DetailButton) MouseOut() {
	b.Button.MouseOut()
	b.hide()
}

// show puts the explanation under the button, or leaves it where it is if it
// is already there. Opening a second one over the first is what a click after
// a hover would otherwise do.
func (b *DetailButton) show() {
	if b.open != nil {
		return
	}
	b.open = showDetail(b, b.detail)
}

func (b *DetailButton) hide() {
	if b.open == nil {
		return
	}
	b.open.Hide()
	b.open = nil
}

// showDetail opens the explanation under the button that asked for it.
//
// It asks the toolkit for the canvas the button is on rather than being handed
// one, which is what keeps this package free of the app package - the property
// that lets every screen here be built and rendered with no C compiler and no
// graphics environment.
//
// A canvas it cannot find means nothing opens. That happens where a tree is
// built and measured without ever being shown, which several guards do, and a
// button that panics there would make this package untestable in exactly the
// place it was designed to be testable.
func showDetail(near fyne.CanvasObject, detail string) *widget.PopUp {
	app := fyne.CurrentApp()
	if app == nil {
		return nil
	}
	driver := app.Driver()
	canvas := driver.CanvasForObject(near)
	if canvas == nil {
		return nil
	}

	body := container.NewPadded(Prose(detail))
	pop := widget.NewPopUp(body, canvas)

	// Sized twice, and this is the same finding the render probe records rather
	// than superstition. A wrapping label reports the height it needs for the
	// width it currently knows about, and before the first resize that is not
	// the width it ends up with - so a single pass gives a box one line tall
	// with the rest of the paragraph outside it.
	pop.Resize(fyne.NewSize(DetailWidth, body.MinSize().Height))
	pop.Resize(fyne.NewSize(DetailWidth, body.MinSize().Height))

	pop.ShowAtPosition(below(driver, near, canvas))
	return pop
}

// below is where the explanation opens: under the button, and never past the
// right hand edge of the window.
//
// The clamp is not decoration. These buttons sit beside field names, the right
// hand column of a two column row starts past the middle of the window, and a
// box opened at that x with a fixed width runs off the screen - where the part
// that falls off is the end of every line.
func below(driver fyne.Driver, near fyne.CanvasObject, canvas fyne.Canvas) fyne.Position {
	at := driver.AbsolutePositionForObject(near)
	at.Y += near.Size().Height
	if rightmost := canvas.Size().Width - DetailWidth; at.X > rightmost {
		at.X = fyne.Max(0, rightmost)
	}
	return at
}
