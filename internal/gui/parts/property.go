package parts

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

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

// An empty control means "not stated", and no field is filled in with its
// declared default. That is a correction rather than a preference, and it was
// found by a guard rather than by reading.
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
// What the default is stays visible, in the sentence under the field - Allowed
// ends with ", default 10mb" - and in the placeholder.

// choiceField is a closed set, so it is a list rather than a box to type in.
// Nobody can misspell a value that is not typed.
func choiceField(p format.Property) PropertyField {
	sel := NewChooser(p.Choices, nil)
	sel.PlaceHolder = leftAlone(p)
	return PropertyField{
		Name:    p.Name,
		Control: sel,
		Value:   func() string { return sel.Selected },
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
	}
}

// textField covers a number, a size and free text alike, because all three are
// a box somebody types into and the difference between them is what the engine
// accepts rather than what the box does.
func textField(p format.Property) PropertyField {
	entry := widget.NewEntry()
	entry.SetPlaceHolder(leftAlone(p))
	return PropertyField{
		Name:    p.Name,
		Control: entry,
		Value:   func() string { return entry.Text },
	}
}

// leftAlone is what happens if this field is not touched. A declaration with no
// default means the format works the value out from the size it was asked for.
func leftAlone(p format.Property) string {
	if p.Default == "" {
		return text.PlaceholderWorkedOut
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
func PropertyFields(d format.Descriptor, into *Fields) ([]PropertyField, []fyne.CanvasObject) {
	fields := make([]PropertyField, 0, len(d.Properties))
	objects := make([]fyne.CanvasObject, 0, len(d.Properties))

	for _, p := range d.Properties {
		f := FromProperty(p)
		fields = append(fields, f)
		// No second explanation behind a button here, and that is the
		// declaration's doing rather than a gap. What a property takes is one
		// sentence built from Allowed, so there is nothing to hold back - and a
		// button that opened the line already printed underneath would be the
		// same words twice.
		objects = append(objects, into.Add(p.Name, p.Name, detailOf(p), NoDetail, f.Control))
	}

	// A rule binding two settings belongs beside them and nowhere else. Drawn
	// from Min and Max alone, two number boxes would offer twenty thousand by
	// twenty thousand and the run would refuse the pair - which is the defect
	// JointLimit was declared to close.
	for _, j := range d.JointLimits {
		objects = append(objects, Note(j.Describe()))
	}
	return fields, objects
}

// detailOf is what a property takes and what it is for, in that order. What it
// takes comes first because that is what somebody looking at an empty box needs.
func detailOf(p format.Property) string {
	detail := p.Allowed()
	if p.Detail != "" {
		detail += ". " + p.Detail
	}
	return detail
}
