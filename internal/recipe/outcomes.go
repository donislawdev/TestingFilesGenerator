package recipe

import "sort"

// The two closed lists an expectation is written from: what a system under
// test is expected to do with a file, and the rule that case is about. Apart
// from target.go, which reads a target and asks these, because they are the
// vocabulary the command line and the window offer as well.

// reasons is the closed list from docs/MANIFEST.md section 5.
//
// It is closed on purpose. A reason nobody recognises is a typo, and a typo
// accepted in silence becomes an expectation no test will ever check - the
// same failure as an unknown outcome, one level down.
// The names are neutral about the verdict on purpose, and one of them was not
// until 2026-08-19.
//
// A reason says which rule a case is ABOUT, not what the system did - which is
// what lets the same reason sit under any outcome. MANIFEST.md's own example for
// an unspecified outcome carries size_zero, and a boundary set expects the file
// a byte under the limit to be ACCEPTED with size_limit as the rule in play.
//
// extension_denied broke that: it was the only entry of fifteen whose name
// carried a verdict, so "accept with extension_denied" read as a contradiction
// while "accept with size_limit" read fine. Reported by the owner, who picked
// the one entry that did not fit. Renamed to extension_rule rather than the
// field being restricted, because restricting it would have broken the boundary
// set - the tool's own flagship case.
var reasons = map[string]bool{
	"size_limit": true, "size_zero": true, "count_limit": true,
	"extension_rule": true, "mime_mismatch": true, "content_malformed": true,
	"filename_invalid": true, "filename_too_long": true, "filename_traversal": true,
	"dimensions_limit": true, "nesting_depth": true, "encoding_invalid": true,
	"malware_signature": true, "duplicate": true, "none": true,
}

// Reasons is the closed list, for the surfaces that have to offer it.
//
// Exported because the command line needs the same list the recipe uses. It
// had no way to state a reason at all until 2026-08-03 - "--expected reject"
// worked and the reason beside it did not exist - so a run driven by flags
// could never fill the category the list exists to make countable. Two copies
// of a closed list is how the two surfaces drift, which is D1 one level down.
func Reasons() []string {
	out := make([]string, 0, len(reasons))
	for r := range reasons {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

// KnownReason says whether a reason is on the list.
func KnownReason(r string) bool { return reasons[r] }

// outcomes is the closed list of what a system under test can be expected to do
// with a file, from docs/MANIFEST.md.
//
// A variable rather than four words repeated at each place that checks them,
// which is what this was until 2026-08-18: the sentence "accept, reject,
// sanitize or unspecified" appeared in the recipe reader twice more and in the
// command line twice, and the window was about to be the fifth copy. That is
// exactly the argument written above Reasons - two copies of a closed list is
// how the surfaces drift, which is D1 one level down.
var outcomes = []string{outcomeAccept, "reject", "sanitize", "unspecified"}

// outcomeAccept is the one outcome a damaged file cannot have.
//
// Named rather than typed twice, because the refusal in damage.go is about this
// exact word. Below the list rather than above it: the paragraph over the list
// documents the LIST, and a declaration slipped in between would quietly take
// that paragraph for itself and leave the list with none.
const outcomeAccept = "accept"

// Outcomes is the closed list, for the surfaces that have to offer it.
//
// Sorted, because a closed set has one order everywhere it is shown and a guard
// asks for exactly that. These four are alphabetical as written, and sorting
// says so rather than relying on it.
func Outcomes() []string {
	out := make([]string, len(outcomes))
	copy(out, outcomes)
	sort.Strings(out)
	return out
}

// KnownOutcome says whether an outcome is on the list.
func KnownOutcome(o string) bool {
	for _, known := range outcomes {
		if known == o {
			return true
		}
	}
	return false
}
