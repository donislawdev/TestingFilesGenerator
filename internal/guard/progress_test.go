package guard

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/engine"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// A run of ten thousand files takes about eighteen seconds and a large one
// takes minutes, and until 2026-08-03 every second of it was silent. Past ten
// seconds a person cannot tell a working tool from a hung one, which is how
// this was reported: not as a missing feature but as "the terminal is stuck
// and I do not know whether to wait".

// The report has to arrive while a single file is still being written, not
// only when one finishes. Counting finished files alone would leave one 5 GB
// file reporting exactly once, at the end - the case where the silence is
// worst and the one a per file counter looks like it solves.
func TestProgressArrivesWhileOneFileIsStillBeingWritten(t *testing.T) {
	dir := t.TempDir()

	var reports []engine.Progress
	opt := engine.Options{
		OutDir: dir, Seed: 7741, Command: "test",
		ManifestName: engine.DefaultManifestName,
		OnProgress:   func(p engine.Progress) { reports = append(reports, p) },
	}
	// One file, big enough that the generator writes it in several goes.
	planned, err := engine.Plan([]engine.Target{txtTarget("files", 1, 4<<20)}, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}
	if _, err := engine.Run(context.Background(), planned, opt); err != nil {
		t.Fatalf("the run failed: %v", err)
	}

	if len(reports) < 3 {
		t.Fatalf("one 4 MiB file produced %d reports - a run that only counts "+
			"finished files leaves a big file silent until it is done", len(reports))
	}

	last := reports[len(reports)-1]
	if last.FilesDone != last.FilesTotal || last.BytesDone != last.BytesTotal {
		t.Errorf("the last report said %d/%d files and %d/%d bytes, so the bar "+
			"never reaches the end it promised", last.FilesDone, last.FilesTotal,
			last.BytesDone, last.BytesTotal)
	}

	// Going backwards would read as the run undoing itself.
	var prev int64
	for i, p := range reports {
		if p.BytesDone < prev {
			t.Fatalf("report %d went backwards, from %d B to %d B", i, prev, p.BytesDone)
		}
		prev = p.BytesDone
	}
}

// Across several files the counts have to add up rather than restart, or the
// percentage resets at every file.
func TestProgressCountsTheWholeRunNotOneFile(t *testing.T) {
	dir := t.TempDir()

	var reports []engine.Progress
	opt := engine.Options{
		OutDir: dir, Seed: 7741, Command: "test",
		ManifestName: engine.DefaultManifestName,
		OnProgress:   func(p engine.Progress) { reports = append(reports, p) },
	}
	planned, err := engine.Plan([]engine.Target{txtTarget("files", 4, 64<<10)}, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}
	if _, err := engine.Run(context.Background(), planned, opt); err != nil {
		t.Fatalf("the run failed: %v", err)
	}

	last := reports[len(reports)-1]
	if last.FilesTotal != 4 || last.BytesTotal != 4*(64<<10) {
		t.Errorf("the run reported a total of %d files and %d B, expected 4 and %d",
			last.FilesTotal, last.BytesTotal, 4*(64<<10))
	}
	if last.FilesDone != 4 {
		t.Errorf("the run finished at %d of 4 files", last.FilesDone)
	}

	// The running total carries across files rather than restarting inside
	// each one. Checking only the final report would miss that entirely, since
	// the report after a finished file is right either way - it is the ones
	// during the next file that would fall back to near zero and make the
	// percentage jump backwards on every file.
	var prev int64
	for i, p := range reports {
		if p.BytesDone < prev {
			t.Fatalf("report %d fell from %d B back to %d B, so the count restarts "+
				"inside each file instead of describing the run", i, prev, p.BytesDone)
		}
		prev = p.BytesDone
	}
}

