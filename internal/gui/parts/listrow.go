package parts

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ListRow is one value in an open list.
//
// Its own widget rather than a container, for two things a container cannot do:
// answer the pointer itself, so a row is taken by pressing anywhere along it,
// and draw its own surface for the three states a row has - plain, under the
// pointer, and holding the keyboard.
//
// It is built once per position of the list and refilled when what the list
// holds changes. Once per VISIBLE row until 2026-09-23, inside widget.List,
// which is what made a list cost the same at thirteen values and at a hundred
// thousand - measured in tools/probes/fynelist on 2026-08-18, flat at 0.1 MB
// where a box holding every row costs 449 MB at the largest. Why every
// position now, and when that stops being right, is in rowView.
type ListRow struct {
	widget.BaseWidget

	label  string
	marked bool
	active bool
	onTap  func()
	// kind is the picture drawn in front of the words, or nil for a list whose
	// values are not things of different kinds.
	kind fyne.Resource
	// heading says this row names the kind of the values under it, or says
	// that the filter left nothing. It draws its words and nothing else, and
	// answers neither the pointer nor a press.
	heading bool
	// from and to are where what was typed into the list's filter stands in
	// the words, drawn in bold. Equal when nothing is to be bold.
	from, to int

	hovered bool
}

// Label and Marked are what this row is DRAWING, for a guard.
//
// Exported so that the question can be asked of the screen rather than of the
// list. Asking the list what it would mark passes for a list that computes the
// right answer and draws something else - measured on 2026-08-18, when a
// mutation blanking the drawn mark left the guard green because the guard was
// reading a second copy of the rule.
func (r *ListRow) Label() string { return r.label }

// Marked says whether this row carries the tick that means "this is the value
// in the box".
func (r *ListRow) Marked() bool { return r.marked }

// Kind is the picture this row is drawing in front of its words, for a guard.
// Nil where the list does not sort its values into kinds.
func (r *ListRow) Kind() fyne.Resource { return r.kind }

// Heading says whether this row is a heading or the notice that nothing
// matched, rather than a value somebody can take - for a guard.
func (r *ListRow) Heading() bool { return r.heading }

func newListRow() *ListRow {
	r := &ListRow{}
	r.ExtendBaseWidget(r)
	return r
}

func (r *ListRow) Tapped(*fyne.PointEvent) {
	if r.onTap != nil {
		r.onTap()
	}
}

// MouseIn lights a value up under the pointer. A heading stays dark: lit, it
// would say a press there takes something.
func (r *ListRow) MouseIn(*desktop.MouseEvent) {
	if r.heading {
		return
	}
	r.hovered = true
	r.Refresh()
}

func (r *ListRow) MouseOut() {
	r.hovered = false
	r.Refresh()
}

func (r *ListRow) MouseMoved(*desktop.MouseEvent) {}

func (r *ListRow) CreateRenderer() fyne.WidgetRenderer {
	back := canvas.NewRectangle(color.Transparent)
	back.CornerRadius = RadiusField
	tick := canvas.NewImageFromResource(theme.ConfirmIcon())
	kind := canvas.NewImageFromResource(nil)
	label := canvas.NewText("", Theme().Color(theme.ColorNameForeground, theme.VariantDark))
	label.TextSize = Theme().Size(theme.SizeNameText)
	strong := canvas.NewText("", Theme().Color(theme.ColorNameForeground, theme.VariantDark))
	strong.TextStyle = fyne.TextStyle{Bold: true}
	rest := canvas.NewText("", Theme().Color(theme.ColorNameForeground, theme.VariantDark))
	rr := &listRowRenderer{row: r, back: back, tick: tick, kind: kind, label: label, strong: strong, rest: rest}
	rr.Refresh()
	return rr
}

// listRowRenderer draws a row's words as up to three pieces: label holds all
// of them, or only what comes before the part that matched the filter, which
// strong draws in bold, with rest after it. One text when nothing matched, so
// a list nobody typed into draws exactly what it drew before the filter.
type listRowRenderer struct {
	row    *ListRow
	back   *canvas.Rectangle
	tick   *canvas.Image
	kind   *canvas.Image
	label  *canvas.Text
	strong *canvas.Text
	rest   *canvas.Text
}

