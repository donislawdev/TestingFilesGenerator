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
func FieldSaying(label string, detail Detail, required bool, trailing, control fyne.CanvasObject) Built {
	marked, ring := WithRing(shapedForItsValues(control))
	area := NewErrorArea()
	area.edge = ring
	pieces := []fyne.CanvasObject{headingRow(label, detail, required), marked}
	if trailing != nil {
		pieces = append(pieces, trailing)
	}
	body := FieldStack(pieces...)
	return Built{Object: cellOf(body, area.Object()), Body: body, Area: area}
}

// Built is what building a field comes to, in one hand: the whole thing to
// put on a screen, the part of it without the room for a refusal, and the
// refusal area itself. One value rather than three results, so the registry
// takes one argument for what was built and the two builders cannot come to
// hand it back in two different orders.
type Built struct {
	// Object is the field whole, with the room under it for a refusal.
	Object fyne.CanvasObject
	// Body is the field without that room, so a table row can lay every
	// refusal in it across the whole width.
	Body fyne.CanvasObject
	// Area is where the field says what a run said about it.
	Area *ErrorArea
}

// CellSaying is a field drawn as a cell of a table: the control alone, for a
// list of rows that all have the same columns. A row of the form puts the name
// beside the control - see FieldSaying - and a table puts the names once,
// over the columns, in Table.Header.
//
// The sentence above was true of the intent and false of the drawing until
// 2026-09-16: every cell carried its own name over its control, so three
// files inside an archive read as nine names for three kinds of value, and
// the second row of the table repeated the first row's names a control's
// height below them. The name is still registered with the field - a refusal
// names the box by it - it is just not drawn here.
func CellSaying(control fyne.CanvasObject) Built {
	marked, ring := WithRing(shapedForItsValues(control))
	area := NewErrorArea()
	area.edge = ring
	return Built{Object: Column(GapTight, marked, area.Object()), Body: marked, Area: area}
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
// It is QuietText since 2026-09-23 and was a quiet label before, which is O213:
// widget.LowImportance draws in ColorNameDisabled, #C2C8CD at 80 L*, a step
// brighter than the hint a caption is meant to be. QuietText draws the
// placeholder, #9DA3A8 at 66.7 L*, 5.61:1 on a panel. O70 and O71.
func Note(content string) fyne.CanvasObject {
	note := NewQuietText(content)
	note.Wrapping = fyne.TextWrapWord
	note.SizeName = theme.SizeNameCaptionText
	return note
}

// ErrorArea is where a field says what a run said about it.
type ErrorArea struct {
	label *widget.Label
	box   fyne.CanvasObject
	// edge is the line round the control, if it has one. Marked and cleared
	// with the sentence, so the two never disagree.
	edge *Ring
	// fix is a button under the sentence that puts right what it is about,
	// when the refusal carries the value that would - see Offer. fixRow is
	// what is shown and hidden, so a field with nothing to offer keeps no
	// room for a button.
	fix    *Button
	fixRow fyne.CanvasObject
}

// NewErrorArea builds one that stands on its own, for the line at the foot of
// the form that speaks for the whole run.
func NewErrorArea() *ErrorArea {
	label := widget.NewLabel("")
	label.Wrapping = fyne.TextWrapWord
	label.Importance = widget.DangerImportance
	// The sentence starts under the control it is about, on the edge the
	// control starts on - which is the field's edge since the name stands
	// over the control rather than beside it.
	fix := NewButton(Secondary, "", nil)
	fix.Hide()
	fixRow := container.NewHBox(fix)
	area := &ErrorArea{label: label, fix: fix, fixRow: fixRow,
		box: Column(GapLabel, inkTight(label), fixRow)}
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
	a.hideFix()
	a.box.Show()
	a.mark(true)
}

// Offer puts a button under the sentence that does what the sentence asks
// for, so nobody has to copy a number out of a refusal into the box above it.
// Called after Say, and taken away by the next Say or Clear.
func (a *ErrorArea) Offer(label string, apply func()) {
	a.fix.SetText(label)
	a.fix.OnTapped = apply
	a.fix.Show()
	a.fixRow.Show()
}

// hideFix takes the button away with its row. The button itself as well,
// because a button still marked visible inside a hidden row is a button a
// guard walking the tree counts as there to press, at 0x0.
func (a *ErrorArea) hideFix() {
	a.fix.Hide()
	a.fixRow.Hide()
}

// Offered is the words on the button under the sentence, or nothing, for a
// guard.
func (a *ErrorArea) Offered() string {
	if !a.fixRow.Visible() {
		return ""
	}
	return a.fix.Text
}

// Clear takes the sentence away and gives the room back.
func (a *ErrorArea) Clear() {
	a.label.SetText("")
	a.hideFix()
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
