package guard

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// A recipe key given a shape it cannot have is refused in our words, not the
// parser's.
//
// Found on 2026-08-20 from a code scanning alert that was about something else.
// Writing engine as a mapping answered:
//
//	cannot unmarshal map[string]interface {} into Go struct field
//	rawRecipe.Engine of type string
//
// which names a type nobody outside this repository can see, in a sentence
// nobody can act on. The audit that recorded it thought it was one key. It was
// every key held as a string or a boolean - eleven of them - and most are keys
// people use every day: format, name, id, group, the output directory and the
// manifest name. A stray indent in YAML turns a value into a mapping, so this
// is not an exotic way to hold the tool wrong.
//
// The numbers a recipe carries were already immune, and the reason is written
// at the top of internal/recipe/scalar.go: they are read as the text the author
// typed rather than let YAML decide their type. This guard extends that
// property to the rest, and holds it.
//
// The keys are read out of the source rather than listed here, because a list
// typed into a test is a list that falls behind the tree - and the failure mode
// is silent, since a key nobody tests looks exactly like a key that passes.
func TestNoRecipeKeyAnswersWithTheWordsOfTheParser(t *testing.T) {
	keys := recipeKeys(t)
	if len(keys) == 0 {
		t.Fatal("no keys were read out of the recipe package, so this guard would pass without checking anything")
	}

	dir := t.TempDir()
	checked := 0
	for _, key := range keys {
		template, ok := shapeCases[key]
		if !ok {
			t.Errorf("the recipe has a key %q that this guard has no case for.\n"+
				"What to do: add it to shapeCases with a recipe that sets it to a mapping. "+
				"A key nobody feeds a wrong shape is a key that can start answering with Go type names "+
				"without anybody noticing.", key)
			continue
		}
		if template == skipShapeCase {
			continue
		}

		path := writeRecipe(t, dir, template)
		code, _, errOut := run(t, "validate", path)
		checked++

		for _, leak := range []string{
			"cannot unmarshal", "interface {}", "Go struct field",
			"rawRecipe", "rawTarget", "rawOutput", "rawDefaults",
		} {
			if strings.Contains(errOut, leak) {
				t.Errorf("%s: the refusal carries %q, which is the parser talking about our types:\n%s",
					key, leak, errOut)
				break
			}
		}
		// Refused, and refused as a recipe problem. A wrong shape is the
		// author's, so it belongs under the recipe code rather than the one
		// that means the tool broke.
		if code != cli.ExitRecipe {
			t.Errorf("%s: exit %d, expected %d - a shape a recipe cannot have is a recipe problem:\n%s",
				key, code, cli.ExitRecipe, errOut)
		}
		if err := os.Remove(path); err != nil {
			t.Fatalf("clearing between cases: %v", err)
		}
	}

	if checked == 0 {
		t.Fatal("every key was skipped, so this guard would pass without running the tool once")
	}
	t.Logf("%d key(s) fed a shape they cannot have", checked)
}

// skipShapeCase marks a key that cannot be given a wrong shape here, with the
// reason. Written out rather than left absent, because an absent key and a key
// deliberately left alone look identical in a list.
const skipShapeCase = "SKIP"

// shapeCases gives one recipe per key, with that key written as a mapping - the
// shape a stray indent produces and the one that used to reach the parser.
var shapeCases = map[string]string{
	// The three that take a mapping or a list by design, so a mapping is not a
	// wrong shape for them at all.
	"defaults": skipShapeCase,
	"output":   skipShapeCase,
	"targets":  skipShapeCase,
	"policy":   skipShapeCase,
	"with":     skipShapeCase,
	"contains": skipShapeCase,
	// expected takes either a word or a mapping, which is the whole point of
	// the short and long forms.
	"expected": skipShapeCase,
	// properties is a mapping of names to values.
	"properties": skipShapeCase,
	"mutations":  skipShapeCase,

	"version":                "version: {a: b}\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\noutput:\n  dir: ./o\n",
	"seed":                   "version: 1\nseed: {a: b}\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\noutput:\n  dir: ./o\n",
	"engine":                 "version: 1\nengine: {a: b}\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\noutput:\n  dir: ./o\n",
	"locale":                 "version: 1\nlocale: {a: b}\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\noutput:\n  dir: ./o\n",
	"extends":                "version: 1\nextends: {a: b}\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\noutput:\n  dir: ./o\n",
	"allow_nondeterministic": "version: 1\nallow_nondeterministic: {a: b}\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\noutput:\n  dir: ./o\n",

	"label": "version: 1\ndefaults:\n  label: {a: b}\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\noutput:\n  dir: ./o\n",
	"fill":  "version: 1\ndefaults:\n  fill: {a: b}\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\noutput:\n  dir: ./o\n",

	"dir":             "version: 1\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\noutput:\n  dir: {a: b}\n",
	"manifest":        "version: 1\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\noutput:\n  dir: ./o\n  manifest: {a: b}\n",
	"split_threshold": "version: 1\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\noutput:\n  dir: ./o\n  split_threshold: {a: b}\n",

	"id":         "version: 1\ntargets:\n  - id: {a: b}\n    format: txt\n    size: 1kb\noutput:\n  dir: ./o\n",
	"format":     "version: 1\ntargets:\n  - id: a\n    format: {a: b}\n    size: 1kb\noutput:\n  dir: ./o\n",
	"name":       "version: 1\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\n    name: {a: b}\noutput:\n  dir: ./o\n",
	"group":      "version: 1\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\n    group: {a: b}\noutput:\n  dir: ./o\n",
	"count":      "version: 1\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\n    count: {a: b}\noutput:\n  dir: ./o\n",
	"size":       "version: 1\ntargets:\n  - id: a\n    format: txt\n    size: {a: b}\noutput:\n  dir: ./o\n",
	"size-range": "version: 1\ntargets:\n  - id: a\n    format: txt\n    size-range: {a: b}\noutput:\n  dir: ./o\n",
	"boundary":   "version: 1\ntargets:\n  - id: a\n    format: txt\n    boundary: {a: b}\noutput:\n  dir: ./o\n",
}

var yamlTag = regexp.MustCompile(`yaml:"([a-z_-]+)"`)

// recipeKeys reads the keys out of the struct tags of the recipe package.
func recipeKeys(t *testing.T) []string {
	t.Helper()

	dir := filepath.Join("..", "recipe")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the recipe package: %v", err)
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		for _, m := range yamlTag.FindAllStringSubmatch(string(body), -1) {
			seen[m[1]] = true
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
