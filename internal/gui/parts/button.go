package parts

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Look is which of the window's button faces one wears.
type Look int

const (
	// Primary is the one button on a screen that does the work - Generate. A
	// filled face in the accent colour, so the eye lands on it.
	Primary Look = iota
	// Secondary is a button beside the primary one - Preview, Choose, Add a
	// batch. A raised face in the surface a field has, with an edge, so it
	// reads as a button without competing with the one in the accent colour.
	// It was an outline round nothing until 2026-09-21 - see buttonFace.
	Secondary
	// Quiet is a button that is not about the work in front of you - Donate.
	// Words with a surface only under the pointer, an outline round
	// transparency, because it stands on more than one background and a fill
	// in the colour of any one of them would be wrong on the others (the Fyne
	// guide, section 3.11).
	Quiet
	// Glyph is a button that is a single mark rather than a word - the little i
	// that opens an explanation, the arrow that folds a section. The quietest
	// face, in the room of a glyph rather than of a label.
	Glyph
)

// Button is a button this window draws itself, in place of widget.Button.
//
// # Why it stopped being the toolkit's, on 2026-09-15
//
// The toolkit's button draws its "the keyboard is here" mark by blending the
// focus colour into its fill - widget/button.go, buttonColorNames - and on the
// filled primary button that is blue over blue: measured at 1.11 contrast on
// Generate on 2026-09-15, a mark drawn and not seen (O207). Its hover is the
// same shape and the same problem, 1.12 on the same button (O205). It has no
// pressed state at all, only an animation, and it answers the keyboard on the
// space bar and not on Enter. None of that is reachable through a theme, which
// can reach a colour and a radius and not the arithmetic that blends two of
// them - see the Fyne guide, section 4.
//
// # What this costs, and what it pays back
//
// A renderer of ours is a piece of the toolkit written again, and this project
// caps the number of those. Drawing the button itself is also what LETS the
// focus ring stand clear of the face, in one colour on every look, rather than
// a fill the primary face swallows. The states are read off fields here rather
// than out of the toolkit's private ones (the Fyne guide, section 3.26), so
// what the renderer draws is a fact this type owns.
type Button struct {
	widget.DisableableWidget

	// Text and OnTapped match widget.Button, so a caller moving between them
	// learns no new spelling and the guards that read Text keep working.
	Text     string
	OnTapped func()

	// Icon is the mark a Glyph button draws, and the picture before the word on
	// any other - nil for none.
	Icon fyne.Resource

	look Look

	// inBar is whether this button stands in the bar at the foot, where it
	// keeps more room round its words - see InTheBar.
	inBar bool

	// The pointer's state and the keyboard's, kept here because the toolkit
	// keeps its own in unexported fields a renderer of ours cannot read.
	//
	// hovered and pressed are the pointer. marked is the keyboard, and it is
	// true only when a key put the focus here. A press with the mouse never
	// focuses this control - the driver unfocuses on a tap and does not focus
	// the tapped widget (glfw/window.go, mouseClicked), read rather than
	// assumed - so nothing has to make the focus quiet the way a menu does.
	hovered bool
	pressed bool
	marked  bool
}

// The driver asks for these by type, so a button that quietly stops answering
// one goes on compiling and stops answering the mouse or the keyboard
// (glfw/window.go, findObjectAtPositionMatching). Asserted here so that is a
// build error instead.
var (
	_ fyne.Tappable     = (*Button)(nil)
	_ fyne.Focusable    = (*Button)(nil)
	_ desktop.Hoverable = (*Button)(nil)
	_ desktop.Mouseable = (*Button)(nil)
)

// NewButton builds a button of one of the four looks.
func NewButton(look Look, label string, tapped func()) *Button {
	b := &Button{look: look, Text: label, OnTapped: tapped}
	b.ExtendBaseWidget(b)
	return b
}

// InTheBar gives a button the size of the bar at the foot of a screen, where
// the buttons that run something stand. A parameter of the one button rather
// than a second kind of button, so every look can stand there.
func (b *Button) InTheBar() *Button {
	b.inBar = true
	b.Refresh()
	return b
}

// NewGlyphButton builds the small mark-only button an icon stands in.
func NewGlyphButton(icon fyne.Resource, tapped func()) *Button {
	b := &Button{look: Glyph, Icon: icon, OnTapped: tapped}
	b.ExtendBaseWidget(b)
	return b
}

// SetText changes the label and redraws.
func (b *Button) SetText(label string) {
	b.Text = label
	b.Refresh()
}