// The bar never reaches a log file or a pipe. docs/CLI.md puts it on the error
// channel so the data channel stays usable, and that is worth nothing if the
// bar itself fills a CI log with thousands of redrawn lines.
func TestProgressStaysOffWhenNothingIsWatching(t *testing.T) {
	dir := t.TempDir()

	// A real file on disk, which is what a CI job gets when it redirects the
	// error channel into its log. A buffer would not do here: it is not an
	// operating system file at all, so it never reaches the question of
	// whether the destination is a terminal - and a test that stops earlier
	// than the code it guards proves the wrong half.
	logPath := filepath.Join(dir, "ci.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatalf("creating the log: %v", err)
	}

	// Enough bytes that the run outlasts the interval between redraws. With a
	// smaller run the bar would stay silent because there was no time to draw
	// it, and the guard would pass without ever reaching the question it asks.
	// That is not hypothetical - it is what the first version of this test did,
	// and the mutation runner is what said so.
	var out bytes.Buffer
	code := cli.Run(context.Background(), []string{
		"generate", "--format", "txt", "--size", "8mb", "--count", "8",
		"--out", filepath.Join(dir, "files"),
	}, &out, logFile)
	logFile.Close()
	if code != 0 {
		t.Fatalf("the run ended with %d", code)
	}

	body, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading the log: %v", err)
	}
	// A carriage return is how the line is redrawn in place, so its absence is
	// what says the bar never started.
	if strings.Contains(string(body), "\r") {
		t.Errorf("a progress bar reached a log file rather than a terminal, which is "+
			"thousands of redrawn lines in somebody's CI output: %q", string(body))
	}
}

// No two reports are ever in flight at once, which is what the window's rate
// limiter is built on.
//
// throttle in internal/gui/window keeps a time.Time and reads and writes it
// without a mutex, and the command line bar moves last and printed the same
// way. Both are correct for one reason: Options.OnProgress says what a caller
// may assume. An outside review read the window on its own, saw shared state
// with no lock, called it a defect, then withdrew it on finding the contract -
// and pointed out that nothing pinned the contract down. So this is the pin.
//
// The contract this pins is not the one it used to pin, and the change is
// the reason this comment is long. Until 2026-09-06 it read "called from the
// same goroutine doing the work", and this guard asked exactly that: one
// goroutine, counted from its stack. That promise died with the sequential
// write loop, and this guard is what said so - it went red on the first build
// that wrote files in parallel, naming six goroutines, which is what it was
// written to do. What replaces it is the weaker promise that costs both
// callers nothing: NEVER TWO AT ONCE. A lock gives happens-before just as a
// single goroutine does, so neither caller needed a line of change.
//
// Two things are asked, and the second is why this is not simply weaker than
// what it replaced:
//
//   - no two callbacks overlap. A run that dropped the lock fails here.
//   - MORE THAN ONE goroutine reported. Without that, a build that quietly
//     went back to writing one file at a time would satisfy the first
//     question by never having anything to serialise, and this guard would be
//     green about a question it had stopped asking - which is the shape O118
//     names. GOMAXPROCS is raised for the duration so the question is the same
//     on a one core runner as it is here.
//
// The goroutine is identified from its stack because Go does not offer the
// number any other way. That is a thing to do in a test and nowhere else.
func TestNoTwoProgressReportsArriveAtOnce(t *testing.T) {
	dir := t.TempDir()

	// Raised so several writers exist wherever this runs. widthFor asks
	// GOMAXPROCS, so a runner with one hardware thread would otherwise write
	// one file at a time and leave the overlap question unasked. Restored
	// afterwards - nothing in this package runs in parallel with it.
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(8))

	// inside counts callbacks in flight. Anything but one on the way in means
	// two were running together, which is the defect.
	var inside atomic.Int64
	var overlapped atomic.Bool

	// Under a lock, because a guard that has to survive the very thing it
	// looks for cannot share a bare map with it. Without this, a run that did
	// report from several goroutines would end the test binary with "concurrent
	// map writes" instead of saying how many there were.
	var mu sync.Mutex
	seen := map[string]bool{}

	opt := engine.Options{
		OutDir: dir, Seed: 4457, Command: "test",
		ManifestName: engine.DefaultManifestName,
		OnProgress: func(engine.Progress) {
			if inside.Add(1) != 1 {
				overlapped.Store(true)
			}
			mu.Lock()
			seen[goroutineName()] = true
			mu.Unlock()
			// Widens the window a broken build would have to hit. Without it
			// two callbacks that are genuinely concurrent can still take turns
			// by luck, and a guard that needs luck to fail is one that passes
			// on the runner and fails on somebody's machine.
			runtime.Gosched()
			inside.Add(-1)
		},
	}
	// Several files, and each big enough to report from inside itself, so the
	// callback is reached both between files and during one.
	planned, err := engine.Plan([]engine.Target{txtTarget("files", 32, 2<<20)}, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}
	if _, err := engine.Run(context.Background(), planned, opt); err != nil {
		t.Fatalf("the run failed: %v", err)
	}

	if len(seen) == 0 {
		t.Fatal("no progress arrived at all, so this proves nothing about how it arrives")
	}
	if len(seen) < 2 {
		t.Fatalf("progress arrived on %d goroutine(s), so nothing was ever serialised and "+
			"this guard did not reach the question it asks. Either the run stopped writing "+
			"files beside each other, or GOMAXPROCS could not be raised.", len(seen))
	}
	if overlapped.Load() {
		t.Errorf("two progress reports were in flight at once, across %d goroutines.\n"+
			"Reason: the window's rate limiter reads and writes a timestamp without a lock,\n"+
			"and so does the command line bar, on the strength of the contract in\n"+
			"engine.Options.OnProgress. Two at once here is a race there, showing up as a\n"+
			"bar that stutters or a report that is never drawn - and only sometimes.",
			len(seen))
	}
}

