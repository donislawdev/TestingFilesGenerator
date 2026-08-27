package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Publishing the website happens for a push to this repository, and nothing else.
//
// The job that deploys web/public triggers on workflow_run. That event fires in
// the BASE repository, with the base repository's permissions, and this job
// holds pages: write. CI runs on pull_request, and the branches filter on
// workflow_run matches the branch NAME of the run that triggered it - which for
// a pull request from a fork is a name the fork chose. A fork whose branch was
// called main, opening a pull request that passed CI, would otherwise satisfy
// every condition this job had before 2026-08-27.
//
// Nothing in the job executes repository code. It checks three files exist and
// uploads a directory, so what was reachable was the content of the published
// site rather than a secret. That is defacement of the project website, which is
// worth a condition.
//
// Read as text rather than by parsing the expression, because what matters is
// that both questions are asked at all. A YAML parser would hand back one
// string and this would still have to look inside it.
func TestTheSiteIsPublishedOnlyForAPushToThisRepository(t *testing.T) {
	path := filepath.Join(repoRoot(t), ".github", "workflows", "pages.yml")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("the publishing workflow is not here: %v", err)
	}
	text := withoutYamlComments(string(body))

	// The trigger this is all about. If it ever stops being workflow_run the
	// conditions below are answering a question nobody asked any more, and this
	// guard should be revisited rather than quietly kept passing.
	if !strings.Contains(text, "workflow_run:") {
		t.Fatalf("pages.yml no longer triggers on workflow_run, so this guard is " +
			"defending a shape that is gone. Read it again before deleting it.")
	}

	for _, want := range []struct{ needle, why string }{
		{
			"head_repository.full_name == github.repository",
			"without it a fork's pull request run can reach a job that publishes the website",
		},
		{
			"workflow_run.event == 'push'",
			"a run triggered by a pull request must never publish, whoever opened it",
		},
		{
			"workflow_run.conclusion == 'success'",
			"a red suite means the site does not move, which is why this job waits for CI at all",
		},
	} {
		if !strings.Contains(text, want.needle) {
			t.Errorf("the publish job does not ask for %q.\n"+
				"That condition is there because %s.\n"+
				"This job runs with the base repository's permissions and holds pages: write.",
				want.needle, want.why)
		}
	}
}
