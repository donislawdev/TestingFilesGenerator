package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goccy/go-yaml"
)

// Every checkout in a workflow turns the job's token off once the tree is on
// disk.
//
// actions/checkout writes the token it cloned with into .git/config and leaves
// it there, so that a later step can push. No step in these workflows pushes:
// the release, the attestation and the pages talk to GitHub through gh with a
// token in the environment, or through actions that carry their own. What the
// jobs do after a checkout is run go test over the code of the pull request
// under test - and that code can read .git/config. The token of a pull request
// from a fork is read only, so the class is real and the weight is small, and
// it is the same class on every one of the twenty four checkouts, which is why
// the answer is every one of them and a guard, not a fix at the job somebody
// happened to review (O230).
//
// Asked through the YAML parser rather than by searching the text, because
// `with:` can sit under a comment block and the key can sit anywhere inside
// it - a regular expression tying the key to the line after `uses:` would read
// pages.yml wrong today. The text is still read, once, for a second count of
// the checkouts: a parser that stopped seeing some of the steps would
// otherwise report a clean tree over the ones it dropped. Two readers that
// have to agree replace a number somebody would keep by hand - an outside
// review of #118 asked for an exact count of twenty four, and that constant
// would go stale with the next workflow while proving nothing the agreement
// does not.
func TestEveryCheckoutTurnsItsTokenOff(t *testing.T) {
	// Asking the predicate about shapes the tree does not currently contain.
	// Every checkout DOES turn its token off today, so a change that weakened
	// the rule would find nothing to let through and stay green. These cases
	// keep a hold on the rule itself.
	for _, c := range []struct {
		with map[string]any
		off  bool
		why  string
	}{
		{map[string]any{"persist-credentials": false}, true, "the key is there and false"},
		{map[string]any{"persist-credentials": "false"}, true, "the action reads its inputs as strings, so a quoted false is the same answer"},
		{nil, false, "no with block at all means the default, which keeps the token"},
		{map[string]any{"fetch-depth": 0}, false, "a with block that says nothing about the token keeps it"},
		{map[string]any{"persist-credentials": true}, false, "the key is there and true"},
		{map[string]any{"persist-credentials": "no"}, false, "a word that is not false is not false"},
	} {
		if tokenTurnedOff(c.with) != c.off {
			t.Errorf("tokenTurnedOff(%v) should be %v, because %s", c.with, c.off, c.why)
		}
	}

	dir := filepath.Join(repoRoot(t), ".github", "workflows")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("the workflows are not here: %v", err)
	}

	seen, inText := 0, 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || (!strings.HasSuffix(name, ".yml") && !strings.HasSuffix(name, ".yaml")) {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		inText += strings.Count(withoutYamlComments(string(body)), "uses: actions/checkout@")
		var workflow struct {
			Jobs map[string]struct {
				Steps []struct {
					Uses string         `yaml:"uses"`
					With map[string]any `yaml:"with"`
				} `yaml:"steps"`
			} `yaml:"jobs"`
		}
		if err := yaml.Unmarshal(body, &workflow); err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		for jobName, job := range workflow.Jobs {
			for _, step := range job.Steps {
				if !strings.HasPrefix(step.Uses, "actions/checkout@") {
					continue
				}
				seen++
				if tokenTurnedOff(step.With) {
					continue
				}
				t.Errorf("%s, job %q checks out with the token left in .git/config, and the steps "+
					"after it run the pull request's own code. Nothing in these workflows pushes, "+
					"so turn it off:\n"+
					"    with:\n"+
					"      persist-credentials: false",
					name, jobName)
			}
		}
	}

	// The two readings have to agree, and the text has to have found
	// something. A parser that stopped seeing steps - a renamed key, a
	// changed suffix, a shape it does not decode - would otherwise report a
	// clean tree over the checkouts it dropped, which is the way this guard
	// is most likely to break. Measured 2026-09-22: twenty four, by both.
	if inText == 0 {
		t.Errorf("no checkout was found in the text under %s, so this guard checked nothing. "+
			"Either the workflows moved or the way this reads them stopped working", dir)
	}
	if seen != inText {
		t.Errorf("the YAML walk found %d checkouts and the text holds %d. The walk is the one "+
			"that judges them, so every checkout it does not see is one it does not ask "+
			"- find out which shape it stopped decoding", seen, inText)
	}
}

// tokenTurnedOff reports whether a checkout's with block says
// persist-credentials: false.
//
// A bare false parses as a boolean and a quoted one as a string, and the
// action reads either as false, so both are accepted. Anything else - the key
// missing, true, or a word - leaves the token where the action puts it.
func tokenTurnedOff(with map[string]any) bool {
	switch v := with["persist-credentials"].(type) {
	case bool:
		return !v
	case string:
		return v == "false"
	}
	return false
}
