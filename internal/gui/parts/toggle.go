package parts

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Toggle is a switch this window draws itself: a square in the column of
// controls, with its name in the column of names like every other field.
//
// # Why the name is not on it any more, on 2026-09-15
//
// It carried its own words until then, and that was the answer to O72: a switch
// given a heading ABOVE it arrived as a bare square with nothing to read on the
// thing you click. In a grid of names beside controls the name stands level
// with the square, in the same column as every other name, so there is
// something to read next to it without the switch being the one control that
// breaks the grid (GUI rule 13). This is the deliberate other side of O72, not
// a slip back into it: the name did not move onto the square, the form grew a
// column to hold it.
//
// # Why it is drawn here rather than through widget.Check
//
// widget.Check draws the mark that says the keyboard is here as a disc behind
// the square, filled with the focus colour, and it fills that disc whether the
// keyboard arrived by key or by press - see PointerFocus for the whole of that.
// Drawing the square ourselves puts the keyboard mark where every other control
// in this window puts it, a ring standing clear of the control, and makes the
// pointer draw a halo the toolkit's check never did (O205).
type Toggle struct {
	widget.DisableableWidget

	// Checked is the value, exported so a screen can read it and hold a pointer
	// to it the way the two label switches do.
	Checked   bool
	OnChanged func(bool)

	hovered bool
	marked  bool
}

var (
	_ fyne.Tappable     = (*Toggle)(nil)
	_ fyne.Focusable    = (*Toggle)(nil)
	_ desktop.Hoverable = (*Toggle)(nil)
)

// NewToggle makes a switch, off, that calls changed when it flips.
func NewToggle(changed func(bool)) *Toggle {
	t := &Toggle{OnChanged: changed}
	t.ExtendBaseWidget(t)
	return t
}

// SetChecked sets the value and redraws, calling OnChanged only when it moved -
// the toolkit's own rule, so a screen that resets a switch to what it already
// was does not fire the callback.
func (t *Toggle) SetChecked(on bool) {
	if t.Checked == on {
		return
	}
	t.Checked = on
	t.Refresh()
	if t.OnChanged != nil {
		t.OnChanged(on)
	}
}

// Hovered and Marked report the drawn state, for a guard.
func (t *Toggle) Hovered() bool { return t.hovered }
func (t *Toggle) Marked() bool  { return t.marked }

// drawsOwnEdge tells WithRing to leave this alone: the square has its own
// border and its own ring, see selfEdged.
func (t *Toggle) drawsOwnEdge() {}

// Tapped flips the switch. The keyboard is not taken, for the reason the button
// gives: the driver does not focus a tapped widget, so a press leaves the mark
// off without anything having to make the focus quiet.
func (t *Toggle) Tapped(*fyne.PointEvent) {
	if t.Disabled() {
		return
	}
	t.SetChecked(!t.Checked)
}

func (t *Toggle) MouseIn(*desktop.MouseEvent) {
	t.hovered = true
	t.Refresh()
}
func (t *Toggle) MouseMoved(*desktop.MouseEvent) {}
func (t *Toggle) MouseOut() {
	t.hovered = false
	t.Refresh()
}

// FocusGained draws the ring, and only the keyboard ever brings the focus here.
func (t *Toggle) FocusGained() {
	t.marked = true
	t.Refresh()
}

func (t *Toggle) FocusLost() {
	t.marked = false
	t.Refresh()
}

func (t *Toggle) TypedRune(r rune) {
	if r == ' ' {
		t.Tapped(nil)
	}
}

func (t *Toggle) TypedKey(event *fyne.KeyEvent) {
	if event != nil && event.Name == fyne.KeySpace {
		t.Tapped(nil)
	}
}

