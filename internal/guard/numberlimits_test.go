package guard

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// Two numbers a person can write that the tool could not honour, and used to
// try anyway.
//
// Both were found by audit on 2026-08-03 and both ended outside the frozen exit
// code table, which is what makes them one class rather than two bugs:
//
//	--count 9223372036854775807
//	  -> panic: runtime error: makeslice: len out of range, exit 2
//	the same count in a recipe
//	  -> runtime: VirtualAlloc of 13358710784 bytes failed
//	     fatal error: out of memory, exit 2
//	--size 4611686018427387904 --count 2
//	  -> "2 file(s) in 1 target(s), -9223372036854775808 B total", exit 0
//
// The first two print a Go stack trace, which docs/SECURITY.md section 2.5 says
// never happens, under exit code 2, which the table says means the caller
// mistyped a flag. The third is worse than a crash: the run is accepted, the
// free space guard compares a negative requirement against the disk, finds it
// satisfied, and starts writing.
//
// So the rule both of these guard is one sentence: a number this tool cannot
// honour is refused with a reason, never wrapped and never allocated.

// A count no machine could hold is a refusal, not an allocation. The number
// below is the largest int64, which is what a script computing a count from
// something else can hand over by accident.
func TestAnImpossibleFileCountIsRefusedRatherThanAllocated(t *testing.T) {
	dir := t.TempDir()

	for _, count := range []string{"9223372036854775807", "100000000000", "2000000"} {
		t.Run(count, func(t *testing.T) {
			code, stdout, errOut := run(t,
				"generate", "--format", "txt", "--size", "1kb",
				"--count", count, "--dry-run", "--out", dir)

			if code == cli.ExitOK {
				t.Fatalf("a count of %s was accepted:\n%s", count, stdout)
			}
			// The ending has to be one CI can read as "you asked for something
			// impossible", not one that means the tool broke.
			if code != cli.ExitRecipe {
				t.Errorf("exit %d, expected %d - asking for more files than the tool will plan is the caller's request, not a fault in the tool:\n%s", code, cli.ExitRecipe, errOut)
			}
			if !strings.Contains(errOut, "1000000") {
				t.Errorf("the refusal does not say what the limit is, so nobody can pick a number that works:\n%s", errOut)
			}
			if strings.Contains(errOut, "goroutine") || strings.Contains(errOut, "runtime error") {
				t.Errorf("a stack trace reached the user:\n%s", errOut)
			}
			if stdout != "" {
				t.Errorf("a failed run wrote to stdout:\n%s", stdout)
			}
		})
	}
}

// The same number through the other door. A recipe travels between teams, so
// this is the copy of the check that faces a file somebody else wrote - and it
// took the worse of the two failures, because the recipe reader grows the list
// one entry at a time and got 13 GB into it before the allocator gave up.
func TestAnImpossibleFileCountInARecipeIsRefused(t *testing.T) {
	dir := t.TempDir()
	path := writeRecipe(t, dir, `version: 1
targets:
  - id: boom
    format: txt
    size: 1kb
    count: 9223372036854775807
`)

	code, stdout, errOut := run(t, "validate", path)
	if code != cli.ExitRecipe {
		t.Errorf("exit %d, expected %d:\n%s", code, cli.ExitRecipe, errOut)
	}
	if !strings.Contains(errOut, "1000000") {
		t.Errorf("the refusal does not say what the limit is:\n%s", errOut)
	}
	if strings.Contains(errOut, "fatal error") || strings.Contains(errOut, "out of memory") {
		t.Errorf("the allocator was reached instead of the check:\n%s", errOut)
	}
	if stdout != "" {
		t.Errorf("a failed run wrote to stdout:\n%s", stdout)
	}
}

// The same number, inside a container rather than on the target.
//
// This is the sibling path the guard above never walked, and it was still
// broken on 2026-08-20 - three weeks after the target path was fixed. CodeQL
// pointed at the narrowing conversion behind it and the measurement turned it
// from a warning into an incident: validating this recipe took the process to
// 12.9 GB before it was killed, which is the same allocator the comment at the
// top of this file records from 2026-08-03.
//
// So the lesson this test carries is not about counts. A rule enforced at one
// place that reads a number is enforced at ONE place, and every sibling that
// reads the same number is a separate question - `contains` had its own reader,
// its own switch and its own missing case.
//
// validate rather than generate, deliberately: the refusal has to come before
// anything is planned, and a test that produced files would be measuring a
// later stage.
func TestAnImpossibleFileCountInsideAContainerIsRefused(t *testing.T) {
	dir := t.TempDir()
	path := writeRecipe(t, dir, `version: 1
targets:
  - id: boom
    format: zip
    contains:
      - format: txt
        count: 9223372036854775807
        size: 1kb
`)

	code, stdout, errOut := run(t, "validate", path)
	if code != cli.ExitRecipe {
		t.Errorf("exit %d, expected %d:\n%s", code, cli.ExitRecipe, errOut)
	}
	if !strings.Contains(errOut, "1000000") {
		t.Errorf("the refusal does not say what the limit is:\n%s", errOut)
	}
	// The address matters as much as the refusal here. A container entry is one
	// of several, so "too many files" without saying which entry sends somebody
	// back to a list to guess.
	if !strings.Contains(errOut, "contains") {
		t.Errorf("the refusal does not say which part of the recipe it is about:\n%s", errOut)
	}
	if strings.Contains(errOut, "fatal error") || strings.Contains(errOut, "out of memory") {
		t.Errorf("the allocator was reached instead of the check:\n%s", errOut)
	}
	if stdout != "" {
		t.Errorf("a failed run wrote to stdout:\n%s", stdout)
	}
}

