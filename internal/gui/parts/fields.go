package parts

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

// Field is one labelled control that can say it was the one refused.
//
// A TYPE rather than a function, changed on 2026-08-12, and the change is
// structural rather than tidy. Marking a refused box arrived the day before as
// two functions - Field for a plain one, FieldSaying for one that could carry a
// refusal - so whether a box could be marked depended on which of the two
// somebody typed at the call site. Measured on the generate screen: five could
// and three could not, and every setting a format declares was in the second
// group. With thirteen formats that is dozens of boxes whose refusal appears at
// the foot of the form under an unmarked field.
//
// There is one constructor now and it takes the setting the field is about.
// A field with nowhere to put its refusal is no longer something anybody can
// write, which is the only kind of fix that survives the fiftieth field. See
// docs/UX.md section 7.0.
type Field struct {
	// Setting is the key a refusal names this field by - a recipe key, a
	// format property or a preset parameter. Never a label: the labels are
	// translated and the keys are the vocabulary both surfaces share.
	Setting string
	// Label is what somebody reads above the control.
	Label string
	// Control is the widget itself, without the edge drawn round it.
	Control fyne.CanvasObject

	area   *ErrorArea
	object fyne.CanvasObject
}

// Object is the field, to put on a screen.
func (f *Field) Object() fyne.CanvasObject { return f.object }

// Say shows a refusal about this field and marks its box. Both, always: the
// colour alone says nothing to a reader who cannot tell it from the others, and
// the sentence alone leaves them counting boxes.
func (f *Field) Say(message string) { f.area.Say(message) }

// Clear takes the refusal and the mark back together.
func (f *Field) Clear() { f.area.Clear() }

// Saying is what this field is currently complaining about, for a guard.
func (f *Field) Saying() string { return f.area.Text() }

// Fields is every field of one screen, in the order they were built.
//
// It exists so that a refusal is placed by asking a registry rather than by
// looking up a map somebody filled in by hand. The map was real and it was
// filled in at eight call sites, three of which nobody had got to.
type Fields struct {
	list []*Field
	by   map[string]*Field

	// tell is called with the setting whose box was typed into.
	//
	// It lives here rather than at the call sites because the call sites are
	// what this type exists to stop trusting. A screen wires it once and every
	// field ever added is wired with it, including the ones a chosen format or
	// a chosen preset declares long after the screen was built.
	tell func(setting string)
}

// NewFields starts an empty screen.
func NewFields() *Fields { return &Fields{by: map[string]*Field{}} }

// Add builds a field and hands back the thing to put on the screen.
//
// Every field gets an area for its refusal and an edge round its control. There
// is no variant without them, on purpose: the variant IS the defect.
func (s *Fields) Add(setting, label, hint string, detail Detail, control fyne.CanvasObject) fyne.CanvasObject {
	object, area := FieldSaying(label, hint, detail, control)
	f := &Field{Setting: setting, Label: label, Control: control, area: area, object: object}
	s.list = append(s.list, f)
	// Last one wins, which is what a rebuilt screen needs: the preset screen
	// throws its parameter fields away and draws the new preset's, and an entry
	// left over would point at a box that is no longer on the screen.
	s.by[setting] = f
	s.listen(setting, control)
	return object
}

// AddToggle is a switch, which is the one control that carries its own name.
//
// It goes through the same registry as everything else. A switch cannot hold a
// value the engine refuses today, and leaving it out would be an exception to
// remember - which is the class of thing this type exists to end.
func (s *Fields) AddToggle(setting, name, hint string, detail Detail, check *Toggle) fyne.CanvasObject {
	object, area := ToggleSaying(name, hint, detail, check)
	f := &Field{Setting: setting, Label: name, Control: check, area: area, object: object}
	s.list = append(s.list, f)
	s.by[setting] = f
	return object
}

// WhenTypedIn asks to be told whenever anything on this screen is typed into.
//
// Set once, before or after the fields exist - Add wires whatever is added from
// then on, and the fields already there are wired here. A screen that had to
// call this per field would be the map filled in by hand all over again, which
// is the defect this whole type was built to end. See docs/UX.md section 7.0.
func (s *Fields) WhenTypedIn(tell func(setting string)) {
	s.tell = tell
	for _, f := range s.list {
		s.listen(f.Setting, f.Control)
	}
}

