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
	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
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
func checkDamage(t *Target) error {
	if err := checkDamageFloor(t); err != nil {
		return err
	}
	return checkDamageExpectation(t)
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
func checkDamageFloor(t *Target) error {
	if len(t.Damage) == 0 || t.SizeFromContents {
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
func checkDamageExpectation(t *Target) error {
	return t.Damage.ConflictsWithExpectation(t.Expected)
}

// damageFor records what was broken about this file, with the settings
// resolved rather than as written.
//
// Nil for a file nothing damaged, so the key is absent rather than empty and
// every manifest written before this existed is unchanged.
func damageFor(f PlannedFile) []manifest.Damage {
	if !f.Damaged() {
		return nil
	}
	out := make([]manifest.Damage, 0, len(f.Target.Damage))
	for _, s := range f.Target.Damage {
		out = append(out, manifest.Damage{Type: s.ID, Settings: s.Resolved()})
	}
	return out
}
