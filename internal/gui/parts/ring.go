package parts

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Ring is the line round a control that says something about its state.
//
// Two states with an order between them: a control the run refused draws in the
// error colour, a control holding the keyboard draws in the primary one, and a
// refused control with the keyboard in it draws red. The refusal is about what
// will happen and the focus is about where you are, so the one that stops the
// run wins.
//
// It is a LINE and not a fill, and that is arithmetic rather than taste. A fill
// cannot carry either state at the thresholds this palette is held to, which was
// measured on 2026-08-12 after the format menu was reported as staying blue
// after a choice:
//
//	the value written on a control is #E6E6E6, and 4.5:1 needs the thing under
//	it to sit at or below 0.137 relative luminance. Telling that same thing from
//	an untouched box, #2E2E30 at 0.027, needs 3.0:1, which is 0.182 or above.
//	There is no colour and no alpha between those two numbers, so any fill that
//	is visible enough to mean something has already swallowed the text.
//
// The measured example is what the window shipped: an opaque focus colour filled
// the whole of a chosen format menu at #4C9DF0, and the format name on it came
// out at 2.28:1 against a threshold of 4.5.
//
// A line has no such problem because nothing is written on it.
type Ring struct {
	rect *canvas.Rectangle

	refused bool
	focused bool
}

// ringWidth is how thick the line is.
//
// Twice the toolkit's input border, so it reads as a deliberate edge rather than
// as the box's own outline changing colour. One pixel lands between pixels and
// is anti-aliased away to something fainter than the number suggests - measured
// on the section surface on 2026-08-12, where a one pixel stroke of a 29.4 L*
// colour came out at 22.3.
const ringWidth = 2

// WithRing puts a control on the screen with an edge it can draw when there is
// something to say about it.
//
// The line is drawn OVER the control rather than under it, because the controls
// it goes round paint their own background - a menu fills its whole area - and
// an edge underneath one would be an edge nobody sees. It intercepts nothing:
// the driver looks for objects answering to the mouse and a rectangle answers to
// nothing, so the search walks past it to the control underneath. That is the
// same property the explanation sheet relies on, and the reason both are
// content rather than overlays.
func WithRing(control fyne.CanvasObject) (fyne.CanvasObject, *Ring) {
	rect := canvas.NewRectangle(color.Transparent)
	rect.CornerRadius = Theme().Size(theme.SizeNameInputRadius)
	ring := &Ring{rect: rect}
	ring.draw()

	// A box held to the width of a number is marked round the BOX rather than
	// round the half column it stands in. Seen on a render on 2026-08-12: the
	// line ran the full 400 px of the slot while the box in it was 140, so the
	// mark pointed at the empty space beside the field as much as at the field.
	// Numeric names its layout so that this can reach inside it.
	if box, narrow := control.(*fyne.Container); narrow {
		if _, ok := box.Layout.(fixedWidth); ok && len(box.Objects) == 1 {
			inner := box.Objects[0]
			wireRing(inner, ring)
			box.Objects[0] = container.NewStack(inner, rect)
			return box, ring
		}
	}

	wireRing(control, ring)
	return container.NewStack(control, rect), ring
}

// wireRing hands the ring to a control that knows when the keyboard arrives, so
// focus and refusal are drawn by one thing rather than by two that have to be
// kept in step. Everything else keeps whatever the toolkit does for it - a box
// to type in already draws its own border in the primary colour.
func wireRing(control fyne.CanvasObject, ring *Ring) {
	if r, ok := control.(ringed); ok {
		r.useRing(ring)
	}
}

// ringed is a control that reports the keyboard arriving and leaving.
type ringed interface{ useRing(*Ring) }

// Refuse turns the edge red, or takes the red away. Called with what the run
// said about this setting rather than with a judgement made here - G1.
func (r *Ring) Refuse(refused bool) {
	r.refused = refused
	r.draw()
}

// Focus marks the control the keyboard is in.
func (r *Ring) Focus(focused bool) {
	r.focused = focused
	r.draw()
}

// Refused says whether this edge is currently the red one, for a guard to read.
func (r *Ring) Refused() bool { return r.refused }

func (r *Ring) draw() {
	switch {
	case r.refused:
		r.rect.StrokeColor = PaletteColour(theme.ColorNameError, theme.VariantDark)
		r.rect.StrokeWidth = ringWidth
	case r.focused:
		r.rect.StrokeColor = PaletteColour(theme.ColorNamePrimary, theme.VariantDark)
		r.rect.StrokeWidth = ringWidth
	default:
		// No line at all rather than one in the background colour. A stroke
		// that is meant to be invisible is a thing that shows up the day the
		// surface behind it changes.
		r.rect.StrokeWidth = 0
	}
	r.rect.Refresh()
}

