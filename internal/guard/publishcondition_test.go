package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

// Publishing the website happens for a push to this repository after CI passed
// on it, and for nothing else.
//
// The jobs of pages.yml trigger on workflow_run, and the deploy job that
// publishes runs only after them. The event fires in the BASE repository, with
// the base repository's permissions, and the deploy job holds pages: write. CI
// runs on pull_request, and the branches filter on workflow_run matches the
// branch NAME of the run that triggered it - which for a pull request from a
// fork is a name the fork chose. A fork whose branch was called main, opening a
// pull request that passed CI, would otherwise satisfy every condition the
// upload job had before 2026-08-27.
//
// Whether CI passed is the ci job's question, answered by site_gate.py with
// the jobs on macOS left out (O371). Every job that checks out the commit CI
// ran on - the one that makes the sample files and the one that uploads the
// pages - waits for that answer and asks the two questions about the push in
// its own condition.
//
// Read job by job rather than over the whole file. Each condition is written
// in more than one job, so a search of the file finds the copy that is left
// when one job loses its own.
func TestTheSiteIsPublishedOnlyForAPushToThisRepository(t *testing.T) {
	path := filepath.Join(repoRoot(t), ".github", "workflows", "pages.yml")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("the publishing workflow is not here: %v", err)
	}

	// The trigger this is all about. If it ever stops being workflow_run the
	// conditions below are answering a question nobody asked any more, and this
	// guard should be revisited rather than quietly kept passing.
	if !strings.Contains(withoutYamlComments(string(body)), "workflow_run:") {
		t.Fatalf("pages.yml no longer triggers on workflow_run, so this guard is " +
			"defending a shape that is gone. Read it again before deleting it.")
	}
	for _, p := range publishConditionProblems(body) {
		t.Errorf("pages.yml: %s.\nThe jobs run with the base repository's permissions, and the deploy job after them holds pages: write.", p)
	}

	// The canaries. The first is sound and has to pass, so the rest are not
	// the check refusing every workflow.
	sound := publishCanary(pushHere, "needs.ci.outputs.publish == 'true' && "+pushHere, "ci")
	if p := publishConditionProblems(sound); len(p) != 0 {
		t.Errorf("the canary of a sound workflow was refused: %v", p)
	}
	for _, c := range []struct{ name, gateIf, buildIf, needs string }{
		{"a job that builds the site does not ask whose repository it is", pushHere,
			"needs.ci.outputs.publish == 'true' && github.event.workflow_run.event == 'push'", "ci"},
		{"a job that builds the site does not wait for the verdict", pushHere, pushHere, "ci"},
		{"a job that builds the site does not need the ci job", pushHere,
			"needs.ci.outputs.publish == 'true' && " + pushHere, "other"},
		{"a job that builds the site runs whatever the verdict was", pushHere,
			"always() && needs.ci.outputs.publish == 'true' && " + pushHere, "ci"},
		{"the ci job does not ask whether the run was a push",
			"github.event.workflow_run.head_repository.full_name == github.repository",
			"needs.ci.outputs.publish == 'true' && " + pushHere, "ci"},
	} {
		if len(publishConditionProblems(publishCanary(c.gateIf, c.buildIf, c.needs))) == 0 {
			t.Errorf("the canary %q passed, so the check cannot see that", c.name)
		}
	}
}

// pushHere is the condition that the run which triggered the site was a push,
// to this repository rather than a fork.
const pushHere = "github.event.workflow_run.event == 'push' && " +
	"github.event.workflow_run.head_repository.full_name == github.repository"

// pushQuestions is what each job asks of the run that triggered it, and why.
var pushQuestions = []struct{ needle, why string }{
	{"github.event.workflow_run.head_repository.full_name == github.repository",
		"without it a fork's pull request run can reach a job that publishes the website"},
	{"github.event.workflow_run.event == 'push'",
		"a run triggered by a pull request must never publish, whoever opened it"},
}

// checksOutTheRunsCommit tells whether a job checks out the commit CI ran on.
func checksOutTheRunsCommit(job pagesJob) bool {
	for _, s := range job.Steps {
		if ref, _ := s.With["ref"].(string); strings.HasPrefix(s.Uses, "actions/checkout@") &&
			strings.Contains(ref, "workflow_run.head_sha") {
			return true
		}
	}
	return false
}

