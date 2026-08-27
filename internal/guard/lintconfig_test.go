package guard

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The linter set is a ratchet, and it is written down twice on purpose.
//
// Nothing otherwise stops a future change from deleting a linter out of
// .golangci.yml to turn a red build green, and a gate that has vanished cannot
// fail to announce itself. That is the same shape as a threshold raised to get
// unblocked, one level up: the number stays honest and the question disappears.
//
// Growing the list is free - add the linter in both places. Shrinking it
// reddens this.
var lintersEnabled = []string{
	"errcheck",
	"errorlint",
	"ineffassign",
	"misspell",
	"nolintlint",
	"gosec",
	"exhaustive",
}

// The three that must NOT appear there, each with the measurement behind it.
//
// This half is ours rather than borrowed, and it exists because all three
// already run somewhere else. Adding one here would not be a second copy of
// the same question - it would be a DIFFERENT question wearing the same name,
// and the difference is the kind that goes unnoticed:
//
//   - staticcheck under golangci-lint does not honour //lint:ignore, so the
//     one refusal in internal/engine/names.go that carries its reason in the
//     code would be reported anyway. It also brings the QF family, which the
//     pinned staticcheck does not run - fourteen findings on the day this was
//     measured, all of them style.
//   - govet under golangci-lint carries analysers that go vet does not.
//   - unused already lives inside the pinned staticcheck. Proven rather than
//     assumed on 2026-08-27: a function nobody calls was put into
//     internal/core and staticcheck reported it as U1000.
//
// Measured 2026-08-27, and written here rather than only in a comment in the
// configuration, because a comment is the thing this guard exists to replace.
var lintersKeptOut = []string{
	"staticcheck",
	"govet",
	"unused",
}

func TestTheLinterSetOnlyEverGrows(t *testing.T) {
	root := repoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, ".golangci.yml"))
	if err != nil {
		t.Fatalf("reading .golangci.yml: %v", err)
	}
	text := strings.ReplaceAll(string(body), "\r\n", "\n")

	// The default set moves between releases of somebody else's tool, so a
	// configuration inheriting it reports something different every year for no
	// reason of ours. Naming every linter is what makes the list above mean
	// anything at all.
	if !strings.Contains(text, "\n  default: none\n") {
		t.Error(".golangci.yml does not say `default: none`, so the set is whatever the tool ships with.\n" +
			"The list in this guard then describes an addition rather than the whole set, and stops being a ratchet.")
	}

	enabled := enabledLinters(text)
	if len(enabled) == 0 {
		t.Fatal("no linter was found in .golangci.yml - this guard would pass without checking anything")
	}

	have := map[string]bool{}
	for _, name := range enabled {
		have[name] = true
	}

	for _, name := range lintersEnabled {
		if !have[name] {
			t.Errorf("%s is written down here and is no longer enabled in .golangci.yml.\n"+
				"  Dropping a linter is a decision, so change this list too - that is what makes it stick.", name)
		}
	}
	for _, name := range enabled {
		if !contains(lintersEnabled, name) {
			t.Errorf("%s is enabled in .golangci.yml and is not written down here.\n"+
				"  Add it to lintersEnabled, which is what stops the next change from quietly removing it.", name)
		}
	}
	for _, name := range lintersKeptOut {
		if have[name] {
			t.Errorf("%s is enabled in .golangci.yml and it must not be - the reason is written beside lintersKeptOut.\n"+
				"  It runs elsewhere already, and here it would answer a different question under the same name.", name)
		}
	}

	sort.Strings(enabled)
	t.Logf("%d linter(s) enabled: %s", len(enabled), strings.Join(enabled, ", "))
}

// enabledLinters reads the names under the enable: key of the linters block.
//
// Read as text rather than through a YAML library on purpose. The file is ours
// and small, and a parser would answer about a document rather than about the
// lines somebody edits - a name commented out would then disappear from the
// answer instead of showing up as a removal.
func enabledLinters(text string) []string {
	lines := strings.Split(text, "\n")
	var out []string
	inside := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "enable:" {
			inside = true
			continue
		}
		if !inside {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !strings.HasPrefix(trimmed, "- ") {
			// The block ended. The formatters section further down has an
			// enable: of its own, so stopping here rather than reading to the
			// end of the file is what keeps gofmt out of the answer.
			break
		}
		out = append(out, strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")))
	}
	return out
}

// The configuration is only a gate if something runs it, and running it is two
// commands rather than one.
//
// A plain run ACCEPTS a configuration that `config verify` refuses: a key that
// does not exist, or a value outside what the schema allows, is ignored in
// silence. A linter reading a setting nobody applied is a gate that reports
// what it feels like. Measured on the sister project, where a configuration
// the linter accepted locally was refused by CI for exactly that reason.
func TestTheWorkflowRunsTheLinterAndVerifiesItsConfiguration(t *testing.T) {
	root := repoRoot(t)
	body, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("reading ci.yml: %v", err)
	}
	text := withoutYamlComments(string(body))

	const tool = "golangci-lint"
	if !strings.Contains(text, tool+"@") {
		t.Fatalf("no pinned %s in ci.yml - this guard would pass without checking anything.\n"+
			"An unpinned analyser turns somebody else's release into a red build on a commit that changed nothing.", tool)
	}
	if !strings.Contains(text, "config verify") {
		t.Error("ci.yml never runs `golangci-lint config verify`.\n" +
			"Without it a key the schema refuses is ignored in silence, and the linter runs with a setting nobody applied.")
	}
	// Asked of the line that names the tool rather than of the file, because
	// "run ./..." on its own would be satisfied by any other command in the
	// workflow that happens to end the same way.
	runs := false
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, tool+"@") && strings.Contains(line, "run ./...") {
			runs = true
		}
	}
	if !runs {
		t.Error("ci.yml never runs `golangci-lint run ./...`, so the configuration is read and nothing is checked with it.")
	}

	// Both commands have to name the SAME version. Two pins drift apart, and
	// the pair that drifts here would verify one schema and run another.
	versions := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		i := strings.Index(line, tool+"@")
		if i < 0 {
			continue
		}
		rest := line[i+len(tool)+1:]
		if end := strings.IndexAny(rest, " \t"); end >= 0 {
			rest = rest[:end]
		}
		versions[rest] = true
	}
	if len(versions) != 1 {
		var seen []string
		for v := range versions {
			seen = append(seen, v)
		}
		sort.Strings(seen)
		t.Errorf("ci.yml pins %d different versions of %s: %v.\n"+
			"  One command would verify a schema the other does not run.", len(versions), tool, seen)
	}
}
