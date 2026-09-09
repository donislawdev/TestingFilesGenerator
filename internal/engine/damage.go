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
