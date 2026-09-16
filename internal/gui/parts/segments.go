package parts

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Segments is a row of words joined into one control, of which exactly one is
// chosen - the three ways of saying how big, in place of three radio circles.
//
// # Why it is one control rather than the toolkit's radio group
//
// widget.RadioGroup is three separate focusable circles, so Tab stopped at each
// of them and the guard for reachability had to reach into its renderer to
// find them at all (reachability_test.go, before 2026-09-15). A segmented
// switch is one Tab stop whose arrows move the choice, which is what a row of
// mutually exclusive options is on every desktop - the toolkit draws its radio
// as a stack of discs it fills with the focus colour, the same mark this window
// took off its menus and switches (see PointerFocus). It carries the same four
// names a caller used on the radio - Options, Selected, SetSelected, OnChanged
// - so the switch behind the three ways of stating a size changed type and not
// the code around it.
//
// It draws its own border and its own ring, so a field built round it gets
// neither from WithRing (see selfEdged): the three ways cannot be refused, only
// the box the chosen way shows can.
type Segments struct {
	widget.DisableableWidget

	Options   []string
	Selected  string
	OnChanged func(string)

	hovered int // the segment under the pointer, or -1
	marked  bool
	// from knows whether the keyboard arrived by press or by key - see
	// PointerFocus and Toggle, which follow the same rule.
	from PointerFocus
}

var (
	_ fyne.Tappable     = (*Segments)(nil)
	_ fyne.Focusable    = (*Segments)(nil)
	_ desktop.Hoverable = (*Segments)(nil)
)

// NewSegments builds a switch offering these words, open on the first.
func NewSegments(options []string, changed func(string)) *Segments {
	s := &Segments{Options: options, OnChanged: changed, hovered: -1}
	if len(options) > 0 {
		s.Selected = options[0]
	}
	s.ExtendBaseWidget(s)
	return s
}

func (s *Segments) drawsOwnEdge() {}

// Marked and Hovered report the drawn state, for a guard.
func (s *Segments) Marked() bool   { return s.marked }
func (s *Segments) HoveredAt() int { return s.hovered }

// SetSelected chooses a word by name, firing OnChanged only when it moves and
// only for a word the switch actually offers - a name it does not hold changes
// nothing, the way a switch of fixed choices should.
func (s *Segments) SetSelected(value string) {
	if value == s.Selected || s.indexOf(value) < 0 {
		return
	}
	s.Selected = value
	s.Refresh()
	if s.OnChanged != nil {
		s.OnChanged(value)
	}
}

func (s *Segments) indexOf(value string) int {
	for i, o := range s.Options {
		if o == value {
			return i
		}
	}
	return -1
}

// Tapped chooses the segment under the pointer and puts the keyboard on the
// switch, quietly - the way the toolkit's radio item does (widget/radio_item.go
// Tapped), so the arrows step on from what was clicked. See Toggle.Tapped for
// the whole of why, and for what stood here until 2026-09-16.
func (s *Segments) Tapped(event *fyne.PointEvent) {
	if s.Disabled() || event == nil {
		return
	}
	s.takeTheKeyboardQuietly()
	if at := s.segmentAt(event.Position.X); at >= 0 {
		s.SetSelected(s.Options[at])
	}
}

// takeTheKeyboardQuietly moves the focus here without the mark, unless it is
// here already.
func (s *Segments) takeTheKeyboardQuietly() {
	app := fyne.CurrentApp()
	if app == nil {
		return
	}
	canvas := app.Driver().CanvasForObject(s)
	if canvas == nil || canvas.Focused() == s {
		return
	}
	s.from.Quietly(func() { canvas.Focus(s) })
}

// Quietly runs a focus change without drawing the mark. See PointerFocus and
// FocusQuietly.
func (s *Segments) Quietly(focus func()) { s.from.Quietly(focus) }

// segmentAt is the segment a point falls in, or -1 past the last one.
func (s *Segments) segmentAt(x float32) int {
	edge := float32(0)
	for i, o := range s.Options {
		edge += s.segmentWidth(o)
		if x < edge {
			return i
		}
		edge += Hairline
	}
	return -1
}

func (s *Segments) segmentWidth(word string) float32 {
	return fyne.MeasureText(word, TextBody, fyne.TextStyle{}).Width + ControlInset*2
}

