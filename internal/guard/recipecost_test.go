package guard

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// Reading a recipe costs what its size says, and not the square of it.
//
// The batch screen settles the whole recipe on every keystroke - compose it,
// parse it, hash it - so the cost of reading is the cost of typing. Measured on
// 2026-08-27 with tools/probes/settlecost: one hundred batches took 259 ms
// against the 100 ms threshold in docs/UX.md, and nothing caps how many batches
// somebody adds.
//
// The square was ours rather than the library's, which is the part worth
// remembering. Handing goccy a BytesUnmarshaler means it has to render each
// node back into text before calling it, and it does that by building a
// formatter over the document - so reading N values walks the document N times.
// Switching scalar to a NodeUnmarshaler for the kinds that are one value took
// one hundred batches from 259 ms to 14 ms.
//
// This asks for the RATIO rather than for a duration, and that is deliberate.
// A wall clock gate is flaky on a shared CI runner and a flaky guard gets
// switched off - this file already says so about the allocation ceiling next
// door. A ratio does not care how slow the machine is: doubling the input
// doubles linear work and quadruples quadratic work, whatever the clock.
//
// Bytes allocated rather than seconds, for the same reason. The formatter's
// cost is dominated by what it builds, so the allocation figure carries the
// shape with far less noise than time does.
func TestReadingARecipeDoesNotCostTheSquareOfItsSize(t *testing.T) {
	const (
		small = 50
		large = 100
		// Doubling the input may double the work. Measured after the change:
		// 2.0 to 2.1. Measured before it, when the work was quadratic: 3.6 to
		// 4.9. The threshold sits between those two measurements rather than
		// being picked, the same rule as the arm64 pixel threshold.
		mostItMayGrow = 3.0
	)

	costOf := func(batches int) float64 {
		src := recipeWithBatches(batches)

		// Once to warm anything the first parse sets up, so the reading is
		// about the work and not about initialisation.
		if _, err := recipe.Parse(src, "recipe"); err != nil {
			t.Fatalf("parsing %d batches: %v", batches, err)
		}

		runtime.GC()
		var before runtime.MemStats
		runtime.ReadMemStats(&before)

		const rounds = 3
		for i := 0; i < rounds; i++ {
			if _, err := recipe.Parse(src, "recipe"); err != nil {
				t.Fatalf("parsing %d batches: %v", batches, err)
			}
		}

		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		return float64(after.TotalAlloc-before.TotalAlloc) / rounds
	}

	smallCost := costOf(small)
	largeCost := costOf(large)
	if smallCost == 0 {
		t.Fatal("reading a recipe allocated nothing, so this measures the wrong thing")
	}

	grew := largeCost / smallCost
	t.Logf("%d batches: %.0f B, %d batches: %.0f B, ratio %.2f",
		small, smallCost, large, largeCost, grew)

	if grew > mostItMayGrow {
		t.Errorf("doubling the recipe from %d to %d batches multiplied the reading cost by %.2f, "+
			"and %.1f is where linear stops and quadratic starts. Reading N values is walking the "+
			"whole document N times - the batch screen settles on every keystroke, so this is the "+
			"cost of typing",
			small, large, grew, mostItMayGrow)
	}
}

// recipeWithBatches writes a recipe with the given number of targets.
//
// Written out rather than composed, so this measures the reader and not our own
// encoder beside it.
func recipeWithBatches(n int) []byte {
	var b strings.Builder
	b.WriteString("version: 1\nseed: 7\ntargets:\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "  - id: batch-%d\n    format: txt\n    count: 2\n    size: 1kb\n", i)
	}
	b.WriteString("output:\n  dir: out\n")
	return []byte(b.String())
}
