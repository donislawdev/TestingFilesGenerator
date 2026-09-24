package parts

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
)

// What an open list draws, worked out without the toolkit.
//
// Kept apart from OpenList so that the rules - which values a typed filter
// keeps, which heading a value stands under, where the keyboard lands - can be
// asked directly, and so that the widget is only the drawing of an answer made
// here (GUI rule 15). Nothing in this file knows what a row looks like.

// entryKind says what one row of an open list is.
type entryKind int

const (
	// entryValue is a value somebody can choose.
	entryValue entryKind = iota
	// entryHeading names the kind of the values under it. Nobody chooses it,
	// and the keyboard steps over it.
	entryHeading
	// entryNotice says the filter left nothing. Nobody chooses it either - it
	// is there so an empty list reads as an answer rather than as a fault.
	entryNotice
)

// listEntry is one row of an open list. On a value, from and to are where
// what was typed stands in its words, drawn in bold - equal when nothing in
// the words matched, which is a value kept for the heading it stands under.
// name is what the value is called, drawn beside it, with its own bold span in
// nameFrom and nameTo - empty on a list whose values have no names.
type listEntry struct {
	kind             entryKind
	text             string
	from, to         int
	name             string
	nameFrom, nameTo int
}

// labels is what a list knows about its values beyond the values themselves:
// the heading each stands under and the name each is called by. Either may be
// nil, and on most lists both are.
type labels struct {
	headingOf func(string) string
	nameOf    func(string) string
}

// arrange is what an open list draws: the values the typed text keeps, each
// under the heading of its kind when the list has kinds.
//
// Headings are in the order of their own words, which is the rule every closed
// set in this window follows (TestEveryClosedSetIsRegisteredInOrder) rather
// than an order somebody preferred - and it is the order of the words on the
// screen, so a translation reorders them with it. The values under a heading
// keep the order they were given in, which for formats is the registry's.
//
// A heading with nothing under it is not drawn: a kind with no format yet, or
// one the filter emptied. A value whose heading is empty stands first, with no
// heading over it, rather than under a heading made up here.
func arrange(values []string, by labels, typed string) []listEntry {
	kept := narrow(values, by, typed)
	if len(kept) == 0 {
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		return []listEntry{{kind: entryNotice, text: text.ListNothingMatches()}}
	}
	if by.headingOf == nil {
		out := make([]listEntry, 0, len(kept))
		for _, v := range kept {
			out = append(out, valueEntry(v, by.nameOf, typed))
		}
		return out
	}

	groups := map[string][]string{}
	headings := []string{}
	for _, v := range kept {
		h := by.headingOf(v)
		if _, seen := groups[h]; !seen {
			headings = append(headings, h)
		}
		groups[h] = append(groups[h], v)
	}
	sort.Strings(headings)

	out := make([]listEntry, 0, len(kept)+len(headings))
	for _, h := range headings {
		if h != "" {
			out = append(out, listEntry{kind: entryHeading, text: text.ListHeadingCount(h, len(groups[h]))})
		}
		for _, v := range groups[h] {
			out = append(out, valueEntry(v, by.nameOf, typed))
		}
	}
	return out
}

// valueEntry is one value's row, with what was typed found in its words and
// in its name.
//
// In the words anywhere, as the filter keeps them. In the name only at the
// start of a word, as the filter keeps them too - so the bold is always the
// reason the row is there, and never a match the filter did not count.
//
// Found only where lowering keeps the length, so the span marks the same
// letters as are drawn - true of every format and every format name, which
// are ASCII (TestEveryFormatDeclaresTheFullSet), and a value for which it is
// not simply gets no bold.
func valueEntry(v string, nameOf func(string) string, typed string) listEntry {
	e := listEntry{kind: entryValue, text: v}
	if nameOf != nil {
		e.name = nameOf(v)
	}
	want := strings.ToLower(strings.TrimSpace(typed))
	if want == "" {
		return e
	}
	if lower := strings.ToLower(v); len(lower) == len(v) {
		if at := strings.Index(lower, want); at >= 0 {
			e.from, e.to = at, at+len(want)
		}
	}
	if len(strings.ToLower(e.name)) == len(e.name) {
		if at := wordStart(e.name, want); at >= 0 {
			e.nameFrom, e.nameTo = at, at+len(want)
		}
	}
	return e
}

