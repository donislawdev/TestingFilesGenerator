package preset

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

const (
	boundariesID      = "size-boundaries"
	defaultLimitText  = "10mb"
	defaultSpreadText = "1B,1kb,1mb"
	defaultFormat     = "pdf"

	// boundariesQuestion is announced by the preset AND written into the header
	// of an ejected recipe. Named once rather than typed twice: the two copies
	// had already been sitting in this file since it was written.
	boundariesQuestion = "Is a size limit enforced exactly where it is declared?"
)

func init() {
	Register(Preset{
		ID:       boundariesID,
		Title:    "Size boundaries",
		Question: boundariesQuestion,

		Parameters: []format.Property{
			{
				Name: "limit", Kind: format.PropertySize,
				Default: defaultLimitText,
				Detail:  "The size limit your system declares. Everything else is measured from it.",
			},
			{
				Name: "spread", Kind: format.PropertyText,
				Shape:   "sizes separated by commas",
				Default: defaultSpreadText,
				Detail:  "How far either side of the limit to reach, as a list of sizes.",
			},
		},
		Reads: []string{"format"},

		SaidWhenDefaulted: map[string]string{
			"limit": "no limit was given, so this set is built around " + defaultLimitText +
				" - that is our placeholder and not your system's limit. Pass the limit your system declares, or the files say nothing about it.",
		},

		Requires: []string{"MVP"},
		Catches: []string{
			"off by one errors at the limit",
			"MB confused with MiB, which is 4.8 per cent and enough to let a file through that should not pass",
			"a limit enforced in the browser and not on the server",
		},

		Expand: expandSizeBoundaries,
	})
}

// offset is one step either side of the limit, with the text it was written as
// so the files can name themselves after it.
type offset struct {
	text  string
	bytes int64
}

// spreadList is how the distances either side of the limit are written.
//
// The shared parser does the splitting, the duplicate and the refusal, and this
// says what one distance has to look like. Two equal distances make two steps
// of the set that are the same file twice and collide on the id built from the
// distance - found by fuzzing on 2026-08-05, where the collision surfaced as a
// recipe the parser refused, complaining about target ids nobody typed.
// Compared as bytes rather than as text, so 1024 and 1kb are caught as well as
// 1B and 1b.
var spreadList = commaList{
	preset:    boundariesID,
	param:     "spread",
	empty:     "no distances were given, so there is nothing either side of the limit",
	check:     checkDistance,
	same:      sizeKey,
	keep:      lower,
	duplicate: repeatedDistance,
}

func repeatedDistance(first string) string {
	return fmt.Sprintf(
		"it is the same distance as %q and the set would hold that step twice. Every distance has to be different, because each one names one file either side of the limit",
		first)
}

// badSpread is a value the spread parameter does not accept.
func badSpread(value, reason string) error {
	return spreadList.refuse(value, reason)
}

// checkDistance answers why a piece of the spread is not a distance.
func checkDistance(piece string) string {
	// The text of a distance becomes the id of a target and the name of a
	// file, so it has to be made of what a size is made of and nothing else.
	// Found by fuzzing on 2026-08-05: "1\rB" parses as one byte, because the
	// size parser trims the ends and this carriage return is in the middle -
	// and the character then reached the recipe source raw and broke the
	// document.
	if bad := firstUnusable(piece); bad != "" {
		return fmt.Sprintf(
			"it holds %s, and a distance is written with digits, letters and a dot - such as 1kb, 512 or 1.5mb. Its text becomes the name of a file", bad)
	}
	n, err := core.ParseSize(piece)
	if err != nil {
		return err.Error()
	}
	if n <= 0 {
		return "a distance from the limit has to be more than nothing"
	}
	return ""
}

// sizeKey is what makes two distances the same one, for the duplicate check.
// Anything checkDistance has passed parses here, so a failure cannot arrive.
func sizeKey(piece string) string {
	n, err := core.ParseSize(piece)
	if err != nil {
		return piece
	}
	return fmt.Sprintf("%d", n)
}

// firstUnusable names the first character that cannot appear in a distance,
// quoted so a space or a control character is visible in the message.
func firstUnusable(piece string) string {
	for _, r := range piece {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '.':
		default:
			return fmt.Sprintf("%q", r)
		}
	}
	return ""
}

func parseSpread(raw string) ([]offset, error) {
	pieces, err := spreadList.parse(raw)
	if err != nil {
		return nil, err
	}
	out := make([]offset, 0, len(pieces))
	for _, piece := range pieces {
		// Parsed again rather than carried through the list, because the list
		// deals in text for every preset and only this one wants the number.
		// checkDistance has already refused anything that would fail here.
		n, err := core.ParseSize(piece)
		if err != nil {
			return nil, badSpread(piece, err.Error())
		}
		out = append(out, offset{text: piece, bytes: n})
	}
	return out, nil
}

// step is one file of the set: how big, what it is called, and what a
// reasonably built system should do with it.
type step struct {
	id     string
	size   int64
	accept bool
}

// steps lays the set out, largest distance below the limit first, then the
// limit, then upward. The order is the order somebody reads a table in.
func steps(limit int64, spread []offset) []step {
	out := make([]step, 0, 2*len(spread)+1)
	for i := len(spread) - 1; i >= 0; i-- {
		out = append(out, step{
			id: "under_" + spread[i].text, size: limit - spread[i].bytes, accept: true,
		})
	}
	out = append(out, step{id: "at_limit", size: limit, accept: true})
	for _, o := range spread {
		out = append(out, step{id: "over_" + o.text, size: limit + o.bytes, accept: false})
	}
	return out
}

