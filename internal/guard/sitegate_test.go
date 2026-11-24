package guard

import (
	"errors"
	"os/exec"
	"path/filepath"
	"testing"
)

// The site goes out after CI with the jobs on macOS left out (O371): the ci
// job of pages.yml reads the jobs of the run that triggered it and asks
// .github/scripts/site_gate.py whether the site may go out.

// siteGateRun asks the gate about a list of jobs and returns its exit code
// and what it printed.
func siteGateRun(t *testing.T, python string, jobs []string, head, tip string) (int, string) {
	t.Helper()
	script := filepath.Join(repoRoot(t), ".github", "scripts", "site_gate.py")
	out, err := exec.Command(python, script, writeLines(t, "jobs.tsv", jobs...), head, tip).CombinedOutput()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return 0, string(out)
	case errors.As(err, &exit):
		return exit.ExitCode(), string(out)
	}
	t.Fatalf("running %s: %v", script, err)
	return 0, ""
}

// TestTheSiteWaitsForEveryJobOfCIButTheOnesOnMacOS hands the gate the jobs of
// a run the way the API lists them, a name, a conclusion and the runner labels.
//
// The first two cases have to pass, so the refusals are not the script
// refusing everything: one is a green run, the other the run of 2026-11-24
// whose macOS jobs GitHub cancelled for want of a runner, with the verdict
// "test on macos-latest" red on a Linux runner. Every refusal after them is
// one way the site could go out while the tree it shows was not good: a Linux
// or Windows job red, cancelled or still running, a list with no verdict, a
// commit main has moved past. A list it cannot read is neither yes nor no, so
// the ci job goes red instead of holding the site back unseen.
func TestTheSiteWaitsForEveryJobOfCIButTheOnesOnMacOS(t *testing.T) {
	python := pythonForGate(t)
	const head, publish, hold = "1111111", 0, 3
	green := []string{
		"guards on ubuntu-latest (part 0 of 2)\tsuccess\tubuntu-latest",
		"guards on windows-latest (part 0 of 2)\tsuccess\twindows-latest",
		"guards on macos-latest (part 0 of 2)\tsuccess\tmacos-latest",
		"test on ubuntu-latest\tsuccess\tubuntu-latest",
		"test on windows-latest\tsuccess\tubuntu-latest",
		"test on macos-latest\tsuccess\tubuntu-latest",
		"fuzzing with a time budget\tskipped\tubuntu-latest",
	}
	with := func(extra ...string) []string { return append(append([]string{}, green...), extra...) }
	cases := []struct {
		name string
		jobs []string
		tip  string
		code int
	}{
		{"every job passed", green, head, publish},
		{"the macOS jobs were cancelled and their verdict is red", with(
			"npm package on macos-latest\tcancelled\tmacos-latest",
			"guards on macos-latest (part 1 of 2)\tcancelled\tmacos-latest",
			"test on macos-latest\tfailure\tubuntu-latest",
			"Python package with Python 3.10\tcancelled\tmacos-14"), head, publish},
		{"a Linux part failed", with("guards on ubuntu-latest (part 1 of 2)\tfailure\tubuntu-latest"), head, hold},
		{"a Windows job was cancelled", with("the installer installs and leaves\tcancelled\twindows-latest"), head, hold},
		{"a job outside macOS is still running", with("linters\tnone\tubuntu-latest"), head, hold},
		{"no verdict outside macOS", []string{
			"guards on ubuntu-latest (part 0 of 2)\tsuccess\tubuntu-latest",
			"test on macos-latest\tsuccess\tubuntu-latest"}, head, hold},
		{"main has moved past the commit", green, "2222222", hold},
		{"no job at all", nil, head, 2},
		{"a line without a conclusion", []string{"linters"}, head, 2},
	}
	for _, c := range cases {
		code, out := siteGateRun(t, python, c.jobs, head, c.tip)
		if code != c.code {
			t.Errorf("%s: the gate exited %d and should exit %d:\n%s", c.name, code, c.code, out)
		}
	}
}
