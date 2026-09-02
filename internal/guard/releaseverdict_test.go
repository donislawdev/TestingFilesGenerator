package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

// What this defends. The job that checks a published release asks every one of
// its questions and answers with all of them, rather than stopping at the first
// one that goes wrong.
//
// Why it needed a guard. Measured on the v0.2.0 publish, 2026-08-28. The job
// ran, the step checking the two commands offline went red, and the three steps
// after it were skipped - the Windows signatures, whether the page promises what
// was checked, and whether a full release is the one people are offered. So
// three questions about the published release were never asked at all, and the
// run said nothing about that. One of the three would have gone red too: the
// published notes carry none of the three sentences the notes step looks for,
// measured the same way.
//
// Why it matters more here than in most places. This job runs once per release
// and a person reads it once. A gate that reports one problem per release turns
// a list into a queue, and every trip round that queue costs a publish. It is
// the rule the recipe already follows when it refuses, RC7, applied to the one
// gate standing over the thing strangers download.
//
// Why the macOS job is not held to this. It has a single check, so there is
// nothing for a first failure to hide. The rule here is about a step covering
// for the ones behind it, which needs at least two.
//
// What this does NOT check. That the checks themselves are right, or that the
// list of them is complete. It checks that whatever list exists is asked in
// full and reported in full.

// releaseWorkflow is the shape of a workflow file, as much of it as these
// questions need.
type releaseWorkflow struct {
	Jobs map[string]struct {
		Steps []struct {
			Name            string            `yaml:"name"`
			ID              string            `yaml:"id"`
			ContinueOnError bool              `yaml:"continue-on-error"`
			If              string            `yaml:"if"`
			Env             map[string]string `yaml:"env"`
			Run             string            `yaml:"run"`
			Uses            string            `yaml:"uses"`
		} `yaml:"steps"`
	} `yaml:"jobs"`
}

// The step the checks begin after. Named here rather than counted, because a
// position is the thing that moves when somebody inserts a step.
const releaseSetupEndsAfter = "download every published asset"

func readReleaseWorkflow(t *testing.T) releaseWorkflow {
	t.Helper()

	body, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "verify-release.yml"))
	if err != nil {
		t.Skipf("no verify-release.yml here: %v", err)
	}

	var parsed releaseWorkflow
	if err := yaml.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("verify-release.yml does not parse as YAML, so the job it describes cannot run: %v", err)
	}
	if len(parsed.Jobs) == 0 {
		t.Fatal("verify-release.yml declares no jobs, so this proved nothing")
	}
	return parsed
}

// Every check is asked, and every answer reaches the verdict.
func TestNoReleaseCheckCanHideTheOnesBehindIt(t *testing.T) {
	parsed := readReleaseWorkflow(t)

	job, ok := parsed.Jobs["verify"]
	if !ok {
		t.Fatalf("verify-release.yml has no job called verify. It has: %v", jobNames(parsed))
	}
	if len(job.Steps) < 3 {
		t.Fatalf("the verify job has %d steps, which is too few for it to be the job this "+
			"guard describes", len(job.Steps))
	}

	// The verdict is the last step, and it has to be able to run after a red
	// one and to fail on what it read.
	verdict := job.Steps[len(job.Steps)-1]
	if !strings.Contains(verdict.If, "always()") {
		t.Errorf("the last step of the verify job is %q and it runs on %q.\n"+
			"It has to run on always(), because the steps before it are allowed to go red "+
			"and this is the step that reports them. Without it the job ends silently at "+
			"the first failure, which is the state measured on the v0.2.0 publish.",
			verdict.Name, verdict.If)
	}
	if !strings.Contains(verdict.Run, "exit 1") {
		t.Errorf("the last step of the verify job, %q, has no way to fail.\n"+
			"Every check before it carries continue-on-error, so this step is the only "+
			"thing that can turn a red check into a red job. A gate that cannot fail is "+
			"not a gate.", verdict.Name)
	}

	// Where the checks start. Anything after the setup and before the verdict
	// is a check and has to be collected.
	start := -1
	for i, s := range job.Steps {
		if s.Name == releaseSetupEndsAfter {
			start = i + 1
			break
		}
	}
	if start < 0 {
		t.Fatalf("no step of the verify job is called %q, so this guard cannot tell setup "+
			"from checks. Rename it back or teach this guard the new name.", releaseSetupEndsAfter)
	}

	checks := job.Steps[start : len(job.Steps)-1]
	if len(checks) == 0 {
		t.Fatal("the verify job has no check steps between the download and the verdict, " +
			"so the loop below proved nothing")
	}

	// Everything the verdict reads, in one string, so a missing wire is a
	// missing substring rather than a guess about how it was written.
	wiring := strings.Join(values(verdict.Env), "\n")

	for _, s := range checks {
		if !s.ContinueOnError {
			t.Errorf("the check %q stops the job when it fails, so every check after it is "+
				"skipped and its question goes unasked.\n"+
				"Give it continue-on-error: true and an id, and read that id in %q.",
				s.Name, verdict.Name)
			continue
		}
		if s.ID == "" {
			t.Errorf("the check %q is allowed to fail without stopping the job and has no id, "+
				"so nothing can read how it went. Its failure would be invisible, which is "+
				"worse than stopping the job.", s.Name)
			continue
		}

		want := "steps." + s.ID + ".outcome"
		if !strings.Contains(wiring, want) {
			t.Errorf("the check %q (id %q) is allowed to fail and %q never reads it.\n"+
				"Add %s to that step's env. A check whose result nothing reads is a check "+
				"whose failure is a warning nobody sees.",
				s.Name, s.ID, verdict.Name, want)
		}
	}

	// conclusion is the field continue-on-error rewrites to success. Reading it
	// here would make the verdict report every check as passing, whatever
	// happened, and the job would be green for the rest of its life.
	if strings.Contains(wiring, ".conclusion") {
		t.Errorf("%q reads conclusion for at least one check.\n"+
			"continue-on-error rewrites conclusion to success and leaves outcome alone, so "+
			"this verdict would pass however the checks went. Read outcome.\n"+
			"  it reads: %s", verdict.Name, wiring)
	}
}

func jobNames(w releaseWorkflow) []string {
	out := make([]string, 0, len(w.Jobs))
	for name := range w.Jobs {
		out = append(out, name)
	}
	return out
}

func values(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}