// Chooser is a menu that says when the keyboard is in it.
//
// It exists because widget.Select fills its entire background with the focus
// colour - select.go, bgColor - so the toolkit's only mark for "the keyboard is
// here" is the one thing that cannot be used on a control with a value written
// across it. See Ring for the measurement.
//
// A box to type in needs none of this: the toolkit already draws its border in
// the primary colour when it has focus, which is a line and not a fill.
type Chooser struct {
	widget.Select

	ring *Ring
	// from says whether the focus arriving now was put here by the pointer,
	// and marked says whether the mark is currently drawn. See PointerFocus.
	from   PointerFocus
	marked bool
	// opened is the list this menu last dropped down, and it is here for a
	// guard: the canvas says whether a list appeared, and this says what was
	// in it. Neither alone is worth anything - a list built correctly and never
	// shown looks right from the widget's side.
	opened *OpenList
}

// Opened is the list this menu last dropped down, or nil if it never has.
func (c *Chooser) Opened() *OpenList { return c.opened }

// NewChooser makes a menu of options, in the order it is given them.
func NewChooser(options []string, changed func(string)) *Chooser {
	c := &Chooser{}
	c.Options = options
	c.OnChanged = changed
	c.ExtendBaseWidget(c)
	return c
}

func (c *Chooser) useRing(r *Ring) { c.ring = r }

// FocusGained draws the mark only when the keyboard is what put the focus here.
//
// A press with the mouse still moves the keyboard into this control, because it
// has to - the list that opens is driven by the arrow keys. What it no longer
// does is SAY so. Reported twice from the screen, on 2026-08-12 and again on
// 2026-08-18: a menu stays painted blue after a value is chosen, and there is
// nothing left to press that would take the paint off.
//
// See PointerFocus for why this is one rule rather than a fix per control.
func (c *Chooser) FocusGained() {
	if c.from.Quiet() {
		return
	}
	c.mark()
}

// mark turns the drawn state on: the toolkit's own first, because the widget's
// appearance depends on it, and then the edge.
func (c *Chooser) mark() {
	c.marked = true
	c.Select.FocusGained()
	if c.ring != nil {
		c.ring.Focus(true)
	}
}

func (c *Chooser) FocusLost() {
	c.marked = false
	c.Select.FocusLost()
	if c.ring != nil {
		c.ring.Focus(false)
	}
}

// Marked says whether the keyboard mark is drawn, for a guard. The toolkit
// keeps the same answer in an unexported field of two different widgets, so
// reading it off the canvas means reading a colour and deciding what it meant.
func (c *Chooser) Marked() bool { return c.marked }

// Tapped drops the list down, with the value in the box marked in it.
//
// The list is ours since 2026-08-18 and the box is still the toolkit's. That
// split is the whole change: what was reported twice was the LIST - its density,
// and that a letter typed at it did nothing - and none of it could be reached
// through widget.PopUpMenu. See OpenList for the measurement.
//
// The overlay is right here, unlike the sheet the field explanations are drawn
// on. An open list SHOULD take the pointer away from everything under it, which
// is the same property that made an overlay wrong for a tooltip.
func (c *Chooser) Tapped(*fyne.PointEvent) {
	if c.Disabled() {
		return
	}
	app := fyne.CurrentApp()
	if app == nil {
		return
	}
	surface := app.Driver().CanvasForObject(c)
	if surface == nil {
		// Detached between the tap being delivered and this call. The toolkit
		// answers the same case by doing nothing rather than crashing inside
		// the overlay machinery.
		return
	}
	// Quietly: the keyboard comes here so the arrows work, and nothing is drawn
	// to announce it. See FocusGained.
	c.from.Quietly(func() { surface.Focus(c) })
	c.drop(surface)
}

// drop builds and shows the list. Split out because the shape gate caps a
// function at eighty lines of logic and because what it does is one thing.
func (c *Chooser) drop(surface fyne.Canvas) {
	var pop *widget.PopUp
	list := NewOpenList(c.Options, c.Selected,
		func(value string, byKeyboard bool) {
			pop.Hide()
			c.SetSelected(value)
			c.giveBack(surface, byKeyboard)
		},
		func(byKeyboard bool) {
			pop.Hide()
			c.giveBack(surface, byKeyboard)
		})
	pop = widget.NewPopUp(list, surface)
	c.opened = list

	at := fyne.CurrentApp().Driver().AbsolutePositionForObject(c)
	// As wide as the box, so the list reads as belonging to that field. How
	// tall and which side of the box it goes on is worked out from the room
	// that is actually left - see roomForList.
	height, top := roomForList(surface.Size().Height, at.Y, c.Size().Height, list.MinSize().Height)
	// Told to the list rather than only to the popup, because a popup is never
	// laid out smaller than its content's minimum - so resizing alone left the
	// list its full height and the shortening did nothing.
	list.LimitTo(height)
	pop.Resize(fyne.NewSize(c.Size().Width, height))
	pop.ShowAtPosition(fyne.NewPos(at.X, top))

	surface.Focus(list)
	// On the value already in the box, so that pressing Down once does not go
	// to the first value while the box shows the ninth.
	list.StartOn(c.Selected)
}