func (s *Segments) MouseIn(e *desktop.MouseEvent) { s.MouseMoved(e) }
func (s *Segments) MouseMoved(e *desktop.MouseEvent) {
	at := -1
	if e != nil {
		at = s.segmentAt(e.Position.X)
	}
	if at != s.hovered {
		s.hovered = at
		s.Refresh()
	}
}
func (s *Segments) MouseOut() {
	if s.hovered != -1 {
		s.hovered = -1
		s.Refresh()
	}
}

// FocusGained draws the ring when the keyboard is what brought the focus here.
// A press brings it quietly and the first key turns the ring on.
func (s *Segments) FocusGained() {
	if s.from.Quiet() {
		return
	}
	s.mark()
}

func (s *Segments) mark() {
	s.marked = true
	s.Refresh()
}
func (s *Segments) FocusLost() {
	s.marked = false
	s.Refresh()
}

func (s *Segments) TypedRune(rune) {}

// TypedKey moves the choice. Left and right step, Home and End jump - and each
// MOVES the choice rather than only the focus, because a row of exclusive
// options is chosen by arrowing through it, the manual-activation exception a
// tab strip makes (see TabWord) not applying where the choice does not move the
// keyboard off the control.
func (s *Segments) TypedKey(event *fyne.KeyEvent) {
	// Disabled asked here and not left to the renderer: a switch frozen for
	// the length of a run keeps the keyboard if it had it, because the focus
	// manager checks Disabled only when it MOVES the focus (internal/app/
	// focus_manager.go), and the driver hands every key to whatever is
	// focused. Without this the arrows moved the choice under a form drawn as
	// frozen - an outside review of the pull request named it, 2026-09-16.
	if event == nil || s.Disabled() || len(s.Options) == 0 {
		return
	}
	if !s.marked {
		s.mark()
	}
	at := s.indexOf(s.Selected)
	switch event.Name {
	case fyne.KeyLeft:
		if at > 0 {
			s.SetSelected(s.Options[at-1])
		}
	case fyne.KeyRight:
		if at >= 0 && at < len(s.Options)-1 {
			s.SetSelected(s.Options[at+1])
		}
	case fyne.KeyHome:
		s.SetSelected(s.Options[0])
	case fyne.KeyEnd:
		s.SetSelected(s.Options[len(s.Options)-1])
	}
}

// CreateRenderer draws the joined words, the chosen one filled.
func (s *Segments) CreateRenderer() fyne.WidgetRenderer {
	s.ExtendBaseWidget(s)
	border := canvas.NewRectangle(color.Transparent)
	border.CornerRadius = RadiusField
	border.StrokeColor = PaletteColour(theme.ColorNameInputBorder, theme.VariantDark)
	border.StrokeWidth = edgeWidth
	ring := canvas.NewRectangle(color.Transparent)
	ring.CornerRadius = RadiusField
	ring.StrokeColor = PaletteColour(theme.ColorNamePrimary, theme.VariantDark)
	r := &segmentsRenderer{seg: s, border: border, ring: ring}
	r.build()
	r.Refresh()
	return r
}

type segmentsRenderer struct {
	seg    *Segments
	border *canvas.Rectangle
	ring   *canvas.Rectangle
	fills  []*canvas.Rectangle
	words  []*canvas.Text
	rules  []*canvas.Rectangle
}

// build makes one fill and one word per option, and a rule between each pair.
// The count is fixed once, because a switch does not gain options after it is
// built.
func (r *segmentsRenderer) build() {
	for range r.seg.Options {
		fill := canvas.NewRectangle(color.Transparent)
		// Rounded to sit inside the border's own curve. Until 2026-09-16 the
		// fill was a sharp rectangle under a rounded border, so the corner of
		// the chosen segment stood out past the border's arc - one of the
		// three geometries the owner saw at once (O219).
		fill.CornerRadius = RadiusField - edgeWidth
		r.fills = append(r.fills, fill)
		r.words = append(r.words, canvas.NewText("", color.Transparent))
	}
	for i := 1; i < len(r.seg.Options); i++ {
		r.rules = append(r.rules, canvas.NewRectangle(PaletteColour(theme.ColorNameInputBorder, theme.VariantDark)))
	}
}

