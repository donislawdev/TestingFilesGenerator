package parts

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// PropertyField is a control drawn from a declaration, with the way to read it
// back.
type PropertyField struct {
	// Name is the key this value goes under, exactly as the format declared it.
	// Never translated - it is a contract key, G8.
	Name string
	// Control is the widget itself.
	Control fyne.CanvasObject
	// Value is what the user has put in, as text. Text because that is what a
	// recipe and a --set flag both carry, so the engine judges one thing however
	// it was asked.
	Value func() string
	// Chosen says whether what Value returns is something somebody picked,
	// rather than what the field started on.
	//
	// It exists because a menu cannot be empty: it opens on its declared
	// default and so always has a value, which is fine for what is SENT - a
	// default present and a default absent mean the same to a format - and
	// wrong for what is SAID. A folded section that lists every setting it
	// holds, whether or not anybody touched one, stops being a summary: log
	// declares seven and the line ran off the edge of the window, reported
	// from a screenshot on 2026-08-31.
	//
	// A box somebody types in answers this the old way, on emptiness, and
	// that difference is deliberate. For a preset parameter an empty box and
	// a typed default are NOT the same thing - the manifest records which
	// numbers were ours through defaulted, untouchable rule 5 - so hiding a
	// typed value there would hide a real one.
	Chosen func() bool
}

// FromProperty draws the field a declaration describes.
//
// This is the whole reason properties are declared rather than only named. A
// declaration carries a name, a kind, a range or a closed set, a default and a
// sentence - which is everything needed to draw a field, so a format that gains
// a property gains its field with no window code at all.
//
// It validates nothing, and that is G1 rather than an omission. The value goes
// to the engine as text and the registry refuses what the declaration forbids,
// in the words the declaration builds. A number box that stops at its own idea
// of the range would be a second copy of the rules, and the copy would be the
// one that drifts - with the window quietly accepting or refusing something the
// command line does not.
//
// An empty control sends nothing rather than sending emptiness. The registry
// reads an empty value as "not stated", which is what leaving a field alone
// means, so a format that works its own answer out still gets to.
func FromProperty(p format.Property) PropertyField {
	switch p.Kind {
	case format.PropertyChoice:
		return choiceField(p)
	case format.PropertyBool:
		return boolField(p)
	default:
		return textField(p)
	}
}

// An empty control means "not stated", and no box somebody types in is filled
// in with its declared default. That is a correction rather than a preference,
// and it was found by a guard rather than by reading.
//
// Filling the default in makes "I did not state this" impossible to express
// from a window, because every field arrives at the engine carrying a value.
// For a format setting that is harmless - the value equals the default either
// way. For a preset parameter it destroys a contract: the manifest records
// WHICH numbers were ours through defaulted, untouchable rule 5, and a window
// that states everything would never mark anything. Measured on 2026-08-05 by
// running one preset from both surfaces: the command line recorded
// defaulted: [spread] and the window recorded nothing.
//
// Menus stopped being part of that on 2026-08-27, and the exception is measured
// rather than granted. The paragraph above stays because it is still true of
// every box a person types in - see choiceField for the two ends of the
// measurement that took menus out of it.
//
// What the default is stays visible, in the sentence under the field - Allowed
// ends with ", default 10mb" - and in the placeholder.

// choiceField is a closed set, so it is a list rather than a box to type in.
// Nobody can misspell a value that is not typed. A setting with a declared
// default opens on that default.
//
// It used to open on an extra first entry reading "not stated - a4", and the
// reason written here was that the entry is what lets the manifest record a
// value as defaulted rather than chosen, untouchable rule 5. Measured on
// 2026-08-27, at both ends, and that is not what happens at either:
//
//   - A format setting. An ICO run with embed left alone and one asking for
//     embed=bmp produce the same bytes and the same manifest, which writes
//     embed: bmp in both. The word defaulted appears in neither.
//   - A preset setting. Defaulted is built from the parameters a preset
//     DECLARES, and format is a global flag rather than one of them, so a run
//     that never states it records defaulted: [spread] and never names format.
//
// So the entry cost every menu a third value that read as a state and was not
// one, beside two entries that already did the same thing. The one thing it did
// change is recipe_hash, since two recipes differing by a line are two recipes,
// and no screen in this window writes a recipe out.
//
// Boxes somebody types in are NOT this case and were left alone: a preset's
// limit and spread are parameters, so an empty box there does reach the
// manifest as defaulted. Measured in the same run.
func choiceField(p format.Property) PropertyField {
	sel := NewChooser(p.Choices, nil)
	sel.PlaceHolder = leftAlone(p)
	if p.Default != "" {
		sel.SetSelected(p.Default)
	}
	return PropertyField{
		Name:    p.Name,
		Control: sel,
		Value:   func() string { return sel.Selected },
		Chosen:  func() bool { return sel.Selected != "" && sel.Selected != p.Default },
	}
}