// A run over several writers still moves the bar forwards only, and still
// lands it on the totals it promised.
//
// The sequential loop got this for nothing: one file at a time, a running
// total, and the last report was the last file. With several writers the total
// is shared and the last report comes from whichever writer finished last, so
// both properties are now arithmetic that can be got wrong - a writer topping
// up by the whole file rather than by what it had not yet reported would sail
// past the total, and one that forgot to top up at all would stop short of it.
//
// Under the race detector this is also the densest contention in the suite:
// thirty two files, several writers, and a callback on every write inside each
// of them.
func TestProgressOverSeveralWritersStillReachesTheEndAndNeverGoesBack(t *testing.T) {
	dir := t.TempDir()
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(8))

	var mu sync.Mutex
	var reports []engine.Progress
	opt := engine.Options{
		OutDir: dir, Seed: 8821, Command: "test",
		ManifestName: engine.DefaultManifestName,
		OnProgress: func(p engine.Progress) {
			mu.Lock()
			reports = append(reports, p)
			mu.Unlock()
		},
	}
	planned, err := engine.Plan([]engine.Target{txtTarget("files", 32, 1<<20)}, opt)
	if err != nil {
		t.Fatalf("planning: %v", err)
	}
	if _, err := engine.Run(context.Background(), planned, opt); err != nil {
		t.Fatalf("the run failed: %v", err)
	}

	if len(reports) < 32 {
		t.Fatalf("thirty two files produced %d reports, so this guard is not looking at "+
			"a run that reported from inside its files", len(reports))
	}
	last := reports[len(reports)-1]
	if last.FilesDone != last.FilesTotal || last.BytesDone != last.BytesTotal {
		t.Errorf("the last report said %d/%d files and %d/%d bytes, so the bar never "+
			"reaches the end it promised", last.FilesDone, last.FilesTotal,
			last.BytesDone, last.BytesTotal)
	}
	var prevBytes int64
	var prevFiles int
	for i, p := range reports {
		if p.BytesDone < prevBytes {
			t.Fatalf("report %d went backwards, from %d B to %d B - with several writers "+
				"sharing one total, a writer that gives back more than it added does this",
				i, prevBytes, p.BytesDone)
		}
		if p.FilesDone < prevFiles {
			t.Fatalf("report %d counted %d finished files after %d", i, p.FilesDone, prevFiles)
		}
		if p.BytesDone > p.BytesTotal {
			t.Fatalf("report %d claimed %d B of %d B, which is more than the run will write",
				i, p.BytesDone, p.BytesTotal)
		}
		prevBytes, prevFiles = p.BytesDone, p.FilesDone
	}
}

// goroutineName is the identity of the goroutine calling it, taken from the
// first line of its own stack: "goroutine 17 [running]:". There is no
// supported way to ask, which is why this is confined to one guard.
func goroutineName() string {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	return strings.Fields(string(buf[:n]))[1]
}
