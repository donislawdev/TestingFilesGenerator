package core

import "fmt"

// A window in English says the engine's sentences in the command line's
// English, with two kinds of value in its own words: a value of a list by the
// name its menu shows, and a setting by the label above its box. Until
// 2026-11-17 it said them to the byte, so a refusal read "ask for compression:
// none" under a box labelled Compression - docs/SIZE-FIXES-2026-11-16.md
// section 6.1, M3a.
//
// The command line and the window render through the one function, render,
// and the command line is the window with nothing reworded. So the sentence
// around the values cannot drift between the two, and a guard holds that a
// wording answering for nothing gives the command line's text to the byte for
// every kind of value.

// Wording answers for the values a surface words differently from the command
// line, and false for every value it says the same way. Nil is the command
// line.
type Wording func(v any) (string, bool)

// word is what w says a value is, and false where w has nothing to say.
func (w Wording) word(v any) (string, bool) {
	if w == nil {
		return "", false
	}
	return w(v)
}

// value is v as a sentence reworded by w takes it.
//
// A value w answers for is printed as the answer, without the quotes the
// layout gives a value somebody typed - a name from a list is not something
// typed (the owner's decision of 2026-10-05). A value that holds sentences is
// reworded all the way down and printed with the verb the layout gives it, as
// its own Format would print it. Anything else is the value itself.
func (w Wording) value(v any) any {
	if w == nil {
		return v
	}
	if said, ok := w(v); ok {
		return verbatim(said)
	}
	switch v := v.(type) {
	case Said:
		return rendered(v.render(w))
	case Conjoined:
		return rendered(v.joined(w))
	case Sentences:
		return rendered(v.joined(w))
	case Lines:
		return rendered(v.joined(w))
	case Choices:
		return rendered(v.joined(w))
	case error:
		return w.errorValue(v)
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
func (w Wording) errorValue(err error) any {
	if r, ok := err.(interface{ Reworded(string, Wording) string }); ok {
		return rendered(r.Reworded("", w))
	}
	if said, ok := err.(interface{ Said() Said }); ok && said.Said().String() == err.Error() {
		return rendered(said.Said().render(w))
	}
	return err
}

// Reworded is the sentence with its setting slots filled with name, the way
// InTheWordsOf fills them, and its values worded by w.
func (s Said) Reworded(name string, w Wording) string {
	if name == "" {
		return s.render(w)
	}
	filled := s
	filled.layout = InTheWordsOf(s.layout, name)
	if s.one != "" {
		filled.one = InTheWordsOf(s.one, name)
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
