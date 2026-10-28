package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// --memory is read the same way on every command that plans - generate,
// validate and preset show - docs/STABILITY-REVIEW-2026-10-26.md section 10.8.
//
// One function registers it on all three, and this is what says each of them
// really hands it to the planner: a ceiling far under the plan refuses the plan
// on each, and the refusal names the setting. The plan is about 42 MB of text
// files on the first two and about 9 MB of the largest preset on the third,
// against ceilings of 4 MB and 1 MB, so the margin is the one
// TestAPlanTooBigToHoldIsRefusedWhileItIsBuilt measured as clear of the noise a
// heap reading carries in this process. The help is asked too, and preset show
// is why: its help is written out by hand rather than listed by the flag
// package, so a flag registered there is a flag nobody is told about unless
// the sentence is added beside it.
func TestTheMemoryFlagReachesThePlanOnEveryCommandThatPlans(t *testing.T) {
	dir := t.TempDir()
	recipe := filepath.Join(dir, "many.yaml")
	if err := os.WriteFile(recipe, []byte("version: 1\ntargets:\n  - id: many\n    format: txt\n    size: 4kb\n    count: 50000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out")
	for _, c := range []struct {
		name    string
		args    []string
		ceiling string
	}{
		{"generate", []string{"generate", "--format", "txt", "--size", "4kb", "--count", "50000", "--dry-run", "--out", out}, "4mb"},
		{"validate", []string{"validate", recipe}, "4mb"},
		// Text files in the mass upload rather than the pictures it holds by
		// default, which are coded while planning and take a minute for ten
		// thousand on main as on this branch (2026-10-27).
		{"preset show", []string{"preset", "show", "upload-validation", "--allow", "txt", "--bulk", "10000"}, "1mb"},
	} {
		code, _, errOut := run(t, append(c.args, "--memory", c.ceiling)...)
		if code != cli.ExitRecipe || !strings.Contains(errOut, "the memory setting of this run gives it") {
			t.Errorf("%s --memory %s: exit %d, expected %d and a refusal naming the memory setting:\n%s", c.name, c.ceiling, code, cli.ExitRecipe, errOut)
		}
		// The same command without the flag is auto, at least two gigabytes
		// of plan, and the plan goes through - so the refusal above was the
		// flag's doing rather than the shape's.
		if code, _, errOut := run(t, c.args...); code != cli.ExitOK {
			t.Errorf("%s with no --memory: exit %d, expected the plan to pass under auto:\n%s", c.name, code, errOut)
		}

		// A value that is not a setting is the caller's typing, exit 2 with
		// the sentence of the parser, on every one of the three.
		code, _, errOut = run(t, append(c.args, "--memory", "0")...)
		if code != cli.ExitUsage || !strings.Contains(errOut, "leaves no room at all") {
			t.Errorf("%s --memory 0: exit %d, expected %d and why nought is refused:\n%s", c.name, code, cli.ExitUsage, errOut)
		}

		// And the help says the flag is there, and what auto means - with
		// the floor the plan keeps under it, or a machine of four gigabytes
		// or less reads a refusal the run would never give (review on #208).
		_, help, _ := run(t, append(c.args[:len(c.args):len(c.args)], "--help")...)
		floor := "a plan of up to " + core.HumanBytes(core.MaxPlanBytes) + " is never refused"
		if !strings.Contains(help, "-memory") || !strings.Contains(help, "half of what this machine gives the program") ||
			!strings.Contains(help, floor) {
			t.Errorf("%s --help does not offer --memory and say what auto is, floor included:\n%s", c.name, help)
		}
	}
}

// A file written past the memory setting is said on the command line, once.
//
// The engine owes the note (TestTheFilesWrittenAtOnceHoldNoMoreThanTheMemorySettingLeaves
// asks it of the result), and this asks that the command line passes it on:
// the note lives beside the manifest rather than in it, so the loop that
// prints the manifest's notes does not print it. Three jpgs of three
// megapixels declare about fourteen megabytes each against eight of setting.
// The same three files with no setting at all are byte for byte the same -
// the queue changes when a file is written, never what is in it.
func TestAFileWrittenPastTheMemorySettingIsSaidOnTheCommandLine(t *testing.T) {
	dir := t.TempDir()
	shape := []string{"generate", "--format", "jpg", "--size", "4mb", "--set", "width=2000", "--set", "height=1500", "--count", "3"}
	code, _, errOut := run(t, append(shape, "--memory", "8mb", "--out", filepath.Join(dir, "held"))...)
	if code != cli.ExitOK {
		t.Fatalf("exit %d:\n%s", code, errOut)
	}
	if n := strings.Count(errOut, "note: files_0001.jpg holds about"); n != 1 {
		t.Errorf("the note about the file written past the setting was printed %d times, expected once:\n%s", n, errOut)
	}
	if code, _, errOut := run(t, append(shape, "--out", filepath.Join(dir, "auto"))...); code != cli.ExitOK {
		t.Fatalf("exit %d:\n%s", code, errOut)
	}
	for _, name := range []string{"files_0001.jpg", "files_0002.jpg", "files_0003.jpg"} {
		held, err := os.ReadFile(filepath.Join(dir, "held", name))
		if err != nil {
			t.Fatal(err)
		}
		auto, err := os.ReadFile(filepath.Join(dir, "auto", name))
		if err != nil {
			t.Fatal(err)
		}
		if string(held) != string(auto) {
			t.Errorf("%s came out different under --memory 8mb - the setting moved a byte", name)
		}
	}
}
