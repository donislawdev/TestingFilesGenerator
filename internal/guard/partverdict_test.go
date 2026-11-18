package guard

import (
	"path/filepath"
	"strings"
	"testing"
)

// The test job and coverage of ci.yml run in parts, and main's rules require
// one check per system and one for coverage under the names those jobs had
// before (O361). A verdict job carries each of those names and asks
// race_parts.py verdict whether every part ran and passed.

// TestAVerdictPassesOnlyWhenEveryPartRanAndPassed hands the verdict the jobs of
// a run, the way the API lists them, and asks it about the guards on one
// system.
//
// Every refusal is one way a run could look green while a part of the guards
// ran nowhere: a part that failed, a part missing from the matrix, a part that
// was cancelled, two jobs claiming one part, parts that disagree about how many
// there are. The first case passes, so the refusals are not the script
// refusing everything.
func TestAVerdictPassesOnlyWhenEveryPartRanAndPassed(t *testing.T) {
	python := pythonForGate(t)
	other := "guards on ubuntu-latest (part 0 of 2)\tfailure"
	cases := []struct {
		name string
		jobs []string
		pass bool
	}{
		{"every part passed, a part elsewhere failed", []string{
			"guards on windows-latest (part 0 of 2)\tsuccess", "guards on windows-latest (part 1 of 2)\tsuccess", other}, true},
		{"one part failed", []string{
			"guards on windows-latest (part 0 of 2)\tsuccess", "guards on windows-latest (part 1 of 2)\tfailure"}, false},
		{"one part never ran", []string{"guards on windows-latest (part 0 of 2)\tsuccess", other}, false},
		{"one part was cancelled", []string{
			"guards on windows-latest (part 0 of 2)\tcancelled", "guards on windows-latest (part 1 of 2)\tsuccess"}, false},
		{"two jobs are one part", []string{
			"guards on windows-latest (part 0 of 2)\tsuccess", "guards on windows-latest (part 0 of 2)\tsuccess",
			"guards on windows-latest (part 1 of 2)\tsuccess"}, false},
		{"the parts disagree about their number", []string{
			"guards on windows-latest (part 0 of 2)\tsuccess", "guards on windows-latest (part 1 of 3)\tsuccess"}, false},
		{"a part past the number", []string{
			"guards on windows-latest (part 0 of 1)\tsuccess", "guards on windows-latest (part 1 of 1)\tsuccess"}, false},
		{"no part at all", []string{other}, false},
	}
	for _, c := range cases {
		jobs := writeLines(t, "jobs.tsv", c.jobs...)
		out, errOut, code := runRaceParts(t, python, "verdict", "guards on windows-latest", jobs)
		if (code == 0) != c.pass {
			t.Errorf("%s: the verdict exited %d, and it should %s:\n%s%s", c.name, code, map[bool]string{true: "pass", false: "fail"}[c.pass], out, errOut)
		}
	}
}

// TestAVerdictOfPartsRunsWhateverThePartsDid holds every verdict job of ci.yml
// to running after its parts whatever they concluded.
//
// A job whose needs failed is skipped by default, and GitHub counts a skipped
// required check as passed. A verdict without always() would therefore turn
// a red part into a green merge - silently, which is the one failure that
// matters here. The canary is the same reading of a verdict without it.
func TestAVerdictOfPartsRunsWhateverThePartsDid(t *testing.T) {
	jobs := ciJobs(readFile(t, filepath.Join(repoRoot(t), ".github", "workflows", "ci.yml")))
	verdicts := 0
	for id, block := range jobs {
		if !strings.Contains(block, "race_parts.py verdict ") {
			continue
		}
		verdicts++
		if problem := verdictProblem(block); problem != "" {
			t.Errorf("the job %s of ci.yml %s", id, problem)
		}
	}
	for _, id := range []string{"verdict", "coverage-gate"} {
		if !strings.Contains(jobs[id], "race_parts.py verdict ") {
			t.Errorf("ci.yml has no job %s asking for a verdict on parts, and main's rules require the check it carries", id)
		}
	}
	if verdicts < 2 {
		t.Errorf("ci.yml holds %d verdict jobs, and there are two: the tests on each system and coverage", verdicts)
	}
	canary := strings.Replace(jobs["verdict"], "if: always()", "if: success()", 1)
	if verdictProblem(canary) == "" {
		t.Error("a verdict job without always() was read as fine, so the check above proves nothing")
	}
}

// verdictProblem says what is wrong with one verdict job, or nothing.
func verdictProblem(block string) string {
	switch {
	case !strings.Contains(block, "\n    needs: "):
		return "asks for a verdict on parts it does not wait for (no needs:)"
	case !strings.Contains(block, "\n    if: always()\n"):
		return "is skipped when a part fails, and a skipped required check counts as passed. Give it if: always()"
	case !strings.Contains(block, "actions: read"):
		return "cannot read the jobs of its run without actions: read"
	}
	return ""
}
