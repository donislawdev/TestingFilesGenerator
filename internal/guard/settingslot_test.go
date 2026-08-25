package guard

import (
	"errors"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/parts"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// brokenRecipes are documents that refuse, one per refusal that names its own
// setting. Written out rather than generated, because what is being checked is
// the WORDING of each refusal and a generated document would only produce the
// ones somebody thought to generate.
var brokenRecipes = map[string]string{
	"no id": `version: 1
targets:
  - format: txt
    size: 1kb
`,
	"no size": `version: 1
targets:
  - id: a
    format: txt
`,
	"duplicate id": `version: 1
targets:
  - id: a
    format: txt
    size: 1kb
  - id: a
    format: txt
    size: 1kb
`,
	"count that is not a number": `version: 1
targets:
  - id: a
    format: txt
    size: 1kb
    count: "many"
`,
	"size and range together": `version: 1
targets:
  - id: a
    format: txt
    size: 1kb
    size-range: 1kb-2kb
`,
	"boundary and range together": `version: 1
targets:
  - id: a
    format: txt
    boundary: 10kb
    size-range: 1kb-2kb
`,
	"count beside a boundary": `version: 1
targets:
  - id: a
    format: txt
    boundary: 10kb
    count: 3
`,
	"boundary of nothing": `version: 1
targets:
  - id: a
    format: txt
    boundary: 0
`,
	"a size that is not a size": `version: 1
targets:
  - id: a
    format: txt
    size: "abc"
`,
	"a range that runs backwards": `version: 1
targets:
  - id: a
    format: txt
    size-range: 8kb-1kb
`,
}

// refusalsOfBrokenRecipes is every problem those documents produce.
func refusalsOfBrokenRecipes(t *testing.T) []recipe.Problem {
	t.Helper()
	var out []recipe.Problem
	for name, doc := range brokenRecipes {
		_, err := recipe.Parse([]byte(doc), name)
		if err == nil {
			t.Fatalf("%q was supposed to be refused and was not", name)
		}
		var bad *recipe.ValidationError
		if !errors.As(err, &bad) {
			continue
		}
		out = append(out, bad.Problems...)
	}
	if len(out) == 0 {
		t.Fatal("no refusal was collected, so this guard is asserting about nothing")
	}
	return out
}

// TestNoRefusalEverShowsASlotToAPerson is the one thing a slot must never do.
//
// A slot marks where a refusal names the setting it is about, so the command
// line can say "id" where a window says "Group name" - see core.SettingSlot. It
// is machinery and not a word, so a sentence that carries one and is never
// asked to render it puts "{setting}" in front of somebody.
//
// That is the failure this mechanism can have that the old one could not, and
// it is invisible from the side that writes the sentence: a slot added to a
// refusal whose type has no InTheWordsOf compiles, passes vet, and reads
// correctly in the source. Only a person running the tool would see it.
//
// Both renderings are asked for, because they are separate paths: Error is what
// the command line prints and InTheWordsOf is what a window shows, and a type
// that renders one and forgets the other is exactly the shape this catches.
func TestNoRefusalEverShowsASlotToAPerson(t *testing.T) {
	leaks := func(t *testing.T, where, message string) {
		t.Helper()
		for _, slot := range []string{core.SettingSlot, core.ArticleSlot} {
			if strings.Contains(message, slot) {
				t.Errorf("%s shows %q to a person:\n  %s", where, slot, message)
			}
		}
	}

	for _, p := range refusalsOfBrokenRecipes(t) {
		leaks(t, "the command line", p.Error())
		leaks(t, "a window", p.InTheWordsOf("Group name"))
		// The empty name is what a field with no label would ask for, and it
		// has to leave the sentence readable rather than half rendered.
		leaks(t, "a field with no label", p.InTheWordsOf(""))
	}

	// A size is parsed one layer below the recipe and refuses in its own words,
	// so it is a second path to the same mistake.
	for _, bad := range []string{"", "abc", "10qb", "-1", "1e5", "1.5b"} {
		if _, err := core.ParseSize(bad); err != nil {
			leaks(t, "a refused size", err.Error())
			var reworded interface{ InTheWordsOf(string) string }
			if errors.As(err, &reworded) {
				leaks(t, "a refused size in a window", reworded.InTheWordsOf("Size"))
			}
		}
	}

	// A declared setting refuses through the registry, which assembles its
	// sentence rather than writing it out.
	value := &format.PropertyValueError{
		Format: "png", Key: "width", Value: "99999",
		Reason: "it takes a whole number of pixels from 1 to 20000",
	}
	leaks(t, "the command line", value.Error())
	leaks(t, "a window", value.InTheWordsOf("Width"))
}

// TestEveryNameARefusalCanBeGivenTakesTheArticleThisRuleGivesIt holds the
// article rule to the names it is actually used with.
//
// core.articleFor answers from the first written letter, which is not the whole
// of English - "an hour" and "a university" both break it. That is safe only
// while every name either surface can put after a slot obeys it, and those
// names are ours: the recipe keys are a closed list and so are the labels above
// the boxes.
//
// The expected article is written out per name rather than worked out, because
// a table computed by the rule under test agrees with the rule whatever the
// rule says. A name with no entry is a failure rather than a pass, so the first
// setting or label added after this comes here to be read out loud.
func TestEveryNameARefusalCanBeGivenTakesTheArticleThisRuleGivesIt(t *testing.T) {
	want := map[string]string{
		// Recipe keys, which is what the command line prints.
		"id": "an", "count": "a", "name": "a", "size": "a", "format": "a",
		"seed": "a", "label": "a", "dir": "a", "group": "a", "boundary": "a",
		"size-range": "a", "expected": "an", "reason": "a", "manifest": "a",
		"contains": "a", "preset": "a", "limit": "a", "spread": "a",
		"width": "a", "height": "a", "quality": "a", "pages": "a",
		"entries": "an", "bit_depth": "a", "sample_rate": "a", "channels": "a",
		"paragraphs": "a", "rows": "a", "columns": "a", "slides": "a",
		"depth": "a", "colours": "a", "records": "a", "lines": "a",
		// Labels, which is what a window shows.
		"Batch name": "a", "How many files": "a", "File names": "a", "Size": "a",
		"Format": "a", "Seed": "a", "Output directory": "an", "Kind of case": "a",
		"Around a limit": "an", "Size range": "a", "Expected outcome": "an",
		"Limit to test": "a", "One size": "a", "A range": "a",
		"Rule being tested": "a", "Manifest file name": "a", "Preset": "a",
		"Limit": "a", "Spread": "a", "Width": "a", "Height": "a",
		"Write a label inside each file": "a",
	}

	check := func(name, source string) {
		t.Helper()
		article, listed := want[name]
		if !listed {
			t.Errorf("%s is named %q and no article is written down for it.\n"+
				"Add it to this guard, reading the sentence out loud first - the rule in core.articleFor\n"+
				"answers from the first letter and cannot be trusted to know about a name like \"hour\".",
				source, name)
			return
		}
		got := core.InTheWordsOf(core.ArticleSlot+" "+core.SettingSlot, name)
		if want := article + " " + name; got != want {
			t.Errorf("%s is named %q and a refusal says %q where it should say %q",
				source, name, got, want)
		}
	}

	// Every box either window screen draws, by both of its names. This is the
	// real list rather than a copy of it, so a field added to a screen arrives
	// here without anybody remembering to bring it.
	host := &fakeHost{}
	seen := map[string]bool{}
	for _, s := range []struct {
		name   string
		fields *parts.Fields
	}{
		{"the generate screen", window.NewGenerate(host).Fields()},
		{"the preset screen", window.NewPreset(host).Fields()},
		{"the batches screen", window.NewRecipe(host).Fields()},
	} {
		for _, f := range s.fields.All() {
			if key := core.LastSettingSegment(f.Setting); key != "" && !seen[key] {
				seen[key] = true
				check(key, "a setting on "+s.name)
			}
			if f.Label != "" && !seen[f.Label] {
				seen[f.Label] = true
				check(f.Label, "a box on "+s.name)
			}
		}
	}
	if len(seen) == 0 {
		t.Fatal("no name was collected, so this guard is asserting about nothing")
	}
}
