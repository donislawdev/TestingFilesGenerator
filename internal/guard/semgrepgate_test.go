package guard

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// semgrepGatePath is the one place the script is named.
func semgrepGatePath(t *testing.T) string {
	t.Helper()
	return filepath.Join(repoRoot(t), ".github", "scripts", "semgrep_gate.py")
}

// runSemgrepGate writes one report and returns the gate's exit code.
func runSemgrepGate(t *testing.T, python string, report any) int {
	t.Helper()
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("building the report: %v", err)
	}
	path := filepath.Join(t.TempDir(), "semgrep.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("writing the report: %v", err)
	}
	out, err := exec.Command(python, semgrepGatePath(t), path).CombinedOutput()
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

// scanned is the shape of a report that did look at something.
func scanned(results []any, errs []any) map[string]any {
	return map[string]any{
		"results": results,
		"errors":  errs,
		"paths":   map[string]any{"scanned": []string{"a.go", "b.go"}},
	}
}

func finding(severity string) any {
	return map[string]any{
		"check_id": "some.rule",
		"path":     "a.go",
		"start":    map[string]any{"line": 1},
		"extra":    map[string]any{"severity": severity, "message": "something"},
	}
}

// The gate blocks on the severities the scanner's own flag cannot express.
//
// `semgrep --severity` accepts INFO, WARNING and ERROR only, while rules from
// the registry also carry HIGH and CRITICAL. A gate built on that flag would
// quietly ignore exactly the severities it was asked to block, which is why the
// decision is made from the report here instead.
func TestTheSemgrepGateBlocksTheSeveritiesTheFlagCannotExpress(t *testing.T) {
	python := pythonForGate(t)

	for _, c := range []struct {
		severity string
		want     int
	}{
		{"ERROR", 1},
		{"HIGH", 1},
		{"CRITICAL", 1},
		{"WARNING", 0},
		{"INFO", 0},
	} {
		got := runSemgrepGate(t, python, scanned([]any{finding(c.severity)}, nil))
		if got != c.want {
			t.Errorf("a %s finding made the gate exit %d, and it has to exit %d",
				c.severity, got, c.want)
		}
	}
}

// A rule that could not run is not a rule that found nothing.
//
// Only an entry at level "error" counts. Measured on this repository: a healthy
// run still carries entries at level "warn", all of them semgrep declining to
// parse a shell snippet inside a workflow, and stopping on those would make the
// gate fire on every run.
func TestASemgrepScanErrorIsNotACleanScan(t *testing.T) {
	python := pythonForGate(t)

	fatal := map[string]any{"level": "error", "type": "SemgrepError", "message": "a rule failed"}
	if got := runSemgrepGate(t, python, scanned(nil, []any{fatal})); got != 1 {
		t.Errorf("a scan error exited %d, and it has to exit 1.\n"+
			"'the scan was green' must never be able to mean 'the scan did not happen'.", got)
	}

	noisy := map[string]any{"level": "warn", "type": "PartialParsing", "message": "a snippet"}
	if got := runSemgrepGate(t, python, scanned(nil, []any{noisy})); got != 0 {
		t.Errorf("a warning-level scan entry exited %d, and it has to exit 0.\n"+
			"This repository produces those on every healthy run.", got)
	}
}

// A scanner that fails can succeed, so the report is the answer.
//
// Measured on the development machine 2026-08-27, semgrep 1.175.0 on Windows:
// the scan failed its own rule validation, printed four RPC errors, EXITED 0,
// and wrote 23 bytes that were not JSON. A gate reading the exit code would
// have called that a clean tree.
//
// The second case is the quiet one. A wrong working directory writes a valid
// report with an empty result list, and every reader downstream would call that
// a pass.
func TestTheSemgrepGateFailsClosedWhenTheScanDidNotHappen(t *testing.T) {
	python := pythonForGate(t)

	missing := filepath.Join(t.TempDir(), "was-never-written.json")
	err := exec.Command(python, semgrepGatePath(t), missing).Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("the gate accepted a report that does not exist, err=%v", err)
	}
	if exit.ExitCode() != 2 {
		t.Errorf("an unreadable report exited %d, and it has to exit 2", exit.ExitCode())
	}

	nothingRead := map[string]any{
		"results": []any{}, "errors": []any{},
		"paths": map[string]any{"scanned": []string{}},
	}
	if got := runSemgrepGate(t, python, nothingRead); got != 2 {
		t.Errorf("a report saying no file was scanned exited %d, and it has to exit 2.\n"+
			"An empty result list from a scan that read nothing is not a clean tree.", got)
	}
}

// CI has to run the scanner and decide from what it wrote.
func TestCIRunsSemgrepAndDecidesFromTheReportRatherThanTheExitCode(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("reading ci.yml: %v", err)
	}
	text := withoutYamlComments(string(body))

	for _, want := range []struct{ needle, why string }{
		{"requirements-semgrep.txt", "the version has to come from somewhere, and that file is where it is"},
		{"--config p/default", "the rules come from the registry rather than from this repository"},
		{"--metrics=off", "this repository sends no telemetry about its own source"},
		{".github/scripts/semgrep_gate.py", "the report is what decides, not the scanner's exit code"},
	} {
		if !strings.Contains(text, want.needle) {
			t.Errorf("the semgrep job never mentions %q, and %s", want.needle, want.why)
		}
	}

	// And the file that name points at really pins a version.
	//
	// This used to be one check: the job itself had to carry "semgrep==". The
	// pin moved out on 2026-09-02 so that Dependabot could see it - a version
	// written into a workflow step is watched by nothing, which is how
	// staticcheck came to sit two releases behind a compiler it could not read.
	//
	// Splitting it in two rather than dropping it, because the thing being
	// guarded never changed: an unpinned analyser turns somebody else's release
	// into a red build on a commit that changed nothing. Asking only for the
	// file name would let an empty file pass.
	req, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "requirements-semgrep.txt"))
	if err != nil {
		t.Fatalf("the semgrep job installs from a requirements file that is not here: %v", err)
	}
	if !regexp.MustCompile(`(?m)^semgrep==\d`).Match(req) {
		t.Errorf("requirements-semgrep.txt does not pin semgrep to an exact version, and %s\n  %s",
			"an unpinned analyser turns somebody else's release into a red build", req)
	}
}