// The other direction, so the fix cannot be "refuse everything". An ordinary
// count has to keep working, and the boundary itself has to be reachable in
// principle rather than off by one.
func TestAnOrdinaryFileCountStillWorks(t *testing.T) {
	dir := t.TempDir()
	code, _, errOut := run(t,
		"generate", "--format", "txt", "--size", "1kb",
		"--count", "12", "--dry-run", "--out", dir)
	if code != cli.ExitOK {
		t.Fatalf("an ordinary count was refused: exit %d\n%s", code, errOut)
	}
	if !strings.Contains(errOut, "12 files") {
		t.Errorf("the plan does not describe the twelve files:\n%s", errOut)
	}
}

// A total that does not fit in the type it is measured in.
//
// This is the one that never crashed and is the most dangerous of the three:
// the sum wraps to a negative number, the free space check asks whether the
// disk has that many bytes free, the answer is yes, and the run starts. The
// dry run reported success for it - which is measurable proof the preflight
// ran and was satisfied, because the same command with a boundary that does
// fit ends with the space code.
func TestATotalThatDoesNotFitIsRefused(t *testing.T) {
	dir := t.TempDir()

	// Two files just over half the range each. Neither is refusable on its own
	// size, and together they wrap.
	code, stdout, errOut := run(t,
		"generate", "--format", "txt", "--size", "4611686018427387904",
		"--count", "2", "--dry-run", "--out", dir)

	if code == cli.ExitOK {
		t.Fatalf("a run whose total does not fit was accepted:\n%s%s", stdout, errOut)
	}
	if code != cli.ExitRecipe {
		t.Errorf("exit %d, expected %d:\n%s", code, cli.ExitRecipe, errOut)
	}
	// The number the user was shown is the defect somebody would report, so it
	// is what this asserts on. A negative byte count is never an answer.
	if strings.Contains(stdout+errOut, "-") && strings.Contains(stdout+errOut, "B total") {
		t.Errorf("a negative byte count was printed:\n%s%s", stdout, errOut)
	}
	if !strings.Contains(errOut, "too large") {
		t.Errorf("the refusal does not say what is wrong:\n%s", errOut)
	}
}

// The same wrap reached through a boundary set, where the third size is one
// byte above the limit and the limit is the largest number there is.
func TestABoundaryThatCannotHoldItsOwnSetIsRefused(t *testing.T) {
	dir := t.TempDir()
	code, _, errOut := run(t,
		"generate", "--format", "txt", "--boundary", "9223372036854775807",
		"--dry-run", "--out", dir)

	if code == cli.ExitOK {
		t.Fatal("a boundary with no room for the file above it was accepted")
	}
	// Before the fix this reported "TXT cannot be smaller than 0 B ...
	// Requested: -9223372036854775808 B" - an answer about the bottom of the
	// range to a question about the top of it.
	if strings.Contains(errOut, "cannot be smaller") {
		t.Errorf("the message answers about the wrong end of the range:\n%s", errOut)
	}
	if !strings.Contains(errOut, "9223372036854775807") {
		t.Errorf("the refusal does not name the limit that was asked for:\n%s", errOut)
	}
}

// A run spread across many targets reaches the same ceiling. Checking only the
// count of one target would leave the whole thing reachable by writing the
// number out in pieces.
func TestTheCeilingCountsTheWholeRunNotOneTarget(t *testing.T) {
	dir := t.TempDir()
	var b strings.Builder
	b.WriteString("version: 1\ntargets:\n")
	// Twelve targets of 90 000 files come to 1 080 000, which is over the
	// ceiling while no single target is.
	for i := 0; i < 12; i++ {
		b.WriteString("  - id: t")
		b.WriteString(string(rune('a' + i)))
		b.WriteString("\n    format: txt\n    size: 1b\n    count: 90000\n")
	}
	path := writeRecipe(t, dir, b.String())

	code, stdout, errOut := run(t, "validate", path)
	if code == cli.ExitOK {
		t.Fatalf("a run of over a million files spread across targets was accepted:\n%s", stdout)
	}
	if !strings.Contains(errOut, "1000000") {
		t.Errorf("the refusal does not name the ceiling:\n%s", errOut)
	}
	_ = filepath.Join
}
