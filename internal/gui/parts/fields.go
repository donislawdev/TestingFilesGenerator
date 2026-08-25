package parts

import (
	"errors"
	"strings"

	"fyne.io/fyne/v2"
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
	// body is the field without the room under it for a refusal, so a row can
	// put the refusals of every field in it across the whole width.
	body fyne.CanvasObject
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

	// required are the settings this screen will not run without, by the same
	// key a refusal names them by.
	//
	// A set on the registry rather than an argument at the thirty-two places a
	// field is built. Both would work and only one of them stays true: a flag
	// typed at a call site is a second opinion about what the engine refuses,
	// and it is the copy that drifts. Declared in one line per screen and
	// checked against the engine itself by
	// TestAStarIsOnEveryBoxTheRunWillNotDoWithout, which blanks each box in
	// turn and asks whether the run actually refuses it - so the star cannot
	// quietly come to mean something the run does not do.
	//
	// Read by Add, so a screen names these BEFORE it builds its fields. Getting
	// that order wrong is not silent: the guard renders the screen and counts
	// the stars that are really there.
	required map[string]bool

	// inBytes are the settings whose value is one size, so their field shows
	// what that size counts out to. See InBytes.
	inBytes map[string]bool

	// tell is called with the setting whose box was typed into.
	//
	// It lives here rather than at the call sites because the call sites are
	// what this type exists to stop trusting. A screen wires it once and every
	// field ever added is wired with it, including the ones a chosen format or
	// a chosen preset declares long after the screen was built.
	tell func(setting string)

	// shortcuts is where a box sends a shortcut it has no use for.
	//
	// Here rather than at the call sites for the same reason tell is: the call
	// sites are what this type exists to stop trusting. A screen wires it once
	// and every box ever added is wired with it, including the ones a chosen
	// format declares long after the screen was built - and a box that missed
	// the wiring would be one where Ctrl+Enter does nothing, which is a defect
	// nobody would find by looking.
	shortcuts func(fyne.Shortcut)
}

// PassShortcutsTo says where the boxes of this screen should send a shortcut
// they have no use for. Called once, before the fields are built.
func (s *Fields) PassShortcutsTo(deliver func(fyne.Shortcut)) {
	s.shortcuts = deliver
	// The boxes that already exist, for a screen that wires this late.
	for _, f := range s.list {
		s.wireShortcuts(f.Control)
	}
}

// wireShortcuts hands every box inside a control the way out.
func (s *Fields) wireShortcuts(control fyne.CanvasObject) {
	if s.shortcuts == nil {
		return
	}
	for _, box := range boxesIn(control) {
		box.PassShortcutsTo(s.shortcuts)
	}
}

// NewFields starts an empty screen.
func NewFields() *Fields {
	return &Fields{by: map[string]*Field{}, required: map[string]bool{}, inBytes: map[string]bool{}}
}

// Require names the settings this screen will not run without, so their fields
// are drawn with the mark that says so.
//
// Called before the fields are built, and repeatedly where a screen rebuilds -
// the batch screen addresses a setting by the batch it belongs to, so the same
// setting of a second batch is a different name and has to be named again.
//
// What is NOT named here is as deliberate as what is. Where a rule binds
// several boxes together - one of size, size range or a boundary - no single
// one of them is required, because filling either of the others satisfies the
// run. Marking all three would say "fill all three", which is false. The line
// above them says "Fill in one of these three." and that is the sentence that
// carries it. The guard uses the same definition: a setting is required when
// blanking THAT box alone, with everything else answered, makes the run refuse.
func (s *Fields) Require(settings ...string) {
	for _, setting := range settings {
		s.required[setting] = true
	}
}

// Required says whether this screen was told it cannot run without a setting,
// for a guard to compare against what the engine actually does.
func (s *Fields) Required(setting string) bool { return s.required[setting] }

