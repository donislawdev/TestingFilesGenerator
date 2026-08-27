package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ciHeader is everything above the jobs, with comments removed.
//
// Scoped rather than searched whole, because a trigger is a claim about when
// the suite runs and a match anywhere else in a six hundred line file would
// satisfy a careless assertion without meaning anything.
func ciHeader(t *testing.T) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("reading ci.yml: %v", err)
	}
	text := withoutYamlComments(string(body))
	head, _, found := strings.Cut(text, "\njobs:")
	if !found {
		t.Fatalf("ci.yml has no jobs block, so this guard cannot tell a trigger from a job")
	}
	return head
}

// The suite runs once for one change.
//
// Measured on pull request 4, the first this project had: every job appeared
// twice, once for the branch push and once for the pull request event, and the
// race detector ran on both at about ten minutes each. An unfiltered `push:`
// means every branch push runs the whole suite, and every branch here now
// reaches main through a pull request, which runs it again.
//
// The pull request trigger is asserted alongside, because removing that instead
// would also stop the doubling and would be much worse: a pull request from a
// fork would never be checked at all, and the contexts the branch ruleset
// requires would never report.
func TestTheSuiteRunsOncePerChange(t *testing.T) {
	head := ciHeader(t)

	if !strings.Contains(head, "branches: [main]") {
		t.Error("ci.yml runs on every push to every branch.\n" +
			"With work reaching main through pull requests, that runs the whole suite " +
			"twice for one change - measured on pull request 4, including the race " +
			"detector at about ten minutes a time.\n" +
			"Limit the push trigger: `push:` with `branches: [main]` under it.")
	}
	if !strings.Contains(head, "pull_request:") {
		t.Error("ci.yml no longer runs on pull_request.\n" +
			"That is not a way to stop the suite running twice. A pull request from a " +
			"fork would go unchecked, and the contexts the branch ruleset requires " +
			"would never report, so nothing could be merged.")
	}
}

// The race detector is asked about the right pair of commits.
//
// It runs when one of the three files concurrency lives in has changed, decided
// on 2026-08-20 after it measured 10m31s against about a minute for the rest.
// The comparison used `github.event.before`, which belongs to a push and is
// empty on a pull request - so from the day this project started taking pull
// requests, every one of them answered "touched" and the detector was back to
// running on everything. The decision was undone by a change somewhere else,
// which is the failure this project keeps meeting.
//
// On a pull request the commit to compare against is the base, and github.sha
// is the merge commit, so the difference between them is what the pull request
// changes.
func TestTheRaceDetectorAsksAboutWhatThePullRequestChanges(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("reading ci.yml: %v", err)
	}
	text := withoutYamlComments(string(body))

	if !strings.Contains(text, "github.event.pull_request.base.sha") {
		t.Error("the job deciding whether the race detector runs never looks at the " +
			"pull request's base.\n" +
			"github.event.before belongs to a push and is empty on a pull request, so " +
			"the comparison falls through to 'anything unclear counts as touched' and " +
			"the detector runs on every pull request - which is the thing 2026-08-20 " +
			"decided it should stop doing.")
	}

	// The fallback has to survive too. A push to main has no pull request, and
	// dropping `before` would send every push down the unclear branch instead -
	// the same defect facing the other way.
	if !strings.Contains(text, "github.event.before") {
		t.Error("the comparison no longer falls back to github.event.before, so a push " +
			"to main has nothing to compare against and counts as touched every time.")
	}
}
