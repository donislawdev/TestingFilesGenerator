package preset

import (
	"fmt"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

const (
	boundariesID      = "size-boundaries"
	defaultLimitText  = "10mb"
	defaultSpreadText = "1B,1kb,1mb"
	defaultFormat     = "pdf"
)

func init() {
	Register(Preset{
		ID:       boundariesID,
		Title:    "Size boundaries",
		Question: "Is a size limit enforced exactly where it is declared?",

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
			"limit": "no --limit was given, so this set is built around " + defaultLimitText +
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

// badSpread is a value the spread parameter does not accept.
//
// The same type the format registry raises for a value outside its declaration,
// rather than a plain error, and that is a repair rather than a preference. A
// plain error falls through the classifier to RUNTIME, so "--spread notasize"
// told CI this program had a bug instead of saying the value was wrong -
// measured on 2026-08-05, exit 1. The same class as "--set width=abc", which
// was fixed for the same reason two days earlier.
func badSpread(value, reason string) error {
	return &format.PropertyValueError{
		Format: boundariesID, Key: "spread", Value: value, Reason: reason,
	}
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
	var out []offset
	seen := map[int64]string{}
	for _, piece := range strings.Split(raw, ",") {
		piece = strings.TrimSpace(piece)
		if piece == "" {
			continue
		}
		// The text of a distance becomes the id of a target and the name of a
		// file, so it has to be made of what a size is made of and nothing
		// else. Found by fuzzing on 2026-08-05: "1\rB" parses as one byte,
		// because the size parser trims the ends and this carriage return is in
		// the middle - and the character then reached the recipe source raw and
		// broke the document. The comment on render() claimed no value there
		// needed quoting. That was true of every value except this one.
		if bad := firstUnusable(piece); bad != "" {
			return nil, badSpread(piece, fmt.Sprintf(
				"it holds %s, and a distance is written with digits, letters and a dot - such as 1kb, 512 or 1.5mb. Its text becomes the name of a file", bad))
		}
		n, err := core.ParseSize(piece)
		if err != nil {
			return nil, badSpread(piece, err.Error())
		}
		if n <= 0 {
			return nil, badSpread(piece, "a distance from the limit has to be more than nothing")
		}
		// Two equal distances make two steps of the set that are the same file
		// twice, and they collide on the id built from the distance. Found by
		// fuzzing on 2026-08-05: the collision surfaced as a recipe the parser
		// refused, complaining about target ids nobody typed. Compared as bytes
		// rather than as text, so 1024 and 1kb are caught as well as 1B and 1b.
		if first, repeated := seen[n]; repeated {
			return nil, badSpread(piece, fmt.Sprintf(
				"it is the same distance as %q and the set would hold that step twice. Every distance has to be different, because each one names one file either side of the limit",
				first))
		}
		seen[n] = piece
		out = append(out, offset{text: strings.ToLower(piece), bytes: n})
	}
	if len(out) == 0 {
		return nil, badSpread(raw, "no distances were given, so there is nothing either side of the limit")
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

	plan := steps(limit, spread)
	if err := reachable(plan, desc, limit); err != nil {
		return nil, err
	}
	return render(plan, desc), nil
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
			Detail: what,
			Hint: fmt.Sprintf(
				"Raise --limit above %d B, narrow --spread, or choose a format with a smaller minimum. The limit asked for was %d B.",
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

// render writes the recipe.
//
// Source rather than a structure, so eject prints what a run consumes.
//
// Nothing here is quoted, and that is safe because of where the values come
// from rather than because writing YAML by hand is safe: a byte count, a format
// id the registry knows, and an id built from the text of a distance.
//
// That last one is the one to watch, and it was wrong until 2026-08-05. This
// comment used to say every value was one the package built itself, and the id
// carries the caller's own text - so "1\rB" reached the document raw and broke
// it, because the size parser trims the ends and that carriage return sat in
// the middle. Found by fuzzing, not by reading. parseSpread now refuses any
// character a size is not written with, which is what makes the sentence above
// true rather than merely confident.
func render(plan []step, desc format.Descriptor) []byte {
	var b strings.Builder
	b.WriteString("# Generated by: tfg preset eject " + boundariesID + "\n")
	b.WriteString("# " + "Is a size limit enforced exactly where it is declared?" + "\n")
	b.WriteString("#\n")
	b.WriteString("# Edit it, commit it, it is an ordinary recipe from here on.\n\n")
	b.WriteString("version: 1\n")
	b.WriteString("targets:\n")

	for _, s := range plan {
		fmt.Fprintf(&b, "  - id: %s\n", s.id)
		fmt.Fprintf(&b, "    format: %s\n", desc.ID)
		fmt.Fprintf(&b, "    count: 1\n")
		fmt.Fprintf(&b, "    size: %d\n", s.size)
		fmt.Fprintf(&b, "    name: %s%s\n", s.id, desc.Extension)
		fmt.Fprintf(&b, "    group: %s\n", boundariesID)
		if s.accept {
			b.WriteString("    expected: accept\n")
			continue
		}
		b.WriteString("    expected:\n")
		b.WriteString("      outcome: reject\n")
		b.WriteString("      reason: size_limit\n")
	}
	return []byte(b.String())
}