// InBytes names the settings whose value is one size, so their fields show what
// that size comes to.
//
// Declared rather than guessed, and the guessing version is worth naming
// because it is the obvious one: trying core.ParseSize on every box and showing
// a count wherever it succeeds. "How many" holds 1, which parses as one byte,
// so every count box on every screen would have grown a caption reading "1 B".
//
// A size RANGE and a list of sizes are deliberately not named here. Both hold
// more than one number, so one count underneath would be answering about half
// of what is in the box.
func (s *Fields) InBytes(settings ...string) {
	for _, setting := range settings {
		s.inBytes[setting] = true
	}
}

// counter is the caption showing what the size in a field comes to, or nil for
// a field that does not hold one.
//
// Wired here rather than at a call site for the reason the rest of this type
// exists: a screen names the size settings once and every field ever added
// under one of those names gets its count, including the ones a chosen format
// declares long after the screen was built.
func (s *Fields) counter(setting string, control fyne.CanvasObject) fyne.CanvasObject {
	if !s.inBytes[setting] {
		return nil
	}
	count := newByteCount()
	for _, b := range boxesIn(control) {
		b := b
		already := b.OnChanged
		b.OnChanged = func(value string) {
			if already != nil {
				already(value)
			}
			count.show(value)
		}
		// And once now, for a box that arrives with a size already in it.
		count.show(b.Text)
	}
	return count
}

// Add builds a field and hands back the thing to put on the screen.
//
// Every field gets an area for its refusal and an edge round its control. There
// is no variant without them, on purpose: the variant IS the defect.
func (s *Fields) Add(setting, label, hint string, detail Detail, control fyne.CanvasObject) fyne.CanvasObject {
	// The line under a field is now the first sentence behind its button - see
	// alsoSaying. Folded here rather than at the thirty-two call sites, so a
	// field that still carries one is not something anybody can write.
	object, body, area := FieldSaying(label, "", alsoSaying(hint, detail),
		s.required[setting], s.counter(setting, control), control)
	f := &Field{Setting: setting, Label: label, Control: control, area: area, object: object, body: body}
	s.list = append(s.list, f)
	// Last one wins, which is what a rebuilt screen needs: the preset screen
	// throws its parameter fields away and draws the new preset's, and an entry
	// left over would point at a box that is no longer on the screen.
	s.by[setting] = f
	s.listen(setting, control)
	s.wireShortcuts(control)
	return object
}

// AddToggle is a switch, which is the one control that carries its own name.
//
// It goes through the same registry as everything else. A switch cannot hold a
// value the engine refuses today, and leaving it out would be an exception to
// remember - which is the class of thing this type exists to end.
func (s *Fields) AddToggle(setting, name, hint string, detail Detail, check *Toggle) fyne.CanvasObject {
	object, body, area := ToggleSaying(name, "", alsoSaying(hint, detail), check)
	f := &Field{Setting: setting, Label: name, Control: check, area: area, object: object, body: body}
	s.list = append(s.list, f)
	s.by[setting] = f
	return object
}

// Row puts fields side by side and gives their refusals the whole width.
//
// A refusal in this tool has four parts - what happened, why, what is allowed,
// what to do instead - so it is a sentence and not a word. Inside a column of
// a row it gets half the form to say that in. Measured off a render on
// 2026-08-20: a size below what BMP can make wrapped onto four lines in the
// left column while the right half of the panel was empty, and the four lines
// pushed everything under them down by three.
//
// So the controls share the row and the messages do not. The message still
// belongs to its own field - it is the same area, marked and cleared with the
// same box - it is just laid out where there is room to read it.
//
// Given something that is not a field it falls back to putting it in the row
// whole, because a row of one field and one plain object is a shape this
// screen is allowed to build.
func (s *Fields) Row(objects ...fyne.CanvasObject) fyne.CanvasObject {
	bodies := make([]fyne.CanvasObject, 0, len(objects))
	areas := make([]fyne.CanvasObject, 0, len(objects))
	for _, o := range objects {
		f := s.holding(o)
		if f == nil || f.body == nil {
			bodies = append(bodies, o)
			continue
		}
		bodies = append(bodies, f.body)
		areas = append(areas, f.area.Object())
	}
	return Column(GapTight, append([]fyne.CanvasObject{Row(bodies...)}, areas...)...)
}

