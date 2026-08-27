package guard

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// pythonForGate finds an interpreter, or says why it could not.
//
// The gate is a script rather than Go because it reads an answer `gh` fetches
// in the same job, and CI runners carry python. Here it may genuinely be
// missing, and a skip that names the reason beats a guard that quietly passes.
func pythonForGate(t *testing.T) string {
	t.Helper()
	for _, name := range []string{"python", "python3"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	t.Skip("neither python nor python3 is on PATH, so the gate cannot be asked anything")
	return ""
}

// gatePath is the one place the script is named, so a move breaks one line.
func gatePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), ".github", "scripts", "dependency_gate.py")
}

// runGate writes one dependency review answer and returns the gate's exit code.
func runGate(t *testing.T, python string, review any) int {
	t.Helper()
	body, err := json.Marshal(review)
	if err != nil {
		t.Fatalf("building the review: %v", err)
	}
	path := filepath.Join(t.TempDir(), "deps.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("writing the review: %v", err)
	}
	out, err := exec.Command(python, gatePath(t), path).CombinedOutput()
	if err == nil {
		return 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	t.Fatalf("running the gate: %v\n%s", err, out)
	return -1
}

type dep map[string]any

// The gate answers for a licence rather than only for a vulnerability.
//
// actions/dependency-review-action fails a pull request that adds a dependency
// with a known vulnerability and says plainly that it will NOT fail on a licence
// it could not identify. This project is GPL-3.0 and ships a binary, so "we
// could not tell" is the one answer nobody can act on, and the gate beside the
// action exists for exactly that half.
//
// Asked here by running it, because a list of allowed identifiers read back by
// a test would only agree with itself.
func TestTheDependencyGateRefusesALicenceThisProjectCannotDistribute(t *testing.T) {
	python := pythonForGate(t)

	for _, c := range []struct {
		what   string
		review []dep
		want   int
	}{
		{
			"a permissive licence we already carry",
			[]dep{{"change_type": "added", "ecosystem": "gomod",
				"name": "example.com/fine", "version": "v1.0.0", "license": "MIT"}},
			0,
		},
		{
			// Measured on this repository 2026-08-27: golang.org/x/sys, x/net
			// and x/image all report this pair, because the Go project ships a
			// PATENTS file beside its licence. Three modules already in the tree
			// would be blocked if the second half were not allowed.
			"the Go patents grant, which adds rights rather than restricting them",
			[]dep{{"change_type": "added", "ecosystem": "gomod",
				"name": "golang.org/x/sys", "version": "v0.47.0",
				"license": "BSD-3-Clause AND LicenseRef-scancode-google-patent-license-golang"}},
			0,
		},
		{
			"a licence GitHub could not determine, which is the case the action skips",
			[]dep{{"change_type": "added", "ecosystem": "gomod",
				"name": "example.com/mystery", "version": "v1.0.0", "license": nil}},
			1,
		},
		{
			// The one identifier the licence policy names as a blocker rather
			// than a formality, because it cannot be combined with GPL-3.0.
			"GPL-2.0-only",
			[]dep{{"change_type": "added", "ecosystem": "gomod",
				"name": "example.com/gpl2", "version": "v1.0.0", "license": "GPL-2.0-only"}},
			1,
		},
		{
			"a compound expression where one half is denied",
			[]dep{{"change_type": "added", "ecosystem": "gomod",
				"name": "example.com/half", "version": "v1.0.0", "license": "MIT AND GPL-2.0-only"}},
			1,
		},
		{
			// An action never reaches a user, so it creates no distribution
			// obligation. What actions are held to instead is stricter and sits
			// in TestEveryActionIsPinnedToACommitRatherThanATag.
			"an action, for which GitHub reports no licence at all",
			[]dep{{"change_type": "added", "ecosystem": "actions",
				"name": "actions/checkout", "version": "v7", "license": nil}},
			0,
		},
		{
			"a dependency being REMOVED, which creates no obligation",
			[]dep{{"change_type": "removed", "ecosystem": "gomod",
				"name": "example.com/gpl2", "version": "v1.0.0", "license": "GPL-2.0-only"}},
			0,
		},
	} {
		if got := runGate(t, python, c.review); got != c.want {
			t.Errorf("the gate answered %d for %s, and it has to answer %d", got, c.what, c.want)
		}
	}
}

// A gate that cannot read its answer has not passed anything.
//
// The dependency graph endpoint refuses some repository shapes, and a 403 has
// to fail the run rather than look like an empty list of problems. This is the
// same shape as the semgrep gate beside it, and both were written that way
// because a scanner that fails can exit zero.
func TestTheDependencyGateFailsClosedWhenItCannotReadTheAnswer(t *testing.T) {
	python := pythonForGate(t)
	missing := filepath.Join(t.TempDir(), "was-never-written.json")

	err := exec.Command(python, gatePath(t), missing).Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("the gate accepted an answer that does not exist, err=%v", err)
	}
	if exit.ExitCode() != 2 {
		t.Errorf("an unreadable answer exits %d, and it has to exit 2.\n"+
			"Exit 1 would read as 'found problems' and exit 0 as 'there were none', "+
			"and neither is true when the gate never saw the data.", exit.ExitCode())
	}
}

// CI has to actually ask the gate, on the event where the question exists.
//
// The action compares the base and the head of a pull request, so it is
// meaningful on pull_request only. A gate nothing runs is a file, not a gate.
func TestCIAsksTheDependencyGateOnEveryPullRequest(t *testing.T) {
	path := filepath.Join(repoRoot(t), ".github", "workflows", "dependency-review.yml")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the dependency review workflow: %v", err)
	}
	text := withoutYamlComments(string(body))

	for _, want := range []struct{ needle, why string }{
		{"pull_request", "the manifests of a base and a head only exist on a pull request"},
		{"dependency-review-action", "the vulnerability half comes from the action"},
		{".github/scripts/dependency_gate.py", "the licence half comes from the gate beside it"},
		{"fail-on-severity: moderate", "anything at or above moderate has to fail rather than warn"},
	} {
		if !strings.Contains(text, want.needle) {
			t.Errorf("the dependency review workflow never mentions %q, and %s", want.needle, want.why)
		}
	}
}