func (r *listRowRenderer) Layout(size fyne.Size) {
	icon := Theme().Size(theme.SizeNameInlineIcon)
	r.back.Resize(size)
	r.tick.Resize(fyne.NewSquareSize(icon))

	// Two shapes of row, decided by whether the list draws pictures, and both
	// are the owner's, from the running window.
	//
	// A row WITHOUT a picture puts its words at the gutter - where the word
	// in the box above the list starts - and the tick at the far end. Until
	// 2026-09-16 the tick was in front, and its column was kept whether or
	// not anything in the list was ticked, so the words of every list stood a
	// column to the right of the word in the box, and a list with no picture
	// and nothing chosen read as words floating in a rectangle (O220).
	//
	// A row WITH a picture keeps the tick in front, then the picture, then
	// the words - the shape the list of formats had before that day, which is
	// the shape the owner had said looked right. Moving its tick to the end
	// with the others pulled the picture and the word a column to the left,
	// and the report of 2026-09-21 was that the list had been broken: what
	// stood in the middle of the box now hugged its edge. The column in front
	// makes the picture and the word sit where they did, and the tick fills
	// it or leaves it empty without the row changing width.
	//
	// Either way the row is the same width for a chosen value as for any
	// other, because the tick's column is kept in both shapes.
	left, right := float32(rowGutter), float32(rowGutter)
	if r.row.heading {
		// A heading is its words at the gutter, as wide as the row. It has no
		// tick column: nothing under it is chosen, and a column kept empty in
		// front of a heading would move it off the edge the values' ticks
		// stand on.
		r.tick.Move(fyne.NewPos(left, (size.Height-icon)/2))
		r.kind.Resize(fyne.NewSquareSize(0))
		text := r.label.MinSize()
		r.label.Move(fyne.NewPos(left, (size.Height-text.Height)/2))
		r.label.Resize(fyne.NewSize(size.Width-left-right, text.Height))
		return
	}
	if r.row.kind != nil {
		r.tick.Move(fyne.NewPos(left, (size.Height-icon)/2))
		left += icon + rowGap
		r.kind.Resize(fyne.NewSquareSize(icon))
		r.kind.Move(fyne.NewPos(left, (size.Height-icon)/2))
		left += icon + rowGap
	} else {
		r.tick.Move(fyne.NewPos(size.Width-rowGutter-icon, (size.Height-icon)/2))
		right += icon + rowGap
		r.kind.Resize(fyne.NewSquareSize(0))
	}

	r.placeWords(left, size.Width-left-right, size.Height)
}

// placeWords lays the pieces of the words end to end from left, the last of
// them taking what is left of the row.
func (r *listRowRenderer) placeWords(left, room, height float32) {
	text := r.label.MinSize()
	y := (height - text.Height) / 2
	if !r.strong.Visible() {
		r.label.Move(fyne.NewPos(left, y))
		r.label.Resize(fyne.NewSize(room, text.Height))
		return
	}
	before, match := text.Width, r.strong.MinSize().Width
	r.label.Move(fyne.NewPos(left, y))
	r.label.Resize(fyne.NewSize(before, text.Height))
	r.strong.Move(fyne.NewPos(left+before, y))
	r.strong.Resize(fyne.NewSize(match, text.Height))
	r.rest.Move(fyne.NewPos(left+before+match, y))
	r.rest.Resize(fyne.NewSize(fyne.Max(0, room-before-match), text.Height))
}

// splitWords cuts the words where the filter matched them, or leaves them
// whole. A span that does not fit the words - a row refilled with a shorter
// value - is no span, rather than a slice out of range.
func (r *listRowRenderer) splitWords() {
	words, from, to := r.row.label, r.row.from, r.row.to
	if r.row.heading || from < 0 || to <= from || to > len(words) {
		r.strong.Text, r.rest.Text = "", ""
		r.strong.Hide()
		r.rest.Hide()
		return
	}
	r.label.Text, r.strong.Text, r.rest.Text = words[:from], words[from:to], words[to:]
	for _, piece := range []*canvas.Text{r.strong, r.rest} {
		piece.Color = r.label.Color
		piece.TextSize = r.label.TextSize
		piece.Show()
	}
}

// wordsWidth is how wide the words are drawn, all pieces together.
func (r *listRowRenderer) wordsWidth() float32 {
	width := r.label.MinSize().Width
	if r.strong.Visible() {
		width += r.strong.MinSize().Width + r.rest.MinSize().Width
	}
	return width
}

