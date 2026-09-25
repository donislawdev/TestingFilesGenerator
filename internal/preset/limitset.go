package preset

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// The set a declared limit produces: the limit itself, and one file either
// side of it for every distance asked for.
//
// It lives on its own because two presets lay it out. size-boundaries is a
// microscope on this one axis and reaches as far either side as it is told.
// upload-validation is a survey across many axes, of which size is one, and
// takes a single step either side plus one file well past the limit - the
// verdict of PRESET-FEASIBILITY-2026-09-08.md section 5, where the alternative
// was two presets doing literally the same thing.
//
// The sentence "under the limit is accepted, over it is refused for size_limit"
// then exists once. Writing it twice is what the same document calls an
// invitation for the two to drift, and the drift would be invisible: both sets
// would still run, still verify, and disagree about what a limit means.
//
// D11 gate on the extraction, measured 2026-09-22: eject size-boundaries at its
// defaults gives 1298 B and the same sha256 recorded on 2026-09-08, because the
// bytes of an ejected recipe reach other people's manifests as a recipe_hash.

// offset is one step either side of the limit, with the text it was written as
// so the files can name themselves after it.
type offset struct {
	text  string
	bytes int64
}

// step is one file of the set: how big, what it is called, and what a
// reasonably built system should do with it.
type step struct {
	id     string
	size   int64
	accept bool
}

// limitSet is a declared limit and everything needed to lay files out around it.
type limitSet struct {
	// preset and setting name what a refusal is about: which preset could not
	// build the set, and which of its parameters a window should mark.
	preset, setting string
	// group is what the files call themselves collectively in the manifest, so
	// a test can assert on the whole class at once.
	group string
	desc  format.Descriptor
	limit int64
	// limitText is the limit as it was typed. It leads every file name, so two
	// sets built around different limits cannot be told apart only by opening
	// the files - reported from use on 2026-08-11, where a directory holding
	// two runs was a directory of guesses.
	limitText string
	spread    []offset
}

// steps lays the set out, largest distance below the limit first, then the
// limit, then upward. The order is the order somebody reads a table in.
func (s limitSet) steps() []step {
	out := make([]step, 0, 2*len(s.spread)+1)
	for i := len(s.spread) - 1; i >= 0; i-- {
		out = append(out, step{
			id: "under_" + s.spread[i].text, size: s.limit - s.spread[i].bytes, accept: true,
		})
	}
	out = append(out, step{id: "at_limit", size: s.limit, accept: true})
	for _, o := range s.spread {
		out = append(out, step{id: "over_" + o.text, size: s.limit + o.bytes, accept: false})
	}
	return out
}

// reachable refuses the whole set when any one file of it is out of reach.
//
// PR7, and the untouchable rule about silence. A set missing three of its seven
// files still looks like a set, and the three that are missing are the ones the
// run was about - the ones nearest the limit.
func (s limitSet) reachable(set []step) error {
	floor := format.SmallestRemembered(s.desc, format.Request{Seed: 1, Label: true})
	for _, one := range set {
		if one.size >= floor {
			continue
		}
		what := fmt.Sprintf("%s would be %d B and the smallest %s this build makes is %d B",
			one.id, one.size, strings.ToUpper(s.desc.ID), floor)
		if one.size <= 0 {
			what = fmt.Sprintf("%s would be %d B, and a file cannot be smaller than nothing", one.id, one.size)
		}
		return &ImpossibleError{
			Preset: s.preset,
			// The limit rather than the spread, although the sentence offers
			// both ways out. The limit is the one number the set is measured
			// from, so it is where somebody types first - and a message can
			// only stand beside one box.
			Setting: s.setting,
			Detail:  what,
			Hint: fmt.Sprintf(
				// The settings are named without a leading dash on purpose. This
				// sentence is built in the engine and both surfaces show it word
				// for word, so a spelling only one of them has sends the other's
				// reader translating: the window labels these fields "limit" and
				// "spread", and there is no "--limit" anywhere on it. Seen on
				// screen 2026-08-11, O79.
				"Raise the {setting} above %d B, narrow the spread, or choose a format with a smaller minimum. The {setting} asked for was %d B.",
				floor+deepest(set, s.limit), s.limit),
		}
	}
	return nil
}

// deepest is how far below the limit the set reaches, so the hint can name a
// limit that would work rather than only the one that did not.
func deepest(set []step, limit int64) int64 {
	var found int64
	for _, one := range set {
		if d := limit - one.size; d > found {
			found = d
		}
	}
	return found
}

// drafts is the set as targets, ready for the composer.
//
// This used to print the document itself, line by line, with a comment saying
// that was safe because every value was one the package built itself. The
// comment was wrong until 2026-08-05: the id carries the caller's own text, so
// "1\rB" reached the document raw and broke it, because the size parser trims
// the ends and that carriage return sat in the middle. Found by fuzzing rather
// than by reading.
//
// parseSpread refuses that character now, and this no longer writes YAML at
// all - plan.source hands the values to the marshaller, which does the quoting
// and owns the shape of the document. Two defences rather than one, and the
// second one cannot be forgotten by the next preset.
func (s limitSet) drafts(set []step) []recipe.TargetDraft {
	out := make([]recipe.TargetDraft, 0, len(set))
	for _, one := range set {
		draft := recipe.TargetDraft{
			ID:     one.id,
			Format: s.desc.ID,
			Count:  "1",
			Size:   strconv.FormatInt(one.size, 10),
			// The id stays as it was. It derives the seed, so putting the limit
			// in it would move the bytes of every file in this set for a change
			// that is about telling two directories apart.
			Name:     s.limitText + "_" + one.id + s.desc.Extension,
			Group:    s.group,
			Expected: "accept",
			Purpose:  s.purpose(one),
		}
		if !one.accept {
			draft.Expected = "reject"
			draft.ExpectedReason = "size_limit"
		}
		out = append(out, draft)
	}
	return out
}

// purpose is what the instructions say about one file of the set.
func (s limitSet) purpose(one step) string {
	name := s.desc.Name
	switch {
	case one.size == s.limit:
		return fmt.Sprintf("A %s file of exactly %s, the limit itself. Your system should take it. "+
			"A refusal here usually means the comparison is off by one, or your system counts a kilobyte as 1000 bytes where this set counts 1024.", name, s.limitText)
	case one.accept:
		return fmt.Sprintf("A %s file %s under the limit of %s. Your system should take it - "+
			"a refusal means the limit is enforced lower than it is declared.", name, core.ExactBytes(s.limit-one.size), s.limitText)
	default:
		return fmt.Sprintf("A %s file %s over the limit of %s. Your system should turn it away - "+
			"taking it means the limit is not enforced, or enforced only in the browser.", name, core.ExactBytes(one.size-s.limit), s.limitText)
	}
}
