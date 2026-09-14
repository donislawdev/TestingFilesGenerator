package parts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Field is one labelled control, with the longer explanation behind a button
// beside its name.
//
// Two lengths rather than one, split on 2026-08-12. The sentence used to be
// whole and permanent, so a form of eight settings carried eight grey
// paragraphs and the explanations took more vertical room than the controls
// did - measured on the generate screen, where the help outweighed the fields
// it was helping. Since 2026-08-25 the line that says what a field does is the
// first line of the explanation, so at rest a field is its name and its box.
//
// It is a button rather than a tooltip because the toolkit has no tooltips -
// issue 1650 is still open - and it is visible rather than a hover because a
// hover is not reachable from a keyboard, which is UX9. A hint the widget never
// renders is the shape docs/CLAUDE.md warns about: text with no reader becomes
// pressure on the text beside it, and the label starts trying to say
// everything on its own.
//
// The plain Field function is gone as of 2026-08-12, and its absence is the
// fix rather than a side effect of one. It built a field with nowhere to put a
// refusal about it, so which boxes could be marked depended on which of two
// functions somebody typed - and three of eight on one screen got the wrong
// one, along with every setting a format declares. Fields.Add is the only way
// in now. See parts.Fields.

// FieldSaying is a field that can carry a refusal of its own, underneath it.
//
// One row: the name in the column of names, the control beside it, and
// whatever the control has to say about its value - the count of bytes a
// size comes to - after the control. See fieldRow for why a row and not a
// name over a box.
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
func FieldSaying(names float32, label string, detail Detail, required bool, trailing, control fyne.CanvasObject) (object, body fyne.CanvasObject, area *ErrorArea) {
	marked, ring := WithRing(shapedForItsValues(control))
	area = newErrorArea(names)
	area.edge = ring
	cells := []fyne.CanvasObject{headingRow(label, detail, required), marked}
	if trailing != nil {
		cells = append(cells, trailing)
	}
	body = FieldRow(names, cells...)
	return Column(GapTight, body, area.Object()), body, area
}

// CellSaying is a field drawn as a cell of a table: the name over the control,
// for a list of rows that all have the same columns. A row of the form puts
// the name beside the control - see FieldSaying - and a table puts the names
// once, over the columns, where three files inside an archive would otherwise
// carry nine names for three kinds of value.
func CellSaying(label string, detail Detail, required bool, control fyne.CanvasObject) (object, body fyne.CanvasObject, area *ErrorArea) {
	marked, ring := WithRing(shapedForItsValues(control))
	area = newErrorArea(0)
	area.edge = ring
	body = Column(GapLabel, headingRow(label, detail, required), marked)
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

// ToggleSaying lays out a switch that carries its own name, with the
// explanation behind the button beside it.
//
// A switch is the one control that does not take a name in the column of
// names. Given one it arrives as a bare square with the words somewhere else,
// and nothing to read on the thing you click - which is what O72 saw on
// screen. Putting the name on the switch makes the words part of the target,
// which is the difference between a click and an aimed click. So the name
// cell of its row is empty and the switch stands in the column of controls,
// where the eye is already looking for the thing to change.
//
// It carries no edge of its own. A ring round a switch is a ring round the
// words as well as the square - measured from a screenshot on 2026-08-12,
// where it read as a box drawn around a sentence - and a switch has two
// positions, neither of which the engine can refuse. What it does get is
// somewhere to speak, because "every field has one" is worth more than the one
// exception nobody would remember.
func ToggleSaying(names float32, name string, detail Detail, check *Toggle) (object, body fyne.CanvasObject, area *ErrorArea) {
	check.Text = name
	area = newErrorArea(names)
	body = FieldRow(names, Clear(), withDetail(WithRoomForItsName(check), detail))
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
	label.SizeName = theme.SizeNameCaptionText
	return inkTight(label)
}

// ErrorArea is where a field says what a run said about it.
type ErrorArea struct {
	label *widget.Label
	box   fyne.CanvasObject
	// edge is the line round the control, if it has one. Marked and cleared
	// with the sentence, so the two never disagree.
	edge *Ring
}

// NewErrorArea builds one that stands on its own, for the line at the foot of
// the form that speaks for the whole run.
func NewErrorArea() *ErrorArea { return newErrorArea(0) }

// newErrorArea builds one for a field whose names stand in a column that
// wide, so the sentence starts under the control it is about rather than
// under the name. Nought for a cell of a table, where the sentence starts
// under the cell.
func newErrorArea(names float32) *ErrorArea {
	label := widget.NewLabel("")
	label.Wrapping = fyne.TextWrapWord
	label.Importance = widget.DangerImportance
	var box fyne.CanvasObject
	if names > 0 {
		box = FieldRow(names, Clear(), inkTight(label))
	} else {
		box = container.NewVBox(inkTight(label))
	}
	area := &ErrorArea{label: label, box: box}
	area.Clear()
	return area
}

// Object is what the area puts on the screen.
func (a *ErrorArea) Object() fyne.CanvasObject { return a.box }

// Say shows a sentence, or clears the area when handed nothing.
func (a *ErrorArea) Say(text string) {
	if text == "" {
		a.Clear()
		return
	}
	a.label.SetText(text)
	a.box.Show()
	a.mark(true)
}

// Clear takes the sentence away and gives the room back.
func (a *ErrorArea) Clear() {
	a.label.SetText("")
	a.box.Hide()
	a.mark(false)
}

func (a *ErrorArea) mark(refused bool) {
	if a.edge != nil {
		a.edge.Refuse(refused)
	}
}

// Text is what the area says, for the guards that read it back.
func (a *ErrorArea) Text() string { return a.label.Text }
