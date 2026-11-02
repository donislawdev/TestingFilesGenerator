package guard

import (
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
)

// A plan is a list reserved once, not one grown a file at a time.
//
// Grown by append, a plan of a million text files took 1.9-2.2 GB from the
// system for 0.94 GB of plan - every doubling leaves the last array to the
// collector - and reserved it took 1.2 GB with five collections instead of
// fourteen (docs/FILE-CEILING-2026-11-02.md section 2). A list that was grown
// ends with room to spare almost always, so a plan whose capacity is exactly
// its length is a plan that was reserved.
//
// What this does not hold: that the reservation stops at the memory setting.
// That half differs only in how much the process takes at its peak, and the
// heap of this test binary is shared by every guard running beside it (O162) -
// a measurement here would be about the neighbours. It is said rather than
// pretended.
func TestAPlanIsReservedRatherThanGrown(t *testing.T) {
	const n = 1000
	files, err := engine.Plan([]engine.Target{{ID: "files", Format: "txt", Sizes: engine.Uniform(n, 100)}},
		engine.Options{OutDir: t.TempDir(), ManifestName: "manifest.json"})
	if err != nil {
		t.Fatalf("planning %d text files: %v", n, err)
	}
	if len(files) != n {
		t.Fatalf("the plan holds %d files, not the %d asked for, so this is not the case it names", len(files), n)
	}
	if cap(files) != len(files) {
		t.Errorf("a plan of %d files came back with room for %d - it was grown, not reserved, "+
			"and grown it takes about twice its own size from the system", len(files), cap(files))
	}
}
