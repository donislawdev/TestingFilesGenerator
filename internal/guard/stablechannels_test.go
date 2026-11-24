package guard

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

// What this defends. npm, PyPI and the container image only ever get stable
// releases. A release candidate stays a prerelease on GitHub, and its
// packages and image are made and tried without being sent anywhere (owner,
// 2026-11-18, docs/RELEASE-CHECK-PLAN.md section 8, O368).
//
// Why it runs the scripts. The decision is one shell step in each workflow,
// the which step, and its text says nothing certain about what bash does with
// a tag. So the step is run the way the runner runs it, with a gh in front of
// it answering the two questions it asks, and what it writes to GITHUB_OUTPUT
// is read back.
//
// The canary is a which step that sends whatever it is given. The same cases
// have to find it sending a release candidate, or they are not looking.

// stableChannel is one place a release can be sent, and the output of its
// which step that says whether it is.
type stableChannel struct {
	workflow string
	key      string
	sending  string // the value of key that sends the release
}

var stableChannels = []stableChannel{
	{workflow: "npm.yml", key: "stage", sending: "true"},
	{workflow: "pypi.yml", key: "index", sending: "pypi"},
	{workflow: "docker.yml", key: "publish", sending: "true"},
}

// stableChannelCase is one way a which step is started.
type stableChannelCase struct {
	name  string
	event string
	tag   string
	sends bool // whether the release has to be sent
	fails bool // whether the step has to stop with an error
}

var stableChannelCases = []stableChannelCase{
	{name: "a stable release published", event: "release", tag: "v0.6.0", sends: true},
	{name: "a release candidate published", event: "release", tag: "v0.7.0-rc1"},
	{name: "a release candidate run by hand", event: "workflow_dispatch", tag: "v0.7.0-rc1", fails: true},
}

// stableChannelGh answers the two questions a which step asks: the latest
// release, and whether a tag is a draft. Anything else is a question this
// guard does not know the answer to, and stops the step.
const stableChannelGh = `#!/bin/sh
case "$1 $2" in
"api repos/"*) echo v0.6.0; exit 0 ;;
"release view") echo false; exit 0 ;;
esac
echo "gh stub: not a question this stub answers: $*" >&2
exit 2
`

func TestAReleaseCandidateIsSentNowhereButGitHub(t *testing.T) {
	bash := scannerTestBash(t)
	for _, ch := range stableChannels {
		canary := "echo \"" + ch.key + "=" + ch.sending + "\" >> \"$GITHUB_OUTPUT\"\n"
		if problems := stableChannelProblems(t, bash, ch, canary); len(problems) == 0 {
			t.Fatalf("canary: a which step that sends everything passed every case for %s, "+
				"so this guard cannot see what the step decides", ch.workflow)
		}
		for _, p := range stableChannelProblems(t, bash, ch, whichStep(t, ch.workflow)) {
			t.Errorf("%s: %s\nWhat to do: a tag with a hyphen is a release candidate. Its which step "+
				"has to send nothing when the release is published, and stop with an error when it is "+
				"run by hand.", ch.workflow, p)
		}
	}

	// TestPyPI, by hand, may still take a release candidate: it is a
	// rehearsal, and nobody installs from it by default.
	out, sent, err := runWhichStep(t, bash, whichStep(t, "pypi.yml"), "workflow_dispatch", "v0.7.0-rc1", "testpypi")
	if err != nil || sent["index"] != "testpypi" {
		t.Errorf("pypi.yml run by hand for a release candidate to testpypi said index=%q (%v), "+
			"and TestPyPI may still take one. It printed:\n%s", sent["index"], err, out)
	}
}

// stableChannelProblems runs one which step through every case and says what
// it got wrong.
func stableChannelProblems(t *testing.T, bash string, ch stableChannel, script string) []string {
	t.Helper()
	var problems []string
	for _, c := range stableChannelCases {
		out, sent, err := runWhichStep(t, bash, script, c.event, c.tag, ch.sending)
		switch {
		case c.fails && err == nil:
			problems = append(problems, c.name+" did not stop with an error")
		case !c.fails && err != nil:
			problems = append(problems, c.name+" stopped with an error ("+err.Error()+"):\n"+out)
		case !c.fails && (sent[ch.key] == ch.sending) != c.sends:
			problems = append(problems, c.name+" set "+ch.key+"="+sent[ch.key])
		}
	}
	return problems
}

// whichStep is the run of the step called which, in the job called which.
func whichStep(t *testing.T, workflow string) string {
	t.Helper()
	var parsed struct {
		Jobs map[string]struct {
			Steps []struct {
				ID  string `yaml:"id"`
				Run string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(workflowText(t, workflow)), &parsed); err != nil {
		t.Fatalf("%s does not parse as YAML: %v", workflow, err)
	}
	for _, step := range parsed.Jobs["which"].Steps {
		if step.ID == "which" && strings.Contains(step.Run, "GITHUB_OUTPUT") {
			return step.Run
		}
	}
	t.Fatalf("%s has no step with id which, writing to GITHUB_OUTPUT, in a job called which, "+
		"so this guard is not reading the decision it describes", workflow)
	return ""
}

// runWhichStep runs a which step for one event and tag, and says what it
// printed, what it wrote to GITHUB_OUTPUT, and how it ended. index is what a
// run of pypi.yml by hand asks for, and the other two ignore it.
func runWhichStep(t *testing.T, bash, script, event, tag, index string) (string, map[string]string, error) {
	t.Helper()
	dir := t.TempDir()
	for name, body := range map[string]string{"gh": stableChannelGh, "which.sh": script} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
	output := filepath.Join(dir, "output")
	release, input := tag, ""
	if event == "workflow_dispatch" {
		release, input = "", tag
	}
	cmd := exec.Command(bash, filepath.ToSlash(filepath.Join(dir, "which.sh")))
	cmd.Env = append(os.Environ(),
		"PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"GH_TOKEN=stub", "GITHUB_REPOSITORY=donislawdev/TestingFilesGenerator",
		"GITHUB_OUTPUT="+filepath.ToSlash(output),
		"EVENT="+event, "RELEASE_TAG="+release, "INPUT_TAG="+input, "INPUT_INDEX="+index)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		t.Fatalf("bash did not run the which step: %v", err)
	}
	sent := map[string]string{}
	written, _ := os.ReadFile(output)
	for _, line := range strings.Split(string(written), "\n") {
		if k, v, ok := strings.Cut(strings.TrimRight(line, "\r"), "="); ok {
			sent[k] = v
		}
	}
	return string(out), sent, err
}