// boolField is the one kind that cannot be left unstated. A switch has two
// positions and no third one for silence, so it starts where the declaration
// says and always sends what it shows.
func boolField(p format.Property) PropertyField {
	// A Toggle rather than the toolkit's own switch, for the reason Toggle
	// gives: pressed with the mouse, widget.Check leaves a disc behind it in
	// the focus colour and never takes it off. Declared settings are the second
	// place switches come from and the one nobody would remember, because there
	// is no bool property in the registry today - the first format to declare
	// one would have arrived with the defect already fixed everywhere else.
	check := NewToggle("", nil)
	check.SetChecked(p.Default == "true")
	return PropertyField{
		Name:    p.Name,
		Control: check,
		Value:   func() string { return strconv.FormatBool(check.Checked) },
		Chosen:  func() bool { return strconv.FormatBool(check.Checked) != p.Default },
	}
}

// textField covers a number, a size and free text alike, because all three are
// a box somebody types into and the difference between them is what the engine
// accepts rather than what the box does.
func textField(p format.Property) PropertyField {
	entry := NewEntry()
	entry.SetPlaceHolder(leftAlone(p))
	return PropertyField{
		Name:    p.Name,
		Control: entry,
		Value:   func() string { return entry.Text },
		// A box answers on emptiness, for the reason on Chosen.
		Chosen: func() bool { return entry.Text != "" },
	}
}

// leftAlone is what happens if this field is not touched. A declaration with no
// default means the format works the value out from the size it was asked for.
func leftAlone(p format.Property) string {
	if p.Default == "" {
		return text.PlaceholderWorkedOut()
	}
	return text.PlaceholderLeftEmpty(p.Default)
}

// PropertyFields draws every field one format declares, in the order it
// declared them, each with the sentence saying what it takes.
//
// The sentence comes from Property.Allowed, which is the one "tfg formats"
// prints. Two surfaces describing one format in two ways is D1 breaking in the
// place nobody thinks to compare, so there is one sentence and both read it.
// It registers each one with the screen, so a setting a format declares can be
// told it was the one refused. Until 2026-08-12 these were the only fields on
// either screen that could not: they were built with the plain Field function,
// which had nowhere to put a refusal, so "width must be between 1 and 20000"
// appeared at the foot of the form with nothing marked. Thirteen formats
// declare fourteen of these between them.
func PropertyFields(d format.Descriptor, into *Fields, tips *Tips) ([]PropertyField, []fyne.CanvasObject) {
	fields, objects := DeclaredFields(d.Properties, into, tips)

	// A rule binding two settings belongs beside them and nowhere else. Drawn
	// from Min and Max alone, two number boxes would offer twenty thousand by
	// twenty thousand and the run would refuse the pair - which is the defect
	// JointLimit was declared to close.
	//
	// Here rather than in DeclaredFields because only a format has them. A
	// preset declares parameters and no joint limits, so asking it for some
	// would be a field nothing fills.
	for _, j := range d.JointLimits {
		objects = append(objects, Note(j.Describe()))
	}
	return fields, objects
}

// DeclaredFields draws every field a list of declared settings describes, in
// the order it was declared, each with the sentence saying what it takes.
//
// THREE screens draw declared settings, not two, and until 2026-08-24 only two
// of them came through here. The preset screen had a loop of its own, because a
// preset declares its parameters as the same format.Property and it grew its
// own way of drawing them - so everything added here had to be remembered
// there. It already had not been: pairing two narrow settings onto one row went
// in on 2026-08-20 and never reached the preset screen, and the count of bytes
// beside a size had to be written twice on the day it was added, which is what
// made this obvious.
//
// The visible change today is none, and that is worth saying rather than
// hiding: the one preset this build registers declares a single narrow
// parameter, so there is no pair to make. What changes is that the fourth thing
// added to a declared field arrives on all three screens instead of two.
func DeclaredFields(declared []format.Property, into *Fields, tips *Tips) ([]PropertyField, []fyne.CanvasObject) {
	fields := make([]PropertyField, 0, len(declared))
	objects := make([]fyne.CanvasObject, 0, len(declared))

	pair := PairNarrow(into.Row)
	flush := func() { objects = append(objects, pair.rest()...) }

	for _, p := range declared {
		f := FromProperty(p)
		fields = append(fields, f)
		// A setting the format itself calls a size gets its count of bytes.
		// Read off the declaration rather than off the name, so a format that
		// declares a size tomorrow gets this without a line of window code -
		// the same reason the field is drawn from the declaration at all.
		if p.Kind == format.PropertySize {
			into.InBytes(p.Name)
		}
		// The label is the window's own wording and the key a recipe writes
		// is behind the button. Until 2026-08-20 the key WAS the label, so a
		// screen where every other name is capitalised and spaced carried
		// bit_depth and entry_size among them.
		//
		// The button is what makes that safe rather than a loss: this is a tool
		// whose window and whose recipe file are two ways into one engine, so
		// somebody who finds a setting here has to be able to write it down.
		object := into.Add(p.Name, text.SettingLabel(p.Name), PropertyDetail(p),
			tips.Say(text.SettingKey(p.Name)), ShapedFor(p, f.Control))
		if narrowOnAScreen(p) {
			pair.add(object)
			continue
		}
		flush()
		objects = append(objects, object)
	}
	flush()
	return fields, objects
}

