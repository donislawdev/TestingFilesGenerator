// Package parts holds the pieces every window is built from.
//
// It exists because of what this interface is going to become rather than what
// it is today: many windows, many fields, many buttons, the same shapes over
// and over. A window that builds its own labelled entry is a window that will
// word its own error, size its own gap and validate its own value - and the
// third of those breaks G1 in the way nobody sees, because a form with its own
// rule is a second copy of rules the engine already owns.
//
// Two properties hold this package together.
//
// It knows nothing about windows. Nothing here opens, closes or navigates, so
// a part can be rendered on its own - which is what lets the golden images sit
// on parts rather than on whole screens. An image of a whole window changes
// with every layout change and stops being read after the third time. An image
// of one field in four states is stable and says something.
//
// It never reaches the toolkit's app package. Everything here builds a widget
// tree and nothing drives one, so this package compiles and tests with
// CGO_ENABLED=0, on a runner with no graphics and no C compiler.
package parts

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Prose renders a block of text that a person reads rather than edits.
//
// Wrapping rather than truncating, and that is G9 rather than taste. An error
// in this tool has four parts - what happened, why, what is allowed, what to do
// instead - and a widget that shows one line forces a message that has one of
// the four. The rule in docs/GUI.md is a requirement on the layout, so it is
// answered here once instead of in every window that shows a sentence.
func Prose(text string) fyne.CanvasObject {
	label := widget.NewLabel(text)
	label.Wrapping = fyne.TextWrapWord
	return label
}

// Heading is the name of one field, above its control.
func Heading(text string) fyne.CanvasObject {
	label := widget.NewLabel(text)
	label.TextStyle = fyne.TextStyle{Bold: true}
	return label
}

// Title is the one line that says what a screen is for.
//
// Bigger than a field's name rather than the same size in the same weight,
// which is what it was until 2026-08-11: the title of the screen and the label
// of every field were one style, so nothing led the eye and the first point of
// the UX section 7 checklist - squint, and see what stands out - had no answer.
func Title(text string) fyne.CanvasObject {
	label := widget.NewLabel(text)
	label.TextStyle = fyne.TextStyle{Bold: true}
	label.SizeName = theme.SizeNameSubHeadingText
	return label
}

// Section groups fields that answer one question, under a name.
//
// A card rather than a run of fields with a bold line above them. The screens
// were one long column with no grouping, so a form of eight settings read as
// eight unrelated things - and the toolkit's own card is what draws a surface
// with a title on it, which means no custom widget to keep working for years.
func Section(title string, content ...fyne.CanvasObject) fyne.CanvasObject {
	return widget.NewCard(title, "", container.NewVBox(content...))
}

// Row puts fields side by side, for the ones that are read together.
//
// Size and how many are one thought, and so are the id and the name template.
// Stacked, each took a full width it did not need and pushed the next one off
// the screen.
func Row(fields ...fyne.CanvasObject) fyne.CanvasObject {
	return container.NewGridWithColumns(len(fields), fields...)
}

// ActionBar is the strip that stays put while the form scrolls under it.
//
// On a surface of its own rather than floating, and that is not decoration:
// pinned over a transparent background the scrolling text ran underneath the
// buttons and through their labels. A card is what the toolkit draws a surface
// with, and it is what every section on the screen above already is.
func ActionBar(content ...fyne.CanvasObject) fyne.CanvasObject {
	return widget.NewCard("", "", container.NewVBox(content...))
}

// Screen stacks sections with a heading on top.
//
// Windows compose sections rather than laying themselves out in one function.
// That is not tidiness: the shape gate caps a function at eighty lines of
// logic and window layout is long by nature, so a window written as one
// function would arrive as an argument for raising the cap. The cap is a
// ratchet and only goes down, so the composition has to come first.
func Screen(heading string, sections ...fyne.CanvasObject) fyne.CanvasObject {
	all := make([]fyne.CanvasObject, 0, len(sections)+1)
	all = append(all, Title(heading))
	all = append(all, sections...)
	return container.New(readableWidth{}, container.NewVBox(all...))
}

// ColumnWidth is as wide as this form is allowed to get, whatever the window
// does. O72, measured on 2026-08-10 and again on 2026-08-11: maximised to
// 3862 px, every box was 3848 to 3854 px of it - 99.7 per cent - so the seed
// field holding "0" was nearly four thousand pixels wide. UX6 puts it as a
// question rather than a rule: run your eye along a row to the right edge, and
// if you got lost the row is too long.
//
// 820 comes from the longest sentence the form actually holds, which ends at
// 797 px - the hint under the self describing label - so nothing rewraps and
// this change only stops the stretching. It is not a claim about the ideal
// measure: prose is easiest at 45 to 75 characters a line and 820 px is about
// 112, so the typography pass has room to tighten this. It cannot widen it.
const ColumnWidth = 820

// readableWidth gives its one child the lesser of the space offered and
// ColumnWidth, at the left. A VBox stretches its children to whatever it is
// given, which is the whole window, and that is the entire defect.
type readableWidth struct{}

func (readableWidth) MinSize(objects []fyne.CanvasObject) fyne.Size {
	if len(objects) == 0 {
		return fyne.Size{}
	}
	size := objects[0].MinSize()
	// The height is the child's and is never capped. Only the width is a
	// choice - a form too tall scrolls, a form too wide cannot be read.
	size.Width = fyne.Min(size.Width, ColumnWidth)
	return size
}

func (readableWidth) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	if len(objects) == 0 {
		return
	}
	objects[0].Resize(fyne.NewSize(fyne.Min(size.Width, ColumnWidth), size.Height))
	objects[0].Move(fyne.NewPos(0, 0))
}
