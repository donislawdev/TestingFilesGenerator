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
// It is built once per visible row and refilled as the list scrolls, which is
// what makes the list cost the same at thirteen values and at a hundred
// thousand. Measured in tools/probes/fynelist on 2026-08-18: widget.List is
// flat at 0.1 MB across 1000, 10 000 and 100 000 rows, where a box holding
// every row costs 449 MB at the largest.
type ListRow struct {
	widget.BaseWidget

	label  string
	marked bool
	active bool
	onTap  func()
	// kind is the picture drawn in front of the words, or nil for a list whose
	// values are not things of different kinds.
	kind fyne.Resource

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

func (r *ListRow) MouseIn(*desktop.MouseEvent) {
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
	back.CornerRadius = Theme().Size(theme.SizeNameInputRadius)
	tick := canvas.NewImageFromResource(theme.ConfirmIcon())
	kind := canvas.NewImageFromResource(nil)
	label := canvas.NewText("", Theme().Color(theme.ColorNameForeground, theme.VariantDark))
	label.TextSize = Theme().Size(theme.SizeNameText)
	rr := &listRowRenderer{row: r, back: back, tick: tick, kind: kind, label: label}
	rr.Refresh()
	return rr
}

type listRowRenderer struct {
	row   *ListRow
	back  *canvas.Rectangle
	tick  *canvas.Image
	kind  *canvas.Image
	label *canvas.Text
}

func (r *listRowRenderer) Layout(size fyne.Size) {
	icon := Theme().Size(theme.SizeNameInlineIcon)
	r.back.Resize(size)
	r.tick.Resize(fyne.NewSquareSize(icon))
	r.tick.Move(fyne.NewPos(rowGutter, (size.Height-icon)/2))

	// The kind sits between the tick and the words, and takes no room at all
	// where there is none - so a list of paper sizes is drawn exactly as it was.
	left := rowGutter + icon + rowGap
	if r.row.kind != nil {
		r.kind.Resize(fyne.NewSquareSize(icon))
		r.kind.Move(fyne.NewPos(left, (size.Height-icon)/2))
		left += icon + rowGap
	} else {
		r.kind.Resize(fyne.NewSquareSize(0))
	}

	text := r.label.MinSize()
	r.label.Move(fyne.NewPos(left, (size.Height-text.Height)/2))
	r.label.Resize(fyne.NewSize(size.Width-left-rowGutter, text.Height))
}

func (r *listRowRenderer) MinSize() fyne.Size {
	return fyne.NewSize(RowWidthFor(r.label.MinSize().Width, r.row.kind != nil), ListRowHeight())
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
	width := rowGutter + icon + rowGap + word + rowGutter
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

	switch {
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

	r.back.Refresh()
	r.tick.Refresh()
	r.kind.Refresh()
	r.label.Refresh()
	// The width a row asks for changes with the picture, and the row is laid
	// out by the list rather than by this renderer.
	r.Layout(r.row.Size())
}

func (r *listRowRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.back, r.tick, r.kind, r.label}
}

func (r *listRowRenderer) Destroy() {}
