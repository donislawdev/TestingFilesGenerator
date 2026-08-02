package guard

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
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