// SetIcon changes the mark and redraws.
func (b *Button) SetIcon(icon fyne.Resource) {
	b.Icon = icon
	b.Refresh()
}

// Hovered, Pressed and Marked report the drawn state, for a guard - the toolkit
// keeps the same three in unexported fields, so the alternative is reading a
// colour off the canvas and deciding what it meant.
func (b *Button) Hovered() bool { return b.hovered }
func (b *Button) Pressed() bool { return b.pressed }
func (b *Button) Marked() bool  { return b.marked }

// Look is which face this button wears, for a guard asking whether the way to
// stop a run is drawn as a button rather than as bare words.
func (b *Button) Look() Look { return b.look }

// Tapped presses the button, unless it cannot be pressed.
func (b *Button) Tapped(*fyne.PointEvent) {
	if b.Disabled() || b.OnTapped == nil {
		return
	}
	b.OnTapped()
}

// MouseIn notes the pointer arrived. A disabled button draws nothing under the
// pointer, so this records the fact and the face throws it away - the state of
// the pointer is about the pointer, whether it means anything is about the
// control.
func (b *Button) MouseIn(*desktop.MouseEvent) {
	b.hovered = true
	b.Refresh()
}

// MouseMoved has nothing to do: the whole button looks the same wherever the
// pointer is inside it.
func (b *Button) MouseMoved(*desktop.MouseEvent) {}

// MouseOut forgets the pointer and any press it was holding.
//
// The press is forgotten here as well as in MouseUp because MouseUp does not
// necessarily come back: the driver sends the release to whatever is under the
// pointer when the button is let go (glfw/window.go, the Fyne guide section
// 3.27), so pressing this, sliding off and releasing would otherwise leave it
// drawn as pressed for the rest of its life.
func (b *Button) MouseOut() {
	b.hovered, b.pressed = false, false
	b.Refresh()
}

// MouseDown draws the pressed face.
func (b *Button) MouseDown(*desktop.MouseEvent) {
	b.pressed = true
	b.Refresh()
}

// MouseUp lets the press go.
func (b *Button) MouseUp(*desktop.MouseEvent) {
	b.pressed = false
	b.Refresh()
}

// FocusGained draws the ring, because the keyboard is now here - and it only
// ever arrives here from the keyboard, see the note on marked.
func (b *Button) FocusGained() {
	b.marked = true
	b.Refresh()
}

// FocusLost takes the ring away.
func (b *Button) FocusLost() {
	b.marked = false
	b.Refresh()
}

// TypedRune answers nothing, and that is the whole of it. One press of the
// space bar reaches a focused control TWICE from the desktop driver: as the
// key (internal/driver/glfw/window.go, processKeyPressed, which ends in
// TypedKey) and as the character (processCharInput, which ends in
// TypedRune) - measured in the pinned module on 2026-09-16, after an outside
// review said so. The toolkit's own button leaves TypedRune empty for exactly
// this reason. Until that day this pressed on the space character as well,
// on the sentence that the toolkit's button "answers to space", which is true
// of its TypedKey and not of this hook - so one press of space on Add batch
// added two batches in the real window, and no guard saw it, because the test
// driver delivers a key and a character as two separate calls.
func (b *Button) TypedRune(rune) {}

// TypedKey presses on Enter as well as the space bar. The toolkit answers space
// alone, so Enter on a focused button did nothing - and silence in place of an
// answer is the one thing this window refuses everywhere else. Both names of
// the key, because a keyboard has two and nobody thinks of them as different.
func (b *Button) TypedKey(event *fyne.KeyEvent) {
	if event == nil {
		return
	}
	switch event.Name {
	case fyne.KeyReturn, fyne.KeyEnter, fyne.KeySpace:
		b.Tapped(nil)
	}
}

// CreateRenderer draws the face.
func (b *Button) CreateRenderer() fyne.WidgetRenderer {
	b.ExtendBaseWidget(b)
	ring := canvas.NewRectangle(color.Transparent)
	ring.CornerRadius = RadiusField
	ring.StrokeColor = PaletteColour(theme.ColorNamePrimary, theme.VariantDark)
	bg := canvas.NewRectangle(color.Transparent)
	bg.CornerRadius = RadiusField
	label := canvas.NewText("", color.Transparent)
	label.TextStyle = fyne.TextStyle{Bold: true}
	label.Alignment = fyne.TextAlignCenter
	icon := canvas.NewImageFromResource(nil)
	icon.FillMode = canvas.ImageFillContain
	r := &buttonRenderer{button: b, ring: ring, bg: bg, label: label, icon: icon}
	r.Refresh()
	return r
}

