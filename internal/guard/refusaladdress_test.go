package guard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// A refused recipe says which setting each of its problems is about, and says it
// in a form something other than a person can act on.
//
// RC7 has always reported every problem at once, and the sentences have always
// named the target: "target 2 has no id". That is the right thing to read and
// the wrong thing to look a widget up by. The window marks the box a refusal is
// about by asking a registry keyed on the setting (internal/gui/parts.Fields),
// so a screen that edits a recipe would have had to take the sentence apart to
// find the box - and the sentence names a target by its id, which is exactly the
// thing a target refused for having no id does not have.
//
// So a problem carries an address beside its prose: a recipe key with a 1-based
// index wherever a list is involved. Added on 2026-08-18 for the recipe screen,
// and it reaches "validate --json" in the same step, because a script grouping a
// report by field needed it for the same reason and had the same nothing.
//
// Two halves, and the second is the one that keeps this honest. The first says
// the addresses are the ones expected, so an address that loses its setting and
// points at the target as a whole is caught. The second says every address
// resolves against recipe.Keys() - which is derived from the structures that
// read a recipe rather than typed out - so an address naming a setting no recipe
// has cannot pass. Neither half catches what the other does: a plausible address
// for the wrong field passes the second, and a made up vocabulary that is
// internally consistent passes the first.
func TestEveryRecipeRefusalSaysWhichSettingItIsAbout(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []string
	}{
		{
			name: "the settings of the document itself",
			src: `version: 1
seed: nonsense
locale: fr
targets:
  - id: a
    format: txt
    size: 1kb
`,
			// In document order, which is the order refuseUnsupported runs in
			// before the settings are applied. RC7 reports every problem at once
			// and the order has to be the same on two runs of one recipe.
			want: []string{"locale", "seed"},
		},
		{
			name: "a target missing the two things it cannot do without",
			src: `version: 1
targets:
  - size: 1kb
`,
			want: []string{"targets[1].id", "targets[1].format"},
		},
		{
			// The prose says target "pngs" and the address says targets[1],
			// and that difference is the reason this field exists.
			name: "a count that is not a count",
			src: `version: 1
targets:
  - id: pngs
    format: png
    count: many
    size: 1kb
`,
			want: []string{"targets[1].count"},
		},
		{
			// Each clash is addressed to the first setting its sentence names,
			// because a refusal has one address and a clash has two boxes.
			name: "two ways of stating one size",
			src: `version: 1
targets:
  - id: a
    format: txt
    size: 1kb
    size-range: 1kb-8kb
`,
			want: []string{"targets[1].size"},
		},
		{
			name: "a boundary beside a count",
			src: `version: 1
targets:
  - id: a
    format: txt
    boundary: 10mb
    count: 3
`,
			want: []string{"targets[1].count"},
		},
		{
			name: "a range whose ends are not sizes",
			src: `version: 1
targets:
  - id: a
    format: txt
    size-range: small-large
`,
			want: []string{"targets[1].size-range"},
		},
		{
			name: "an expectation and the reason inside it",
			src: `version: 1
targets:
  - id: a
    format: txt
    size: 1kb
    expected:
      outcome: reject
      reason: made_up
`,
			want: []string{"targets[1].expected.reason"},
		},
		{
			name: "an outcome nobody knows",
			src: `version: 1
targets:
  - id: a
    format: txt
    size: 1kb
    expected: probably
`,
			want: []string{"targets[1].expected"},
		},
		{
			// A value the declaration forbids, addressed to the box that holds
			// it. Until 2026-08-25 this one came back as "bmp: width cannot be
			// ..." with no address at all, because the check lived a layer up
			// where a target is an entry in a list rather than targets[2] - so
			// at twenty BMP batches nothing said which one meant it, and a form
			// had nothing to mark. Two targets on purpose: with one, an address
			// that lost the position would still look right.
			name: "a property value the format forbids",
			src: `version: 1
targets:
  - id: a
    format: bmp
    size: 40kb
  - id: b
    format: bmp
    size: 40kb
    properties:
      width: not a number
`,
			want: []string{"targets[2].properties.width"},
		},
		{
			// A key no format declares, addressed the same way. Its refusal had
			// the subject and no way to say it - the sibling type had carried
			// AboutSetting since 2026-08-12 and this one had not.
			name: "a property the format does not have",
			src: `version: 1
targets:
  - id: a
    format: bmp
    size: 40kb
    properties:
      widht: 640
`,
			want: []string{"targets[1].properties.widht"},
		},
		{
			// The property name is part of the address, because a format
			// declares a field per property and every one of them is drawn.
			name: "a property that is a block",
			src: `version: 1
targets:
  - id: a
    format: png
    size: 4kb
    properties:
      width:
        - 1
        - 2
`,
			want: []string{"targets[1].properties.width"},
		},
		{
			name: "an entry of a contains list",
			src: `version: 1
targets:
  - id: arch
    format: zip
    contains:
      - format: pdf
`,
			want: []string{"targets[1].contains[1].size"},
		},
		{
			// A key this build refuses is addressed too. A screen has to be
			// able to mark it rather than leave the reader hunting.
			name: "keys this build has not built",
			src: `version: 1
policy:
  on_missing_font: fail
targets:
  - id: a
    format: txt
    size: 1kb
    fill: random
`,
			want: []string{"policy", "targets[1].fill"},
		},
		{
			name: "a document with no targets at all",
			src: `version: 1
`,
			want: []string{"targets"},
		},
		{
			name: "a version that is not one",
			src: `version: later
targets:
  - id: a
    format: txt
    size: 1kb
`,
			want: []string{"version"},
		},
	}

	known := map[string]bool{}
	for _, k := range recipe.Keys() {
		known[k] = true
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := recipe.Parse([]byte(c.src), "recipe.yaml")
			var bad *recipe.ValidationError
			if !errors.As(err, &bad) {
				t.Fatalf("this recipe was meant to be refused, and the answer was %v", err)
			}

			var got []string
			for _, p := range bad.Problems {
				if p.At == "" {
					t.Errorf("no address on the refusal %q.\n"+
						"Every problem a recipe produces names the setting it is about, so a\n"+
						"surface can mark the box rather than take the sentence apart.", p.What)
					continue
				}
				got = append(got, p.At)
				assertAddressResolves(t, known, p.At, p.What)
				// And it carries the parts a refusal in this tool is made of -
				// what happened, why, what to do instead (D6). Asked of every
				// case here rather than of one recipe, because a path that
				// leaves one of them empty prints a bare dash where the reason
				// should be, and that is exactly what the first version of the
				// property check did on 2026-08-25.
				if p.Why == "" || p.Fix == "" {
					t.Errorf("the refusal %q at %s has no %s.\n"+
						"A refusal here says what happened, why, and what to do instead - a missing\n"+
						"part prints as a bare dash rather than as nothing.",
						p.What, p.At, missingPart(p.Why, p.Fix))
				}
			}

			if strings.Join(got, ", ") != strings.Join(c.want, ", ") {
				t.Errorf("the addresses of this refusal are not the ones expected.\n"+
					"  wanted: %s\n"+
					"     got: %s\n"+
					"An address that names the target but not the setting looks right here and\n"+
					"leaves a window with nothing to mark.",
					strings.Join(c.want, ", "), strings.Join(got, ", "))
			}
		})
	}
}

