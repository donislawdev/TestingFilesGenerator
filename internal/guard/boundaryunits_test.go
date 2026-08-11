package guard

import (
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// A boundary set is the one place where the number belongs to somebody else.
//
// Every other size describes a file this tool makes, and the user checks it in
// Explorer, which counts in 1024s the same way we do. A boundary describes a
// limit in the system under test, and there the unit is whatever that system
// chose - "15 MB" on an upload form is 15000000 B far more often than not.
//
// Reported from manual testing, and the report is the reason this is refused
// rather than hinted at. A set aimed at a 15 MB form produced three files
// around 15728640, every one of them over the real limit, and all three were
// rejected. Nothing was broken and nothing said anything. The tester had
// files, believed they were the right files, and lost the time anyway.
//
// So the ambiguity is an error, the same way the recipe already refuses 0x10
// and 1_000 rather than guessing what they meant.

func TestAnAmbiguousBoundaryUnitIsRefusedRatherThanGuessed(t *testing.T) {
	// Both readings of each spelling, in bytes: the 1000s one most services
	// mean and the 1024s one some do.
	for spelling, readings := range map[string][2]string{
		"15mb": {"15000000", "15728640"},
		"15MB": {"15000000", "15728640"},
		"10kb": {"10000", "10240"},
		"2gb":  {"2000000000", "2147483648"},
		"15m":  {"15000000", "15728640"},
	} {
		dir := t.TempDir()
		code, _, errOut := run(t, "generate", "--format", "txt",
			"--boundary", spelling, "--out", dir)

		if code == cli.ExitOK {
			t.Errorf("--boundary %s was accepted, so a set can still be built around "+
				"a limit nobody stated", spelling)
			continue
		}
		// Both readings as numbers, so the person does not have to work either
		// of them out. A refusal that does not say what to write instead is a
		// worse tool than one that guesses.
		//
		// Asked as numbers rather than as a pasteable "--boundary N" since
		// 2026-08-11: this sentence is written once in the engine and shown by
		// both surfaces, and the window has no flags on it, so a spelling only
		// the command line takes sends the other reader translating. O79, and
		// the cost of it is exactly this - the command line reader now copies a
		// number rather than a whole command.
		for _, want := range readings {
			if !strings.Contains(errOut, want) {
				t.Errorf("--boundary %s was refused without naming %s, one of its two readings: %s",
					spelling, want, errOut)
			}
		}
	}
}

// The unambiguous spellings still work, and they give the sizes they say.
// Without this half, the refusal could be "fixed" by refusing everything.
func TestAnUnambiguousBoundaryStillWorks(t *testing.T) {
	for _, tc := range []struct {
		spelling string
		limit    int64
	}{
		{"15mib", 15 << 20},
		{"15000000", 15_000_000},
		{"1024", 1024},
		{"2gib", 2 << 30},
	} {
		got, err := core.ParseBoundary(tc.spelling)
		if err != nil {
			t.Errorf("--boundary %s was refused and it says exactly which limit it "+
				"means: %v", tc.spelling, err)
			continue
		}
		if got != tc.limit {
			t.Errorf("--boundary %s gave %d B, expected %d B", tc.spelling, got, tc.limit)
		}
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
func TestTheRecipeBoundaryKeyRefusesTheSameSpellings(t *testing.T) {
	dir := t.TempDir()
	path := writeRecipe(t, dir, `version: 1
seed: 7741
targets:
  - id: edges
    format: txt
    boundary: 15mb
output:
  dir: out
`)

	code, _, errOut := run(t, "validate", path)
	if code == cli.ExitOK {
		t.Fatalf("the recipe was accepted, so a recipe can ask for what a flag cannot")
	}
	if !strings.Contains(errOut, "15000000") {
		t.Errorf("the refusal did not name the decimal reading: %s", errOut)
	}
}
