package parts

import (
	"sort"
	"strings"

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
type listEntry struct {
	kind     entryKind
	text     string
	from, to int
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
func arrange(values []string, headingOf func(string) string, typed string) []listEntry {
	kept := narrow(values, headingOf, typed)
	if len(kept) == 0 {
		if strings.TrimSpace(typed) == "" {
			return nil
		}
		return []listEntry{{kind: entryNotice, text: text.ListNothingMatches()}}
	}
	if headingOf == nil {
		out := make([]listEntry, 0, len(kept))
		for _, v := range kept {
			out = append(out, valueEntry(v, typed))
		}
		return out
	}

	groups := map[string][]string{}
	headings := []string{}
	for _, v := range kept {
		h := headingOf(v)
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
			out = append(out, valueEntry(v, typed))
		}
	}
	return out
}

// valueEntry is one value's row, with what was typed found in its words.
// Found only where lowering the words keeps their length, so the span marks
// the same letters in the words as drawn - true of every format name, and a
// value for which it is not simply gets no bold.
func valueEntry(v, typed string) listEntry {
	e := listEntry{kind: entryValue, text: v}
	want := strings.ToLower(strings.TrimSpace(typed))
	lower := strings.ToLower(v)
	if want == "" || len(lower) != len(v) {
		return e
	}
	if at := strings.Index(lower, want); at >= 0 {
		e.from, e.to = at, at+len(want)
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
func narrow(values []string, headingOf func(string) string, typed string) []string {
	want := strings.ToLower(strings.TrimSpace(typed))
	if want == "" {
		return values
	}
	out := make([]string, 0, len(values))
	for _, v := range values {
		if strings.Contains(strings.ToLower(v), want) || (headingOf != nil && aWordStartsWith(headingOf(v), want)) {
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

// aWordStartsWith says whether a word of a heading starts with what was typed,
// which is already lower case.
func aWordStartsWith(heading, want string) bool {
	for _, word := range strings.Fields(strings.ToLower(heading)) {
		if strings.HasPrefix(word, want) {
			return true
		}
	}
	return false
}
