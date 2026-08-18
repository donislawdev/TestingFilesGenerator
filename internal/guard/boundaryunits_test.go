package guard

import (
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// A boundary counts in 1024s, like every other size this tool reads, and the
// run says out loud which number that came to.
//
// It used to be REFUSED. The reasoning written here was that a boundary
// describes a limit in somebody else's system, where "15 MB" on an upload form
// means 15000000 B far more often than not - so a set built around 15728640
// would sit entirely past the real limit and test nothing, while looking right.
//
// The owner withdrew that on 2026-08-18 out of their own experience: a limit
// written "15 MB" is worked out in 1024s in almost every system, and the
// decimal reading is the rare one. The premise was backwards, so what stood on
// it went with it.
//
// What is guarded instead is the thing that makes accepting it safe, and it was
// already true: the run prints the limit it built around, in bytes, above the
// three files. Anybody whose system meant the other number sees it in a dry run
// before a byte is written, and a plain byte count still passes through for
// them. That sentence is now load bearing, so it has a guard of its own.

func TestABoundarySpelledInUnitsIsTakenAsTheOneWeUseEverywhere(t *testing.T) {
	// Each spelling with the 1024s value it has to come to. The decimal reading
	// is named in the failure so that anybody reading it sees both numbers.
	// Every shape a boundary can be written in, in one table, because they are
	// one behaviour now. Until 2026-08-18 there were two guards here - one for
	// the spellings refused as ambiguous and one for the spellings that passed -
	// and that split described the refusal rather than the tool. With the refusal
	// gone, 15mb and 15mib are read the same way, so a second guard would have
	// been a second name for one rule.
	for spelling, want := range map[string]struct{ binary, decimal string }{
		"15mb":     {"15728640", "15000000"},
		"15MB":     {"15728640", "15000000"},
		"10kb":     {"10240", "10000"},
		"2gb":      {"2147483648", "2000000000"},
		"15m":      {"15728640", "15000000"},
		"15mib":    {"15728640", ""},
		"2gib":     {"2147483648", ""},
		"15000000": {"15000000", ""},
		"1024":     {"1024", ""},
	} {
		got, err := core.ParseBoundary(spelling)
		if err != nil {
			t.Errorf("--boundary %s was refused: %v", spelling, err)
			continue
		}
		if fmt := strings.TrimSpace(want.binary); got != mustNumber(t, fmt) {
			t.Errorf("--boundary %s came to %d B, and counting in 1024s that is %s (the decimal reading is %s)",
				spelling, got, want.binary, want.decimal)
		}
	}
}

// And the run says which number it used, which is what makes the above safe.
//
// Without this the tool would take a spelling that CAN mean two things and
// never mention which one it took - the failure the refusal existed to prevent,
// arriving quietly instead of loudly.
func TestABoundaryRunSaysTheNumberItBuiltAround(t *testing.T) {
	dir := t.TempDir()
	code, out, errOut := run(t, "generate", "--format", "txt",
		"--boundary", "15mb", "--out", dir, "--dry-run")
	if code != cli.ExitOK {
		t.Fatalf("a boundary of 15mb was refused: exit %d\n%s", code, errOut)
	}
	// The line that ANNOUNCES the set, not the number anywhere in the output.
	// The three file lines carry the byte count as well, so asking whether the
	// number appears at all left this green when the announcement lost it -
	// which the mutation runner said out loud on 2026-08-18.
	if !strings.Contains(errOut, `boundary "files" around 15728640 B`) {
		t.Errorf("the run built a set around 15728640 B and never says so.\n"+
			"Reason: 15mb can be read two ways, and printing the byte count is what lets somebody\n"+
			"whose system meant 15000000 see it before a byte is written.\nWhat it said:\n%s", out)
	}
}

// A size keeps the units it always had. The asymmetry is the point: the two
// numbers answer different questions, and making them agree would mean either
// a size that Explorer disagrees with or a boundary that guesses.
func TestASizeKeepsCountingIn1024s(t *testing.T) {
	got, err := core.ParseSize("15mb")
	if err != nil {
		t.Fatalf("--size 15mb was refused: %v", err)
	}
	if got != 15<<20 {
		t.Errorf("--size 15mb gave %d B, expected %d B - sizes count in 1024s "+
			"because that is what Explorer and ls show", got, int64(15<<20))
	}
}

// The recipe key goes through the same door. Recipes travel between teams, so
// one accepting what the other refuses is the worst of both.
// The recipe key reads a spelling the same way the flag does.
//
// One implementation answers both - core.ParseBoundary - and this is what says
// so from outside, because the two drifting apart is the failure that would
// make a recipe and a command line disagree about the same word.
func TestTheRecipeBoundaryKeyReadsUnitsLikeTheFlag(t *testing.T) {
	// Through a real recipe rather than by calling the parser. The first
	// version of this compared core.ParseBoundary with core.ParseSize, which
	// is the function it calls - so it could not fail, and the mutation runner
	// could not break it. A guard that asks a question with one answer is a
	// guard that proves nothing.
	for spelling, want := range map[string]int64{
		"15mb":  15 << 20,
		"10kb":  10 << 10,
		"15mib": 15 << 20,
		"1024":  1024,
	} {
		rec, err := recipe.Parse([]byte(
			"version: 1\ntargets:\n  - id: files\n    format: txt\n    boundary: "+spelling+"\n"), "guard")
		if err != nil {
			t.Errorf("a recipe with boundary: %s was refused: %v", spelling, err)
			continue
		}
		if len(rec.Targets) != 1 {
			t.Fatalf("the recipe parsed to %d targets", len(rec.Targets))
		}
		if got := rec.Targets[0].BoundaryLimit; got != want {
			t.Errorf("boundary: %s in a recipe came to %d B, and the flag makes it %d B",
				spelling, got, want)
		}
	}
}

// mustNumber turns a spelled out byte count into a number, for the table above.
func mustNumber(t *testing.T, s string) int64 {
	t.Helper()
	n, err := core.ParseSize(s)
	if err != nil {
		t.Fatalf("the guard's own expected value %q does not parse: %v", s, err)
	}
	return n
}
