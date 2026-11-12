// Damage as the engine sees it: what a target may be refused for before
// anything is written, and what the manifest records afterwards.
//
// In its own file rather than in engine.go, and that is a measurement: engine.go
// stood at 431 lines of code against a ceiling of 433, so two more functions
// there would have pushed a file over a gate that exists to stop exactly this.
// Cutting a file does not make a type smaller, but these two were never part of
// what engine.go is about - they are one subject, which is the better reason.
package engine

import (
	"context"
	"errors"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// checkDamage is every refusal a damaged target can earn during planning.
//
// One entry rather than two calls side by side in engine.go, and that is the
// same measurement this file was cut out for: adding the second call put
// engine.go at 411 lines of code against a ceiling of 408, and the answer to a
// ceiling is a cut rather than a larger number. These two belong together
// anyway - both are "what makes this target impossible before a byte is
// written", which is one subject.
//
// The floor first, because it names a number a person can act on. A target
// earning both refusals gets that one.
func checkDamage(t *Target, desc format.Descriptor) error {
	if err := checkDamageFloor(t); err != nil {
		return err
	}
	return checkDamageExpectation(t, desc)
}

// prepareDamage is what a damaged target is refused for before a size is
// drawn, and the format its files are planned through.
//
// Before the draw, because a setting the damage does not allow, a chain that
// cuts away what it added and a damage with no size to work from are wrong
// whatever size comes out of the seed. And the format it hands back is the one
// the draw has to ask: a damage that changes the length needs the generator
// asked for a different size than the one on disk (damagedformat.go).
//
// The settings first, because every question after them reads the settings
// and would answer about the default where the value is out of range.
func prepareDamage(t *Target, desc format.Descriptor) (format.Descriptor, error) {
	if len(t.Damage) == 0 {
		return desc, nil
	}
	if err := t.Damage.Check(); err != nil {
		return desc, err
	}
	if eaten := t.Damage.Eaten(); eaten != nil {
		return desc, eaten
	}
	if sized := t.Damage.SizeConflict(t.SizeFromContents); sized != nil {
		return desc, sized
	}
	if !t.Damage.ChangesLength() {
		return desc, nil
	}
	return damagedFormat(desc, t.Damage), nil
}

// checkDamageFloor refuses a file smaller than the damage it was given.
//
// Here rather than at the moment of writing, and that is the point: a file
// smaller than its damage comes out unchanged, and an unchanged file the
// manifest calls broken is the one thing this axis must not produce. The fault
// is in the recipe and is visible before the first byte, so it is a refusal
// during planning - which is what makes "an invalid recipe writes no files"
// true here as everywhere else.
//
// Reachable rather than theoretical: txt declares a minimum of 0 B and a zero
// byte file really is written, measured 2026-09-09.
//
// A container whose size comes from its contents is left alone. Its entries in
// Sizes carry only the count and their value is not read, so comparing them
// against anything would be comparing against a number nobody set. The damage
// still applies to the finished archive, and the check that it moved a byte
// still runs - what is missing is only the early refusal.
//
// A chain that changes the length is left to the format it plans through
// (damagedformat.go), which knows the format's floor as well as the damage's
// and names the larger. Asked here, a PNG with junk appended would be told 64
// B is enough and then refused at 64 B for needing 138.
func checkDamageFloor(t *Target) error {
	if len(t.Damage) == 0 || t.SizeFromContents || t.Damage.ChangesLength() {
		return nil
	}
	floor, owner, err := t.Damage.Floor()
	if err != nil {
		return err
	}
	for _, size := range t.Sizes {
		if size < floor {
			return &damage.TooSmallError{Damage: owner, Requested: size, Floor: floor}
		}
	}
	return nil
}

// checkDamageExpectation refuses a target that breaks its files and declares
// they will be accepted.
//
// Here as well as in the recipe reader, and that is the whole point of it. The
// condition is one function on the chain, so this is a second CALLER rather
// than a second copy - what it buys is that the command line reaches it, and
// the command line never reads a recipe. Measured on 2026-09-09 before this
// existed: the recipe was refused with code 3 while
// --damage zero-head --expected accept ended with code 0 and wrote a manifest
// claiming a deliberately broken file should be accepted. O199.
//
// The window cannot reach this today - damage sits on the generate screen and
// the expectation on the recipe screen - and being here rather than in the
// reader is what covers it on the day those two meet.
func checkDamageExpectation(t *Target, desc format.Descriptor) error {
	return t.Damage.ConflictsWithExpectation(t.Expected, damage.KindOf(desc, t.Properties))
}

// writeDamaged runs the generator into the writer it is given and, for a
// damaged file, ends the damages once the generator has ended.
//
// A damage that cut the file short stops the generator with
// damage.ErrKeptEnough, and that is the file being complete rather than a
// failure: every byte it keeps has passed, and the rest was never going to
// reach the disk. Only a damaged file can stop that way, so the error is read
// as success for nothing else. A stop of the run arriving at the same moment
// comes back as the context's own error, and the file fails and is removed
// like any other file stopped part way.
func writeDamaged(ctx context.Context, f PlannedFile, w io.Writer, streams []damage.Stream) error {
	err := writeWithoutCrashing(ctx, f, w)
	if !f.Damaged() {
		return err
	}
	if errors.Is(err, damage.ErrKeptEnough) {
		err = nil
	}
	if err != nil {
		return err
	}
	return f.Target.Damage.Finish(streams)
}

// damagedExpectation is what the manifest says a system under test should do
// with a damaged file nobody declared an expectation for.
//
// Reject, with certainty, where the damage breaks the rules of the format: the
// guard TestEveryDamageIsRefusedByTheStructuralCheckInEveryFormat measures
// that a judge refuses every such pair, so untouchable rule 5 is kept - the
// expectation is not invented. Where the damage leaves the file's fate to the
// reader, the manifest says so rather than guessing, in the shape MF5 gives
// that case: unspecified, policy dependent, with both answers offered and the
// reason in a sentence (owner's call, 2026-11-10).
//
// The settings are the ones the target stated, defaults filled in - the
// same question the plan asked when it decided whether accept was allowed.
func damagedExpectation(f PlannedFile) manifest.Expected {
	certain, why, err := f.Target.Damage.Certain(damage.KindOf(f.Desc, f.Target.Properties))
	if err != nil || certain {
		// An error is a damage the registry does not know, which planning
		// refused already - the file could not have been written.
		return manifest.Expected{
			Outcome:    manifest.OutcomeReject,
			Reason:     "content_malformed",
			Confidence: "certain",
		}
	}
	return manifest.Expected{
		Outcome:       manifest.OutcomeUnspecified,
		Reason:        "content_malformed",
		Detail:        why,
		Confidence:    "policy_dependent",
		PolicyOptions: []string{manifest.OutcomeAccept, manifest.OutcomeReject},
	}
}

// damageFor records what was broken about this file, with the settings
// resolved rather than as written.
//
// Nil for a file nothing damaged, so the key is absent rather than empty and
// every manifest written before this existed is unchanged.
//
// cut is where the damage that cut the file fell, and goes to that damage's
// entry alone.
func damageFor(f PlannedFile, cut *manifest.Cut) []manifest.Damage {
	if !f.Damaged() {
		return nil
	}
	at, _, cuts := f.Target.Damage.Cut(f.Plan.Bytes)
	out := make([]manifest.Damage, 0, len(f.Target.Damage))
	for i, s := range f.Target.Damage {
		d := manifest.Damage{Type: s.ID, Settings: s.Resolved()}
		if cuts && i == at {
			d.Cut = cut
		}
		out = append(out, d)
	}
	return out
}
