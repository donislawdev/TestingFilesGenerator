package guard

import (
	"fmt"
	"maps"
	"reflect"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/damage"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// The files of a target are planned a stretch at a time on several goroutines
// and read in order (internal/engine/planstretch.go, O355). Two promises stand
// on that, and each has a test here.
//
// The plan is the same at any width - every name, every number, every byte a
// file will have and every refusal, word for word. That is D11 one step
// earlier than generatorbytes_test.go looks: the bytes of a file follow from
// its plan, and a plan that moved with the number of processors would be a
// hash that moves with the machine. Under the race detector in CI the same
// test is also the one that plans every format beside itself, so a generator
// whose Plan shares something it writes is a red race job rather than a file
// that is different once in a thousand runs.
//
// The pictures planned at once hold no more than the memory setting leaves.
// A picture plans by drawing and encoding itself, so planning sixteen at once
// holds sixteen pictures, the thing the memory setting is for.

// planWidths are the two numbers of processors the plans are compared at. One
// is the plan of one file at a time, eight is more stretches than any case
// below has files for.
var planWidths = [2]int{1, 8}

// besideCases are the shapes that reach each of the three loops planned in
// stretches: the files of a target, the draw of a range, and the walk to the
// size every file of a refused batch accepts - for files that are alike and
// for files that are not (O353). And a damage, which plans every file through
// a generator of the engine's own wrapped round the format's.
func besideCases() map[string][]engine.Target {
	pictures := func() map[string]string { return map[string]string{"width": "640", "height": "480"} }
	return map[string][]engine.Target{
		"png pictures": {{ID: "p", Format: "png", Sizes: engine.Uniform(40, 2<<20), Label: true, Properties: pictures()}},
		"a range of png": {{ID: "r", Format: "png", Sizes: make([]int64, 40), Label: true,
			Range: engine.SizeRange{Used: true, Min: 2 << 10, Max: 96 << 10},
			// 320 by 240 rather than smaller: it encodes to 1573 B, under the
			// range, and takes long enough to plan that a stretch of the draw
			// is still running when plannersBeside looks (64 by 64 was over
			// between two readings, one run in three).
			Properties: map[string]string{"width": "320", "height": "240"}}},
		"png refused at its first file": {{ID: "f", Format: "png", Sizes: engine.Uniform(24, 1), Label: true, Properties: pictures()}},
		"jpg refused, its files unlike": {{ID: "j", Format: "jpg", Sizes: engine.Uniform(24, 36000), Label: true, Properties: pictures()}},
		"damaged pictures": {{ID: "d", Format: "png", Sizes: engine.Uniform(24, 64<<10), Label: true, Properties: pictures(),
			Damage: damage.Chain{{ID: damage.Truncate, Values: damage.Values{damage.SettingMissing: "50"}}}}},
		"two targets": {
			{ID: "a", Format: "jpg", Sizes: engine.Uniform(12, 64<<10), Label: true},
			{ID: "b", Format: "pdf", Sizes: engine.Uniform(12, 16<<10), Label: true},
		},
	}
}

func TestAPlanIsTheSameHoweverManyFilesArePlannedAtOnce(t *testing.T) {
	was := runtime.GOMAXPROCS(0)
	t.Cleanup(func() { runtime.GOMAXPROCS(was) })
	opt := engine.Options{OutDir: t.TempDir(), Seed: 7, Command: "test"}

	// Canary: two plans that differ by their seed have to be told apart, or a
	// comparison that saw nothing would pass every case below. Both accepted
	// and whole, so the difference is the seed's and not a refusal's.
	runtime.GOMAXPROCS(1)
	one, oneErr := engine.Plan(besideCases()["png pictures"], opt)
	other, otherErr := engine.Plan(besideCases()["png pictures"], engine.Options{OutDir: opt.OutDir, Seed: 8, Command: "test"})
	if oneErr != nil || otherErr != nil || len(one) != 40 || len(other) != 40 {
		t.Fatalf("canary: the two plans compared were not both whole (%d files, %v; %d files, %v)", len(one), oneErr, len(other), otherErr)
	}
	if planParts(one, nil, other, nil) == "" {
		t.Fatal("canary: two plans of different seeds compare as one - this test cannot see a plan that moved")
	}

	// Every format, through the first case that pins its bytes, at three
	// files: one planned alone and two beside each other. One case a format
	// rather than all of them, because under the race detector every case is
	// ten times dearer and a format beside itself once is what it is asked.
	cases := map[string][]engine.Target{}
	golden := goldenCases()
	goldenNames := make([]string, 0, len(golden))
	for name := range golden {
		goldenNames = append(goldenNames, name)
	}
	sort.Strings(goldenNames)
	for _, name := range goldenNames {
		target := golden[name]
		if _, taken := cases["golden "+target.Format]; !taken {
			target.Sizes = engine.Uniform(3, target.Sizes[0])
			cases["golden "+target.Format] = []engine.Target{target}
		}
	}
	for name, targets := range besideCases() {
		cases[name] = targets
	}
	names := make([]string, 0, len(cases))
	for name := range cases {
		names = append(names, name)
	}
	sort.Strings(names)

	most := map[string]int{}
	for _, name := range names {
		var planned [2][]engine.PlannedFile
		var errs [2]error
		for i, width := range planWidths {
			runtime.GOMAXPROCS(width)
			targets := append([]engine.Target(nil), cases[name]...)
			for path, seen := range plannersBeside(func() { planned[i], errs[i] = engine.Plan(targets, opt) }) {
				most[path] = max(most[path], seen)
			}
		}
		if why := planParts(planned[0], errs[0], planned[1], errs[1]); why != "" {
			t.Errorf("%s planned at %d processors and at %d is not the same plan: %s", name, planWidths[0], planWidths[1], why)
		}
		if reached := reachedRefusal(name, errs[1]); reached != "" {
			t.Errorf("%s: %s", name, reached)
		}
		// A case that is not named for a refusal has to be planned whole, or
		// two refusals alike would compare as one plan and pass.
		if want := filesOf(cases[name]); !strings.Contains(name, "refused") && (errs[1] != nil || len(planned[1]) != want) {
			t.Errorf("%s was planned to %d of its %d files: %v", name, len(planned[1]), want, errs[1])
		}
	}

	// The state this is about, asserted rather than assumed: at eight
	// processors each of the three loops planned files beside each other, so
	// none of the comparisons above compared one way of planning with itself.
	for _, path := range slices.Sorted(maps.Keys(plannerPaths)) {
		if most[path] < 2 {
			t.Errorf("at most %d goroutines planned %s beside each other at %d processors - nothing of it was planned "+
				"at once, so its plans above were compared with themselves", most[path], path, planWidths[1])
		}
	}
}

// filesOf is how many files targets ask for.
func filesOf(targets []engine.Target) int {
	n := 0
	for _, target := range targets {
		n += len(target.Sizes)
	}
	return n
}

func TestThePicturesPlannedAtOnceHoldNoMoreThanTheMemorySettingLeaves(t *testing.T) {
	was := runtime.GOMAXPROCS(8)
	t.Cleanup(func() { runtime.GOMAXPROCS(was) })
	const mib = int64(1) << 20
	// Two pictures, each holding about 12 MB while it is made, in 16 MB of
	// room - room for one with megabytes to spare, since the heap this test
	// reads is not the heap the engine reads a moment later. A png of 1500 by
	// 1500 holds 11.25 MB while it is drawn and says so for the writer
	// (Holds). A jxl of 600 by 600 is coded while it plans, holds 12.2 MB
	// then and nothing while it is written, so only its plan says so
	// (PlanHolds) - the case a planner reading Holds alone let through.
	//
	// After a target of text, so the pictures are not the first files of the
	// run: the first file of every target is planned alone because nothing
	// yet says what its files hold, and at the first file of a run the
	// ceiling of the plan would hold it alone anyway.
	pictures := map[string]engine.Target{
		"png": {ID: "big", Format: "png", Sizes: engine.Uniform(6, 24*mib), Properties: map[string]string{"width": "1500", "height": "1500"}},
		"jxl": {ID: "big", Format: "jxl", Sizes: engine.Uniform(6, 2*mib), Properties: map[string]string{"width": "600", "height": "600"}},
	}
	for _, id := range slices.Sorted(maps.Keys(pictures)) {
		targets := []engine.Target{{ID: "notes", Format: "txt", Sizes: engine.Uniform(1, 4096)}, pictures[id]}
		files := filesOf(targets)
		plan := func(room int64) (int, []engine.PlannedFile) {
			runtime.GC()
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			opt := engine.Options{OutDir: t.TempDir(), Seed: 7, Command: "test", Memory: core.MemoryOf(int64(m.HeapAlloc) + room)}
			var err error
			var planned []engine.PlannedFile
			seen := plannersBeside(func() { planned, err = engine.Plan(append([]engine.Target(nil), targets...), opt) })
			if err != nil || len(planned) != files {
				t.Fatalf("planning %d files of %s in %d MB of room gave %d: %v", files, id, room/mib, len(planned), err)
			}
			return seen[targetFiles], planned
		}

		// The reach: with room to spare the pictures are planned beside each
		// other, so the count below can see them when they are - and the
		// picture holds what this is about where it is said to.
		seen, planned := plan(2 << 30)
		if seen < 2 {
			t.Fatalf("%s: with 2 GB of room at most %d goroutines planned beside each other - this test cannot see "+
				"pictures planned at once", id, seen)
		}
		if p := planned[1].Plan; max(p.Holds, p.PlanHolds) < 10*mib {
			t.Fatalf("%s: a picture of the case says it holds %d B writing and %d B planning, so the room below "+
				"does not hold it to one at a time", id, p.Holds, p.PlanHolds)
		}
		if seen, _ := plan(16 * mib); seen > 1 {
			t.Errorf("%s: %d goroutines planned pictures that hold about 12 MB each in 16 MB of room - the plan went "+
				"past the memory setting, which leaves room for one at a time", id, seen)
		}
	}
}

// reachedRefusal says how a case named for a refusal missed it, or nothing. A
// case that stopped refusing would compare two accepted plans and stay green
// while the walk of a refused batch went unasked.
func reachedRefusal(name string, err error) string {
	switch {
	case !strings.Contains(name, "refused"):
		return ""
	case err == nil:
		return "is not refused any more, so the walk to the size every file accepts is not compared"
	case strings.Contains(name, "unlike") && !strings.Contains(err.Error(), "files is different"):
		return "is refused without saying its files differ, so no file but the first raised the size: " + err.Error()
	}
	return ""
}

// planParts says where two plans of one run part, or nothing when they are one
// plan: the same files under the same names, each planned to the same bytes
// with the same facts - or the same refusal, word for word.
func planParts(a []engine.PlannedFile, aErr error, b []engine.PlannedFile, bErr error) string {
	if (aErr == nil) != (bErr == nil) || aErr != nil && aErr.Error() != bErr.Error() {
		return fmt.Sprintf("refused with %v against %v", aErr, bErr)
	}
	if len(a) != len(b) {
		return fmt.Sprintf("%d files against %d", len(a), len(b))
	}
	for i := range a {
		x, y := a[i], b[i]
		if x.ID != y.ID || x.Name != y.Name || x.Index != y.Index || x.Seed != y.Seed {
			return fmt.Sprintf("file %d is %s %s against %s %s", i+1, x.ID, x.Name, y.ID, y.Name)
		}
		if !reflect.DeepEqual(x.Plan, y.Plan) {
			return fmt.Sprintf("the plan of %s is\n  %+v\nagainst\n  %+v", x.Name, x.Plan, y.Plan)
		}
	}
	return ""
}

// targetFiles is the loop that plans the files of a target, the one
// TestThePicturesPlannedAtOnceHoldNoMoreThanTheMemorySettingLeaves counts.
const targetFiles = "the files of a target"

// plannerPaths are the three loops planned in stretches, each named by the
// function only a goroutine planning in it has on its stack.
var plannerPaths = map[string]string{
	targetFiles:                   "engine.(*planning).files",
	"the draw of a range":         "engine.drawOne",
	"the walk of a refused batch": "engine.batchWalk.sizeEveryFileTakes",
}

// plannersBeside runs do and answers, for each loop of plannerPaths, the most
// goroutines planning in it at one moment.
//
// A goroutine planning is one with engine.askInTurn on its stack - a
// goroutine of a stretch (beside, in parallel.go) - so a goroutine a format
// starts of its own, or one an earlier test left behind, is not counted. The
// count of every goroutine in the process was what this read until the review
// of #252, which named both ways that could be wrong. Read from the stacks
// rather than told, because the engine says nothing about how it plans, and
// being told would be a promise this test could only believe. A stretch of
// pictures plans for milliseconds, so a reading every 200 us sees it, and a
// name that stops matching after a rename reads as nothing planned at once.
func plannersBeside(do func()) map[string]int {
	most := map[string]int{}
	stop, counted := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(counted)
		buf := make([]byte, 8<<20)
		for {
			now := map[string]int{}
			for _, g := range strings.Split(string(buf[:runtime.Stack(buf, true)]), "\n\n") {
				if !strings.Contains(g, "engine.askInTurn") {
					continue
				}
				for path, frame := range plannerPaths {
					if strings.Contains(g, frame) {
						now[path]++
					}
				}
			}
			for path, n := range now {
				most[path] = max(most[path], n)
			}
			select {
			case <-stop:
				return
			case <-time.After(200 * time.Microsecond):
			}
		}
	}()
	do()
	close(stop)
	<-counted
	return most
}
