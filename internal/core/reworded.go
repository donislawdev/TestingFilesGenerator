package core

import "fmt"

// A window in English says the engine's sentences in its own words where the
// command line's words are the recipe's: a value of a list by the name its menu
// shows, a setting by the label above its box, and a thing of the recipe - a
// target, a line of the file, a command to run - by what the screen calls it.
// docs/SIZE-FIXES-2026-11-16.md section 6.1, M3.
//
// The command line and the window render through the one function, render,
// and the command line is the window with nothing reworded. So the values of a
// sentence cannot come out differently between the two, and a guard holds that
// a wording answering for nothing gives the command line's text to the byte for
// every kind of value.

// Wording is how a surface says the engine's sentences where it does not say
// them as the command line does. Nil is the command line.
type Wording interface {
	// Word is a value in the surface's words, and false for every value it
	// says the way the command line does.
	Word(v any) (string, bool)
	// Layout is a sentence's layout in the surface's words. It keeps every
	// value and every slot of the command line's layout, in the same order -
	// the values are the command line's, so a layout that dropped or moved
	// one would print another value in its place.
	Layout(layout string) string
}

// word is what w says a value is, and false where w has nothing to say.
func word(w Wording, v any) (string, bool) {
	if w == nil {
		return "", false
	}
	return w.Word(v)
}

// laidOut is the sentence with its layouts in w's words.
func laidOut(w Wording, s Said) Said {
	if w == nil {
		return s
	}
	s.layout = w.Layout(s.layout)
	if s.one != "" {
		s.one = w.Layout(s.one)
	}
	return s
}

// value is v as a sentence reworded by w takes it.
//
// A value w answers for is printed as the answer, without the quotes the
// layout gives a value somebody typed - a name from a list is not something
// typed (the owner's decision of 2026-10-05). A value that holds sentences is
// reworded all the way down and printed with the verb the layout gives it, as
// its own Format would print it. Anything else is the value itself.
func value(w Wording, v any) any {
	if w == nil {
		return v
	}
	if said, ok := w.Word(v); ok {
		return verbatim(said)
	}
	switch v := v.(type) {
	case Said:
		return rendered(laidOut(w, v).render(w))
	case Conjoined:
		return rendered(v.joined(w))
	case Sentences:
		return rendered(v.joined(w))
	case Lines:
		return rendered(v.joined(w))
	case Choices:
		return rendered(v.joined(w))
	case error:
		return errorValue(w, v)
	case interface{ Said() Said }:
		// A sentence of ours under a type of its own - the place in a recipe
		// a refusal names - reworded where what it prints IS the sentence, so
		// a type that adds to it keeps what it adds.
		if said := v.Said(); fmt.Sprint(v) == said.String() {
			return rendered(laidOut(w, said).render(w))
		}
	}
	return v
}

// errorValue is an error inside a sentence: reworded when it can say itself in
// other words, and otherwise left to print its own text.
//
// Asked of the error itself rather than of what it wraps, because the command
// line prints the whole of its text. A sentence of ours whose text IS the
// error's text is reworded too. One whose text differs - an error that adds to
// its sentence - is left as it is, because rewording it would drop what it
// adds.
func errorValue(w Wording, err error) any {
	if r, ok := err.(interface{ Reworded(string, Wording) string }); ok {
		return rendered(r.Reworded("", w))
	}
	if said, ok := err.(interface{ Said() Said }); ok && said.Said().String() == err.Error() {
		return rendered(laidOut(w, said.Said()).render(w))
	}
	return err
}

// Reworded is the sentence in w's words, with its setting slots filled with
// name the way InTheWordsOf fills them. The layout is w's before the slots are
// filled, so a name is never read as part of the layout.
func (s Said) Reworded(name string, w Wording) string {
	filled := laidOut(w, s)
	if name == "" {
		return filled.render(w)
	}
	filled.layout = InTheWordsOf(filled.layout, name)
	if filled.one != "" {
		filled.one = InTheWordsOf(filled.one, name)
	}
	return filled.render(w)
}

// verbatim is a word printed as it is, whatever verb the layout gives it, with
// the width the layout asks for.
type verbatim string

func (v verbatim) Format(f fmt.State, _ rune) {
	fmt.Fprintf(f, fmt.FormatString(f, 's'), string(v))
}

// rendered is text a value made of itself, printed with the verb the layout
// gives that value - the way the value's own Format prints its text.
type rendered string

func (r rendered) Format(f fmt.State, verb rune) {
	fmt.Fprintf(f, fmt.FormatString(f, verb), string(r))
}