func expandSizeBoundaries(args Args) ([]byte, error) {
	limit, err := core.ParseSize(args["limit"])
	if err != nil {
		return nil, fmt.Errorf("limit: %w", err)
	}
	// The limit leads every file name, so two sets built around different
	// limits cannot be told apart only by opening the files. Reported from use
	// on 2026-08-11: "at_limit.pdf" says nothing about which limit it sits at,
	// and a directory holding two runs is a directory of guesses.
	//
	// Same check as a distance gets, and for the same reason rather than by
	// analogy: this text now reaches a file name too, and fuzzing already
	// found a carriage return riding through the size parser into the recipe
	// source once.
	limitText := strings.TrimSpace(args["limit"])
	if bad := firstUnusable(limitText); bad != "" {
		return nil, fmt.Errorf(
			"limit: it holds %s, and a limit is written with digits, letters and a dot - "+
				"such as 10mb, 512 or 1.5gb. Its text becomes part of every file name", bad)
	}
	spread, err := parseSpread(args["spread"])
	if err != nil {
		return nil, err
	}
	formatID := defaultFormat
	if v := args["format"]; v != "" {
		formatID = v
	}
	desc, err := format.Get(formatID)
	if err != nil {
		return nil, err
	}

	// The top of the range before the bottom of it, because the wrap happens
	// on the way up and the refusal that followed was about the way down.
	//
	// Measured on 2026-08-26: "--preset size-boundaries --limit
	// 9223372036854775807b" said "over_1b would be -9223372036854775808 B, and
	// a file cannot be smaller than nothing. Raise the limit above 1051991 B" -
	// an answer about the bottom of the range to a question about the top, with
	// advice pointing the wrong way. That is the same defect core already fixed
	// for --boundary on 2026-08-03, so this reaches for the same sentence
	// rather than writing a second one that could drift from it.
	for _, o := range spread {
		if limit+o.bytes < limit {
			return nil, core.ErrBoundaryTooLarge
		}
	}

	set := steps(limit, spread)
	if err := reachable(set, desc, limit); err != nil {
		return nil, err
	}
	return plan{
		preset:   boundariesID,
		question: boundariesQuestion,
		targets:  draftsOfSteps(set, desc, limitText),
	}.source()
}

// reachable refuses the whole set when any one file of it is out of reach.
//
// PR7, and the untouchable rule about silence. A set missing three of its seven
// files still looks like a set, and the three that are missing are the ones the
// run was about - the ones nearest the limit.
func reachable(plan []step, desc format.Descriptor, limit int64) error {
	floor := desc.SmallestAccepted(format.Request{Seed: 1, Label: true})
	for _, s := range plan {
		if s.size >= floor {
			continue
		}
		what := fmt.Sprintf("%s would be %d B and the smallest %s this build makes is %d B",
			s.id, s.size, strings.ToUpper(desc.ID), floor)
		if s.size <= 0 {
			what = fmt.Sprintf("%s would be %d B, and a file cannot be smaller than nothing", s.id, s.size)
		}
		return &ImpossibleError{
			Preset: boundariesID,
			// The limit rather than the spread, although the sentence offers
			// both ways out. The limit is the one number the set is measured
			// from, so it is where somebody types first - and a message can
			// only stand beside one box.
			Setting: "limit",
			Detail:  what,
			Hint: fmt.Sprintf(
				// The settings are named without a leading dash on purpose. This
				// sentence is built in the engine and both surfaces show it word
				// for word, so a spelling only one of them has sends the other's
				// reader translating: the window labels these fields "limit" and
				// "spread", and there is no "--limit" anywhere on it. Seen on
				// screen 2026-08-11, O79.
				"Raise the {setting} above %d B, narrow the spread, or choose a format with a smaller minimum. The {setting} asked for was %d B.",
				floor+largest(plan, limit), limit),
		}
	}
	return nil
}

// largest is how far below the limit the set reaches, so the hint can name a
// limit that would work rather than only the one that did not.
func largest(plan []step, limit int64) int64 {
	var deepest int64
	for _, s := range plan {
		if d := limit - s.size; d > deepest {
			deepest = d
		}
	}
	return deepest
}

// draftsOfSteps is the set as targets, ready for the composer.
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
func draftsOfSteps(set []step, desc format.Descriptor, limitText string) []recipe.TargetDraft {
	out := make([]recipe.TargetDraft, 0, len(set))
	for _, s := range set {
		draft := recipe.TargetDraft{
			ID:     s.id,
			Format: desc.ID,
			Count:  "1",
			Size:   strconv.FormatInt(s.size, 10),
			// The id stays as it was. It derives the seed, so putting the limit
			// in it would move the bytes of every file in this set for a change
			// that is about telling two directories apart.
			Name:     limitText + "_" + s.id + desc.Extension,
			Group:    boundariesID,
			Expected: "accept",
		}
		if !s.accept {
			draft.Expected = "reject"
			draft.ExpectedReason = "size_limit"
		}
		out = append(out, draft)
	}
	return out
}
