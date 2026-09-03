package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

// A whole tree test run in a workflow states its own timeout.
//
// Go allows ten minutes PER PACKAGE by default. Nobody chose that, and in this
// repository it is the wrong shape twice over: almost every test lives in one
// package, internal/guard, so the per package limit is effectively the limit
// for the whole suite - and every job here already declares a timeout-minutes
// of its own, which is the number somebody did choose.
//
// When the default fires first, the run does not fail. It PANICS, out of
// whichever test happened to be running when the alarm went off, and the
// output is a goroutine dump rather than a sentence naming anything. This
// project has a written trap for reading that name as the culprit, and it is
// there because the name is innocent.
//
// It has now happened twice. The race detector met it on 2026-08-25 and a long
// comment was written about it - beside that one job. The coverage gate met the
// same thing on 2026-09-03, because a diagnosis recorded at one job does not
// protect the next one. That is what this guard is for: the reasoning is no
// longer kept in a comment next to whichever step learned it.
//
// Measured on 2026-09-03, the run that prompted this: 399 s on ubuntu, 476 s on
// windows, 491 s on macOS, 479 s under coverage instrumentation. Against 600 s.
// And the same branch swung from 479 s to past 600 s between two runs, so the
// margin was smaller than the fleet's own variance.
//
// Only whole tree runs are asked. A targeted -run walks a handful of tests and
// a fuzz step carries -fuzztime, which is its own budget - demanding a flag
// there would be noise, and a guard that cries wolf gets skipped.
func TestEveryWholeTreeTestRunStatesItsOwnTimeout(t *testing.T) {
	steps := workflowRunSteps(t)
	if len(steps) == 0 {
		t.Fatal("no run step was read out of the workflows - this guard would pass against any of them")
	}

	asked := 0
	for _, step := range steps {
		command := strings.Join(strings.Fields(step.run), " ")
		if !strings.Contains(command, "go test ") || !strings.Contains(command, "./...") {
			continue
		}
		asked++
		if strings.Contains(command, "-timeout ") {
			continue
		}
		t.Errorf("%s, step %q runs the whole tree without stating a timeout, so Go's own ten minutes "+
			"a package decides instead of the %s this job declares. A run past it panics out of "+
			"whichever test was running rather than failing at one:\n  %s",
			step.file, step.name, "timeout-minutes", command)
	}

	// The scan has to have found the runs it is judging. Renaming a key, or a
	// folded block this stops parsing, would otherwise leave it green while
	// reading nothing - the same failure it exists to catch.
	if asked < 3 {
		t.Errorf("only %d whole tree test runs were found across the workflows, and there are at least "+
			"three - the matrix, the coverage gate and the release. Either they moved or the way this "+
			"reads them stopped working", asked)
	}
}

// runStep is one step that runs a command, with enough around it to say where.
type runStep struct {
	file string
	name string
	run  string
}

// workflowRunSteps reads every run step of every workflow.
//
// Through the YAML parser rather than by searching the text, because a run
// block can be folded over several lines and the flag this guard asks about
// would then sit on a different line from the command. The parser puts them
// back together, which a regular expression over the file would not.
func workflowRunSteps(t *testing.T) []runStep {
	t.Helper()
	dir := filepath.Join(repoRoot(t), ".github", "workflows")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("the workflows are not here: %v", err)
	}

	var out []runStep
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yml") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		var workflow struct {
			Jobs map[string]struct {
				Steps []struct {
					Name string `yaml:"name"`
					Run  string `yaml:"run"`
				} `yaml:"steps"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal(body, &workflow); err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		for _, job := range workflow.Jobs {
			for _, step := range job.Steps {
				if step.Run == "" {
					continue
				}
				out = append(out, runStep{file: e.Name(), name: step.Name, run: step.Run})
			}
		}
	}
	return out
}
