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
//
// Quiet by weight rather than by slant. docs/UX.md section 8.5 says italics
// never, with a reason rather than a preference: slanted text is harder to
// read, and hardest for the readers who already have the most trouble. Every
// hint on both screens was italic until 2026-08-11, which is O71.
//
// It is NOT quiet by colour yet, and that is measured rather than postponed
// out of caution. widget.LowImportance looked like the toolkit's own way to
// say this, and it renders a label in the disabled colour: measured 2026-08-11
// as #39393A on #171718, which is 1.55:1 against a threshold of 4.5. That is
// unreadable, and worse than the italics it would have replaced. The quiet
// belongs to text-subdued from docs/UX.md section 8.2, which computes to
// 7.03:1 and arrives with the palette - O70. Until then a hint reads in the
// ordinary text colour at 16.15:1, which is plain but legible.
func Note(text string) fyne.CanvasObject {
	label := widget.NewLabel(text)
	label.Wrapping = fyne.TextWrapWord
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