// buttonRenderer draws one of the four faces, in whatever state the button is.
//
// The objects are made once and their properties changed, rather than a face
// rebuilt each Refresh: the shape of this face does not differ between states,
// only its colours do, so there is nothing to swap. The ring is one of the
// objects rather than a thing laid over the top, and it is placed OUTSIDE the
// face - see ringGap - so that a mark meaning "the keyboard is here" is never
// the same blue as the face it marks.
type buttonRenderer struct {
	button *Button
	ring   *canvas.Rectangle
	bg     *canvas.Rectangle
	label  *canvas.Text
	icon   *canvas.Image
}

func (r *buttonRenderer) Layout(size fyne.Size) {
	r.bg.Resize(size)
	r.bg.Move(fyne.NewPos(0, 0))
	// Outside the face on every side, without the face giving up any room -
	// Fyne clips nothing (the Fyne guide, section 3.3), so a child at a
	// negative offset is drawn there.
	r.ring.Resize(size.Add(fyne.NewSquareSize(ringGap * 2)))
	r.ring.Move(fyne.NewPos(-ringGap, -ringGap))

	glyph := r.button.look == Glyph
	hasIcon := r.button.Icon != nil
	hasLabel := r.button.Text != "" && !glyph
	iconSize := fyne.NewSquareSize(Theme().Size(theme.SizeNameInlineIcon))
	labelSize := r.label.MinSize()

	switch {
	case glyph:
		r.icon.Resize(iconSize)
		r.icon.Move(fyne.NewPos((size.Width-iconSize.Width)/2, (size.Height-iconSize.Height)/2))
	case hasIcon && hasLabel:
		gap := float32(GapInline)
		row := iconSize.Width + gap + labelSize.Width
		x := (size.Width - row) / 2
		r.icon.Resize(iconSize)
		r.icon.Move(fyne.NewPos(x, (size.Height-iconSize.Height)/2))
		r.label.Resize(labelSize)
		r.label.Move(fyne.NewPos(x+iconSize.Width+gap, (size.Height-labelSize.Height)/2))
	default:
		r.label.Resize(labelSize)
		r.label.Move(fyne.NewPos((size.Width-labelSize.Width)/2, (size.Height-labelSize.Height)/2))
	}
}

func (r *buttonRenderer) MinSize() fyne.Size {
	if r.button.look == Glyph {
		return fyne.NewSquareSize(GlyphButton)
	}
	size := r.label.MinSize()
	if r.button.Icon != nil {
		size.Width += Theme().Size(theme.SizeNameInlineIcon) + GapInline
	}
	if r.button.inBar {
		return size.Add(fyne.NewSize(BarButtonInsetX*2, BarButtonInsetY*2))
	}
	// The room inside a box to type in, on both axes, so a button stands the
	// same height as the field beside it.
	return size.Add(fyne.NewSquareSize(ControlInset * 2))
}

func (r *buttonRenderer) Refresh() {
	f := buttonFace(r.button.look, r.state())
	r.bg.FillColor = f.fill
	r.bg.StrokeColor = f.edge
	r.bg.StrokeWidth = f.edgeWidth
	if r.button.marked {
		r.ring.StrokeWidth = ringWidth
	} else {
		r.ring.StrokeWidth = 0
	}
	r.label.Text = r.button.Text
	r.label.Color = PaletteColour(f.ink, theme.VariantDark)
	r.label.TextSize = TextBody
	// A quiet button is words rather than a face, so it drops the weight and
	// a rank of size - the prototype of 2026-09-23, see buttonFace.
	r.label.TextStyle = fyne.TextStyle{Bold: r.button.look != Quiet}
	if r.button.look == Quiet {
		r.label.TextSize = TextCaption
	}
	if r.button.Icon != nil {
		// Coloured by the same ink as the words, so a glyph follows the state
		// of the button it stands in - the toolkit's own way of tinting a
		// resource.
		r.icon.Resource = theme.NewColoredResource(r.button.Icon, f.ink)
		r.icon.Show()
	} else {
		r.icon.Hide()
	}
	if r.button.Text == "" || r.button.look == Glyph {
		r.label.Hide()
	} else {
		r.label.Show()
	}
	// The pieces, never r.button - see redraw. This face is worn by the
	// explanation button through embedding, and a refresh asked for the inner
	// Button reached no canvas there: measured 2026-09-16, O217.
	redraw(r.bg, r.ring, r.label, r.icon)
	r.Layout(r.button.Size())
}

