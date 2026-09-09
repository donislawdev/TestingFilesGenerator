package damage

import (
	"fmt"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// The refusals this package produces, each in the four parts D6 asks for: what
// happened, why, what is allowed, and what to do instead. Kept apart from
// Error() so a report can lay them out without printing the same list twice,
// which is the shape internal/format/refusals.go settled and internal/cli
// reads through What/Why/Instead.

// UnknownError is a damage this build does not know.
type UnknownError struct {
	ID    string
	Known []string
}

// What happened, without the list of names.
func (e *UnknownError) What() string {
	return fmt.Sprintf("there is no damage called %q", e.ID)
}

// Why this is refused rather than skipped.
//
// Silently ignoring it would produce an intact file whose manifest says it is
// broken, which is untouchable rule 6 read backwards: the silence is not a
// missing file but a missing hole in one.
func (e *UnknownError) Why() string {
	return "a damage has to be one this build knows, and one it does not would leave the file intact while the manifest called it broken"
}

// Instead names what there is, from the registry rather than from a list.
func (e *UnknownError) Instead() string {
	if len(e.Known) == 0 {
		return "remove the damage line"
	}
	return "use one of: " + strings.Join(e.Known, ", ")
}

func (e *UnknownError) Error() string {
	if len(e.Known) == 0 {
		return e.What()
	}
	return e.What() + ". This build has: " + strings.Join(e.Known, ", ")
}

// TooSmallError is a file smaller than the damage it was given.
//
// It is a refusal in the plan rather than a failure part way through, because
// the fault is in the recipe and is visible before the first byte. Measured on
// 2026-09-09: txt declares a minimum of 0 B and a zero byte file really is
// written, so this is reachable rather than theoretical.
type TooSmallError struct {
	Damage    string
	Requested int64
	Floor     int64
}

// AboutSetting puts this on the size box rather than at the foot of the form,
// because the size is the half a person can change without giving up the
// damage.
func (e *TooSmallError) AboutSetting() string { return "size" }

func (e *TooSmallError) What() string {
	return fmt.Sprintf("%s needs at least %s and the file is %s",
		e.Damage, core.ExactBytes(e.Floor), core.ExactBytes(e.Requested))
}

func (e *TooSmallError) Why() string {
	return "a file smaller than the damage would come out unchanged, and an unchanged file described as broken is the one thing this tool must not write"
}

func (e *TooSmallError) Instead() string {
	return fmt.Sprintf("Ask for %s or more, or take the damage off this target.",
		core.ExactBytes(e.Floor))
}

func (e *TooSmallError) Error() string {
	return e.What() + ". " + e.Instead()
}

// NoChangeError is a damage that ran and moved nothing.
//
// Not a warning. The file would be well formed, every reader would accept it,
// and the manifest would say expected: reject about it - the tool lying in the
// exact place its value lives. It ends the run.
//
// It is per damage rather than per file on purpose. Damages compose, and two
// of them can cancel out - a second zeroing of a band already zeroed does
// nothing - so the question has to be asked after EACH step rather than once
// at the end, which would pass a list whose second entry was idle.
type NoChangeError struct {
	Damage string
	File   string
}

func (e *NoChangeError) What() string {
	return fmt.Sprintf("%s changed nothing in %s", e.Damage, e.File)
}

func (e *NoChangeError) Why() string {
	return "the file would be accepted by every reader while the manifest called it broken, so the run stops rather than writing it"
}

func (e *NoChangeError) Instead() string {
	return "give the damage a larger file, different settings, or take it off this target"
}

func (e *NoChangeError) Error() string {
	return e.What() + ". " + e.Why()
}

// RuledOutExpectation is the one declared outcome a damaged file cannot have.
//
// It is spelled here rather than imported because internal/manifest sits
// beside this package rather than under it, so the two cannot see each other.
// TestTheOutcomeDamageRulesOutIsTheOneTheManifestKnows compares this against
// manifest.OutcomeAccept and against the list a recipe accepts, which is what
// stops three spellings of one word from drifting apart.
const RuledOutExpectation = "accept"

// ConflictsWithExpectation is the refusal a target earns by damaging its files
// and declaring they will be accepted, or nil when there is no conflict.
//
// It lives here, on the chain, because it is a fact about damage rather than
// about either surface - and both surfaces ask it. Measured on 2026-09-09 with
// the check living in the recipe reader alone: a recipe was refused with code
// 3 while the identical run off the command line ended with code 0 and wrote a
// manifest saying a deliberately broken file should be accepted. The recipe
// reader still asks first, so it keeps reporting this beside every other
// problem of that recipe and with the address of the target - what changed is
// that the engine asks too, so no surface can get past it. See O199.
// Two shapes of one rule, and the pair is deliberate. The engine wants an
// error to hand upwards, while the recipe reader wants the parts - What, Why
// and Instead - to lay out beside the other problems of that recipe.
//
// Written as two functions rather than one returning the concrete type,
// because that one would be the typed nil trap: a nil *ExpectationConflictError
// placed in an error interface is NOT a nil error. Measured 2026-09-09 on a
// four case program - "reject", "sanitize", "" and "accept" all came back
// err != nil - so the engine would have refused EVERY target, damaged or not,
// and the guard beside this one asserts exactly that it does not.
func (c Chain) ConflictsWithExpectation(expected string) error {
	if bad := c.ExpectationConflict(expected); bad != nil {
		return bad
	}
	return nil
}

// ExpectationConflict is the same question answered with the refusal itself,
// or nil. For a caller that needs the parts rather than an error.
func (c Chain) ExpectationConflict(expected string) *ExpectationConflictError {
	if len(c) == 0 || expected != RuledOutExpectation {
		return nil
	}
	return &ExpectationConflictError{Outcome: expected}
}

// ExpectationConflictError is a target that damages a file and expects it to
// be accepted.
//
// Only accept. reject is what damage implies and is the default, while
// sanitize and unspecified are both sensible questions to ask about a broken
// file - a system under test may well be expected to repair it, or the answer
// may be the point of the test. Owner's call on 2026-09-09: refuse the one
// that cannot be true, and leave the two that can.
type ExpectationConflictError struct {
	Outcome string
}

// AboutSetting puts this on the expectation rather than on the damage, because
// the damage is what the target is FOR and the expectation is the line that
// disagrees with it.
func (e *ExpectationConflictError) AboutSetting() string { return "expected" }

func (e *ExpectationConflictError) What() string {
	return fmt.Sprintf("this target damages the file and expects %q", e.Outcome)
}

func (e *ExpectationConflictError) Why() string {
	return "a damaged file is one a judge was measured to refuse, so expecting it to be accepted is an expectation nothing could meet"
}

func (e *ExpectationConflictError) Instead() string {
	return "leave expected out and get reject, or write sanitize if the system under test is meant to repair the file, or unspecified if that is the question"
}

func (e *ExpectationConflictError) Error() string {
	return e.What() + ". " + e.Instead()
}