// Layout puts each fill a border's width inside the switch, so that it
// never reaches under the stroke, and each rule on the seam between two
// segments. The words are centred in their segment, not in their fill.
func (r *segmentsRenderer) Layout(fyne.Size) {
	h := r.MinSize().Height
	x := float32(0)
	for i, word := range r.seg.Options {
		w := r.seg.segmentWidth(word)
		r.fills[i].Resize(fyne.NewSize(w-edgeWidth*2, h-edgeWidth*2))
		r.fills[i].Move(fyne.NewPos(x+edgeWidth, edgeWidth))
		ink := r.words[i].MinSize()
		r.words[i].Resize(ink)
		r.words[i].Move(fyne.NewPos(x+(w-ink.Width)/2, (h-ink.Height)/2))
		if i > 0 {
			r.rules[i-1].Resize(fyne.NewSize(Hairline, h))
			r.rules[i-1].Move(fyne.NewPos(x-Hairline, 0))
		}
		x += w + Hairline
	}
	total := fyne.NewSize(fyne.Max(0, x-Hairline), h)
	r.border.Resize(total)
	r.ring.Resize(total.Add(fyne.NewSquareSize(ringGap * 2)))
	r.ring.Move(fyne.NewPos(-ringGap, -ringGap))
}

func (r *segmentsRenderer) MinSize() fyne.Size {
	width := float32(0)
	for i, word := range r.seg.Options {
		width += r.seg.segmentWidth(word)
		if i > 0 {
			width += Hairline
		}
	}
	// The height of a box to type in, so the switch stands the same height as
	// the fields around it.
	return fyne.NewSize(width, TextBody+ControlInset*2+edgeWidth*2)
}

func (r *segmentsRenderer) Refresh() {
	off := r.seg.Disabled()
	chosen := r.seg.indexOf(r.seg.Selected)
	for i, word := range r.seg.Options {
		r.words[i].Text = word
		r.words[i].TextSize = TextBody
		r.fills[i].FillColor, r.words[i].Color = segmentFace(i == chosen, i == r.seg.hovered, off)
		redraw(r.fills[i], r.words[i])
	}
	for i := range r.rules {
		// A rule stands on the seam between two segments, and only where
		// neither of them is the chosen one: there the chosen fill is the
		// boundary, and a line beside it was the second of the three
		// geometries (O219).
		if chosen != i && chosen != i+1 {
			r.rules[i].Show()
		} else {
			r.rules[i].Hide()
		}
		redraw(r.rules[i])
	}
	if r.seg.marked {
		r.ring.StrokeWidth = ringWidth
	} else {
		r.ring.StrokeWidth = 0
	}
	redraw(r.border, r.ring)
	r.Layout(r.seg.Size())
}

// segmentFace is the fill and the ink of one segment in one state.
//
// A frozen switch keeps the chosen fill, in disabled ink. Until 2026-09-16 it
// lost it - every fill went transparent when the switch was off - so a form
// frozen for a run would not have said which way of stating a size it was
// running with (O223). It never showed, because the switch was not being
// frozen at all, which is the other half of O223.
func segmentFace(chosen, hovered, off bool) (fill, ink color.Color) {
	dark := theme.VariantDark
	fill = color.Transparent
	switch {
	case off:
		ink = PaletteColour(theme.ColorNameDisabled, dark)
		if chosen {
			fill = PaletteColour(theme.ColorNameSelection, dark)
		}
	case chosen:
		fill = PaletteColour(theme.ColorNameSelection, dark)
		ink = PaletteColour(theme.ColorNameForeground, dark)
	case hovered:
		fill = PaletteColour(theme.ColorNameHover, dark)
		ink = PaletteColour(theme.ColorNameForeground, dark)
	default:
		ink = PaletteColour(theme.ColorNamePlaceHolder, dark)
	}
	return fill, ink
}

// Objects draws the fills first, then the rules and border over their edges,
// then the words on top, then the ring outside all of it.
func (r *segmentsRenderer) Objects() []fyne.CanvasObject {
	out := make([]fyne.CanvasObject, 0, len(r.fills)*2+len(r.rules)+2)
	for _, f := range r.fills {
		out = append(out, f)
	}
	for _, rule := range r.rules {
		out = append(out, rule)
	}
	out = append(out, r.border)
	for _, w := range r.words {
		out = append(out, w)
	}
	return append(out, r.ring)
}

func (r *segmentsRenderer) Destroy() {}
