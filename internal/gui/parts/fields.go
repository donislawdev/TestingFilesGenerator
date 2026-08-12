package parts

import (
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
	return object
}

// AddToggle is a switch, which is the one control that carries its own name.
//
// It goes through the same registry as everything else. A switch cannot hold a
// value the engine refuses today, and leaving it out would be an exception to
// remember - which is the class of thing this type exists to end.
func (s *Fields) AddToggle(setting, name, hint string, detail Detail, check *widget.Check) fyne.CanvasObject {
	object, area := ToggleSaying(name, hint, detail, check)
	f := &Field{Setting: setting, Label: name, Control: check, area: area, object: object}
	s.list = append(s.list, f)
	s.by[setting] = f
	return object
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
