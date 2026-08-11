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

// FieldSaying is a field that can carry a refusal of its own, underneath it.
//
// UX8 asks for a message near where the error came from, and in a window that
// means beside the field rather than at the foot of the form. Measured on
// 2026-08-11: the refusal about "how many" sat 748 px below the box it named,
// under every other field - and that distance grows with each field added, not
// with the size of the window.
//
// The area holds nothing until there is something to say, so a field that is
// not being complained about takes exactly the room it took before.
func FieldSaying(label, detail string, control fyne.CanvasObject) (fyne.CanvasObject, *ErrorArea) {
	area := NewErrorArea()
	items := []fyne.CanvasObject{Heading(label), control}
	if detail != "" {
		items = append(items, Note(detail))
	}
	items = append(items, area.Object())
	return container.NewVBox(items...), area
}

// Toggle is a switch that carries its own name, with the explanation under it.
//
// A switch is the one control that does not take a heading above it. Given one
// it arrives as a bare square with the words somewhere else: the name above,
// the sentence below, and nothing to read on the thing you click. That is what
// O72 saw on screen. Putting the name on the switch also makes the words part
// of the target, which is the difference between a click and an aimed click.
func Toggle(name, detail string, check *widget.Check) fyne.CanvasObject {
	check.Text = name
	items := []fyne.CanvasObject{check}
	if detail != "" {
		items = append(items, Note(detail))
	}
	return container.NewVBox(items...)
}

// Note is a quiet line under something, for what a person needs once.
//
// Quiet by weight rather than by slant. docs/UX.md section 8.5 says italics
// never, with a reason rather than a preference: slanted text is harder to
// read, and hardest for the readers who already have the most trouble. Every
// hint on both screens was italic until 2026-08-11, which is O71.
//
// Quiet by colour, which it could not be until the palette was installed.
//
// Worth keeping as a record, because the obvious move was wrong for a reason
// that had nothing to do with the widget: widget.LowImportance renders a label
// in the disabled colour, and under the toolkit's default theme that measured
// #39393A on #171718 - 1.55:1 against a threshold of 4.5, unreadable, and
// worse than the italics it was replacing. So it went in as ordinary text
// until the palette arrived.
//
// Under our palette the same widget.LowImportance is text-subdued, #9DA3A8,
// which computes to 7.03:1. The control was never the problem. O70 and O71.
func Note(text string) fyne.CanvasObject {
	label := widget.NewLabel(text)
	label.Wrapping = fyne.TextWrapWord
	label.Importance = widget.LowImportance
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