// listEdgeGap is the space kept between an open list and the edge of the
// window, so that a list filling the room still reads as sitting inside it.
const listEdgeGap = 8

// roomForList decides how tall an open list may be and where its top goes.
//
// It used to go under the box at its full height, always, which is right until
// the box is near the foot of a form - and every form here is taller than its
// window, so a menu low on one opened straight through the bottom edge. The
// format menu showed four of its twenty values that way, with the rest past the
// edge and the run buttons underneath it (O113).
//
// Two things fix it and both are needed. It opens UPWARD when there is more
// room above the box than below it, which is what every desktop menu does. And
// it is cut to the room on whichever side it lands, rather than to a fixed
// number of rows - the eight row ceiling is about not covering the form, and it
// says nothing about a window that has less than eight rows left.
//
// Arithmetic rather than widgets so that it can be checked directly. The screen
// level guard opens a real menu and measures the overlay, which is the half
// that catches this being wired up wrongly.
func roomForList(canvasHeight, boxTop, boxHeight, wanted float32) (height, top float32) {
	below := canvasHeight - (boxTop + boxHeight) - listEdgeGap
	above := boxTop - listEdgeGap
	if below < 0 {
		below = 0
	}
	if above < 0 {
		above = 0
	}

	if wanted <= below {
		return wanted, boxTop + boxHeight
	}
	if above > below {
		if wanted > above {
			wanted = above
		}
		return wanted, boxTop - wanted
	}
	if wanted > below {
		wanted = below
	}
	return wanted, boxTop + boxHeight
}

// giveBack hands the keyboard back to the box when the list closes.
//
// The ARIA practices for a combobox ask for it by name at Escape, and it is
// what stops the keyboard being left on a control that is no longer on screen.
// Silently when a press closed the list, out loud when a key did - the same
// rule the rest of this control follows.
func (c *Chooser) giveBack(surface fyne.Canvas, byKeyboard bool) {
	c.opened = nil
	if byKeyboard {
		surface.Focus(c)
		// Marked here rather than left to FocusGained. A canvas keeps a
		// separate focus for an overlay, so closing one restores the box
		// WITHOUT FocusGained running - measured on 2026-08-18, where Escape
		// handed the keyboard back and nothing on the box said so.
		if !c.marked {
			c.mark()
		}
		return
	}
	c.from.Quietly(func() { surface.Focus(c) })
}

// TypedKey opens the same list from the keyboard.
//
// Without this the two ways in show two different lists: the toolkit's own
// showPopUp is what its key handling calls, and that one has nothing marked.
// UX9 asks that whatever the mouse can do the keyboard can - which has to mean
// the same thing, not a second version of it.
func (c *Chooser) TypedKey(event *fyne.KeyEvent) {
	// The keyboard has been used, so from here on it is worth saying where it
	// is. Somebody who opened this list with the mouse and then reached for the
	// arrows is somebody who now needs to see which control is listening.
	if !c.marked {
		c.mark()
	}
	switch event.Name {
	case fyne.KeySpace, fyne.KeyUp, fyne.KeyDown:
		c.Tapped(nil)
	default:
		// Left and right step through the values without opening anything, and
		// that is the toolkit's behaviour worth keeping.
		c.Select.TypedKey(event)
	}
}

// TypedRune moves to the next value starting with the letter typed, with the
// list shut.
//
// The toolkit has none: widget.Select.TypedRune is "intentionally left blank".
// So pressing p on a closed format menu did nothing, which is the half of O92c
// that has nothing to do with the list being open. The ARIA practices put both
// halves under one expectation - a printable character moves to the values
// starting with it, collapsed or expanded.
//
// It steps from the value after the current one and wraps, so the same letter
// pressed twice walks through the values sharing it.
func (c *Chooser) TypedRune(r rune) {
	if c.Disabled() || len(c.Options) == 0 {
		return
	}
	if !c.marked {
		c.mark()
	}
	want := strings.ToLower(string(r))
	from := c.SelectedIndex()
	for step := 1; step <= len(c.Options); step++ {
		at := (from + step) % len(c.Options)
		if at < 0 {
			at += len(c.Options)
		}
		if strings.HasPrefix(strings.ToLower(c.Options[at]), want) {
			c.SetSelectedIndex(at)
			return
		}
	}
}

// There was a Switch here for half an hour on 2026-08-12 and taking it out is
// the correction. It was a checkbox with a ring, added so that focus on a
// switch would be as visible as focus on a menu - and a ring round a checkbox
// is a ring round its words as well as its square, which on screen reads as a
// box drawn around a sentence. Reported from a screenshot.
//
// A checkbox keeps the toolkit's own mark: a disc filled behind the square with
// the focus colour, which is a shape rather than a fill over a value, so the
// arithmetic that ruled out a fill for a menu does not apply to it.
