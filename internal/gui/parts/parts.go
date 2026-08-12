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
	"fyne.io/fyne/v2/canvas"
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
// It draws its own surface rather than using widget.Card, and that is a
// correction rather than a preference. Measured on 2026-08-12: a card fills
// itself with ColorNameBackground - card.go line 44 - which is the name the
// page uses, so every section came out at exactly the page colour. Zero L*
// apart, with nothing but a shadow at its edge going darker than the page it
// sat on. The grouping this was introduced for did not exist, and no palette
// value could have supplied it, because the toolkit has no name for a panel.
//
// What replaces the card is three ordinary pieces rather than a widget of our
// own: a rectangle behind, padded content in front. That keeps a section a
// plain container, so anything that walks the tree already knows what it is -
// and a walk that does not know one type stops seeing every field below it,
// which is exactly what happened when cards arrived.
func Section(title string, content ...fyne.CanvasObject) fyne.CanvasObject {
	body := make([]fyne.CanvasObject, 0, len(content)+1)
	body = append(body, sectionTitle(title))
	body = append(body, content...)
	return container.NewStack(panelSurface(), container.NewPadded(container.NewVBox(body...)))
}

// sectionTitle names a section, at the rank between the screen and a field.
//
// The size the card used to draw it at, kept deliberately: the scale is four
// ranks and this is the second, so moving off the toolkit's widget must not
// quietly move the type with it.
func sectionTitle(text string) fyne.CanvasObject {
	label := widget.NewLabel(text)
	label.TextStyle = fyne.TextStyle{Bold: true}
	label.SizeName = theme.SizeNameHeadingText
	return label
}

// panelSurface is what a section and the action bar stand on.
//
// The colours come from the palette directly rather than through the installed
// theme, and that is what "one look" means here: this program answers dark
// whatever the desktop says, so a surface asking the theme what variant is in
// force would be asking a question that has one answer. It is also the same
// function the guard measures, so the picture and the measurement cannot come
// apart.
func panelSurface() *canvas.Rectangle {
	rect := canvas.NewRectangle(PaletteColour(ColorNamePanel, theme.VariantDark))
	rect.StrokeColor = PaletteColour(theme.ColorNameSeparator, theme.VariantDark)
	rect.StrokeWidth = 1
	rect.CornerRadius = Theme().Size(theme.SizeNameCardRadius)
	return rect
}

// Bullets is a list of short statements, drawn as a list.
//
// It replaces a run of labels each starting with a dash typed into the string.
// That was a list in the way a paragraph is a list: the marker was text, so it
// sat on the same baseline as the words and wrapped with them, and each item
// was a separate label carrying a label's full spacing - which put more air
// between the items than between the list and the things around it.
//
// The marker is its own column, so a wrapped item hangs under its own text
// rather than under the marker. These items wrap: one of them is a sentence
// about MB against MiB that runs past the width of this card.
func Bullets(items []string) fyne.CanvasObject {
	rows := make([]fyne.CanvasObject, 0, len(items))
	for _, item := range items {
		marker := widget.NewLabel(bulletMarker)
		marker.Importance = widget.LowImportance
		marker.SizeName = theme.SizeNameCaptionText
		rows = append(rows, container.NewBorder(nil, nil, marker, nil, Note(item)))
	}
	return container.NewVBox(rows...)
}

// bulletMarker is what stands in front of one item.
//
// Not a hyphen. D17 keeps the flat hyphen for prose because an en dash is the
// thing being banned, and this is not prose - it is the marker of a list, where
// a hyphen reads as a word that lost its other half.
const bulletMarker = "•"

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
// buttons and through their labels. It uses the same surface as a section, so
// the fix for an invisible card fixed this at the same time - the bar had been
// drawing itself in the page colour too, which is to say not drawing itself.
//
// The surface runs the whole width and what stands on it does not, which is
// the one place those two differ on purpose. A bar pinned across the foot is
// what makes it read as a bar, and the buttons and the run's own messages line
// up with the form above them instead of starting where the window happens to
// begin. Until 2026-08-12 they did the latter: the form stopped at 822 px and
// a refusal about it ran to 1099.
func ActionBar(content ...fyne.CanvasObject) fyne.CanvasObject {
	standing := container.New(readableWidth{}, container.NewVBox(content...))
	return container.NewStack(panelSurface(), container.NewPadded(standing))
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

// NumericWidth is as wide as a box holding a number gets.
//
// A box is a promise about what goes in it, and one that runs half the window
// while holding "0" promises something the field cannot take. Measured on
// 2026-08-12 before this existed: the seed and the count were 397 and 399 px
// wide for a single digit, because a column split in two hands each half to
// whatever is in it.
//
// 140 px holds eleven digits at the text size this window uses, which covers
// every number any of these fields accepts - the ceiling on files is seven
// digits and the largest size anybody types is eight.
const NumericWidth = 140

// Numeric sizes a control to what it holds rather than to the column it is in.
//
// Only for boxes taking a number. A path, a name template and an id are all
// things whose length nobody can predict, so those still take the column.
func Numeric(control fyne.CanvasObject) fyne.CanvasObject {
	return container.NewGridWrap(fyne.NewSize(NumericWidth, control.MinSize().Height), control)
}

// readableWidth gives its one child the lesser of the space offered and
// ColumnWidth, centred. A VBox stretches its children to whatever it is
// given, which is the whole window, and that is the entire defect.
//
// Centred rather than left aligned, changed on 2026-08-12. Held at the left it
// traded one kind of waste for another: at 1100 px the form ended at 822 and
// left 278 px of nothing down the right hand side, and maximised it left three
// thousand. Space split either side reads as a margin, and the same space all
// on one side reads as a column that failed to fill the window.
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
	width := fyne.Min(size.Width, ColumnWidth)
	objects[0].Resize(fyne.NewSize(width, size.Height))
	objects[0].Move(fyne.NewPos((size.Width-width)/2, 0))
}
