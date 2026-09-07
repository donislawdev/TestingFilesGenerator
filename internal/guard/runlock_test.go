package guard

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/audit"
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
)

// Two runs writing into one directory used to write over each other's files
// and both say they had succeeded.
//
// Measured on 2026-09-07 with two processes started on the same wall clock
// instant, eight times, sixty files of 200 kB each:
//
//	both ended 0    2 of 8   120 files reported, 60 on the disk
//	both ended 8    5 of 8   partial, and only because Windows refuses to
//	                         rename onto a file another process holds open
//	one ended 5     1 of 8   the preflight happened to see the other run
//
// In the first case verify against the first run's manifest reported sixty
// wrong hashes about a run that had been told it succeeded. The second case is
// not a defence: it is a property of one filesystem on one system, and Linux
// renames onto an open file without complaint.
//
// The protection existed and was keyed to the wrong thing. A run claims its
// manifest name for its whole length, so two runs both writing manifest.json
// into one directory have always been refused - measured the same day, four
// times out of four. output.manifest is the one way out of that claim, and it
// was never meant to be a way out of this.
//
// None of the guards below start two processes. They reproduce what two
// processes meet, which is a directory that is already held, and assert on it
// directly - a race somebody has to lose to see fail is not a guard.

// planIn is one run's worth of files, planned into dir.
//
// The count is a parameter because one guard here needs a run long enough to
// be interrupted in the middle of, and four small files is not that.
func planIn(t *testing.T, dir, manifestName string, count int) ([]engine.PlannedFile, engine.Options) {
	t.Helper()
	opt := engine.Options{OutDir: dir, ManifestName: manifestName, Seed: 4242, Command: "test"}
	planned, err := engine.Plan([]engine.Target{txtTarget("files", count, 2048)}, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}
	return planned, opt
}

// The run this refuses points output.manifest at a name of its own, and that
// is the case rather than an incidental detail. It is the one output.manifest
// was letting through, and naming a free manifest here means the older claim
// cannot be what refuses - which is asserted below rather than assumed.
//
// It was two guards until the mutation coverage said otherwise. The second
// one used a non-default manifest name and the first the default, and every
// mutation that reddens one reddens the other, because the lock never sees the
// manifest name at all. Two guards proving one thing is a coverage number that
// is larger than the coverage.
func TestASecondRunIntoADirectoryARunIsHoldingIsRefusedBeforeItWritesAnything(t *testing.T) {
	dir := t.TempDir()
	lock := engine.RunLockPath(dir)
	if err := os.WriteFile(lock, nil, 0o644); err != nil {
		t.Fatalf("standing in for the run that is already here: %v", err)
	}

	const ownName = "manifest-beta.json"
	if _, err := os.Stat(filepath.Join(dir, ownName)); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("%s is not free, so this guard could pass on the manifest claim alone and say nothing about the directory", ownName)
	}

	planned, opt := planIn(t, dir, ownName, 4)
	res, err := engine.Run(context.Background(), planned, opt)

	var inProgress *engine.RunInProgressError
	if !errors.As(err, &inProgress) {
		t.Fatalf("a run started into a directory another run is holding, and got %v.\n"+
			"Two runs writing into one directory write over each other's files, and both of them say they succeeded", err)
	}
	if res.Started {
		t.Error("the refused run says it started, so its caller will go on to write a manifest for files that do not exist")
	}

	// Refused BEFORE anything was written, which is the half that makes the
	// refusal worth having. A run that stops after eight files has already
	// taken eight names off somebody.
	if got := namesIn(t, dir); len(got) != 1 || got[0] != core.RunLockName {
		t.Errorf("the refused run left %v in the directory - it has to refuse before it writes anything", got)
	}

	// The remedy is in the sentence, because a run killed outright cannot give
	// the name back and "another run is writing here" on a machine where
	// nothing runs is a dead end.
	if !strings.Contains(inProgress.Error(), core.RunLockName) {
		t.Errorf("the refusal does not name the file to remove:\n  %s", inProgress.Error())
	}
}

