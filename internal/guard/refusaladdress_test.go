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

// Every address the machine readable report carries says where it is, and the
// ones from below the recipe reader arrive at all.
//
// Two things, and the second is what keeps the first honest.
//
// A refusal the reader never sees is refused by the engine or by a format: the
// ceiling on files is checked on the total, so a recipe whose targets each pass
// the reader is refused underneath it. Until 2026-08-25 those arrived with a
// sentence and no address, so a script grouping a report by field had nothing
// to group by.
//
// And an address is only worth carrying if it says where. A refusal that names
// "size" and not which of twenty targets asked points at all of them while
// looking like something to act on. There was a filter here dropping those,
// and it went the day the engine started giving every refusal about a target
// the position of that target - so this asks the property directly instead,
// over every refusal that reaches the report. A half address getting through
// turns it red wherever it comes from, which the filter could not do.
func TestEveryAddressTheReportCarriesSaysWhereItIs(t *testing.T) {
	report := func(t *testing.T, src string) []struct {
		What string `json:"what"`
		At   string `json:"at"`
	} {
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
		var parsed struct {
			Problems []struct {
				What string `json:"what"`
				At   string `json:"at"`
			} `json:"problems"`
		}
		if err := json.Unmarshal(errOut.Bytes(), &parsed); err != nil {
			t.Fatalf("the report is not readable as JSON: %v\n%s", err, errOut.String())
		}
		if len(parsed.Problems) == 0 {
			t.Fatalf("the report carries no problems at all:\n%s", errOut.String())
		}
		return parsed.Problems
	}

	// The big target second, because the engine tests the running total before
	// planning a target's files - so this refuses having planned one file
	// rather than a million.
	const ceiling = `version: 1
targets:
  - id: a
    format: txt
    count: 1
    size: 1kb
  - id: b
    format: txt
    count: 1000000
    size: 1kb
`
	if got := report(t, ceiling)[0].At; got != core.TargetAddress(2, recipe.KeyCount) {
		t.Errorf("the ceiling was reported at %q and the target that crossed it is %q.\n"+
			"Reason: the ceiling belongs to the run, but the box somebody can change belongs to\n"+
			"a target - a report that cannot say which one leaves a script with the sentence and\n"+
			"nothing else.", got, core.TargetAddress(2, recipe.KeyCount))
	}

	// One refusal per layer that can produce one below the reader: a format
	// refusing a size, the engine refusing a name, the engine refusing the name
	// of the manifest. The last of those is the one that says a document
	// setting counts as placed too - "output.manifest" names no target and is a
	// perfectly good address.
	placed := []struct{ name, src string }{
		{"a size the format cannot deliver", "version: 1\ntargets:\n  - id: a\n    format: pdf\n    size: 10\n"},
		{"a name the host cannot store", "version: 1\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\n    name: \"a<b.txt\"\n"},
		{"a name template with no such placeholder", "version: 1\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\n    name: \"f_{n}.txt\"\n"},
		{"the name of the manifest", "version: 1\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\noutput:\n  manifest: \"report|1.json\"\n"},
	}
	for _, c := range placed {
		t.Run(c.name, func(t *testing.T) {
			for _, p := range report(t, c.src) {
				if p.At == "" {
					t.Errorf("this refusal reaches the report with no address at all.\n"+
						"Reason: a script grouping by field has the sentence and nothing else, and the\n"+
						"sentence names a target by its id, which is exactly what a target refused for\n"+
						"having no id does not have.\nWhat it said: %s", p.What)
					continue
				}
				// Placed means it says where: a position in the list of
				// targets, or a section of the document. A bare word does
				// neither - "size" is a setting every target has.
				if !core.AddressNamesATarget(p.At) && !strings.Contains(p.At, ".") {
					t.Errorf("this refusal is reported at %q, which names a setting and not a place.\n"+
						"Reason: every target has one of those, so in a recipe with twenty of them it\n"+
						"points at all at once while looking like something to act on. That is worse\n"+
						"than saying nothing.\nWhat it said: %s", p.At, p.What)
				}
			}
		})
	}
}