// holding is the field one object is the whole of, or nil.
func (s *Fields) holding(object fyne.CanvasObject) *Field {
	for _, f := range s.list {
		if f.object == object {
			return f
		}
	}
	return nil
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
func boxesIn(o fyne.CanvasObject) []*Entry {
	switch it := o.(type) {
	case *Entry:
		return []*Entry{it}
	case *fyne.Container:
		var out []*Entry
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
func (s *Fields) Mark(setting string, err error) bool {
	f, ok := s.by[setting]
	if !ok {
		return false
	}
	f.Say(inTheWordsOnScreen(f, err))
	return true
}

// inTheWordsOnScreen puts a refusal into the vocabulary of the screen it is
// shown on.
//
// The engine words a refusal once for both surfaces, and it names the setting
// by the key a recipe writes - "bmp: width cannot be ...". On the command line
// that is the only name there is. In this window the box above the message is
// called Width, so an unchanged refusal names something the screen does not
// have. That defect arrived WITH the labels on 2026-08-20: before them the key
// and the label were the same string.
//
// The refusal is ASKED for its own sentence in another name rather than having
// one searched for in it. From 2026-08-20 to 2026-08-25 this ran
// strings.ReplaceAll over the finished message, and rendering every screen in
// its refused state measured what that does:
//
//   - "the output Output directoryectory is empty", because the key behind
//     Output directory is "dir" and it lives inside "directory". Also
//     identifies, formats, sizes and size-range.
//   - "for example Group name: invoices", offering a recipe key no recipe has,
//     because the example quoted the key as a key.
//   - "an Group name", because the article in front of it stayed.
//
// Searching prose for a short word cannot be made safe by more rules about
// which matches to skip - each rule is a guess about English. The sentence
// says where its own name goes instead, and anything without a slot is shown
// exactly as the engine wrote it, which is the state this window was in before
// the labels and is merely plain rather than wrong. See core.SettingSlot.
func inTheWordsOnScreen(f *Field, err error) string {
	var reworded interface {
		InTheWordsOf(string) string
	}
	if f.Label != "" && errors.As(err, &reworded) {
		return reworded.InTheWordsOf(f.Label)
	}
	return err.Error()
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

// Lookup is the field one setting is drawn by, or nil where this screen draws
// none - which is ordinary rather than exceptional, since a refusal can name a
// setting the screen it landed on does not have.
func (s *Fields) Lookup(setting string) *Field { return s.by[setting] }

// Marked is every setting whose box is currently saying something.
//
// It exists for a screen that puts boxes away: scrolling cannot show a box
// inside something folded shut, so the screen has to be told which ones to open
// before the form is moved. Asked for by setting rather than by field, because
// what a screen folds is named the same way everything else here is.
func (s *Fields) Marked() []string {
	var out []string
	for _, f := range s.list {
		if f.Saying() != "" {
			out = append(out, f.Setting)
		}
	}
	return out
}

// Freeze takes every control out of use while a run is going, and gives them
// back afterwards.
//
// The buttons that start a run were already dealt with - two of them looking
// pressable during a run invites a second run into the directory the first one
// is still filling - but the form itself stayed fully editable, with nothing
// saying whether changing the output directory mid run affected the files being
// written into the old one. It almost certainly did not, and the person doing
// it had no way to know that (O106).
//
// Registry wide rather than a list of controls to freeze, for the reason the
// registry exists: a field added later is covered without anybody remembering.
// Controls that cannot be disabled are skipped rather than reported, because
// this is not the place that decides what a control is.
func (s *Fields) Freeze(frozen bool) {
	for _, f := range s.list {
		// The registered object is sometimes a container fixing the control's
		// width rather than the control - see inside, which is where asking
		// the wrapper instead cost this a silent no-op.
		found := inside(f.Control, func(o fyne.CanvasObject) bool {
			_, ok := o.(fyne.Disableable)
			return ok
		})
		if found == nil {
			continue
		}
		control := found.(fyne.Disableable)
		if frozen {
			control.Disable()
			continue
		}
		control.Enable()
	}
}

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
