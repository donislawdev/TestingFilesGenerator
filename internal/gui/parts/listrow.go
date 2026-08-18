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
	label := canvas.NewText("", Theme().Color(theme.ColorNameForeground, theme.VariantDark))
	label.TextSize = Theme().Size(theme.SizeNameText)
	rr := &listRowRenderer{row: r, back: back, tick: tick, label: label}
	rr.Refresh()
	return rr
}

type listRowRenderer struct {
	row   *ListRow
	back  *canvas.Rectangle
	tick  *canvas.Image
	label *canvas.Text
}

func (r *listRowRenderer) Layout(size fyne.Size) {
	icon := Theme().Size(theme.SizeNameInlineIcon)
	r.back.Resize(size)
	r.tick.Resize(fyne.NewSquareSize(icon))
	r.tick.Move(fyne.NewPos(rowGutter, (size.Height-icon)/2))
	text := r.label.MinSize()
	r.label.Move(fyne.NewPos(rowGutter+icon+rowGap, (size.Height-text.Height)/2))
	r.label.Resize(fyne.NewSize(size.Width-rowGutter-icon-rowGap, text.Height))
}

func (r *listRowRenderer) MinSize() fyne.Size {
	icon := Theme().Size(theme.SizeNameInlineIcon)
	text := r.label.MinSize()
	return fyne.NewSize(rowGutter+icon+rowGap+text.Width+rowGutter, ListRowHeight())
}

// Refresh draws the three states a row has. The order is deliberate: the
// keyboard wins over the pointer, because a row somebody is hovering while the
// keyboard sits elsewhere would otherwise show two rows as the current one.
func (r *listRowRenderer) Refresh() {
	r.label.Text = r.row.label
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

	r.back.Refresh()
	r.tick.Refresh()
	r.label.Refresh()
}

func (r *listRowRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.back, r.tick, r.label}
}

func (r *listRowRenderer) Destroy() {}