func (r *buttonRenderer) state() buttonState {
	switch {
	case r.button.Disabled():
		return stateDisabled
	case r.button.pressed:
		return statePressed
	case r.button.hovered:
		return stateHovered
	default:
		return stateRest
	}
}

func (r *buttonRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.ring, r.bg, r.icon, r.label}
}

func (r *buttonRenderer) Destroy() {}

// buttonState is which of a button's four states it is drawn in, in the order
// of precedence the toolkit uses: off wins over everything, then a press, then
// the pointer, then rest.
type buttonState int

const (
	stateRest buttonState = iota
	stateHovered
	statePressed
	stateDisabled
)

// face is the colours one look wears in one state.
//
// The ink is a name rather than a colour, because both the words and a glyph
// have to draw in it and a glyph is coloured by name.
type face struct {
	fill      color.Color
	ink       fyne.ThemeColorName
	edge      color.Color
	edgeWidth float32
}

// buttonFace is the colours of one look in one state, worked out from the
// palette rather than picked. The measurements behind the primary face are on
// ColorNameLift and ColorNameShade in theme.go. The rest follow from what a
// surface a button stands on already is.
func buttonFace(look Look, state buttonState) face {
	dark := theme.VariantDark
	border := face{edge: PaletteColour(theme.ColorNameSeparator, dark), edgeWidth: edgeWidth}
	if state == stateDisabled {
		// No fill on any look when it is off, and a faint outline so the shape
		// is still there to see. The words recede to the disabled ink, which
		// is a step brighter than a hint on purpose - a disabled button is a
		// control somebody may want to read, see ColorNameDisabled.
		border.ink = theme.ColorNameDisabled
		if look == Quiet || look == Glyph {
			border.edgeWidth = 0
		}
		return border
	}

	switch look {
	case Primary:
		f := face{fill: PaletteColour(theme.ColorNamePrimary, dark), ink: theme.ColorNameForegroundOnPrimary}
		switch state {
		case stateHovered:
			f.fill = blended(f.fill, PaletteColour(ColorNameLift, dark))
		case statePressed:
			f.fill = blended(f.fill, PaletteColour(ColorNameShade, dark))
		case stateRest, stateDisabled:
			// The face as it is. Disabled fades the whole button elsewhere.
		}
		return f
	case Secondary:
		// A filled face since 2026-09-21, on the owner's report from the
		// running window: an outline round nothing, with bold words in it,
		// read as a bordered word rather than as something to press -
		// Duplicate, Choose, Preview, Add a batch, all of them. The fill is
		// the surface a box to type in stands on, with the same edge, and
		// that is deliberate rather than a shortcut: on the desktop this
		// runs on a button and a field share a surface and are told apart by
		// their shape, centred bold words against a value at the left. The
		// note on Menu about a control you press drawn as one you type in
		// was about a MENU, whose word sits at the left exactly as a field's
		// does. The pointer lifts the face and a press lifts it further, the
		// way the palette lightens every dark face (ColorNameHover and
		// ColorNamePressed), worked out here as one opaque colour each.
		f := face{ink: theme.ColorNameForeground, edge: PaletteColour(theme.ColorNameInputBorder, dark), edgeWidth: edgeWidth}
		// The button's own surface since 2026-09-21, a step brighter than a
		// box to type in - the owner's report from the running window was
		// that a button in the field's colour read as a field.
		f.fill = PaletteColour(theme.ColorNameButton, dark)
		if wash := pointerFill(state); wash != color.Transparent {
			f.fill = blended(f.fill, wash)
		}
		return f
	default: // Quiet and Glyph: no resting edge, a surface only under the pointer.
		ink := theme.ColorNameForeground
		// Quiet as well since the prototype of 2026-09-23: Donate in bold
		// white at rest read as a heading of the bar, as loud as Generate.
		if (look == Glyph || look == Quiet) && state == stateRest {
			ink = theme.ColorNamePlaceHolder
		}
		return face{fill: pointerFill(state), ink: ink}
	}
}

// pointerFill is the wash a face that is not filled at rest wears under the
// pointer and under a press - translucent, so it is right on whatever surface
// the button stands on.
func pointerFill(state buttonState) color.Color {
	switch state {
	case stateHovered:
		return PaletteColour(theme.ColorNameHover, theme.VariantDark)
	case statePressed:
		return PaletteColour(theme.ColorNamePressed, theme.VariantDark)
	default:
		return color.Transparent
	}
}
