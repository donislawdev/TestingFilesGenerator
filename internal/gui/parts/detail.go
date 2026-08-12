package parts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
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
	return container.NewHBox(head, detailButton(detail))
}

// detailButton is the small button that opens one field's explanation.
//
// An icon rather than a word, because it sits on the same line as the field
// name and a word there competes with it. Low importance so it recedes: it is
// the quietest thing on the row until somebody wants it.
func detailButton(detail string) *widget.Button {
	var button *widget.Button
	button = widget.NewButtonWithIcon("", theme.InfoIcon(), func() {
		showDetail(button, detail)
	})
	button.Importance = widget.LowImportance
	return button
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
func showDetail(near fyne.CanvasObject, detail string) {
	app := fyne.CurrentApp()
	if app == nil {
		return
	}
	driver := app.Driver()
	canvas := driver.CanvasForObject(near)
	if canvas == nil {
		return
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
