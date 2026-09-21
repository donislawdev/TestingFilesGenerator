package parts

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// FoldHead is what the head row of a fold answers to: the pointer, anywhere
// on the row, and the keyboard, as one stop.
//
// Measured on 2026-09-16 (O221): a press on the title of a fold did nothing,
// and only the arrow after it opened the section, because the arrow was the
// control and the title was words beside it - two presses on "Notes for the
// manifest" and the section stayed shut. The owner's decision is that the
// whole row is one target. So this is one control the width of the row: the
// title, the arrow, the line said while the fold is shut, and the room to
// the right of them up to the buttons a batch keeps in its head. The arrow
// is a mark on it rather than a button of its own, which is also why the
// row has one stop for the keyboard rather than the two a button inside a
// row would make.
//
// It draws only its own state and holds no words. A fill under the pointer
// and a ring for the keyboard, under the row's content in a stack - WithRing
// the other way up. The title stays a plain object in a plain container on
// purpose: every guard that reads titles walks containers and stops at a
// widget it was not told about, so a title inside this renderer would be a
// title nothing reads (O118). The arrow is this control's to colour, so it
// follows the row's state the way a glyph button's mark follows its own.
type FoldHead struct {
	widget.BaseWidget

	// under is the words this head stands under - the title, the arrow and
	// the line - so the fill and the ring are drawn to their width rather
	// than across the whole row. Owner's report from the running window,
	// 2026-09-21: a hover the width of the form is enormous. The row stays
	// the target and only the drawing narrows.
	under fyne.CanvasObject

	fold  *Folding
	arrow *widget.Icon

	hovered bool
	marked  bool
	from    PointerFocus
}

var (
	_ fyne.Tappable     = (*FoldHead)(nil)
	_ fyne.Focusable    = (*FoldHead)(nil)
	_ desktop.Hoverable = (*FoldHead)(nil)
)

func newFoldHead(fold *Folding, arrow *widget.Icon) *FoldHead {
	h := &FoldHead{fold: fold, arrow: arrow}
	h.ExtendBaseWidget(h)
	h.drawArrow()
	return h
}

// Title is the words in the head, for a guard that finds a fold by them.
func (h *FoldHead) Title() string { return h.fold.title }

// Open says whether the fold this heads is open.
func (h *FoldHead) Open() bool { return h.fold.open }

// Hovered and Marked report the drawn state, for a guard.
func (h *FoldHead) Hovered() bool { return h.hovered }
func (h *FoldHead) Marked() bool  { return h.marked }

// Tapped opens the fold or puts it away. The keyboard comes to the row
// quietly first, so the mark meaning "the keyboard is here" is not drawn for
// somebody using the mouse - see PointerFocus. A press on a row that holds
// the keyboard already, with the mark drawn, leaves the mark: that is what
// the switch and the segmented switch do, and one rule for the family is
// worth more than a better rule for one of them.
func (h *FoldHead) Tapped(*fyne.PointEvent) {
	h.from.Take(h)
	h.fold.Set(!h.fold.open)
}

func (h *FoldHead) MouseIn(*desktop.MouseEvent) {
	h.hovered = true
	h.Refresh()
}

func (h *FoldHead) MouseMoved(*desktop.MouseEvent) {}

func (h *FoldHead) MouseOut() {
	h.hovered = false
	h.Refresh()
}

// FocusGained draws the mark only for the keyboard. See PointerFocus.
func (h *FoldHead) FocusGained() {
	if !h.from.Draws() {
		return
	}
	h.marked = true
	h.Refresh()
}

func (h *FoldHead) FocusLost() {
	h.from.Lost(h.marked)
	h.marked = false
	h.Refresh()
}

// WindowReturning is the window saying the next FocusGained is its own
// return to the front. See PointerFocus.
func (h *FoldHead) WindowReturning() { h.from.WindowReturning() }

// TypedRune answers nothing, for the reason Button gives: one press of the
// space bar reaches a focused control twice from the desktop driver, as the
// key and as the character, and a row answering both would open and shut.
func (h *FoldHead) TypedRune(rune) {}