// What validate accepts, generate accepts.
//
// Measured on 2026-08-25 and it was not true: a recipe whose output.manifest
// held a character the host will not store was called valid, and generate
// refused it with code 3 a second later. validate did not pass the manifest
// name to the planner at all, so the one check that reads it never ran. This
// command exists to sit in a pre commit hook, where a missed alarm lets through
// exactly the recipe somebody will run next.
func TestValidateRefusesWhatGenerateWouldRefuse(t *testing.T) {
	for _, src := range []string{
		"version: 1\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\noutput:\n  manifest: \"report|1.json\"\n",
		"version: 1\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\noutput:\n  manifest: \"nul\"\n",
	} {
		dir := t.TempDir()
		path := filepath.Join(dir, "recipe.yaml")
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}

		var vOut, vErr bytes.Buffer
		valid := cli.Run(context.Background(), []string{"validate", path}, &vOut, &vErr)

		var gOut, gErr bytes.Buffer
		gen := cli.Run(context.Background(), []string{"generate", path, "--out", t.TempDir(), "--dry-run"}, &gOut, &gErr)

		if valid != gen {
			t.Errorf("validate ended with %d and generate with %d on the same recipe.\n"+
				"Reason: this command is what a pre commit hook runs, so a recipe it calls valid is\n"+
				"one somebody will run - and finding out at generate time is finding out too late.\n"+
				"validate said: %s\ngenerate said: %s", valid, gen, vOut.String()+vErr.String(), gOut.String()+gErr.String())
		}
	}
}

// A refusal from below the recipe reader arrives in its three parts, and the
// one sentence form still carries them.
//
// The reader has reported what, why and what to do instead since RC7. A
// refusal from underneath it - a format refusing a size, a format that holds
// nothing being asked to hold something - arrived as one sentence, so a script
// grouping a report by reason had to take prose apart to do it, and the prose
// is the one thing here written for a person rather than for a program.
//
// The second half is what keeps the two honest. Each type assembles its own
// sentence, in the order it reads best, and hands out its parts separately -
// so the sentence and the parts are two assemblies of the same facts and could
// drift. Asking that the sentence still contains the why and the fix costs
// nothing and closes that.
func TestARefusalFromBelowTheReaderArrivesInItsThreeParts(t *testing.T) {
	cases := []struct{ name, src string }{
		{"a size the format cannot deliver", "version: 1\ntargets:\n  - id: a\n    format: pdf\n    size: 10\n"},
		{"contains asked of a format that holds nothing", "version: 1\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\n    contains:\n      - format: txt\n        count: 2\n        size: 100\n"},
		{"a value a format setting will not take", "version: 1\ntargets:\n  - id: a\n    format: bmp\n    size: 1mb\n    properties:\n      width: \"99999\"\n"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "recipe.yaml")
			if err := os.WriteFile(path, []byte(c.src), 0o644); err != nil {
				t.Fatal(err)
			}

			var out, errOut bytes.Buffer
			if code := cli.Run(context.Background(), []string{"validate", path, "--json"}, &out, &errOut); code == cli.ExitOK {
				t.Fatalf("this recipe was meant to be refused:\n%s", out.String())
			}
			var report struct {
				Problems []struct {
					What string `json:"what"`
					Why  string `json:"why"`
					Fix  string `json:"fix"`
				} `json:"problems"`
			}
			if err := json.Unmarshal(errOut.Bytes(), &report); err != nil {
				t.Fatalf("the report is not readable as JSON: %v\n%s", err, errOut.String())
			}
			if len(report.Problems) != 1 {
				t.Fatalf("expected one problem and got %d:\n%s", len(report.Problems), errOut.String())
			}
			p := report.Problems[0]
			for _, part := range []struct{ name, value string }{
				{"what", p.What}, {"why", p.Why}, {"fix", p.Fix},
			} {
				if part.value == "" {
					t.Errorf("the report carries no %q for this refusal.\n"+
						"Reason: a script grouping by reason has to take the sentence apart to do it,\n"+
						"and the sentence is the one thing here written for a person.\nIt said: %q",
						part.name, p.What)
				}
			}

			// The same refusal in one sentence, which is what the command line
			// prints. It has to still hold the parts, or the two renderings have
			// come apart and only one of them is right.
			var pOut, pErr bytes.Buffer
			cli.Run(context.Background(), []string{"validate", path}, &pOut, &pErr)
			sentence := pOut.String() + pErr.String()
			for _, part := range []struct{ name, value string }{{"why", p.Why}, {"fix", p.Fix}} {
				if part.value != "" && !strings.Contains(sentence, strings.TrimSuffix(part.value, ".")) {
					t.Errorf("the sentence the command line prints does not contain the %q the report gives.\n"+
						"Reason: each of these types writes its own sentence and hands out its parts\n"+
						"separately, so the two are assemblies of the same facts and can drift. This is\n"+
						"what stops that.\nthe %s: %q\nthe sentence: %s",
						part.name, part.name, part.value, sentence)
				}
			}
		})
	}
}
