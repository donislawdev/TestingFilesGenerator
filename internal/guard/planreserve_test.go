package guard

import (
	"runtime"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
)

// And the reservation waits for a file that passed its checks.
//
// Reserved before the first target, a million files asking for a size their
// format cannot make were refused at the first file after taking 441-450 MB,
// against 27-30 MB before the reservation existed (review of #212, measured on
// 2026-11-02 with two runs each). In a container of half a gigabyte that is a
// refusal that can kill the process instead of being said. Asked of what Plan
// allocated, with a margin the two answers cannot cross: the reserved list is
// 440 MB, everything else here a few. Only two guards in this package run in
// parallel, so the neighbours do not reach that margin.
//
// A size ABOVE what the format can make, since 2026-11-17. A size below it is
// no longer refused at the first file: the engine asks every file of the
// batch for the size all of them take (internal/engine/batchminimum.go), which
// holds nothing per file and allocates as it plans - a million plans of
// garbage, which this sum cannot tell from a reservation. A BMP over 4 GiB is
// refused by the format at the first file and walks nothing.
func TestARunRefusedAtItsFirstFileReservesNothing(t *testing.T) {
	const n = 1_000_000
	targets := []engine.Target{{ID: "files", Format: "bmp", Sizes: engine.Uniform(n, 5<<30)}}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err := engine.Plan(targets, engine.Options{OutDir: t.TempDir(), ManifestName: "manifest.json"})
	runtime.ReadMemStats(&after)
	if err == nil || !strings.Contains(err.Error(), "larger than") {
		t.Fatalf("a BMP of 5 GiB was not refused for its size at the first file, so this is not the case it names: %v", err)
	}
	if took := after.TotalAlloc - before.TotalAlloc; took > 100<<20 {
		t.Errorf("planning %d files refused at the first one allocated %d MB. "+
			"The list for the whole plan was reserved before any file passed its checks", n, took>>20)
	}
}

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