// TypedKey opens or shuts on the space bar and on both names of Enter. Any
// key draws the mark first, the way every control of this family does: a
// press put the keyboard here quietly, and the first key after it is the
// moment somebody reaches for the keyboard and needs to see which control is
// listening - see TestReachingForTheKeyboardTurnsTheMarkOn. Until 2026-09-17
// the row alone did not, so a fold pressed with the mouse and then opened
// with Space held the keyboard with nothing drawn to say so.
func (h *FoldHead) TypedKey(event *fyne.KeyEvent) {
	if event == nil {
		return
	}
	if !h.marked {
		h.marked = true
		h.Refresh()
	}
	switch event.Name {
	case fyne.KeyReturn, fyne.KeyEnter, fyne.KeySpace:
		h.Tapped(nil)
	}
}

// Refresh redraws the state, and recolours the arrow with it - the arrow
// stands in the row's content rather than in this renderer, so it is told
// here rather than by the renderer. widget.Icon repaints itself when its
// resource is set.
func (h *FoldHead) Refresh() {
	h.drawArrow()
	h.BaseWidget.Refresh()
}

// drawArrow points the arrow the way the fold is and inks it the way the row
// is: the hint's colour at rest and the foreground under the pointer, which
// is what a glyph button does with its mark.
func (h *FoldHead) drawArrow() {
	icon := theme.MenuDropDownIcon()
	if !h.fold.open {
		icon = theme.MenuExpandIcon()
	}
	ink := theme.ColorNamePlaceHolder
	if h.hovered {
		ink = theme.ColorNameForeground
	}
	h.arrow.SetResource(theme.NewColoredResource(icon, ink))
}

func (h *FoldHead) CreateRenderer() fyne.WidgetRenderer {
	back := canvas.NewRectangle(color.Transparent)
	back.CornerRadius = RadiusField
	ring := canvas.NewRectangle(color.Transparent)
	ring.CornerRadius = RadiusField
	ring.StrokeColor = PaletteColour(theme.ColorNamePrimary, theme.VariantDark)
	r := &foldHeadRenderer{head: h, back: back, ring: ring}
	r.Refresh()
	return r
}

type foldHeadRenderer struct {
	head *FoldHead
	back *canvas.Rectangle
	ring *canvas.Rectangle
}

func (r *foldHeadRenderer) Layout(size fyne.Size) {
	if r.head.under != nil {
		size.Width = fyne.Min(size.Width, r.head.under.MinSize().Width)
	}
	r.back.Resize(size)
	r.ring.Resize(size)
}

// MinSize is nothing: the row's content in the stack above this decides the
// size, and this takes whatever the stack gives.
func (r *foldHeadRenderer) MinSize() fyne.Size { return fyne.NewSize(0, 0) }

// Refresh draws the state: a fill under the pointer, a ring for the keyboard,
// and neither at rest. The ring is a line and not a fill, for the reason Ring
// gives.
func (r *foldHeadRenderer) Refresh() {
	if r.head.hovered {
		r.back.FillColor = PaletteColour(theme.ColorNameHover, theme.VariantDark)
	} else {
		r.back.FillColor = color.Transparent
	}
	if r.head.marked {
		r.ring.StrokeWidth = ringWidth
	} else {
		r.ring.StrokeWidth = 0
	}
	redraw(r.back, r.ring)
}

func (r *foldHeadRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.back, r.ring}
}

func (r *foldHeadRenderer) Destroy() {}

// overhang lays its one child out reaching a distance to the LEFT of the
// container's own edge, and that much wider, so that what the child keeps as
// room inside its left edge lands on the container's edge.
//
// It exists for the head of a fold. The row keeps TabInset inside its own
// box, the way a word on the strip does, so that the fill under the pointer
// and the ring round the keyboard have room to draw in rather than sitting on
// the ink. But the title has to stay on the edge everything else on the
// screen starts on - TestEverythingAPersonReadsStartsOnOneLeftEdge - and
// moving the words in by TabInset would move them off it. So the row starts
// TabInset to the left instead, into the panel's own inset, which is twice
// as wide. Its right edge is unchanged, so the buttons a batch keeps in its
// head still end where the fields under them end.
type overhang struct{ by float32 }

func (o overhang) MinSize(objects []fyne.CanvasObject) fyne.Size {
	size := fyne.NewSize(0, 0)
	for _, child := range objects {
		min := child.MinSize()
		size.Width = fyne.Max(size.Width, min.Width-o.by)
		size.Height = fyne.Max(size.Height, min.Height)
	}
	return size
}

func (o overhang) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	for _, child := range objects {
		child.Move(fyne.NewPos(-o.by, 0))
		child.Resize(fyne.NewSize(size.Width+o.by, size.Height))
	}
}
