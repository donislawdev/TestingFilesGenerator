package parts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// Field is one labelled control with the sentence that explains it.
//
// The sentence is part of the field rather than a tooltip, and that is G9
// arriving early. A hint the widget never renders is the shape docs/CLAUDE.md
// warns about: text with no reader becomes pressure on the text beside it, and
// the label starts trying to say everything on its own.
func Field(label, detail string, control fyne.CanvasObject) fyne.CanvasObject {
	items := []fyne.CanvasObject{Heading(label), control}
	if detail != "" {
		items = append(items, Note(detail))
	}
	return container.NewVBox(items...)
}

// Note is a quiet line under something, for what a person needs once.
func Note(text string) fyne.CanvasObject {
	label := widget.NewLabel(text)
	label.Wrapping = fyne.TextWrapWord
	label.TextStyle = fyne.TextStyle{Italic: true}
	return label
}

// ErrorArea is where a refusal is shown, and it is sized for a real one.
//
// G9 is a requirement on the layout rather than on the wording: a refusal in
// this tool has four parts - what happened, why, what is allowed, what to do
// instead - and a control that shows one line forces a message carrying one of
// the four. So this wraps, it never truncates, and it holds nothing at all
// until there is something to say. An empty red box on a fresh screen reads as
// a fault that has already happened.
type ErrorArea struct {
	label *widget.Label
	box   *fyne.Container
}

// NewErrorArea returns an area with nothing in it.
func NewErrorArea() *ErrorArea {
	label := widget.NewLabel("")
	label.Wrapping = fyne.TextWrapWord
	label.Importance = widget.DangerImportance

	area := &ErrorArea{label: label, box: container.NewVBox(label)}
	area.Clear()
	return area
}

// Object is the area, to put on a screen.
func (a *ErrorArea) Object() fyne.CanvasObject { return a.box }

// Say shows one refusal, whole. The text arrives as the engine wrote it - the
// window does not shorten it, because every one of the four parts is there
// because somebody could not act without it.
func (a *ErrorArea) Say(text string) {
	if text == "" {
		a.Clear()
		return
	}
	a.label.SetText(text)
	a.box.Show()
}

// Clear takes the last refusal back, which is what every fresh attempt starts
// with. A message left over from the previous press describes a state that is
// no longer true.
func (a *ErrorArea) Clear() {
	a.label.SetText("")
	a.box.Hide()
}

// Text is what the area is currently saying, for a guard to read.
func (a *ErrorArea) Text() string { return a.label.Text }
