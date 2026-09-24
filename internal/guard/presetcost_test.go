package guard

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// A new upload limit does not ask the same questions again.
//
// Expanding upload-validation took 137-142 ms and 57 MB whenever a value
// changed, which in the window is every key typed into one of its settings or
// into the batch screen built on it. Two causes, both a question asked again
// with the answer already known: the smallest size of a format, found for
// every file about a name by encoding pictures at growing sizes, and one small
// picture planned under a dozen names (docs/GUI-MEMORY-2026-09-23.md section
// 4j). Measured 2026-09-24, least of five: 9.58 MB with both answered once,
// 18.95 MB with the picture planned under every name, 44.29 MB with the floor
// worked out for every file, 9.67 MB under -race. The line sits between the
// first two.
//
// Asked with a limit this process has not expanded before, because the same
// limit twice is the window's memory's business and says nothing about this.
// The least of several readings, because the counter is the whole process's
// and a reading can only be too high - see
// TestTheMinimalSetIsWorkedOutOnceAndNotAtEveryExpansion.
//
// If it goes red after a change that is not about this, measure the four
// numbers above again before moving the line: a line moved to the new reading
// no longer stands between anything.
func TestANewUploadLimitDoesNotAskTheSameQuestionsAgain(t *testing.T) {
	const ceiling = 13<<20 + 1<<19 // 13.5 MB
	// The state this is about: files that ask one question under several
	// names. A set without them would expand cheaply whatever this code did.
	if most := mostFilesAskingOneQuestion(t, "upload-validation", preset.Args{}); most < 5 {
		t.Fatalf("the most files of upload-validation asking one question is %d, so the set no longer holds what this guard is about", most)
	}
	least := leastAllocatedByAnExpansion(t, "upload-validation", func(i int) preset.Args {
		return preset.Args{"limit": fmt.Sprintf("%dmb", 21+i)}
	})
	if least > ceiling {
		t.Errorf("expanding upload-validation at a new limit allocated %d bytes, over %d - "+
			"a format's smallest size or a file already planned is being worked out again", least, ceiling)
	}
}

// A new row count works the sheet's smallest size out once, not twice.
//
// tabular-import asks for its sheet at the smallest size the rows and columns
// allow, and finding that size means building the sheet at growing sizes. The
// size was asked twice per expansion - once to check the file, once to write
// it into the set - and nothing remembered it. A new row count does change the
// sheet, so one working out is the real work and the second was the waste.
// The wide CSV, the other file asked at its floor, is the same at every
// expansion and now comes from memory.
//
// Measured 2026-09-24 at 300 rows, least of five: 19.84 MB remembered, 38.59 MB
// worked out twice, 20.63 MB under -race. The line sits between. The numbers
// grow with the rows - 700 rows allocate 39 MB remembered - so the rows asked
// here stay where they were measured.
func TestANewRowCountWorksTheSheetsSmallestSizeOutOnce(t *testing.T) {
	const ceiling = 27 << 20
	if sheets := filesOfFormat(t, "tabular-import", preset.Args{}, "xlsx"); sheets != 1 {
		t.Fatalf("tabular-import holds %d sheets, so the set no longer holds the file this guard is about", sheets)
	}
	least := leastAllocatedByAnExpansion(t, "tabular-import", func(i int) preset.Args {
		return preset.Args{"rows": fmt.Sprintf("%d", 300+i)}
	})
	if least > ceiling {
		t.Errorf("expanding tabular-import at a new row count allocated %d bytes, over %d - "+
			"the sheet's smallest size is being worked out more than once", least, ceiling)
	}
}

// leastAllocatedByAnExpansion expands a preset once, then five times with
// values it has not been given, and returns the least any of the five
// allocated.
func leastAllocatedByAnExpansion(t *testing.T, id string, fresh func(int) preset.Args) uint64 {
	t.Helper()
	if _, err := preset.Expand(id, preset.Args{}); err != nil {
		t.Fatalf("%s did not expand, so nothing was asked: %v", id, err)
	}
	least := ^uint64(0)
	for i := 0; i < 5; i++ {
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		if _, err := preset.Expand(id, fresh(i)); err != nil {
			t.Fatalf("%s at %v: %v", id, fresh(i), err)
		}
		runtime.ReadMemStats(&after)
		if spent := after.TotalAlloc - before.TotalAlloc; spent < least {
			least = spent
		}
	}
	return least
}

// mostFilesAskingOneQuestion is how many targets of an expanded set ask their
// format the same thing, at most - one format, one size, one set of settings.
func mostFilesAskingOneQuestion(t *testing.T, id string, args preset.Args) int {
	t.Helper()
	same := map[string]int{}
	most := 0
	for _, target := range expandedTargets(t, id, args) {
		if len(target.Sizes) == 0 {
			continue
		}
		key, ok := format.RequestKey(target.Format, format.Request{
			Label: true, Properties: target.Properties, Bytes: target.Sizes[0],
		})
		if !ok {
			continue
		}
		same[key]++
		if same[key] > most {
			most = same[key]
		}
	}
	return most
}

func filesOfFormat(t *testing.T, id string, args preset.Args, formatID string) int {
	t.Helper()
	n := 0
	for _, target := range expandedTargets(t, id, args) {
		if target.Format == formatID {
			n++
		}
	}
	return n
}

func expandedTargets(t *testing.T, id string, args preset.Args) []recipe.Target {
	t.Helper()
	expanded, err := preset.Expand(id, args)
	if err != nil {
		t.Fatalf("%s did not expand: %v", id, err)
	}
	rec, err := recipe.Parse(expanded.Source, id)
	if err != nil {
		t.Fatalf("%s expanded into a recipe that does not read: %v", id, err)
	}
	return rec.Targets
}
