package guard

import (
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// The ceiling on files in one run is counted by the recipe reader, and not only
// by the engine underneath it (O302).
//
// The reader bounded each target on its own and left the run to the engine -
// but the engine is handed every target with its list of sizes already built,
// so a recipe of fifty targets at the count ceiling took 2.5 GB to validate
// before it was refused, and the window, which reads the recipe on every key,
// spent 40 MiB a key on every batch of a million. Measured on 2026-11-03 and
// 2026-11-04, docs/FILE-CEILING-2026-11-02.md section 14.

// A target the run has no room left for is read, and its list is not built.
//
// Asked of what the reader allocates rather than of what it says, because the
// refusal reads the same either way - the defect was the list built before it.
// The first reading is the control: one target at the ceiling is legal and its
// list is eight bytes a file, so a measurement that cannot see that one cannot
// see the one this is here for.
func TestATargetPastTheRunCeilingIsNotBuilt(t *testing.T) {
	legal := targetsOf(core.MaxFilesPerRun)
	over := targetsOf(1, core.MaxFilesPerRun)

	built := readingCost(t, legal)
	if _, err := recipe.Parse(legal, "legal"); err != nil {
		t.Fatalf("one target at the ceiling was refused, so the control is not the case it names: %v", err)
	}
	notBuilt := readingCost(t, over)
	t.Logf("one target of %d: %d B, the same target past one more file: %d B", core.MaxFilesPerRun, built, notBuilt)
	ceilingAt(t, over, 2)

	if built < 8*core.MaxFilesPerRun {
		t.Fatalf("reading one target of %d files allocated %d B, under the eight bytes a file its list "+
			"takes - this reading does not see the list, so it cannot say that one was not built", core.MaxFilesPerRun, built)
	}
	if notBuilt > core.MaxFilesPerRun {
		t.Errorf("reading a recipe whose second target takes the run past the ceiling allocated %d B, over "+
			"a byte a file of that target. Its list was built before the run was refused - which is what "+
			"took fifty such targets to 2.5 GB", notBuilt)
	}
}

// A list of sizes is reserved at its length, not grown to it.
//
// Grown one file at a time, a million files took 40 B a file from the
// collector for a list of eight - the window's batch screen paid that on every
// key, and the Generate screen, which reserves (engine.Uniform), paid eight. A
// list that was grown ends with room to spare, so one whose capacity is its
// length was reserved - the reading planreserve_test.go makes of the plan.
// Each of the three ways a count turns into a list is asked.
func TestATargetsListOfSizesIsReservedAtItsLength(t *testing.T) {
	const count = 1000
	src := fmt.Sprintf(`version: 1
targets:
  - id: sized
    format: txt
    size: 1kb
    count: %[1]d
  - id: ranged
    format: txt
    size-range: 1kb-2kb
    count: %[1]d
  - id: holding
    format: zip
    count: %[1]d
    contains:
      - format: txt
        count: 1
        size: 10
`, count)

	rec, err := recipe.Parse([]byte(src), "reserved")
	if err != nil {
		t.Fatalf("the recipe was refused: %v", err)
	}
	for _, target := range rec.Targets {
		if len(target.Sizes) != count {
			t.Fatalf("target %q has %d sizes, expected %d - this is not the list it names", target.ID, len(target.Sizes), count)
		}
		if cap(target.Sizes) != len(target.Sizes) {
			t.Errorf("target %q has a list of %d sizes with room for %d, so it was grown rather than reserved. "+
				"Grown, a million files cost 40 B a file to read instead of 8, on every key of the batch screen",
				target.ID, len(target.Sizes), cap(target.Sizes))
		}
	}
}

// The run's ceiling sits exactly at a million, counted the way the engine counts.
//
// A million across two targets is legal, and one file more is not. A boundary
// set is three files whatever the count says, so it is asked at the edge as
// well: counted as the one file of the default count, it would let a run of a
// million and two files through to the engine. And a count
// refused on its own asks for nothing - counted as the default, it would take a
// million beside it past the ceiling and say one mistake twice.
func TestTheRunCeilingSitsExactlyAtAMillion(t *testing.T) {
	boundary := func(before int) []byte {
		return []byte(fmt.Sprintf("version: 1\ntargets:\n  - id: t1\n    format: txt\n    size: 1b\n    count: %d\n"+
			"  - id: t2\n    format: txt\n    boundary: 1kb\n", before))
	}
	for name, src := range map[string][]byte{
		"a million in two targets":                       targetsOf(core.MaxFilesPerRun-1, 1),
		"a boundary set that takes the run to a million": boundary(core.MaxFilesPerRun - 3),
	} {
		if _, err := recipe.Parse(src, name); err != nil {
			t.Errorf("%s was refused: %v", name, err)
		}
	}
	for name, src := range map[string][]byte{
		"a million and one in two targets":                   targetsOf(core.MaxFilesPerRun, 1),
		"a boundary set that takes the run one file past it": boundary(core.MaxFilesPerRun - 2),
	} {
		t.Run(name, func(t *testing.T) { ceilingAt(t, src, 2) })
	}

	_, err := recipe.Parse(targetsOf(0, core.MaxFilesPerRun), "refused count")
	var invalid *recipe.ValidationError
	if !errors.As(err, &invalid) || len(invalid.Problems) != 1 || invalid.Problems[0].At != core.TargetAddress(1, recipe.KeyCount) {
		t.Errorf("a count of 0 beside a target of a million should be refused once, for its own count: %v", err)
	}
}

// The refusal says the files the whole run asks for, at the target that
// crossed the ceiling.
//
// Both are refused where the run crosses, so the running total there was the
// number they said - ten targets of a million were told they asked for two
// million across ten targets (review of #219, measured with the binary before
// O302 as well). The target after the one that crosses is what tells the two
// numbers apart.
func TestTheRunCeilingSaysWhatTheWholeRunAsksFor(t *testing.T) {
	src := targetsOf(1, core.MaxFilesPerRun, core.MaxFilesPerRun)
	ceilingAt(t, src, 2)
	_, err := recipe.Parse(src, "whole run")
	if !strings.Contains(err.Error(), "2000001 files across 3 targets") {
		t.Errorf("the refusal does not say the files the whole run asks for, 2000001:\n%v", err)
	}
}

// A run past the ceiling is reported together with whatever else is wrong.
//
// Every problem of a recipe arrives at once (RC7). While the engine counted the
// run, a recipe with any other problem was refused for that one alone, and the
// ceiling came out only once it was fixed - one more round for somebody who
// could have been told both.
func TestARunPastTheCeilingIsReportedBesideTheRecipesOtherProblems(t *testing.T) {
	path := writeRecipe(t, t.TempDir(), `version: 1
targets:
  - id: small
    format: txt
    size: 1kb
    count: 1
    properties:
      no-such-property: 1
  - id: big
    format: txt
    size: 1kb
    count: 1000000
`)
	code, stdout, errOut := run(t, "validate", "--json", path)
	if code != cli.ExitRecipe {
		t.Errorf("exit %d, expected %d:\n%s", code, cli.ExitRecipe, errOut)
	}
	if stdout != "" {
		t.Errorf("a refused recipe wrote to stdout:\n%s", stdout)
	}
	at := map[string]string{}
	for _, p := range reportedProblems(t, errOut) {
		at[p.At] = p.What
	}
	if _, ok := at["targets[1].properties.no-such-property"]; !ok {
		t.Errorf("the property nobody declared is not in the report, so this is not the case it names:\n%s", errOut)
	}
	if what := at[core.TargetAddress(2, recipe.KeyCount)]; !strings.Contains(what, "1000001 files across 2 targets") {
		t.Errorf("the run past the ceiling is not reported beside the other problem at %s.\n"+
			"Reason: a person fixes the property, runs again and only then hears about the ceiling.\n%s",
			core.TargetAddress(2, recipe.KeyCount), errOut)
	}
}

// The target that crosses the ceiling is named where the file has it, also in a
// recipe built on a preset.
//
// The engine counts the merged list, the preset's targets first, and knows
// nothing of where the file's own begin - so a recipe of one target on top of
// size-boundaries was refused at targets[8].count, a box neither the file nor
// the screen has. Measured on 2026-11-04. The reader knows the offset
// (spotOfTarget), and the plain recipe is asked beside it so the two halves
// cannot both be wrong in the same way.
func TestTheTargetThatCrossesTheRunCeilingIsPlacedInTheFile(t *testing.T) {
	cases := []struct{ name, src string }{
		{"a recipe of its own", `version: 1
targets:
  - id: small
    format: txt
    size: 1kb
    count: 1
  - id: big
    format: txt
    size: 1kb
    count: 1000000
`},
		{"a recipe built on a preset", `version: 1
extends: preset:size-boundaries
with:
  limit: 4mb
  format: txt
targets:
  - id: small
    format: txt
    size: 1kb
    count: 1
  - id: big
    format: txt
    size: 1kb
    count: 1000000
`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := writeRecipe(t, t.TempDir(), c.src)
			code, _, errOut := run(t, "validate", "--json", path)
			if code != cli.ExitRecipe {
				t.Fatalf("exit %d, expected %d:\n%s", code, cli.ExitRecipe, errOut)
			}
			problems := reportedProblems(t, errOut)
			if len(problems) != 1 || !strings.Contains(problems[0].What, "files across") {
				t.Fatalf("expected the ceiling and nothing else:\n%s", errOut)
			}
			if want := core.TargetAddress(2, recipe.KeyCount); problems[0].At != want {
				t.Errorf("the ceiling was reported at %q and the target of the file that crossed it is %q.\n"+
					"Reason: an address is the box a screen marks and the line a script points at, and one\n"+
					"counted through the preset's targets names neither.", problems[0].At, want)
			}
		})
	}
}