// listen makes every box under one control report what is typed into it.
//
// Chained rather than assigned, because a control that already had something to
// do on a change keeps doing it. Nothing in this window does today, and a
// silently dropped callback is the kind of thing nobody notices until the
// screen stops reacting.
func (s *Fields) listen(setting string, control fyne.CanvasObject) {
	for _, box := range boxesIn(control) {
		already := box.OnChanged
		box.OnChanged = func(value string) {
			if already != nil {
				already(value)
			}
			if s.tell != nil {
				s.tell(setting)
			}
		}
	}
}

// Blank says whether this field is a box with nothing in it.
//
// A screen checking values while somebody types has to tell "wrong" from "not
// filled in yet". Emptying a box to retype it is the commonest thing anybody
// does in a form, and a form that turns red the moment the box is empty is
// worse than one that waits - which is why the toolkit suppresses its own
// validation while a box has the keyboard, widget/entry_validation.go. Pressing
// Preview or Generate marks it anyway: by then the person has said they are
// done.
func (s *Fields) Blank(setting string) bool {
	f, found := s.by[setting]
	if !found {
		return false
	}
	boxes := boxesIn(f.Control)
	if len(boxes) == 0 {
		return false
	}
	for _, box := range boxes {
		if strings.TrimSpace(box.Text) != "" {
			return false
		}
	}
	return true
}

// boxesIn is every box somebody types into under one control.
//
// A walk rather than a cast, because a field's control is rarely the box
// itself: a number is held to a fixed width by a container round it, and the
// output directory carries a button beside it.
func boxesIn(o fyne.CanvasObject) []*widget.Entry {
	switch it := o.(type) {
	case *widget.Entry:
		return []*widget.Entry{it}
	case *fyne.Container:
		var out []*widget.Entry
		for _, child := range it.Objects {
			out = append(out, boxesIn(child)...)
		}
		return out
	}
	return nil
}

// Len is how many fields there are, which is what a screen remembers before it
// adds the ones a chosen format or preset declares.
func (s *Fields) Len() int { return len(s.list) }

// KeepFirst throws away everything added after the count given.
//
// For a screen redrawing part of itself: the settings of a PNG are not the
// settings of a WAV, and the fields above them do not change. Keeping the whole
// registry would leave a refusal pointing at a box that is no longer on the
// screen, and clearing all of it would take the fields that never moved.
func (s *Fields) KeepFirst(n int) {
	if n < 0 || n > len(s.list) {
		return
	}
	s.list = s.list[:n]
	// Rebuilt rather than deleted from, because the same key can have been
	// added twice and the map holds the last one. Walking forwards leaves
	// exactly the entry the list agrees with.
	s.by = make(map[string]*Field, n)
	for _, f := range s.list {
		s.by[f.Setting] = f
	}
}

// Mark shows a refusal under the field it is about, and says whether it found
// one. A caller that gets false has a message about something this screen has
// no box for, and it belongs at the foot of the form.
func (s *Fields) Mark(setting, message string) bool {
	f, ok := s.by[setting]
	if !ok {
		return false
	}
	f.Say(message)
	return true
}

// Clear takes back whatever one field was complaining about.
//
// One rather than all, because the others are complaining about values nobody
// has touched and those complaints are still true. Wiping them the moment a
// different box is typed into makes a refusal look answered when it is not -
// found on 2026-08-18 by the stored tree, where a size the format had refused
// went unmarked because the scene pinned the output directory afterwards.
func (s *Fields) Clear(setting string) {
	if f, found := s.by[setting]; found {
		f.Clear()
	}
}

// ClearAll empties every field, not only the one last used. Clearing one would
// leave a message under a field whose value was fixed two presses ago.
func (s *Fields) ClearAll() {
	for _, f := range s.list {
		f.Clear()
	}
}

// All is every field, for a guard to walk. In the order they were added, which
// is the order they appear on the screen.
func (s *Fields) All() []*Field { return s.list }

// Controls is every control a person types into, for a guard comparing the
// registry against the tree. That comparison is the answer to "what about the
// fiftieth field": a control on the screen that is not here is a control whose
// refusal has nowhere to go.
func (s *Fields) Controls() []fyne.CanvasObject {
	out := make([]fyne.CanvasObject, 0, len(s.list))
	for _, f := range s.list {
		out = append(out, f.Control)
	}
	return out
}
