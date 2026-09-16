package catalogue

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
)

// The states below are reached the way a person reaches them: MouseIn is the
// pointer arriving, FocusGained is the keyboard arriving, MouseDown is a
// press. Those are the control's own methods, so the catalogue drives its
// state machine rather than reaching into it - and a control whose state
// cannot be reached this way is a control a person cannot reach either.

func button() Entry {
	var states []State
	for _, face := range []struct {
		look parts.Look
		name string
	}{{parts.Primary, "primary"}, {parts.Secondary, "secondary"}, {parts.Quiet, "quiet"}} {
		states = append(states, faceStates(face.look, face.name)...)
	}
	states = append(states, State{"long text", func() fyne.CanvasObject {
		return parts.NewButton(parts.Secondary, longText, func() {})
	}})
	states = append(states, glyphStates()...)
	return Entry{Name: "Button", Covers: []string{"GlyphButton"}, Natural: true, States: states}
}

// faceStates is one face of a button in the four states a person can put it
// in, and the fifth - disabled - for the two faces that draw it differently.
// Disabled, a primary and a secondary button are one face, the outline of a
// button nobody can press, so it is drawn once, under the secondary: the
// guard for identical states refused it twice.
func faceStates(look parts.Look, name string) []State {
	build := func() *parts.Button { return parts.NewButton(look, "Generate", func() {}) }
	states := []State{
		{name + " at rest", func() fyne.CanvasObject { return build() }},
		{name + " under the pointer", func() fyne.CanvasObject {
			b := build()
			b.MouseIn(&desktop.MouseEvent{})
			return b
		}},
		{name + " pressed", func() fyne.CanvasObject {
			b := build()
			b.MouseIn(&desktop.MouseEvent{})
			b.MouseDown(&desktop.MouseEvent{})
			return b
		}},
		{name + " holding the keyboard", func() fyne.CanvasObject {
			b := build()
			b.FocusGained()
			return b
		}},
	}
	if look != parts.Primary {
		states = append(states, State{name + " disabled", func() fyne.CanvasObject {
			b := build()
			b.Disable()
			return b
		}})
	}
	return states
}

// glyphStates is the mark-only button - the (i) beside a name - in its states.
func glyphStates() []State {
	build := func() *parts.Button { return parts.NewGlyphButton(theme.InfoIcon(), func() {}) }
	return []State{
		{"glyph at rest", func() fyne.CanvasObject { return build() }},
		{"glyph under the pointer", func() fyne.CanvasObject {
			b := build()
			b.MouseIn(&desktop.MouseEvent{})
			return b
		}},
		{"glyph holding the keyboard", func() fyne.CanvasObject {
			b := build()
			b.FocusGained()
			return b
		}},
		{"glyph disabled", func() fyne.CanvasObject {
			b := build()
			b.Disable()
			return b
		}},
	}
}

func chooser() Entry {
	options := []string{"png", "jpg", "avif"}
	// On a screen a menu stands inside the ring a field gives it (WithRing),
	// which is what draws the keyboard's mark round it - so the states with
	// something to say are built the way a field builds them. A refused menu
	// holding the keyboard draws as a refused one: the refusal is about what
	// will happen and wins, by the rule written on Ring, so it is not a state
	// of its own here.
	return Entry{Name: "Chooser", Covers: []string{"Ring", "WithRing", "Menu"}, Natural: true, States: []State{
		{"at rest", func() fyne.CanvasObject {
			return parts.Menu(parts.NewChooser(options, func(string) {}))
		}},
		{"chosen", func() fyne.CanvasObject {
			c := parts.NewChooser(options, func(string) {})
			c.SetSelected("avif")
			return parts.Menu(c)
		}},
		{"holding the keyboard", func() fyne.CanvasObject {
			c := parts.NewChooser(options, func(string) {})
			o, _ := parts.WithRing(parts.Menu(c))
			c.FocusGained()
			return o
		}},
		{"refused", func() fyne.CanvasObject {
			o, ring := parts.WithRing(parts.Menu(parts.NewChooser(options, func(string) {})))
			ring.Refuse(true)
			return o
		}},
		{"disabled", func() fyne.CanvasObject {
			c := parts.NewChooser(options, func(string) {})
			c.Disable()
			return parts.Menu(c)
		}},
		{"long value", func() fyne.CanvasObject {
			c := parts.NewChooser([]string{longText, "png"}, func(string) {})
			c.SetSelected(longText)
			return parts.Menu(c)
		}},
	}}
}

func entry() Entry {
	return Entry{Name: "Entry", States: []State{
		{"empty, with its hint", func() fyne.CanvasObject {
			e := parts.NewEntry()
			e.SetPlaceHolder("10mb")
			return e
		}},
		{"typed", func() fyne.CanvasObject {
			e := parts.NewEntry()
			e.SetText("2048")
			return e
		}},
		{"holding the keyboard", func() fyne.CanvasObject {
			e := parts.NewEntry()
			e.SetText("2048")
			e.FocusGained()
			return e
		}},
		{"disabled", func() fyne.CanvasObject {
			e := parts.NewEntry()
			e.SetText("2048")
			e.Disable()
			return e
		}},
		{"long text", func() fyne.CanvasObject {
			e := parts.NewEntry()
			e.SetText(longText)
			return e
		}},
	}}
}

func toggle() Entry {
	return Entry{Name: "Toggle", Natural: true, States: []State{
		{"off", func() fyne.CanvasObject { return parts.NewToggle(func(bool) {}) }},
		{"on", func() fyne.CanvasObject {
			t := parts.NewToggle(func(bool) {})
			t.SetChecked(true)
			return t
		}},
		{"under the pointer", func() fyne.CanvasObject {
			t := parts.NewToggle(func(bool) {})
			t.MouseIn(&desktop.MouseEvent{})
			return t
		}},
		{"holding the keyboard", func() fyne.CanvasObject {
			t := parts.NewToggle(func(bool) {})
			t.SetChecked(true)
			t.FocusGained()
			return t
		}},
		{"disabled", func() fyne.CanvasObject {
			t := parts.NewToggle(func(bool) {})
			t.SetChecked(true)
			t.Disable()
			return t
		}},
	}}
}

func segments() Entry {
	ways := []string{"Exact", "Range", "Boundary"}
	return Entry{Name: "Segments", Natural: true, States: []State{
		{"first chosen", func() fyne.CanvasObject {
			return parts.NewSegments(ways, func(string) {})
		}},
		{"middle chosen", func() fyne.CanvasObject {
			s := parts.NewSegments(ways, func(string) {})
			s.SetSelected("Range")
			return s
		}},
		{"holding the keyboard", func() fyne.CanvasObject {
			s := parts.NewSegments(ways, func(string) {})
			s.FocusGained()
			return s
		}},
		{"the last one under the pointer", func() fyne.CanvasObject {
			s := parts.NewSegments(ways, func(string) {})
			// On the last segment rather than at the origin: the origin is
			// the chosen segment, and a chosen segment under the pointer draws
			// as chosen, which the guard for identical states pointed out.
			s.MouseIn(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(s.MinSize().Width-parts.GapInline, 0)}})
			return s
		}},
		{"long words", func() fyne.CanvasObject {
			return parts.NewSegments([]string{"Exactly this size", "Somewhere in a range", "On a boundary"}, func(string) {})
		}},
	}}
}