// The engine counts the run of targets it is handed directly.
//
// Every surface reads a recipe first, and the reader counts the run since O302
// - so no surface reaches this refusal any more, and the guards that asked it
// through validate and through the window ask the reader now. Plan is handed
// targets by whoever calls it, though, and planCapacity reserves nothing past
// the ceiling on the word of this refusal. The memory setting is there so a
// build that lost the refusal plans until the plan ceiling stops it, a few
// megabytes in, rather than a million files. A third target follows the one
// that crosses, so the number the refusal says is asked to be the whole run's
// rather than the running total where it stopped.
func TestTheEngineCountsTheWholeRunOfTargetsHandedToItDirectly(t *testing.T) {
	targets := []engine.Target{
		{ID: "small", Format: "txt", Sizes: engine.Uniform(1, 1024)},
		{ID: "big", Format: "txt", Sizes: engine.Uniform(core.MaxFilesPerRun, 1024)},
		{ID: "after", Format: "txt", Sizes: engine.Uniform(core.MaxFilesPerRun, 1024)},
	}
	_, err := engine.Plan(targets, engine.Options{OutDir: t.TempDir(), ManifestName: "manifest.json", Memory: core.MemoryOf(32 << 20)})
	if err == nil {
		t.Fatal("a run of a million files more than the ceiling was planned")
	}
	if !strings.Contains(err.Error(), "files across 3 targets") {
		t.Fatalf("the run was refused for something other than its count of files: %v", err)
	}
	if !strings.Contains(err.Error(), "2000001 files across 3 targets") {
		t.Errorf("the refusal does not say the files the whole run asks for, 2000001: %v", err)
	}
	var about interface{ AboutSetting() string }
	if !errors.As(err, &about) || about.AboutSetting() != core.TargetAddress(2, engine.SettingCount) {
		t.Errorf("the refusal is not addressed to the target that crossed the ceiling, %s: %v",
			core.TargetAddress(2, engine.SettingCount), err)
	}
}