// The address reaches the machine readable report, and not only the type inside
// the package.
//
// "validate --json" is what a script reads, and a script grouping a refused
// recipe by field is the reason outside the window for this field to exist. It
// is carried across in one line, which is exactly the kind of line that gets
// dropped in a refactor with every other guard staying green - the address would
// still be right everywhere it was measured and absent everywhere it was used.
func TestTheMachineReadableReportCarriesTheAddressOfEachRefusal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "recipe.yaml")
	// Two problems rather than one, because the report has to carry an address
	// per problem and a single one cannot show that.
	if err := os.WriteFile(path, []byte(`version: 1
targets:
  - size: 1kb
`), 0o644); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer
	if code := cli.Run(context.Background(), []string{"validate", path, "--json"}, &out, &errOut); code == cli.ExitOK {
		t.Fatalf("this recipe was meant to be refused, and validate was happy with it")
	}

	// The refusal goes to the error stream, because a failed run writes nothing
	// to stdout - which is its own row of the regression surface.
	var report struct {
		Problems []struct {
			What string `json:"what"`
			At   string `json:"at"`
		} `json:"problems"`
	}
	if err := json.Unmarshal(errOut.Bytes(), &report); err != nil {
		t.Fatalf("the report is not readable as JSON: %v\n%s", err, errOut.String())
	}
	if len(report.Problems) == 0 {
		t.Fatalf("the report carries no problems at all:\n%s", errOut.String())
	}

	want := map[string]bool{
		"targets[1].id":     true,
		"targets[1].format": true,
	}
	for _, p := range report.Problems {
		if p.At == "" {
			t.Errorf("the report gives no address for %q.\n"+
				"A script that groups a refusal by field has the sentence and nothing\n"+
				"else, and the sentence names a target by an id it does not have.", p.What)
			continue
		}
		delete(want, p.At)
	}
	for missing := range want {
		t.Errorf("no problem in the report is addressed to %q, and one was expected there", missing)
	}
}