// CreateRenderer draws the square, its mark and its ring.
func (t *Toggle) CreateRenderer() fyne.WidgetRenderer {
	t.ExtendBaseWidget(t)
	halo := canvas.NewRectangle(color.Transparent)
	halo.CornerRadius = RadiusField
	ring := canvas.NewRectangle(color.Transparent)
	ring.CornerRadius = RadiusMark
	ring.StrokeColor = PaletteColour(theme.ColorNamePrimary, theme.VariantDark)
	square := canvas.NewRectangle(color.Transparent)
	square.CornerRadius = RadiusMark
	tick := canvas.NewImageFromResource(theme.NewColoredResource(theme.ConfirmIcon(), theme.ColorNameForegroundOnPrimary))
	tick.FillMode = canvas.ImageFillContain
	r := &toggleRenderer{toggle: t, halo: halo, ring: ring, square: square, tick: tick}
	r.Refresh()
	return r
}

// toggleRenderer draws a 20 px square inside a 24 px target, so a switch is the
// same size as the button that explains a field beside it and stands in the
// same box.
type toggleRenderer struct {
	toggle *Toggle
	halo   *canvas.Rectangle
	ring   *canvas.Rectangle
	square *canvas.Rectangle
	tick   *canvas.Image
}

func (r *toggleRenderer) Layout(size fyne.Size) {
	// At the LEFT of whatever room the row gives the control, so the square
	// lands on the same edge as every box above it rather than in the middle
	// of a cell that fills the form. The row makes the control cell as wide as
	// what is left, so a switch centred in it would sit far to the right of the
	// column its name stands in.
	target := fyne.NewSquareSize(GlyphButton)
	top := (size.Height - GlyphButton) / 2
	r.halo.Resize(target)
	r.halo.Move(fyne.NewPos(0, top))
	square := fyne.NewSquareSize(markSide)
	at := fyne.NewPos((GlyphButton-markSide)/2, top+(GlyphButton-markSide)/2)
	r.square.Resize(square)
	r.square.Move(at)
	// The ring hugs the square rather than the target, so the mark is round the
	// thing that changes and not round the room a finger needs.
	r.ring.Resize(square.Add(fyne.NewSquareSize(ringGap * 2)))
	r.ring.Move(at.Subtract(fyne.NewPos(ringGap, ringGap)))
	r.tick.Resize(fyne.NewSquareSize(markSide - GapInline*2))
	r.tick.Move(at.Add(fyne.NewPos(GapInline, GapInline)))
}

func (r *toggleRenderer) MinSize() fyne.Size { return fyne.NewSquareSize(GlyphButton) }

func (r *toggleRenderer) Refresh() {
	dark := theme.VariantDark
	off := r.toggle.Disabled()
	if r.toggle.hovered && !off {
		r.halo.FillColor = PaletteColour(theme.ColorNameHover, dark)
	} else {
		r.halo.FillColor = color.Transparent
	}
	if r.toggle.marked {
		r.ring.StrokeWidth = ringWidth
	} else {
		r.ring.StrokeWidth = 0
	}
	switch {
	case off:
		r.square.FillColor = color.Transparent
		r.square.StrokeColor = PaletteColour(theme.ColorNameDisabled, dark)
		r.square.StrokeWidth = edgeWidth
		r.tick.Hide()
	case r.toggle.Checked:
		r.square.FillColor = PaletteColour(theme.ColorNamePrimary, dark)
		r.square.StrokeWidth = 0
		r.tick.Show()
	default:
		r.square.FillColor = PaletteColour(theme.ColorNameInputBackground, dark)
		r.square.StrokeColor = PaletteColour(theme.ColorNameInputBorder, dark)
		r.square.StrokeWidth = edgeWidth
		r.tick.Hide()
	}
	r.halo.Refresh()
	r.ring.Refresh()
	r.square.Refresh()
	r.tick.Refresh()
	canvas.Refresh(r.toggle)
}

func (r *toggleRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.halo, r.ring, r.square, r.tick}
}

func (r *toggleRenderer) Destroy() {}
