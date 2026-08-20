package recipe

import "fmt"

// A setting written as a block where a single value belongs.
//
// YAML makes this easy to do by accident. One extra level of indentation, or a
// colon inside an unquoted value, and what was meant as a word arrives as a
// mapping. Until 2026-08-20 the answer to that came from the parser rather than
// from us:
//
//	cannot unmarshal map[string]interface {} into Go struct field
//	rawRecipe.Targets[0].Format of type string
//
// which names types nobody outside this repository can see, gives no address a
// window could mark a box by, and tells a person nothing they can act on. It
// reached eleven keys, most of them everyday ones - format, id, name, group,
// the output directory, the manifest name.
//
// The cure is the one internal/recipe/scalar.go already applies to numbers:
// take the source text of the node instead of letting YAML decide the type, and
// judge it ourselves. A scalar accepts any shape, so the parser never has an
// opinion, and scalar.value reports whether what arrived was a single value.

// oneValue reads a setting that has to be written as one value.
//
// The bool is false when the setting was absent AND when it was the wrong
// shape, because both mean the same thing to a caller: there is no value to
// use. The difference is that the wrong shape has already been recorded as a
// problem by the time this returns, so nothing is lost by the caller treating
// them alike.
func oneValue(p *problems, at, subject, example string, s *scalar) (string, bool) {
	if s == nil {
		return "", false
	}
	v, ok := s.value()
	if !ok {
		p.add(at,
			fmt.Sprintf("%s is written as a list or a block", subject),
			"this setting is one value, written beside its name rather than indented under it",
			fmt.Sprintf("write it on one line, such as %s", example))
		return "", false
	}
	return v, true
}

// oneFlag is the same for a setting that is yes or no.
//
// Spelled out rather than handed to a YAML boolean, for the reason the scalar
// type gives about numbers: this parser reads more words as true than a person
// would expect, and a setting that quietly means the opposite of what somebody
// typed is the failure this project refuses to have.
func oneFlag(p *problems, at, subject string, s *scalar) (bool, bool) {
	v, ok := oneValue(p, at, subject, subject+": true", s)
	if !ok {
		return false, false
	}
	switch v {
	case "true":
		return true, true
	case "false":
		return false, true
	}
	p.add(at,
		fmt.Sprintf("%s is %q, which is neither true nor false", subject, v),
		"this setting is one of two words, and anything else would have to be guessed at",
		fmt.Sprintf("use %s: true or %s: false", subject, subject))
	return false, false
}
