package parts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Field is one labelled control, the line that explains it, and the longer
// explanation behind a button.
//
// Two lengths rather than one, split on 2026-08-12. The sentence used to be
// whole and permanent, so a form of eight settings carried eight grey
// paragraphs and the explanations took more vertical room than the controls
// did - measured on the generate screen, where the help outweighed the fields
// it was helping. What stays under a field is the line that says what it does.
// What goes behind the button is the consequence, the example and the units,
// which are things somebody needs once.
//
// It is a button rather than a tooltip because the toolkit has no tooltips -
// issue 1650 is still open - and it is visible rather than a hover because a
// hover is not reachable from a keyboard, which is UX9. A hint the widget never
// renders is the shape docs/CLAUDE.md warns about: text with no reader becomes
// pressure on the text beside it, and the label starts trying to say
// everything on its own.
// The plain Field function is gone as of 2026-08-12, and its absence is the
// fix rather than a side effect of one. It built a field with nowhere to put a
// refusal about it, so which boxes could be marked depended on which of two
// functions somebody typed - and three of eight on one screen got the wrong
// one, along with every setting a format declares. Fields.Add is the only way
// in now. See parts.Fields.

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
//
// The sentence is not alone: the box it is about draws a red edge for as long
// as the sentence is there. Two marks rather than one on purpose - a colour on
// its own says nothing to somebody who cannot tell it from the others, and a
// sentence on its own leaves them looking for which of eight boxes it means.
func FieldSaying(label, hint string, detail Detail, required bool, trailing, control fyne.CanvasObject) (object, body fyne.CanvasObject, area *ErrorArea) {
	marked, ring := WithRing(shapedForItsValues(control))
	area = NewErrorArea()
	area.edge = ring
	body = Column(GapTight, fieldParts(label, hint, detail, required, trailing, marked)...)
	return Column(GapTight, body, area.Object()), body, area
}

// shapedForItsValues holds a menu to the width of what it can show.
//
// Here rather than at the call sites, and that is the same reason the plain
// Field function was taken away above: a shape a caller has to remember to ask
// for is a shape some fields will not have. There are six menus across three
// screens and a seventh appears whenever a format declares a closed set of
// values, so "the one somebody forgot" is not hypothetical.
//
// Anything that is not a menu is handed back untouched. A box to type in has no
// length to promise - see ShapedFor, which does the same job for the settings a
// format declares and says why numbers are the other case that can be sized.
func shapedForItsValues(control fyne.CanvasObject) fyne.CanvasObject {
	if menu, is := control.(*Chooser); is {
		return Menu(menu)
	}
	return control
}

// fieldParts is the run of pieces every field is made of, in order.
//
// One place rather than two, because the pair above differed by a single line
// and the pair is where a field's shape would quietly come apart - one of them
// growing a button and the other not.
func fieldParts(label, hint string, detail Detail, required bool, trailing, control fyne.CanvasObject) []fyne.CanvasObject {
	items := []fyne.CanvasObject{headingRow(label, detail, required, trailing), control}
	if hint != "" {
		items = append(items, Note(hint))
	}
	return items
}

// ToggleSaying lays out a switch that carries its own name, with the
// explanation behind the button beside it.
//
// A switch is the one control that does not take a heading above it. Given one
// it arrives as a bare square with the words somewhere else: the name above,
// the sentence below, and nothing to read on the thing you click. That is what
// O72 saw on screen. Putting the name on the switch also makes the words part
// of the target, which is the difference between a click and an aimed click.
// It carries no edge of its own. A ring round a switch is a ring round the
// words as well as the square - measured from a screenshot on 2026-08-12,
// where it read as a box drawn around a sentence - and a switch has two
// positions, neither of which the engine can refuse. What it does get is
// somewhere to speak, because "every field has one" is worth more than the one
// exception nobody would remember.
func ToggleSaying(name, hint string, detail Detail, check *Toggle) (object, body fyne.CanvasObject, area *ErrorArea) {
	check.Text = name
	area = NewErrorArea()
	items := []fyne.CanvasObject{withDetail(WithRoomForItsName(check), detail)}
	if hint != "" {
		items = append(items, Note(hint))
	}
	body = Column(GapTight, items...)
	return Column(GapTight, body, area.Object()), body, area
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
func Note(content string) fyne.CanvasObject {
	label := widget.NewLabel(content)
	label.Wrapping = fyne.TextWrapWord
	label.Importance = widget.LowImportance
	// Smaller as well as quieter. At the same size as the value above it, an
	// explanation of two lines outweighs the field it explains - which is the
	// screen this one was: more words than controls, and the words winning.
	label.SizeName = theme.SizeNameCaptionText
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

	// edge is the line round the control this area belongs to, or nil for the
	// area at the foot of the form, which is about the run rather than about
	// one box. Set by FieldSaying, so a refusal marks and unmarks the box in
	// the same call that shows and hides the sentence - two things that would
	// otherwise be two calls, and the second one is the one somebody forgets on
	// the path where the run succeeds.
	edge *Ring
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
	a.mark(true)
}

// Clear takes the last refusal back, which is what every fresh attempt starts
// with. A message left over from the previous press describes a state that is
// no longer true.
func (a *ErrorArea) Clear() {
	a.label.SetText("")
	a.box.Hide()
	a.mark(false)
}

// mark turns the edge of the control this area belongs to on or off. The area
// at the foot of the form has no control, and says so by having no edge.
func (a *ErrorArea) mark(refused bool) {
	if a.edge != nil {
		a.edge.Refuse(refused)
	}
}

// Text is what the area is currently saying, for a guard to read.
func (a *ErrorArea) Text() string { return a.label.Text }
