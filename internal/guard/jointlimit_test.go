package guard

import (
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// A refusal about a joint limit never prints the request and the limit as one
// number.
//
// The rule binding two settings reports both counts in a unit a person reads -
// megapixels rather than pixels - and it reported them by dividing, which
// truncates. So a request just over the limit came back as the limit: a picture
// of 20000x2001 is 40 020 000 pixels against a limit of 40 000 000, and the
// sentence read "together they come to 40 megapixels and the limit is 40".
// Two identical numbers, no unit on the second one, and nothing in it a person
// could act on - which is the third part of D6 missing. Measured 2026-09-22,
// O232.
//
// The pairs below are asked of the registry's own declarations rather than of
// numbers written here, so a format arriving with a joint limit of its own is
// covered without anybody remembering to come back.
func TestNoJointLimitRefusalPrintsTheRequestAndTheLimitAsOneNumber(t *testing.T) {
	limits := declaredJointLimits(t)

	for _, l := range limits {
		// Three requests, and the middle one is the whole point: one unit over
		// the limit is exactly where rounding used to hide the difference.
		for _, over := range []int64{1, l.Max / 2, l.Max * 9} {
			got := l.Max + over
			bad := l.Allows(got, 1)
			if bad == "" {
				t.Errorf("%s: %d is past the limit of %d and the rule allowed it",
					l.Of+" times "+l.By, got, l.Max)
				continue
			}
			counts := countsIn(bad)
			if len(counts) < 2 {
				t.Errorf("%s: the refusal does not read as two counts: %q",
					l.Of+" times "+l.By, bad)
				continue
			}
			asked, allowed := counts[0], counts[1]
			if asked.number == allowed.number {
				t.Errorf("%s: asked for %d against a limit of %d and the refusal says %q - "+
					"the two counts print as the same thing, so it says nothing",
					l.Of+" times "+l.By, got, l.Max, bad)
			}
			// Both counts carry the same noun. The limit used to be a bare
			// number taking its noun from four words earlier, which reads as a
			// count of something else - and nothing said so until a mutation
			// took the unit off and every guard stayed green.
			switch {
			case allowed.unit == "":
				t.Errorf("%s: the limit in %q is a bare number with no unit after it",
					l.Of+" times "+l.By, bad)
			case asked.unit != allowed.unit:
				t.Errorf("%s: the refusal counts the request in %q and the limit in %q: %q",
					l.Of+" times "+l.By, asked.unit, allowed.unit, bad)
			}
		}
	}
}

// Every joint limit says what its product counts.
//
// Base is what the exact form of the sentence is built on, so a declaration
// without one ends "together they come to 40 020 000 and the limit is
// 40 000 000 " - a sentence with a hole where its noun should be, and only on
// the path that is taken when the readable form cannot be used. That is the
// path nobody looks at, which is why it is asserted here rather than left to be
// noticed.
func TestEveryJointLimitSaysWhatItCounts(t *testing.T) {
	for _, l := range declaredJointLimits(t) {
		if strings.TrimSpace(l.Base) == "" {
			t.Errorf("the rule binding %s and %s does not say what it counts, so its exact "+
				"refusal has no noun in it", l.Of, l.By)
		}
		if strings.TrimSpace(l.Unit) == "" {
			t.Errorf("the rule binding %s and %s does not say what it reports in", l.Of, l.By)
		}
		if strings.TrimSpace(l.Why) == "" {
			t.Errorf("the rule binding %s and %s gives no reason, and D6 asks for one",
				l.Of, l.By)
		}
	}
}

// declaredJointLimits is every rule the registry holds, and it refuses to hand
// back none - a guard walking an empty list passes against any rule ever
// written.
func declaredJointLimits(t *testing.T) []format.JointLimit {
	t.Helper()
	var out []format.JointLimit
	for _, d := range format.All() {
		out = append(out, d.JointLimits...)
	}
	if len(out) == 0 {
		t.Fatal("no format declares a rule binding two settings, so this guard checked nothing")
	}
	return out
}

// counted is one number in a refusal and the word that follows it.
type counted struct {
	number string
	unit   string
}

// countsIn pulls the counts out of a refusal with the word after each one,
// ignoring the spaces that group digits.
//
// The word is read with firstWordOf from the doc-comment guard, which trims the
// punctuation a sentence puts after its last noun - so a bare count followed by
// a comma comes back with no unit at all, which is exactly the state being
// looked for.
//
// Read out of the sentence rather than recomputed, because what is being
// checked is what a person SEES. A guard comparing the numbers the rule holds
// would agree with the rule and say nothing about the words it chose.
func countsIn(sentence string) []counted {
	var found []counted
	var digits strings.Builder
	flush := func(rest string) {
		if digits.Len() == 0 {
			return
		}
		found = append(found, counted{number: digits.String(), unit: firstWordOf(rest)})
		digits.Reset()
	}
	for i := 0; i < len(sentence); i++ {
		c := sentence[i]
		switch {
		case c >= '0' && c <= '9':
			digits.WriteByte(c)
		case c == ' ' && digits.Len() > 0 && i+1 < len(sentence) &&
			sentence[i+1] >= '0' && sentence[i+1] <= '9':
			// A space inside a grouped number, not the end of one.
		default:
			flush(sentence[i:])
		}
	}
	flush("")
	return found
}