// narrow keeps the values holding what was typed, in the order they came in.
//
// Anywhere in the value rather than only at its start, and without regard to
// case, so "gz" finds targz and "X" finds docx, pptx and xlsx. Where the
// keyboard lands is a separate and stricter question - see landing. Space
// around what was typed is not part of it: a filter holding only a space keeps
// everything rather than nothing.
//
// A value is kept for its heading as well - "pict" keeps every picture - but
// only where a WORD of the heading starts with what was typed. Anywhere in the
// heading, one letter would keep nearly every kind: "t" is in Pictures,
// Documents, Text and data. Decided by the owner on 2026-09-23.
//
// And for its name, on the same rule and for the same reason, since
// 2026-09-24: "excel" keeps xlsx and "vector" svg, while "a" does not keep
// every format whose name has an a somewhere in it.
func narrow(values []string, by labels, typed string) []string {
	want := strings.ToLower(strings.TrimSpace(typed))
	if want == "" {
		return values
	}
	out := make([]string, 0, len(values))
	for _, v := range values {
		if strings.Contains(strings.ToLower(v), want) ||
			(by.headingOf != nil && aWordStartsWith(by.headingOf(v), want)) ||
			(by.nameOf != nil && aWordStartsWith(by.nameOf(v), want)) {
			out = append(out, v)
		}
	}
	return out
}

// landing is the row the keyboard goes to after something was typed: the
// first value STARTING with it, and failing that the first value the filter
// kept, and -1 when there is none.
//
// Two rules rather than the first match, because the first match in a list
// grouped by kind is often the wrong one: "p" keeps zip under Archives before
// pdf under Documents, and somebody typing p means a value that starts with p.
func landing(entries []listEntry, typed string) int {
	want := strings.ToLower(strings.TrimSpace(typed))
	first := -1
	for i, e := range entries {
		if e.kind != entryValue {
			continue
		}
		if first < 0 {
			first = i
		}
		if want != "" && strings.HasPrefix(strings.ToLower(e.text), want) {
			return i
		}
	}
	return first
}

// nextValue is the value row after from in the direction step (+1 or -1),
// stepping over headings, or from itself when there is none that way - a list
// that stops at its ends rather than wrapping, see OpenList.moveTo.
func nextValue(entries []listEntry, from, step int) int {
	for at := from + step; at >= 0 && at < len(entries); at += step {
		if entries[at].kind == entryValue {
			return at
		}
	}
	return from
}

// edgeValue is the first value row from one end: the top for step +1, the
// bottom for step -1. -1 when the list holds no value at all.
func edgeValue(entries []listEntry, step int) int {
	start := -1
	if step < 0 {
		start = len(entries)
	}
	if at := nextValue(entries, start, step); at != start {
		return at
	}
	return -1
}

// aWordStartsWith says whether a word of a heading or a name starts with what
// was typed, which is already lower case.
func aWordStartsWith(words, want string) bool { return wordStart(words, want) >= 0 }

// wordStart is where in words the first word starting with want begins, or -1.
//
// A word is a run of letters and digits, so what stands between words is
// anything else rather than only a space. Split on spaces alone, "office" did
// not find "Word (Office Open XML)", whose word is "(office", and "separated"
// did not find "Comma-Separated Values". The position is in the lowered words,
// which is the position in the words as drawn wherever lowering keeps the
// length - see valueEntry.
func wordStart(words, want string) int {
	lower := strings.ToLower(words)
	for at, r := range lower {
		if !wordRune(r) {
			continue
		}
		if before, _ := utf8.DecodeLastRuneInString(lower[:at]); at > 0 && wordRune(before) {
			continue
		}
		word := lower[at:]
		if strings.HasPrefix(word, want) {
			return at
		}
	}
	return -1
}

// wordRune says whether a character belongs to a word rather than to what
// stands between words.
func wordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }
