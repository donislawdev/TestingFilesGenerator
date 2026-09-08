package guard

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// This tool writes large amounts of data and it runs in directories that
// belong to the user. These guards cover the three ways it could damage
// something rather than merely fail.

func TestNothingIsEverWrittenOverInSilence(t *testing.T) {
	dir := t.TempDir()

	// Something of the user's, sitting where a generated file would land.
	victim := filepath.Join(dir, "files_0001.txt")
	const precious = "work that took an afternoon"
	if err := os.WriteFile(victim, []byte(precious), 0o644); err != nil {
		t.Fatalf("preparing the file: %v", err)
	}

	opt := engine.Options{OutDir: dir, Seed: 7741, Command: "test"}
	planned, err := engine.Plan([]engine.Target{txtTarget("files", 1, 4096)}, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}

	_, runErr := engine.Run(context.Background(), planned, opt)
	if runErr == nil {
		t.Fatal("the run went ahead over an existing file")
	}
	var collision *engine.CollisionError
	if !errors.As(runErr, &collision) {
		t.Errorf("refused with %T, expected a CollisionError so the caller can answer with the right exit code", runErr)
	}

	after, err := os.ReadFile(victim)
	if err != nil {
		t.Fatalf("reading the file back: %v", err)
	}
	if string(after) != precious {
		t.Error("the existing file was destroyed - this is the one failure that cannot be undone by running again")
	}
}

func TestTwoFilesCannotHeadForOneName(t *testing.T) {
	dir := t.TempDir()
	opt := engine.Options{OutDir: dir, Seed: 7741, Command: "test"}

	// A name template with no index means every file of the target wants the
	// same name. Without this check one file survives, the manifest describes
	// three, and the suite reads a manifest that quietly lost two of them.
	targets := []engine.Target{{
		ID: "files", Format: "txt", Sizes: engine.Uniform(3, 512),
		NameTmpl: "same.txt", Label: true,
	}}

	_, err := engine.Plan(targets, opt)
	if err == nil {
		t.Fatal("planning accepted three files heading for one name")
	}
	var recipeErr *engine.RecipeError
	if !errors.As(err, &recipeErr) {
		t.Errorf("refused with %T, expected a RecipeError", err)
	}
	if !strings.Contains(err.Error(), "same.txt") {
		t.Errorf("the message does not name the clashing file: %v", err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("planning left %d entries on disk - it must never touch it", len(entries))
	}
}

func TestARunTooBigForTheDiskIsRefusedBeforeTheFirstByte(t *testing.T) {
	dir := t.TempDir()

	// The free space probe is injected rather than the run being made huge.
	// Asking for a petabyte would prove the same thing, but when the guard is
	// broken that test writes until the disk fills - on a CI runner and on
	// the machine of whoever runs it. Measured: that mutation took 50 seconds
	// before this was changed.
	opt := engine.Options{
		OutDir: dir, Seed: 7741, Command: "test",
		AvailableBytes: func(string) (int64, error) { return 1000, nil },
	}

	planned, err := engine.Plan([]engine.Target{txtTarget("bigger-than-the-disk", 4, 8192)}, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}

	_, runErr := engine.Run(context.Background(), planned, opt)
	if runErr == nil {
		t.Fatal("a run needing a petabyte was allowed to start")
	}
	var spaceErr *engine.SpaceError
	if !errors.As(runErr, &spaceErr) {
		t.Fatalf("refused with %T, expected a SpaceError", runErr)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("a refused run left %d entries behind, expected none", len(entries))
	}
}

func TestAnInterruptedRunLeavesNoPartialFileAndStillWritesAManifest(t *testing.T) {
	dir := t.TempDir()
	opt := engine.Options{OutDir: dir, Seed: 7741, Command: "test"}

	planned, err := engine.Plan([]engine.Target{txtTarget("files", 40, 8192)}, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}

	// Cancel immediately. This stands in for Ctrl+C, a kill and a CI timeout,
	// and it costs no disk to test.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res, runErr := engine.Run(ctx, planned, opt)
	if runErr == nil {
		t.Fatal("a cancelled run reported success")
	}
	if !errors.Is(runErr, context.Canceled) {
		t.Errorf("a cancelled run ended with %v, expected a cancellation", runErr)
	}

	// The invariant: the output directory never holds an incomplete file.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tfg-partial") {
			t.Errorf("%s was left behind - a half written file that looks finished reaches a test suite as a false truth", e.Name())
		}
	}

	// A manifest exists even so, otherwise cleanup has nothing to work with
	// and the leftovers stay for good.
	if res.Manifest == nil {
		t.Fatal("a cancelled run produced no manifest")
	}
	if res.Manifest.Run.Complete {
		t.Error("the manifest of a cancelled run claims the run finished")
	}
}

// A run stopped part way names EVERY file that finished, and names nothing
// else.
//
// This is the one thing a person can observe that changed when the files
// started being written beside each other, and it is a decision of the owner's
// from 2026-09-06 rather than a consequence nobody chose. The sequential loop
// could only ever leave a contiguous prefix: file five was finished before
// file six was begun. With several writers, one can be cut off half way while
// its neighbour has already been renamed into place, so what is left on the
// disk may have a hole in it.
//
// The alternative - recording the prefix and leaving the rest - is worse in a
// way untouchable rule 7 names exactly. cleanup deletes what the manifest
// lists and nothing else, so a finished file with no entry is a file NOTHING
// in this tool can ever remove, and verify reports it for good. The same
// reasoning is why the pool here is not the one in internal/audit, which wants
// the opposite: a sentence about an interrupted verify is a sentence about a
// prefix.
//
// So the property is a two way one, and both halves matter. Every entry that
// claims a file has one, and every file on the disk has an entry.
//
// The first version of this guard PASSED WITHOUT EVER REACHING A HOLE, and
// the mutation runner is what said so. It planned four hundred files of four
// kilobytes, which every writer finishes in one pass - so no writer was ever
// cut off half way, what the run left behind was a prefix after all, and the
// mutation that puts the prefix behaviour back could not redden anything.
// A guard that names the defect and cannot meet it is the shape this project
// has recorded twice.
//
// The second version bought the hole with SIZE: the first file was sixteen
// times the size of the rest, so a smaller one was expected to always finish
// first. That version was flaky, roughly one run in four inside a full suite,
// and the fix is not a bigger first file. Measured on 2026-09-08 with
// tools/probes/stoprace, fifty runs per condition:
//
//	idle machine                 0 failures in 50, and the big file had 165 ms
//	                             of margin it never came close to using
//	heavy disk writing beside it  0 failures in 25
//	CPU starved, 24 busy loops    14 failures in 25
//
// So the flakiness was never about how fast the disk is. Size buys margin in
// WORK, and what decides which writer finishes first is which writer gets a
// processor. Starve the machine and the time to the first finished file goes
// from 11 ms to 5.8 s, three orders of magnitude, at which point sixteen times
// the work means nothing at all. A full suite runs packages beside each other
// and builds binaries in sub processes, so a starved machine is the normal
// condition rather than the exotic one.
//
// This version does not race. The file at index zero is given a generator that
// writes nothing and returns only once the run is cancelled, so it can never
// finish no matter who gets a processor. Descriptor is a value and every
// planned file carries its own copy, so this replaces the generator for that
// one file and leaves the other thirty one writing real bytes through the real
// registry. Every step of the engine below Plan is the one that ships.
func TestARunStoppedPartWayNamesEveryFileThatFinished(t *testing.T) {
	dir := t.TempDir()

	// Raised so several writers exist wherever this runs, because with one
	// writer a stopped run leaves a prefix and the hole this guard is about
	// cannot occur. It is also what keeps the blocked file below from being a
	// deadlock: somebody other than the blocked writer has to finish a file,
	// or the cancellation this guard waits for is never sent.
	//
	// Asserted rather than assumed. The pool is min(GOMAXPROCS, files), so a
	// build that ever answered one here would hang instead of failing, and
	// this guard has already been bitten once by a condition it took for
	// granted.
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(8))
	if n := runtime.GOMAXPROCS(0); n < 2 {
		t.Fatalf("this guard needs at least two writers and GOMAXPROCS is %d, so the "+
			"file held open below would wait for a cancellation nobody can send", n)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancelled from inside the run, the moment the first file is finished.
	// A timer would make this a guard about how fast the machine is.
	var once sync.Once
	opt := engine.Options{
		OutDir: dir, Seed: 7741, Command: "test",
		ManifestName: engine.DefaultManifestName,
		OnProgress: func(p engine.Progress) {
			if p.FilesDone >= 1 {
				once.Do(cancel)
			}
		},
	}
	// Thirty two files of one size. The asymmetry that used to live here was
	// in the sizes and it is now in the generator, which is the whole of the
	// fix - see the note above the function.
	planned, err := engine.Plan([]engine.Target{{
		ID: "files", Format: "txt", Sizes: engine.Uniform(32, 512<<10),
	}}, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}

	// Index zero is held open for the length of the run. Its writer reaches
	// the generator, writes nothing and waits, so a file at a HIGHER index is
	// renamed into place while this one is still owed - which is the only
	// shape a sequential loop could not produce, and the shape this guard
	// exists to be about.
	held := &heldOpenGenerator{}
	planned[0].Desc.Generator = held

	res, runErr := engine.Run(ctx, planned, opt)
	if runErr == nil {
		t.Fatal("the run was cancelled from inside itself and reported success")
	}

	claimed := map[string]bool{}
	for _, f := range res.Manifest.Files {
		if f.Materialized {
			claimed[f.Name] = true
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	onDisk := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if name == engine.DefaultManifestName {
			// The run takes this name before the first file and it holds no
			// entry of its own.
			continue
		}
		if strings.Contains(name, core.PartialMarker) {
			t.Errorf("%s was left behind - a half written file that looks finished "+
				"reaches a test suite as a false truth", name)
			continue
		}
		onDisk[name] = true
	}

	// Asserted rather than assumed, and these three are what decide whether
	// the comparison below means anything. Two empty sets agree with each
	// other, a run that finished everything was never stopped, and a run whose
	// survivors happen to be a prefix is the case this guard exists to be
	// different from (O118).
	if len(onDisk) == 0 {
		t.Fatal("the run was stopped before it finished a single file, so there is no " +
			"finished file to ask about")
	}
	if len(onDisk) >= len(planned) {
		t.Fatalf("the run wrote all %d files before the cancellation reached it, so it "+
			"was never stopped part way", len(planned))
	}
	if onDisk[planned[0].Name] {
		t.Fatalf("%s was held open for the whole run and it is on the disk anyway, so "+
			"the survivors are a prefix and nothing here was cut off half way - which "+
			"is not the case this guard is about", planned[0].Name)
	}
	// The fourth of these, and the one that says WHICH ROAD the hole came by.
	// Without it a run where index zero was never begun reads exactly like a
	// run where it was cut off half way: same hole, same manifest, weaker
	// proof. See heldOpenGenerator.Started for the measurement.
	if !held.Started() {
		t.Fatalf("%s never reached its generator, so it was never begun rather than cut "+
			"off half way. The hole is there and this guard is not the one that proves "+
			"it - drain asks about cancellation after taking an index, and this writer "+
			"lost the processor in between", planned[0].Name)
	}

	for name := range onDisk {
		if !claimed[name] {
			t.Errorf("%s is on the disk and the manifest does not name it. cleanup deletes "+
				"what the manifest lists and nothing else, so this file is one no command "+
				"of this tool can remove", name)
		}
	}
	for name := range claimed {
		if !onDisk[name] {
			t.Errorf("the manifest names %s as written and it is not there, so verify "+
				"reports a file that never existed", name)
		}
	}
}

func TestAFreshRunIntoAnEmptyDirectoryStillWorks(t *testing.T) {
	// The guards above refuse things. This one exists so that refusing
	// everything would not pass as success.
	dir := t.TempDir()
	opt := engine.Options{OutDir: filepath.Join(dir, "nested", "out"), Seed: 7741, Command: "test"}

	planned, err := engine.Plan([]engine.Target{txtTarget("files", 3, 4096)}, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}
	res, err := engine.Run(context.Background(), planned, opt)
	if err != nil {
		t.Fatalf("a normal run into an empty directory failed: %v", err)
	}
	if res.Manifest.Summary.Materialized != 3 {
		t.Errorf("produced %d files, expected 3", res.Manifest.Summary.Materialized)
	}
}

// A name is taken by whatever holds it, including a link that points nowhere.
//
// Until 2026-09-05 preflight asked os.Stat whether the path existed, and
// os.Stat follows a link - so a link pointing at nothing answered "no name
// here" and the run replaced the entry without a word. It reads one listing of
// the directory now, and a directory ENTRY is what a taken name is, whatever
// it points at.
//
// That difference is why the change is not only about speed. Replacing a link
// somebody put there loses their work exactly as replacing a file does, and it
// happened on the quiet path rather than the loud one.
func TestANameTakenByALinkPointingNowhereIsStillTaken(t *testing.T) {
	dir := t.TempDir()
	dangling := filepath.Join(dir, "files_0001.txt")
	if err := os.Symlink(filepath.Join(dir, "nothing-is-here"), dangling); err != nil {
		t.Skipf("this system will not create a link here, so the case cannot be built: %v", err)
	}

	// Asserted rather than assumed, because the whole case is a name that IS
	// there and that os.Stat cannot see. A fixture that quietly resolved would
	// leave this guard green about the old behaviour (O118).
	if _, err := os.Stat(dangling); err == nil {
		t.Fatal("the link resolves, so this is not the state being guarded")
	}
	if _, err := os.Lstat(dangling); err != nil {
		t.Fatalf("there is no entry at all, so there is nothing for a run to collide with: %v", err)
	}

	opt := engine.Options{OutDir: dir, Seed: 7741, Command: "test"}
	planned, err := engine.Plan([]engine.Target{txtTarget("files", 1, 4096)}, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}

	_, runErr := engine.Run(context.Background(), planned, opt)
	if runErr == nil {
		t.Fatal("the run went ahead over a name somebody else's link was holding")
	}
	var collision *engine.CollisionError
	if !errors.As(runErr, &collision) {
		t.Errorf("refused with %T, expected a CollisionError so the caller answers with the right exit code", runErr)
	}
}

// A directory that cannot be LISTED is still asked about every name, one at a
// time.
//
// The listing that made preflight cheap has an answer it cannot give: a
// directory with permission to write and none to read. Both systems allow that
// combination, and a run into one has always worked. Reading nothing there and
// calling it empty would let the run write over whatever is inside - the one
// failure here that running again cannot undo - so a listing that fails means
// "ask file by file" rather than "there is nothing there".
//
// Without this the fallback is a branch nothing can turn red, and this project
// has removed seven of those.
func TestADirectoryThatCannotBeListedIsStillAskedAboutEveryName(t *testing.T) {
	if runtime.GOOS == "windows" {
		// The same reason environment_test.go gives for the sibling of this
		// guard: os.Chmod on Windows moves the read only bit and nothing else,
		// and denying a listing needs an ACL, which is not something a test
		// should be installing.
		t.Skip("a directory that refuses a listing needs an ACL on Windows")
	}

	dir := t.TempDir()
	out := filepath.Join(dir, "writeonly")
	if err := os.Mkdir(out, 0o755); err != nil {
		t.Fatalf("making the directory: %v", err)
	}
	const precious = "work that took an afternoon"
	victim := filepath.Join(out, "files_0001.txt")
	if err := os.WriteFile(victim, []byte(precious), 0o644); err != nil {
		t.Fatalf("preparing the file: %v", err)
	}
	if err := os.Chmod(out, 0o300); err != nil {
		t.Fatalf("taking the read permission away: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(out, 0o755) })

	// Asserted rather than assumed. Root ignores the permission bits, so on a
	// container running as root this state does not exist and the guard would
	// otherwise pass while testing the fast path twice.
	if _, err := os.ReadDir(out); err == nil {
		t.Skip("this process can list a directory with no read permission, so the fallback cannot be reached")
	}

	opt := engine.Options{OutDir: out, Seed: 7741, Command: "test"}
	planned, err := engine.Plan([]engine.Target{txtTarget("files", 1, 4096)}, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}

	_, runErr := engine.Run(context.Background(), planned, opt)
	if runErr == nil {
		t.Fatal("a run into a directory it could not list went ahead over what was inside")
	}
	var collision *engine.CollisionError
	if !errors.As(runErr, &collision) {
		t.Errorf("refused with %T, expected a CollisionError", runErr)
	}

	if err := os.Chmod(out, 0o755); err != nil {
		t.Fatalf("putting the permission back to read the file: %v", err)
	}
	after, err := os.ReadFile(victim)
	if err != nil {
		t.Fatalf("reading the file back: %v", err)
	}
	if string(after) != precious {
		t.Error("the existing file was destroyed - this is the one failure that cannot be undone by running again")
	}
}