func (r *listRowRenderer) MinSize() fyne.Size {
	if r.row.heading {
		return fyne.NewSize(HeadingRowWidthFor(r.label.MinSize().Width), ListRowHeight())
	}
	return fyne.NewSize(RowWidthFor(r.wordsWidth(), r.row.kind != nil), ListRowHeight())
}

// headingText and headingStyle are how a heading in an open list is drawn:
// the caption size, in bold. Named once, because the menu that opens the list
// measures a heading with them to know how wide the box has to be.
const headingText = TextCaption

var headingStyle = fyne.TextStyle{Bold: true}

// HeadingRowWidthFor is the room a heading row needs for words that wide:
// the words between two gutters and nothing else.
func HeadingRowWidthFor(words float32) float32 {
	return rowGutter + words + rowGutter
}

// headingWidth is how wide one heading's words are drawn.
func headingWidth(heading string) float32 {
	return fyne.MeasureText(heading, headingText, headingStyle).Width
}

// RowWidthFor is the room one row of an open list needs for a word that wide.
//
// Here rather than counted twice, because the menu that OPENS this list has to
// be at least this wide - the list is drawn at the width of the box. Two copies
// of this arithmetic is what the sizing defect of 2026-08-25 was, and it came
// back on 2026-08-27 the moment two more menus started drawing pictures: the
// preset screen's format menu was sized for its longest word, the row put a
// picture in front of that word, and "targz" got a 14 px slot for 34 px of
// name. Every one of the twenty formats was cut off, in the list that exists to
// show them.
//
// The term for the picture was in menuWidth until 2026-08-25 and was taken out
// the same hour, correctly: measured across all six menus then in the window,
// the closed box was the wider of the two every time, so the term defended
// nothing and this project does not keep defences nothing can turn red. The
// comment left in its place said what would bring it back - "the day a row
// grows another thing in front of the word, that goes red and this gets a term
// with a number behind it". That day was 2026-08-27 and the guard did go red
// first.
func RowWidthFor(word float32, withKind bool) float32 {
	icon := Theme().Size(theme.SizeNameInlineIcon)
	width := rowGutter + word + rowGutter + icon + rowGap
	if withKind {
		width += icon + rowGap
	}
	return width
}

// Refresh draws the three states a row has. The order is deliberate: the
// keyboard wins over the pointer, because a row somebody is hovering while the
// keyboard sits elsewhere would otherwise show two rows as the current one.
func (r *listRowRenderer) Refresh() {
	r.label.Text = r.row.label
	r.kind.Resource = r.row.kind
	r.label.Color = Theme().Color(theme.ColorNameForeground, theme.VariantDark)
	r.label.TextSize = Theme().Size(theme.SizeNameText)
	r.label.TextStyle = fyne.TextStyle{}
	if r.row.heading {
		// The look a field's name has - a step quieter than a value - in
		// bold at the caption size, so a heading reads as the name over a
		// group and not as one more value to take.
		r.label.Color = PaletteColour(ColorNameLabel, theme.VariantDark)
		r.label.TextSize = headingText
		r.label.TextStyle = headingStyle
	}
	r.splitWords()

	switch {
	case r.row.heading:
		r.back.FillColor = color.Transparent
	case r.row.active:
		r.back.FillColor = Theme().Color(theme.ColorNameSelection, theme.VariantDark)
	case r.row.hovered:
		r.back.FillColor = Theme().Color(theme.ColorNameHover, theme.VariantDark)
	default:
		r.back.FillColor = color.Transparent
	}

	if r.row.marked {
		r.tick.Show()
	} else {
		r.tick.Hide()
	}
	// Shown and hidden through the widget rather than by writing Hidden, which
	// is what the first version did - the field is set and nothing repaints, so
	// the picture was in the tree and never on the screen.
	if r.row.kind != nil {
		r.kind.Show()
	} else {
		r.kind.Hide()
	}

	redraw(r.back, r.tick, r.kind, r.label, r.strong, r.rest)
	// The width a row asks for changes with the picture, and the row is laid
	// out by the list rather than by this renderer.
	r.Layout(r.row.Size())
}

func (r *listRowRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.back, r.tick, r.kind, r.label, r.strong, r.rest}
}

func (r *listRowRenderer) Destroy() {}
