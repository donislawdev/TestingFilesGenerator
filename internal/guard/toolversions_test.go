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

// toolVersionsDriver runs one function of .github/scripts/tool_versions.py
// without the network, so what the weekly job reads out of the workflows can
// be asked here. It prints JSON: {"pins": {...}} or {"error": "..."}.
const toolVersionsDriver = `
import json, sys
sys.path.insert(0, sys.argv[1])
import tool_versions
texts = json.load(open(sys.argv[2], encoding="utf-8"))
if texts is None:
    texts = tool_versions.workflow_text()
try:
    print(json.dumps({"pins": tool_versions.npm_pins(texts)}))
except RuntimeError as exc:
    print(json.dumps({"error": str(exc)}))
`

// npmPinsOf asks the script which npm pins it reads out of texts, or out of
// the real workflows when texts is nil.
func npmPinsOf(t *testing.T, python string, texts map[string]string) (map[string]string, string) {
	t.Helper()
	dir := t.TempDir()
	in := filepath.Join(dir, "texts.json")
	body, err := json.Marshal(texts)
	if err != nil {
		t.Fatalf("building the texts: %v", err)
	}
	if err := os.WriteFile(in, body, 0o600); err != nil {
		t.Fatalf("writing the texts: %v", err)
	}
	scripts := filepath.Join(repoRoot(t), ".github", "scripts")
	out, err := exec.Command(python, "-B", "-c", toolVersionsDriver, scripts, in).CombinedOutput()
	if err != nil {
		t.Fatalf("running tool_versions.npm_pins: %v\n%s", err, out)
	}
	var answer struct {
		Pins  map[string]string `json:"pins"`
		Error string            `json:"error"`
	}
	if err := json.Unmarshal(out, &answer); err != nil {
		t.Fatalf("reading what tool_versions.npm_pins printed: %v\n%s", err, out)
	}
	return answer.Pins, answer.Error
}

// The weekly check reads npm install as a command, not as words in a sentence.
//
// Measured on 2026-11-24: the runs of 2026-11-16 and 2026-11-23 failed on
// `echo "::error::... would make npm install give people an older tfg"` in
// npm.yml, and that one failure ended the run before it read a single pin, so
// for two weeks the job checked nothing while saying only that.
//
// Its own canary: an install that names no version must still fail, because a
// pin taken off a command would otherwise take the package off the check.
func TestTheToolVersionCheckReadsNpmInstallAsACommand(t *testing.T) {
	python := pythonForGate(t)

	real, failure := npmPinsOf(t, python, nil)
	if failure != "" {
		t.Fatalf("the weekly check fails on the workflows as they are: %s", failure)
	}
	if len(real) == 0 {
		t.Fatal("the weekly check found no npm pin in the workflows, so it would compare nothing with the registry")
	}

	for _, c := range []struct {
		why   string
		line  string
		pins  map[string]string
		fails bool
	}{
		{"a message that mentions npm install",
			`echo "::error::staging it would make npm install give people an older tfg"`, map[string]string{}, false},
		{"a step name that mentions npm",
			`- name: install from the files, the way npm installs from the registry`, map[string]string{}, false},
		{"files this run built",
			`npm install --no-audit --no-fund "../tarballs/$platform" "$main"`, map[string]string{}, false},
		{"a pin after an option that takes a value",
			`npm install --prefix "$RUNNER_TEMP/mailparser" mailparser@3.10.0`, map[string]string{"mailparser": "3.10.0"}, false},
		{"a scoped pin after another command",
			`run: cd web && npm install @scope/pkg@1.2.3`, map[string]string{"@scope/pkg": "1.2.3"}, false},
		{"a quoted path that holds a semicolon",
			`npm install --prefix "$RUNNER_TEMP/a;b" mailparser@3.10.0`, map[string]string{"mailparser": "3.10.0"}, false},
		{"a tarball named without a path",
			`npm install fixture.tgz mailparser@3.10.0`, map[string]string{"mailparser": "3.10.0"}, false},
		{"a pin after a redirection",
			`npm install >install.log 2>&1 mailparser@3.10.0`, map[string]string{"mailparser": "3.10.0"}, false},
		{"an install that names no version after a redirection", `npm install >install.log mailparser`, nil, true},
		{"a scoped package whose name ends like a tarball", `npm install @scope/fixture.tgz`, nil, true},
		{"a comment after the pin",
			`npm install mailparser@3.10.0  # the version the tests read`, map[string]string{"mailparser": "3.10.0"}, false},
		{"a step name with an apostrophe that mentions npm install",
			`- name: npm install can't reach the registry from here`, map[string]string{}, false},
		{"an install that names no version", `npm install mailparser`, nil, true},
		{"an install that names no version after run:", `- run: npm install --prefix x mailparser`, nil, true},
	} {
		pins, failure := npmPinsOf(t, python, map[string]string{"x.yml": "steps:\n  " + c.line + "\n"})
		if c.fails {
			if failure == "" {
				t.Errorf("%s: %q passed, and a package without a version has to fail the check", c.why, c.line)
			} else if !strings.Contains(failure, "pins no version") {
				t.Errorf("%s: %q failed for another reason: %s", c.why, c.line, failure)
			}
			continue
		}
		if failure != "" {
			t.Errorf("%s: %q failed the check: %s", c.why, c.line, failure)
			continue
		}
		if len(pins) != len(c.pins) {
			t.Errorf("%s: %q gave the pins %v, and it has %v", c.why, c.line, pins, c.pins)
			continue
		}
		for k, v := range c.pins {
			if pins[k] != v {
				t.Errorf("%s: %q gave the pins %v, and it has %v", c.why, c.line, pins, c.pins)
			}
		}
	}
	t.Logf("%d npm pin(s) in the workflows: %v", len(real), real)
}

// One check that fails does not hide the others.
//
// Before 2026-11-24 the first failure ended the run, so every pin read after
// it went unread. The canary is the failure itself: the first check is made
// to fail and the pins of every check after it must still be printed.
func TestOneFailingToolVersionCheckLeavesTheOthersRunning(t *testing.T) {
	python := pythonForGate(t)
	driver := `
import sys
sys.path.insert(0, sys.argv[1])
import tool_versions as tv
def broken(*a):
    raise RuntimeError("planted failure")
tv.workflow_text = lambda: {}
tv.go_tools = broken
tv.go_toolchain = lambda texts: [("go toolchain", "1.0", "current", None)]
tv.renderer = lambda now: [("renderer", "2.0", "current", None)]
tv.npm_packages = lambda texts, now: [("pkg (npm)", "3.0", "current", None)]
tv.held = lambda texts: [("python", "4.0", "held", None)]
sys.exit(tv.main())
`
	scripts := filepath.Join(repoRoot(t), ".github", "scripts")
	out, err := exec.Command(python, "-B", "-c", driver, scripts).CombinedOutput()
	code := 0
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("running tool_versions.main: %v\n%s", err, out)
		}
		code = exit.ExitCode()
	}
	if code != 1 {
		t.Errorf("a failed check ended the run with %d, and it has to end with 1\n%s", code, out)
	}
	said := string(out)
	if !strings.Contains(said, "FAIL: planted failure") {
		t.Errorf("the failed check is not named in what the run printed:\n%s", said)
	}
	for _, pin := range []string{"go toolchain", "renderer", "pkg (npm)", "python"} {
		if !strings.Contains(said, pin) {
			t.Errorf("the run did not print %q after the first check failed, so one failure hid it:\n%s", pin, said)
		}
	}
}
