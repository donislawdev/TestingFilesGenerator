package guard

import (
	"fmt"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"testing"

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

// besideReach is the case whose plan is watched for files planned at once:
// pictures, which start no goroutines of their own.
const besideReach = "png pictures"

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
			Range:      engine.SizeRange{Used: true, Min: 2 << 10, Max: 96 << 10},
			Properties: map[string]string{"width": "64", "height": "64"}}},
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
	// comparison that saw nothing would pass every case below.
	runtime.GOMAXPROCS(1)
	one, _ := engine.Plan(besideCases()["png pictures"], opt)
	other, _ := engine.Plan(besideCases()["png pictures"], engine.Options{OutDir: opt.OutDir, Seed: 8, Command: "test"})
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

	most := 0
	for _, name := range names {
		var planned [2][]engine.PlannedFile
		var errs [2]error
		for i, width := range planWidths {
			runtime.GOMAXPROCS(width)
			targets := append([]engine.Target(nil), cases[name]...)
			seen := goroutinesBeside(func() { planned[i], errs[i] = engine.Plan(targets, opt) })
			// Counted on pictures alone. A format may start goroutines of
			// its own while it plans, and those would be counted as the
			// plan's - measured so on 2026-11-18, when a plan of one file
			// at a time still showed several beside it across every case.
			if width > 1 && name == besideReach {
				most = seen
			}
		}
		if why := planParts(planned[0], errs[0], planned[1], errs[1]); why != "" {
			t.Errorf("%s planned at %d processors and at %d is not the same plan: %s", name, planWidths[0], planWidths[1], why)
		}
		if reached := reachedRefusal(name, errs[1]); reached != "" {
			t.Errorf("%s: %s", name, reached)
		}
	}

	// The state this is about, asserted rather than assumed: at eight
	// processors the files were planned beside each other at all.
	if most < 2 {
		t.Errorf("at most %d goroutines ran beside the plan of %s at %d processors - nothing was planned at once, "+
			"so the comparison above compared one way of planning with itself", most, besideReach, planWidths[1])
	}
}

func TestThePicturesPlannedAtOnceHoldNoMoreThanTheMemorySettingLeaves(t *testing.T) {
	was := runtime.GOMAXPROCS(8)
	t.Cleanup(func() { runtime.GOMAXPROCS(was) })
	const mib = int64(1) << 20
	// 1500 by 1500, which png says holds 11.25 MB while it is drawn (Holds), in
	// 16 MB of room - room for one with megabytes to spare, since the heap
	// this test reads is not the heap the engine reads a moment later.
	//
	// After a target of text, so the pictures are not the first files of the
	// run: the first file of every target is planned alone because nothing
	// yet says what its files hold, and at the first file of a run the
	// ceiling of the plan would hold it alone anyway. One file of text, since
	// text holds nothing and more of it would be planned beside itself and
	// counted here (measured so on 2026-11-18, three files showed two).
	targets := []engine.Target{
		{ID: "notes", Format: "txt", Sizes: engine.Uniform(1, 4096)},
		{ID: "big", Format: "png", Sizes: engine.Uniform(6, 24*mib), Properties: map[string]string{"width": "1500", "height": "1500"}},
	}
	files := len(targets[0].Sizes) + len(targets[1].Sizes)

	plan := func(room int64) int {
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		opt := engine.Options{OutDir: t.TempDir(), Seed: 7, Command: "test", Memory: core.MemoryOf(int64(m.HeapAlloc) + room)}
		var err error
		var planned []engine.PlannedFile
		seen := goroutinesBeside(func() { planned, err = engine.Plan(append([]engine.Target(nil), targets...), opt) })
		if err != nil || len(planned) != files {
			t.Fatalf("planning %d files in %d MB of room gave %d: %v", files, room/mib, len(planned), err)
		}
		return seen
	}

	// The reach: with room to spare the pictures are planned beside each
	// other, so the count below can see them when they are.
	if seen := plan(2 << 30); seen < 2 {
		t.Fatalf("with 2 GB of room at most %d goroutines planned beside each other - this test cannot see "+
			"pictures planned at once", seen)
	}
	if seen := plan(16 * mib); seen > 1 {
		t.Errorf("%d goroutines planned pictures that hold 11.25 MB each in 16 MB of room - the plan went past the "+
			"memory setting, which leaves room for one at a time", seen)
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

// goroutinesBeside runs do and answers the most goroutines there were beside
// it at any moment - not the ones there before it started, nor the one
// counting.
//
// Counted from outside rather than told, because the engine says nothing
// about how it plans, and being told would be a promise this test could only
// believe.
func goroutinesBeside(do func()) int {
	before := runtime.NumGoroutine()
	var most atomic.Int64
	stop, counted := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(counted)
		for {
			most.Store(max(most.Load(), int64(runtime.NumGoroutine())))
			select {
			case <-stop:
				return
			default:
				runtime.Gosched()
			}
		}
	}()
	do()
	close(stop)
	<-counted
	return int(most.Load()) - before - 1
}