// Held while the run writes, and given back when it ends.
//
// The first half is asked from inside the run rather than around it, because
// "the lock is gone afterwards" is also true of a lock that was never taken.
func TestTheDirectoryIsHeldWhileTheRunWritesAndGivenBackWhenItEnds(t *testing.T) {
	dir := t.TempDir()
	planned, opt := planIn(t, dir, "manifest.json", 4)

	held := false
	asked := false
	opt.OnProgress = func(engine.Progress) {
		asked = true
		if _, err := os.Stat(engine.RunLockPath(dir)); err == nil {
			held = true
		}
	}

	if _, err := engine.Run(context.Background(), planned, opt); err != nil {
		t.Fatalf("running: %v", err)
	}
	if !asked {
		t.Fatal("the run never reported progress, so nothing looked while it was writing and this guard checked nothing")
	}
	if !held {
		t.Error("the directory was not held while the run was writing, so a second run starting in the middle of this one would be let in")
	}
	if _, err := os.Stat(engine.RunLockPath(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Error("the finished run kept the directory, so nothing can ever generate into it again without a person deleting a file")
	}
}

// A run stopped part way gives the directory back too.
//
// It is the ending this cannot afford to get wrong: Ctrl+C is ordinary, and a
// tool that locked a directory every time somebody changed their mind would be
// worse than the fault it fixes.
//
// Stopped from INSIDE the run, and the first version of this was stopped
// before it. Measured 2026-09-07: with the context already cancelled the
// preflight refuses, the run never reaches the claim, and "the lock is gone"
// is true because it was never taken - so removing the release left this guard
// green. The mutation is what said so. It now cancels on the first report of
// progress, and asserts the lock was HELD at that moment rather than assuming
// it.
func TestAStoppedRunGivesTheDirectoryBack(t *testing.T) {
	dir := t.TempDir()
	planned, opt := planIn(t, dir, "manifest.json", 400)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	held := false
	opt.OnProgress = func(engine.Progress) {
		if _, err := os.Stat(engine.RunLockPath(dir)); err == nil {
			held = true
		}
		cancel()
	}

	_, err := engine.Run(ctx, planned, opt)
	if !held {
		t.Fatal("the run never held the directory while it was writing, so this guard is not looking at a stopped run that had it")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("the run was cancelled from inside itself and ended with %v - this guard needs a run long enough to be stopped in the middle of", err)
	}
	if _, err := os.Stat(engine.RunLockPath(dir)); !errors.Is(err, os.ErrNotExist) {
		t.Error("a stopped run kept the directory - every interrupted run would leave one that a person has to delete by hand")
	}
}

// verify names it as ours.
//
// Extra means somebody else put it here, and about our own mark that sends a
// person looking for whoever polluted their directory. It is the same repair
// the two older markers already had, arriving a third time.
func TestVerifyNamesTheRunLockAsOursRatherThanAsSomebodyElses(t *testing.T) {
	dir := t.TempDir()
	planned, opt := planIn(t, dir, "manifest.json", 4)
	res, err := engine.Run(context.Background(), planned, opt)
	if err != nil {
		t.Fatalf("running: %v", err)
	}
	if err := os.WriteFile(engine.RunLockPath(dir), nil, 0o644); err != nil {
		t.Fatalf("standing in for a run that was killed: %v", err)
	}

	diffs, err := audit.Verify(context.Background(), dir, res.Manifest, "manifest.json")
	if err != nil {
		t.Fatalf("verifying: %v", err)
	}
	if len(diffs) != 1 {
		t.Fatalf("expected the lock and nothing else, got %v", diffs)
	}
	if diffs[0].Kind != audit.Leftover {
		t.Errorf("verify called our own run lock %q - that word means somebody else put it here", diffs[0].Kind)
	}
	// The sentence has to hold both endings open. A live run is holding one of
	// these too, and telling somebody to delete it would let a second run in
	// behind the first.
	said := diffs[0].String()
	for _, phrase := range []string{"If a run is going on", "If none is"} {
		if !strings.Contains(said, phrase) {
			t.Errorf("the sentence about the lock does not say what to do when a run IS going on:\n  %s", said)
		}
	}
}

// The claim is empty, and stays empty.
//
// The manifest claim carries the same rule for a reason written where it is
// made: a claim with something in it is a claim somebody's reader will take
// for a document. Nothing reads the lock, so there is nothing in it that could
// be read half written - and that is a decision worth pinning rather than
// rediscovering when somebody wants to put a process id in it.
func TestTheRunLockCarriesNothing(t *testing.T) {
	dir := t.TempDir()
	planned, opt := planIn(t, dir, "manifest.json", 4)

	var size int64 = -1
	opt.OnProgress = func(engine.Progress) {
		if info, err := os.Stat(engine.RunLockPath(dir)); err == nil {
			size = info.Size()
		}
	}
	if _, err := engine.Run(context.Background(), planned, opt); err != nil {
		t.Fatalf("running: %v", err)
	}
	if size < 0 {
		t.Fatal("the lock was never looked at while the run was going, so this guard checked nothing")
	}
	if size != 0 {
		t.Errorf("the run lock carries %d B - it is a claim on a name, and anything in it is something a reader could find half written", size)
	}
}

// A dry run does not take the directory, and it does not lie about one that is
// taken.
//
// Both halves are the same decision seen from two sides. A preview writes
// nothing, so it has no business holding a name - and a preview that says a
// run would succeed, while another run is filling the directory it would write
// into, is answering a question nobody asked.
func TestADryRunNeitherTakesTheDirectoryNorIgnoresIt(t *testing.T) {
	dir := t.TempDir()
	planned, opt := planIn(t, dir, "manifest.json", 4)
	opt.DryRun = true

	if _, err := engine.Run(context.Background(), planned, opt); err != nil {
		t.Fatalf("a dry run into an empty directory: %v", err)
	}
	if got := namesIn(t, dir); len(got) != 0 {
		t.Errorf("a dry run left %v behind - it is supposed to write nothing at all", got)
	}

	if err := os.WriteFile(engine.RunLockPath(dir), nil, 0o644); err != nil {
		t.Fatalf("standing in for the run that is already here: %v", err)
	}
	var inProgress *engine.RunInProgressError
	if _, err := engine.Run(context.Background(), planned, opt); !errors.As(err, &inProgress) {
		t.Errorf("a dry run into a directory another run is holding reported %v.\n"+
			"The preview is the step this project tells people to take before anything large, so it has to give the answer the run would give", err)
	}
}

// The lock is spelled in one place.
//
// The same rule the other two markers carry, and for the same measured reason:
// a second spelling means the writing side and the reading side stop agreeing,
// and verify starts calling our own file somebody else's.
func TestTheRunLockIsSpelledInOnePlace(t *testing.T) {
	dir := t.TempDir()
	if engine.RunLockPath(dir) != filepath.Join(dir, core.RunLockName) {
		t.Error("the engine builds the lock path from something other than core.RunLockName, so the two can drift apart")
	}
	if !core.IsRunLockName(core.RunLockName) {
		t.Error("the reader does not recognise the name the writer uses")
	}
	if core.IsRunLockName(core.RunLockName + ".txt") {
		t.Error("the reader recognises a name that only starts with the lock, so a file somebody else left would be called ours")
	}
	// Not a manifest name, or a run would refuse itself.
	if core.RunLockName == engine.DefaultManifestName {
		t.Error("the lock and the default manifest are the same name, so no run could ever start")
	}
}