// A refused recipe arrives as the problems it holds, and each of them knows
// which field it is about without anybody asking what a recipe is.
//
// This is the join between the address and the thing that uses it. The window
// places a refusal by opening a joined error into the ones it carries and asking
// each whether it implements AboutSetting - the engine, the format registry and
// the preset package all answer it. A refused recipe was the one refusal that
// arrived as a single error, so however many settings it named, all of it landed
// at the foot of the form.
//
// Three things are checked together because the value is in their conjunction. A
// recipe that unwraps but whose problems do not answer AboutSetting gives the
// window nothing to place. Problems that answer it but do not unwrap are never
// reached. And errors.As for the whole has to keep working, because that is how
// the command line decides the exit code - an error that stopped being findable
// would change a code in the frozen table with no guard here going red.
func TestARefusedRecipeArrivesAsItsProblemsEachKnowingItsField(t *testing.T) {
	src := `version: 1
targets:
  - format: txt
    count: lots
`
	_, err := recipe.Parse([]byte(src), "recipe.yaml")

	var bad *recipe.ValidationError
	if !errors.As(err, &bad) {
		t.Fatalf("a refused recipe is no longer findable as a ValidationError, and the\n"+
			"command line decides the exit code with exactly that question. Got %v", err)
	}
	if len(bad.Problems) < 2 {
		t.Fatalf("this recipe was written to have several problems and reported %d", len(bad.Problems))
	}

	joined, several := err.(interface{ Unwrap() []error })
	if !several {
		t.Fatalf("a refused recipe does not open into the problems it holds, so a surface\n" +
			"placing refusals one at a time can only ever place the whole block in one\n" +
			"spot. It needs Unwrap() []error.")
	}

	opened := joined.Unwrap()
	if len(opened) != len(bad.Problems) {
		t.Errorf("the refusal holds %d problems and opens into %d errors",
			len(bad.Problems), len(opened))
	}

	for i, one := range opened {
		var about interface{ AboutSetting() string }
		if !errors.As(one, &about) {
			t.Errorf("problem %d does not say which setting it is about, so the window has\n"+
				"nothing to look a box up by: %v", i+1, one)
			continue
		}
		if got, want := about.AboutSetting(), bad.Problems[i].At; got != want {
			t.Errorf("problem %d says it is about %q and its address is %q", i+1, got, want)
		}
		if about.AboutSetting() == "" {
			t.Errorf("problem %d has an empty address: %v", i+1, one)
		}
	}

	// The whole still reads as the list, because the command line prints this
	// error and a reader there wants every problem rather than the first.
	//
	// What is asked for in the command line's words, since 2026-08-25. The
	// stored sentence leaves a slot where it names its own setting so a window
	// can say Group name where a recipe says id, and the whole error is what
	// the command line prints - so the two are only comparable once both are
	// in the same vocabulary. See core.SettingSlot.
	for _, p := range bad.Problems {
		what := core.InTheWordsOf(p.What, core.LastSettingSegment(p.At))
		if !strings.Contains(err.Error(), what) {
			t.Errorf("the message of the whole refusal no longer mentions %q", what)
		}
	}
}

// index is the 1-based position of a list entry, which comes out of an address
// before it is compared with the key it has to be one of.
var index = regexp.MustCompile(`\[[0-9]+\]`)

// carriesParts is every setting whose address may go one level deeper than the
// keys do, and it is a named list rather than a rule on purpose.
//
// These three hold values a recipe invents: the properties of a format, the
// parts of an expectation, the entries of a contains list. recipe.Keys() stops
// at each of them because there is nothing further to walk - a map and an "any"
// have no fields - so their children cannot be checked against anything. Naming
// them means a fourth one is a deliberate act rather than a hole that opens
// quietly.
var carriesParts = map[string]bool{
	"targets.properties": true,
	"targets.expected":   true,
	"targets.contains":   true,
}