// targetsOf is a recipe of txt targets of 1 B, one for each count given.
func targetsOf(counts ...int) []byte {
	var b strings.Builder
	b.WriteString("version: 1\ntargets:\n")
	for i, n := range counts {
		fmt.Fprintf(&b, "  - id: t%d\n    format: txt\n    size: 1b\n    count: %d\n", i+1, n)
	}
	return []byte(b.String())
}

// readingCost is what reading src allocates, warmed once first so the figure
// is about the recipe and not about what the first reading sets up.
func readingCost(t *testing.T, src []byte) uint64 {
	t.Helper()
	_, _ = recipe.Parse(src, "warm")
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, _ = recipe.Parse(src, "measured")
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

// ceilingAt asks that the reader refuses src for the run's count of files, at
// the target in the given position.
func ceilingAt(t *testing.T, src []byte, position int) {
	t.Helper()
	_, err := recipe.Parse(src, "over")
	var invalid *recipe.ValidationError
	if !errors.As(err, &invalid) {
		t.Fatalf("the reader did not refuse a run past the ceiling: %v", err)
	}
	want := core.TargetAddress(position, recipe.KeyCount)
	for _, p := range invalid.Problems {
		if p.At == want && strings.Contains(p.What.String(), "files across") {
			return
		}
	}
	t.Fatalf("the reader did not refuse the run's count of files at %s: %v", want, err)
}

// reportedProblem is one problem of validate --json.
type reportedProblem struct {
	What string `json:"what"`
	At   string `json:"at"`
}

// reportedProblems reads the problems validate --json put out.
func reportedProblems(t *testing.T, report string) []reportedProblem {
	t.Helper()
	var parsed struct {
		Problems []reportedProblem `json:"problems"`
	}
	if err := json.Unmarshal([]byte(report), &parsed); err != nil {
		t.Fatalf("the report is not readable as JSON: %v\n%s", err, report)
	}
	return parsed.Problems
}