// ShapedFor gives a control the width the value in it needs.
//
// A box is a promise about what goes in it. Measured off a render on
// 2026-08-20: the width and height boxes of a BMP were 806 px wide for a whole
// number from 1 to 20000, on a screen where "how many" - also a whole number -
// was 140. The declaration says which of the two a setting is, so nothing here
// names a format.
//
// Only numbers and sizes. A closed set is as wide as its longest value plus
// the arrow, and free text has no length to promise.
// The width is the wider of the number box every other whole number on these
// screens uses and whatever it takes to show this field's own placeholder.
// Shrinking to the first alone clipped "worked out from the size" mid-word -
// which is the same defect the other way up, since a box has to be able to
// show what it is already showing.
func ShapedFor(p format.Property, control fyne.CanvasObject) fyne.CanvasObject {
	if !narrowOnAScreen(p) {
		return control
	}
	return Sized(fyne.Max(NumericWidth, roomFor(leftAlone(p))), control)
}

// roomFor is how wide a box has to be to show a string without cutting it.
//
// The toolkit lays an entry's text out inside two paddings on each side. Asked
// of the font rather than guessed from a character count, because the font is
// proportional and "worked out from the size" is mostly narrow letters.
func roomFor(words string) float32 {
	size := fyne.MeasureText(words, theme.TextSize(), fyne.TextStyle{})
	return size.Width + theme.InnerPadding()*2 + theme.Padding()*2
}

// narrowOnAScreen is whether a setting's value is short enough that a full
// width box would be lying about it.
//
// It also decides which settings go two to a row, and those are one question
// rather than two: a row of two is only readable when neither of them wanted
// the width in the first place.
func narrowOnAScreen(p format.Property) bool {
	switch p.Kind {
	case format.PropertyInt, format.PropertySize:
		return true
	default:
		return false
	}
}

// PairNarrow lays settings two to a row where both of them are narrow.
//
// Two boxes for a number stacked one above the other cost a row of height each
// and leave two thirds of the panel empty beside them. Which ones are narrow is
// the declared kind, so nothing here names a format - and both screens that
// draw a format's settings go through this, which is the point. The width went
// into one of them first and the other kept drawing full width boxes for a
// commit, which is the shape D1 comes apart in.
func PairNarrow(row func(...fyne.CanvasObject) fyne.CanvasObject) *Pairs {
	return &Pairs{row: row}
}

// Pairs collects narrow fields until something wide arrives or the list ends.
type Pairs struct {
	row     func(...fyne.CanvasObject) fyne.CanvasObject
	pending []fyne.CanvasObject
}

func (p *Pairs) add(object fyne.CanvasObject) { p.pending = append(p.pending, object) }

// Add takes one narrow field, for a caller outside this package.
func (p *Pairs) Add(object fyne.CanvasObject) { p.add(object) }

func (p *Pairs) rest() []fyne.CanvasObject {
	var out []fyne.CanvasObject
	for len(p.pending) > 0 {
		if len(p.pending) == 1 {
			out = append(out, p.pending[0])
			p.pending = nil
			break
		}
		out = append(out, p.row(p.pending[0], p.pending[1]))
		p.pending = p.pending[2:]
	}
	return out
}

// Rest is everything collected so far, in rows, for a caller outside this
// package.
func (p *Pairs) Rest() []fyne.CanvasObject { return p.rest() }

// Narrow says whether a declared setting is one this would pair.
func Narrow(p format.Property) bool { return narrowOnAScreen(p) }

// PropertyDetail is what a property takes and what it is for, in that order.
// What it takes comes first because that is what somebody looking at an empty
// box needs.
//
// Exported because two screens draw these fields now, and the sentence has to be
// composed one way. A screen assembling it itself would be D1 breaking in the
// place nobody compares: two surfaces describing one format in two wordings.
func PropertyDetail(p format.Property) string {
	detail := allowedOnAScreen(p)
	if p.Detail != "" {
		if detail != "" {
			detail += ". "
		}
		detail += p.Detail
	}
	return detail
}

// allowedOnAScreen is what a setting takes, for somebody who can see the
// control as well as the sentence.
//
// It is Property.Allowed everywhere except a closed set of values, and there it
// says nothing, because the menu above the sentence IS the list. The format
// setting spelled all twenty out in prose - two lines of a screen that is
// already taller than its window, duplicating the menu directly above them, and
// growing by one more name with every format this project adds (O105).
//
// The command line keeps the list, and that is the point of doing this here
// rather than in Allowed: in a terminal there is no menu to read it off, so the
// sentence is the only place the values exist.
// The default is not said here either, and that is not an omission: the menu's
// first entry says "not stated" and carries the default in its own words, so
// repeating it under the control would be the same duplication one line down.
func allowedOnAScreen(p format.Property) string {
	if p.Kind == format.PropertyChoice {
		return ""
	}
	return p.Allowed()
}