func assertAddressResolves(t *testing.T, known map[string]bool, at, what string) {
	t.Helper()

	key := index.ReplaceAllString(at, "")
	if known[key] {
		return
	}
	if cut := strings.LastIndex(key, "."); cut > 0 {
		if parent := key[:cut]; known[parent] && carriesParts[parent] {
			return
		}
	}

	t.Errorf("the address %q of the refusal %q is not a setting a recipe has.\n"+
		"Stripped of its indices it reads %q, which is not among the keys\n"+
		"recipe.Keys() derives from the structures that read a recipe. An address\n"+
		"nothing can resolve is worse than none: a surface would look for a box\n"+
		"under that name, find nothing, and say nothing about it.", at, what, key)
}

// missingPart names which half of a refusal is absent, so the failure says it
// rather than leaving somebody to compare two empty strings.
func missingPart(why, fix string) string {
	switch {
	case why == "" && fix == "":
		return "why and no fix"
	case why == "":
		return "why"
	default:
		return "fix"
	}
}

// A refusal from below the recipe reader reaches the machine readable report
// with its address, and only when that address names the target.
//
// Two halves that pull against each other, which is why they are asserted in
// one place. The first: the ceiling on files is checked by the engine on the
// total, so a recipe whose targets each pass the reader is refused here - and
// until 2026-08-25 it arrived with a sentence and no "at" at all, so a script
// grouping a report by field had nothing and a window had no box to mark.
//
// The second: an address that names a setting without its position is not an
// address. A format refusing a size knows it is about "size" and not which of
// twenty targets asked, and "at": "size" in that report points at all of them
// while looking actionable. So it is left out, and leaving it out is a rule
// rather than an accident - which is what the second half of this asserts.
func TestTheReportCarriesAnAddressFromBelowTheReaderOnlyWhenItNamesTheTarget(t *testing.T) {
	read := func(t *testing.T, src string) (what, at string) {
		t.Helper()
		dir := t.TempDir()
		path := filepath.Join(dir, "recipe.yaml")
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		var out, errOut bytes.Buffer
		if code := cli.Run(context.Background(), []string{"validate", path, "--json"}, &out, &errOut); code == cli.ExitOK {
			t.Fatalf("this recipe was meant to be refused and validate was happy with it:\n%s", out.String())
		}
		var report struct {
			Problems []struct {
				What string `json:"what"`
				At   string `json:"at"`
			} `json:"problems"`
		}
		if err := json.Unmarshal(errOut.Bytes(), &report); err != nil {
			t.Fatalf("the report is not readable as JSON: %v\n%s", err, errOut.String())
		}
		if len(report.Problems) != 1 {
			t.Fatalf("expected one problem and got %d:\n%s", len(report.Problems), errOut.String())
		}
		return report.Problems[0].What, report.Problems[0].At
	}

	// The big target second, because the engine tests the running total before
	// planning a target's files - so this refuses having planned one file
	// rather than a million.
	what, at := read(t, `version: 1
targets:
  - id: a
    format: txt
    count: 1
    size: 1kb
  - id: b
    format: txt
    count: 1000000
    size: 1kb
`)
	if want := core.TargetAddress(2, recipe.KeyCount); at != want {
		t.Errorf("the ceiling was reported at %q and the target that crossed it is %q.\n"+
			"Reason: the ceiling belongs to the run, but the box somebody can change belongs to\n"+
			"a target - a report that cannot say which one leaves a script and a window with the\n"+
			"sentence and nothing else.\nWhat it said: %s", at, want, what)
	}

	// And the other way. A size the format cannot deliver is refused below the
	// reader too, and that refusal knows only the setting.
	what, at = read(t, `version: 1
targets:
  - id: a
    format: pdf
    size: 10
`)
	if at != "" {
		t.Errorf("a refusal that knows its setting but not its target was reported at %q.\n"+
			"Reason: %q names a setting every target has. In a recipe with twenty of them it points\n"+
			"at all of them at once while looking like something to act on, which is worse than\n"+
			"saying nothing. Give it its position or leave it out.\nWhat it said: %s", at, at, what)
	}
}