// gateProblems is what is wrong with the ci job: it has to ask about the push,
// give its verdict as an output, hand site_gate.py the run and its commit, and
// judge with the script of main rather than of the commit it judges.
func gateProblems(name string, job pagesJob) []string {
	var problems []string
	for _, q := range pushQuestions {
		if !strings.Contains(job.If, q.needle) {
			problems = append(problems, fmt.Sprintf("the job %s does not ask for %q, and %s", name, q.needle, q.why))
		}
	}
	if job.Outputs["publish"] == "" {
		problems = append(problems, "the job "+name+" gives no publish output, so no job can wait for its verdict")
	}
	if checksOutTheRunsCommit(job) {
		problems = append(problems, "the job "+name+" checks out the commit it judges, so a branch's own script would judge it")
	}
	for _, s := range job.Steps {
		if !strings.Contains(s.Run, "site_gate.py") {
			continue
		}
		env := ""
		for _, v := range s.Env {
			env += v + "\n"
		}
		for _, want := range []string{"github.event.workflow_run.id", "github.event.workflow_run.head_sha"} {
			if !strings.Contains(env, want) {
				problems = append(problems, "the step of "+name+" that asks site_gate.py is not given "+want)
			}
		}
	}
	return problems
}

// publishConditionProblems is what is wrong with how a workflow decides to
// publish the site: nothing when one job asks site_gate.py, and every job
// that checks out the commit CI ran on waits for its verdict and asks about
// the push itself.
func publishConditionProblems(body []byte) []string {
	var workflow struct {
		Jobs map[string]pagesJob `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(body, &workflow); err != nil {
		return []string{"the workflow does not parse: " + err.Error()}
	}
	var gates []string
	for name, job := range workflow.Jobs {
		for _, s := range job.Steps {
			if strings.Contains(s.Run, "site_gate.py") {
				gates = append(gates, name)
				break
			}
		}
	}
	if len(gates) != 1 {
		return []string{fmt.Sprintf("%d jobs ask site_gate.py whether the site may go out, and one has to", len(gates))}
	}
	gate := gates[0]
	problems := gateProblems(gate, workflow.Jobs[gate])
	built := 0
	for name, job := range workflow.Jobs {
		if name == gate || !checksOutTheRunsCommit(job) {
			continue
		}
		built++
		for _, q := range pushQuestions {
			if !strings.Contains(job.If, q.needle) {
				problems = append(problems, fmt.Sprintf("the job %s does not ask for %q, and %s", name, q.needle, q.why))
			}
		}
		if !needsOf(job)[gate] || !strings.Contains(job.If, "needs."+gate+".outputs.publish == 'true'") {
			problems = append(problems, "the job "+name+" does not wait for the verdict of "+gate+
				", and a red suite outside macOS means the site does not move")
		}
		for _, f := range []string{"always()", "failure()", "cancelled()"} {
			if strings.Contains(job.If, f) {
				problems = append(problems, "the job "+name+" asks "+f+", so it runs whatever "+gate+" decided")
			}
		}
	}
	if built == 0 {
		problems = append(problems, "no job checks out the commit CI ran on, so nothing here builds the site and this check reads nothing")
	}
	return problems
}

// publishCanary is a workflow with a ci job and one job that builds the site
// from the commit CI ran on, under the conditions given.
func publishCanary(gateIf, buildIf, buildNeeds string) []byte {
	return []byte(`on:
  workflow_run:
    workflows: ["CI"]
jobs:
  ci:
    if: "` + gateIf + `"
    outputs:
      publish: ${{ steps.verdict.outputs.publish }}
    steps:
      - uses: actions/checkout@0000000000000000000000000000000000000000
      - id: verdict
        env:
          RUN_ID: ${{ github.event.workflow_run.id }}
          HEAD_SHA: ${{ github.event.workflow_run.head_sha }}
        run: python3 .github/scripts/site_gate.py jobs.tsv "$HEAD_SHA" tip
  build:
    needs: ` + buildNeeds + `
    if: "` + buildIf + `"
    steps:
      - uses: actions/checkout@0000000000000000000000000000000000000000
        with:
          ref: ${{ github.event.workflow_run.head_sha }}
`)
}
