package guard

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/recipe"
)

// The recipe is the contract a person edits by hand and commits. These guards
// cover the promises that break quietly: a flag wiping out a value nobody
// touched, a canonical form drifting under a library upgrade, a typo accepted
// in silence, and a declared expectation not arriving in the manifest.

// sampleRecipe is defined here rather than read from a file so that the guard
// depends on nothing outside its own source.
const sampleRecipe = `# Fixtures for the invoice upload form.
# The API limit is 10 MB.
version: 1
seed: 7741

defaults:
  label: true

targets:
  - id: invoices             # mandatory and stable
    format: txt
    count: 2
    size: 4kb
    expected: accept

  - id: oversized
    format: txt
    count: 1
    size: 8kb
    expected:
      outcome: reject
      reason: size_limit

output:
  dir: ./out
`

// canonicalHash pins the canonical form of the recipe above.
//
// This is the same kind of guard as the byte stability one, and it exists for
// the same reason. The manifest carries recipe_hash, and that hash is taken
// from the canonical form - so an upgrade of the YAML library that moves one
// space changes the hash in every manifest of every user. Go promises nothing
// about that, and neither does the library, so measurement is the only
// mechanism available.
//
// Measured on github.com/goccy/go-yaml v1.19.2, 2026-08-01. If this turns red
// after an upgrade, that is a breaking change and a decision for the owner -
// never a value to quietly correct.
const canonicalHash = "sha256:a6504387d5835cbab3ccbf0ce4db04673bb52fd6b38502edab5e44ac8d7c308a"

func TestTheCanonicalFormOfARecipeHasNotDrifted(t *testing.T) {
	got, err := recipe.Hash([]byte(sampleRecipe))
	if err != nil {
		t.Fatalf("hashing the recipe: %v", err)
	}
	if got != canonicalHash {
		t.Errorf("the canonical form moved.\n got: %s\nwant: %s\n"+
			"This changes recipe_hash in every manifest, which is a breaking change. "+
			"Do not update the constant to make this pass.", got, canonicalHash)
	}
}

// A formatter that never settles churns the file on every save, which is the
// failure the canonical form exists to prevent.
func TestFormattingARecipeSettlesAndKeepsWhatAPersonWrote(t *testing.T) {
	once, err := recipe.Canonical([]byte(sampleRecipe), "recipe.yaml")
	if err != nil {
		t.Fatalf("first pass: %v", err)
	}
	twice, err := recipe.Canonical(once, "recipe.yaml")
	if err != nil {
		t.Fatalf("second pass: %v", err)
	}
	if !bytes.Equal(once, twice) {
		t.Errorf("formatting twice gave a different result, so every save would churn the file")
	}

	// Comments are the reason this project chose YAML over JSON at all.
	for _, want := range []string{
		"# Fixtures for the invoice upload form.",
		"# The API limit is 10 MB.",
		"# mandatory and stable",
	} {
		if !strings.Contains(string(once), want) {
			t.Errorf("the comment %q did not survive - without comments the recipe format has no advantage over JSON", want)
		}
	}

	// Blank lines are what keeps a long recipe readable in a pull request.
	if strings.Count(string(once), "\n\n") == 0 {
		t.Error("every blank line was collapsed - a long recipe becomes a wall of text and the diff stops being readable")
	}
}

// The precedence rule from docs/CLI.md section 6, and the whole reason it is
// written down: the tool has to tell "not given" from "given a value equal to
// the default". Reading the flag value back cannot.
func TestAFlagNobodyWroteDoesNotOverrideTheRecipe(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	// No flag at all, so the output directory has to come from the recipe.
	// Passing --out here would make the test assert the opposite of its name.
	path := writeRecipe(t, dir, strings.Replace(sampleRecipe, "./out", filepath.ToSlash(out), 1))

	// --count defaults to 1 and the recipe asks for 2 plus 1. If the default
	// leaked through, the first target would produce one file instead of two.
	code, _, _ := run(t, "generate", path)
	if code != cli.ExitOK {
		t.Fatalf("exit %d", code)
	}

	m := readManifest(t, filepath.Join(out, "manifest.json"))
	if m.Summary.FileCount != 3 {
		t.Errorf("the run produced %d files, the recipe asks for 3 - a default flag value overrode the recipe",
			m.Summary.FileCount)
	}
	if m.Run.Seed != 7741 {
		t.Errorf("seed is %d, the recipe says 7741 - the default --seed 0 overrode the recipe", m.Run.Seed)
	}
	if len(m.Run.Overrides) != 0 {
		t.Errorf("the manifest lists overrides %v and no flag was given", m.Run.Overrides)
	}
}

// The other half of the same rule. A flag that was written has to win, and the
// manifest has to say so - otherwise recipe_hash stops describing the run.
func TestAFlagThatWasWrittenWinsAndIsRecorded(t *testing.T) {
	dir := t.TempDir()
	path := writeRecipe(t, dir, sampleRecipe)
	out := filepath.Join(dir, "out")

	code, _, _ := run(t, "generate", path, "--seed", "99", "--out", out)
	if code != cli.ExitOK {
		t.Fatalf("exit %d", code)
	}

	m := readManifest(t, filepath.Join(out, "manifest.json"))
	if m.Run.Seed != 99 {
		t.Errorf("seed is %d, the flag said 99", m.Run.Seed)
	}
	o, ok := m.Run.Overrides["seed"]
	if !ok {
		t.Fatal("the flag beat the recipe and the manifest does not record it - two runs of one recipe would differ with nothing to explain why")
	}
	if o.FromFlag != float64(99) || o.FromRecipe != float64(7741) {
		t.Errorf("the override records %v -> %v, expected 7741 -> 99", o.FromRecipe, o.FromFlag)
	}
	if m.Run.RecipeHash != canonicalHash {
		t.Errorf("recipe_hash is %q, expected the canonical hash of the recipe", m.Run.RecipeHash)
	}
}

// An expectation is the reason this tool exists rather than being one more
// dummy file generator. It has to arrive in the manifest untouched.
func TestAnExpectationFromTheRecipeReachesTheManifestUnchanged(t *testing.T) {
	dir := t.TempDir()
	path := writeRecipe(t, dir, sampleRecipe)
	out := filepath.Join(dir, "out")

	if code, _, _ := run(t, "generate", path, "--out", out); code != cli.ExitOK {
		t.Fatalf("exit %d", code)
	}

	m := readManifest(t, filepath.Join(out, "manifest.json"))
	want := map[string]string{
		"invoices":  "accept",
		"oversized": "reject",
	}
	seen := map[string]bool{}
	for _, f := range m.Files {
		for id, outcome := range want {
			if strings.HasPrefix(f.Name, id) {
				seen[id] = true
				if f.Expected.Outcome != outcome {
					t.Errorf("%s declares %q, the recipe says %q", f.Name, f.Expected.Outcome, outcome)
				}
			}
		}
	}
	for id := range want {
		if !seen[id] {
			t.Errorf("no file of target %q reached the manifest", id)
		}
	}
}

// A typo silently ignored gives a file of the default size and an hour spent
// wondering why a test passes when it should not.
func TestATypoInARecipeKeyIsRefused(t *testing.T) {
	dir := t.TempDir()
	path := writeRecipe(t, dir, "version: 1\nsiez: 10mb\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\n")

	code, _, errOut := run(t, "validate", path)
	if code != cli.ExitRecipe {
		t.Errorf("exit %d, expected %d for a bad recipe", code, cli.ExitRecipe)
	}
	if !strings.Contains(errOut, "siez") {
		t.Errorf("the message does not name the key that is wrong:\n%s", errOut)
	}
}

// RC7: every problem at once. Fixing a recipe one error per run is the
// cheapest way to make somebody stop using the tool.
func TestAllTheProblemsInARecipeAreReportedAtOnce(t *testing.T) {
	dir := t.TempDir()
	path := writeRecipe(t, dir, `version: 1
targets:
  - id: a
    format: txt
    count: 0
  - id: a
    format: txt
    size: 5zb
  - format: txt
    size: 1kb
    expected: maybe
`)

	code, _, errOut := run(t, "validate", path)
	if code != cli.ExitRecipe {
		t.Fatalf("exit %d, expected %d", code, cli.ExitRecipe)
	}

	// Six distinct problems: a count of zero, a target with no size, a bad
	// unit, a duplicate id, a target with no id, and an unknown outcome.
	for _, want := range []string{
		"asks for 0 files",
		"has no size",
		"5zb",
		"used twice",
		"has no id",
		"maybe",
	} {
		if !strings.Contains(errOut, want) {
			t.Errorf("the report is missing %q, so problems are being reported one at a time:\n%s", want, errOut)
		}
	}
}

// RC7 again, in its other half: a recipe that does not validate writes nothing
// at all.
func TestARecipeThatFailsValidationWritesNothing(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	// Everything here is runnable except the expectation. That matters: a
	// recipe whose only fault is a missing size would be caught by the engine
	// too, and the guard would pass without the validator doing anything.
	path := writeRecipe(t, dir, "version: 1\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\n    expected: maybe\n")

	code, stdout, _ := run(t, "generate", path, "--out", out)
	if code != cli.ExitRecipe {
		t.Errorf("exit %d, expected %d", code, cli.ExitRecipe)
	}
	if stdout != "" {
		t.Errorf("a failed run wrote to standard output: %q", stdout)
	}
	if entries, err := os.ReadDir(out); err == nil && len(entries) > 0 {
		t.Errorf("a recipe that failed validation left %d entries in the output directory", len(entries))
	}
}

// A key this build cannot honour has to say so in those words. Reporting it as
// an unknown key would send the reader looking for a typo that is not there,
// and accepting it in silence would run something other than what was asked.
func TestAKeyThisBuildCannotHonourSaysSoRatherThanBeingIgnored(t *testing.T) {
	dir := t.TempDir()
	path := writeRecipe(t, dir, `version: 1
targets:
  - id: a
    format: txt
    size: 1kb
policy:
  zero_byte_files: reject
`)

	code, _, errOut := run(t, "validate", path)
	if code != cli.ExitRecipe {
		t.Fatalf("exit %d, expected %d", code, cli.ExitRecipe)
	}
	if !strings.Contains(errOut, "policy") || !strings.Contains(errOut, "not in this build yet") {
		t.Errorf("the message does not say that policy is a documented key that has not arrived:\n%s", errOut)
	}
}

// Found by mutation testing, not by reasoning: a file with two YAML documents
// parsed without a word and produced the first one only. Half the fixtures
// somebody asked for, and exit code zero to say it went fine.
func TestAFileHoldingTwoRecipesIsRefusedRatherThanHalfRun(t *testing.T) {
	dir := t.TempDir()
	path := writeRecipe(t, dir, `version: 1
targets:
  - id: a
    format: txt
    size: 1kb
---
version: 1
targets:
  - id: b
    format: txt
    size: 2kb
`)

	code, stdout, errOut := run(t, "validate", path)
	if code != cli.ExitRecipe {
		t.Errorf("exit %d, expected %d - the second document would be dropped in silence", code, cli.ExitRecipe)
	}
	if strings.Contains(stdout, "is valid") {
		t.Errorf("the tool called a half readable file valid:\n%s", stdout)
	}
	if !strings.Contains(errOut, "document") {
		t.Errorf("the message does not explain what is wrong:\n%s", errOut)
	}
}

// A byte order mark is not a typo, and the message used to say it was.
//
// Notepad on Windows writes one by default and this tool is aimed at testers
// on Windows. Left in place the mark reaches the decoder as part of the first
// key, which refuses `version` as an unknown field - and the mark does not
// render, so the reader sees "unknown field version" and has nothing to go on.
// Measured before the fix, that was the whole message.
func TestARecipeCarryingAByteOrderMarkIsRead(t *testing.T) {
	const body = `version: 1
targets:
  - id: a
    format: txt
    size: 1kb
`
	const mark = "\ufeff"
	dir := t.TempDir()
	path := writeRecipe(t, dir, mark+body)

	// The mark has to be there, or this guard proves nothing. A fixture that
	// quietly lost it would pass for the wrong reason.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if !bytes.HasPrefix(raw, []byte(mark)) {
		t.Fatal("the fixture lost its byte order mark, so this guard would pass without testing anything")
	}

	code, stdout, errOut := run(t, "validate", path)
	if code != cli.ExitOK {
		t.Fatalf("a recipe with a byte order mark was refused, exit %d:\n%s", code, errOut)
	}
	if !strings.Contains(stdout, "1 target, 1 file") {
		t.Errorf("the recipe was read but not whole:\n%s", stdout)
	}

	// The settled shape has no mark, so a file that has one is reported as
	// unsettled and -w takes it off. Otherwise the mark rides along forever.
	if code, _, _ := run(t, "recipe", "fmt", "--check", path); code != cli.ExitRecipe {
		t.Errorf("--check called a file with a byte order mark settled, so -w would never remove it")
	}
	if code, _, errOut := run(t, "recipe", "fmt", "-w", path); code != cli.ExitOK {
		t.Fatalf("-w gave %d:\n%s", code, errOut)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if bytes.HasPrefix(after, []byte(mark)) {
		t.Error("-w left the byte order mark in place")
	}

	// A mark at the front is an encoding artefact and comes off. The same
	// character inside a value is not one - there it is a zero width no break
	// space somebody put in a name or an id, and taking it out would hand back
	// a different word than the recipe asked for. That is the quiet name change
	// this project refuses everywhere else.
	//
	// Without this case the difference between taking the mark off the front
	// and deleting it wherever it appears is invisible, and the guard passes on
	// either. Measured 2026-08-04: the mark inside an id survives a settle.
	//
	// Both marks, and that is not decoration. The stripping runs while the text
	// still starts with one, so a recipe carrying only the inner mark never
	// reaches the code being tested - a first attempt at this case used the
	// inner mark alone and the mutation walked straight past it.
	inner := writeRecipe(t, t.TempDir(), mark+`version: 1
targets:
  - id: a`+mark+`b
    format: txt
    size: 1kb
`)
	if code, _, errOut := run(t, "validate", inner); code != cli.ExitOK {
		t.Fatalf("an id holding the mark was refused, exit %d:\n%s", code, errOut)
	}
	if code, _, errOut := run(t, "recipe", "fmt", "-w", inner); code != cli.ExitOK {
		t.Fatalf("settling a recipe whose id holds the mark gave %d:\n%s", code, errOut)
	}
	settled, err := os.ReadFile(inner)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if !bytes.Contains(settled, []byte(mark)) {
		t.Errorf("the mark inside the id was removed, so the target is now called something else:\n%s", settled)
	}
	if code, _, _ := run(t, "recipe", "fmt", "--check", path); code != cli.ExitOK {
		t.Error("the file the formatter just wrote is still not settled")
	}

	// A mark further along the file is content, and has to survive. Checking
	// this through the formatter rather than through an exit code is the
	// point: stripping every mark in the file leaves a recipe that is still
	// perfectly valid, so an exit code would notice nothing. What it destroys
	// is somebody's text, and the formatter is where that becomes visible.
	inside := writeRecipe(t, t.TempDir(), "# a note with a"+mark+"mark in it\n"+body)
	code, stdout, errOut = run(t, "recipe", "fmt", inside)
	if code != cli.ExitOK {
		t.Fatalf("recipe fmt gave %d:\n%s", code, errOut)
	}
	if !strings.Contains(stdout, "a"+mark+"mark") {
		t.Errorf("a mark inside the file was eaten - only a leading one is an encoding artefact, the rest is somebody's text:\n%q", stdout)
	}
}

// One recipe stays one recipe whatever the separators around it look like.
//
// The one document rule used to count raw YAML documents, and this parser
// makes a document out of a comment sitting before a leading "---", and
// another out of a trailing "---" with nothing after it. Both files hold
// exactly one recipe. Both were refused, with a message about separators the
// reader had not got wrong - and a leading "---" is ordinary YAML house style,
// so this was a file somebody had every reason to write.
//
// The layouts are driven through the whole path rather than through the
// parser, because the parser is exactly the thing that was misread.
func TestOneRecipeIsAcceptedWhateverSeparatorsSurroundIt(t *testing.T) {
	const body = `version: 1
targets:
  - id: a
    format: txt
    size: 1kb
`
	accepted := []struct {
		name string
		src  string
	}{
		{"no separator at all", body},
		{"leading separator", "---\n" + body},
		{"comment then leading separator", "# Fixtures for the upload form.\n---\n" + body},
		{"two comments then separator", "# one\n# two\n---\n" + body},
		{"trailing separator", body + "---\n"},
		{"trailing separator and comment", body + "---\n# nothing after this\n"},
		{"a comment at each end", "# header\n---\n" + body + "---\n# footer\n"},
	}
	for _, c := range accepted {
		t.Run(c.name, func(t *testing.T) {
			path := writeRecipe(t, t.TempDir(), c.src)

			code, stdout, errOut := run(t, "validate", path)
			if code != cli.ExitOK {
				t.Fatalf("validate refused a single recipe with exit %d:\n%s", code, errOut)
			}
			// Refusing is one failure. Reading only part of it is the worse
			// one, and it looks like success from the exit code alone.
			if !strings.Contains(stdout, "1 target, 1 file") {
				t.Errorf("the recipe was read but not whole:\n%s", stdout)
			}

			if code, _, errOut := run(t, "recipe", "fmt", path); code != cli.ExitOK {
				t.Errorf("recipe fmt refused a single recipe with exit %d:\n%s", code, errOut)
			}
		})
	}

	// The rule still has to do its job. A file holding two recipes stays
	// refused - loosening the count must not loosen that.
	t.Run("two recipes are still refused", func(t *testing.T) {
		path := writeRecipe(t, t.TempDir(), body+"---\n"+body)
		if code, _, _ := run(t, "validate", path); code != cli.ExitRecipe {
			t.Errorf("exit %d, expected %d - the count was loosened too far", code, cli.ExitRecipe)
		}
	})
	t.Run("two recipes behind a comment are still refused", func(t *testing.T) {
		path := writeRecipe(t, t.TempDir(), "# header\n---\n"+body+"---\n"+body)
		if code, _, _ := run(t, "validate", path); code != cli.ExitRecipe {
			t.Errorf("exit %d, expected %d - a comment must not hide a second recipe", code, cli.ExitRecipe)
		}
	})
}

// The formatter and the reader have to agree on what a recipe is.
//
// They used to disagree in the worst possible direction: "tfg recipe fmt" laid
// out a file holding two recipes without a word and ended with code 0, and
// with -w it settled the file so --check passed too. A pre commit hook went
// green on a file "tfg generate" then refused. The reader was told the layout
// was the problem, and the layout was fine.
//
// The -w case is the one worth naming: a refusal has to leave the file on disk
// exactly as it was, because the alternative is a half rewritten recipe.
func TestTheFormatterRefusesTheSameFileTheReaderRefuses(t *testing.T) {
	dir := t.TempDir()
	const twoRecipes = `version: 1
targets:
  - id: a
    format: txt
    size: 1kb
---
version: 1
targets:
  - id: b
    format: txt
    size: 2kb
`
	path := writeRecipe(t, dir, twoRecipes)

	// Printing. The old behaviour was exit 0 with both recipes laid out.
	code, stdout, errOut := run(t, "recipe", "fmt", path)
	if code != cli.ExitRecipe {
		t.Errorf("recipe fmt gave %d, expected %d - the formatter accepts a file generate refuses", code, cli.ExitRecipe)
	}
	if stdout != "" {
		t.Errorf("a refused run wrote to stdout:\n%s", stdout)
	}
	if !strings.Contains(errOut, "document") {
		t.Errorf("the message does not say the file holds more than one recipe:\n%s", errOut)
	}
	// A hook runs this over a directory. A refusal that does not name the file
	// leaves the reader to find it themselves.
	if !strings.Contains(errOut, path) {
		t.Errorf("the message does not name the file it refused:\n%s", errOut)
	}

	// --check is the hook facing form, so a wrong answer here is the one that
	// reaches a repository. It must not blame the layout.
	code, stdout, errOut = run(t, "recipe", "fmt", "--check", path)
	if code != cli.ExitRecipe {
		t.Errorf("--check gave %d, expected %d", code, cli.ExitRecipe)
	}
	if stdout != "" {
		t.Errorf("--check wrote to stdout:\n%s", stdout)
	}
	if !strings.Contains(errOut, "document") {
		t.Errorf("--check blames the layout for a file that holds two recipes:\n%s", errOut)
	}
	if strings.Contains(errOut, "settled shape") {
		t.Errorf("--check sends the reader to run the formatter, which will not fix this:\n%s", errOut)
	}

	// -w must refuse without touching the file.
	if code, _, _ = run(t, "recipe", "fmt", "-w", path); code != cli.ExitRecipe {
		t.Errorf("-w gave %d, expected %d", code, cli.ExitRecipe)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if string(after) != twoRecipes {
		t.Errorf("a refused -w rewrote the file on disk:\n%s", after)
	}

	// The invariant behind all three: whatever the formatter accepts, the
	// reader accepts. Asserting it directly means a future third command
	// cannot drift away from the pair without this failing.
	validateCode, _, _ := run(t, "validate", path)
	if (code == cli.ExitOK) != (validateCode == cli.ExitOK) {
		t.Errorf("recipe fmt ended %d and validate ended %d - the two disagree about what a recipe is", code, validateCode)
	}
}

// "tfg recipe fmt" is what keeps a saved recipe from producing a whole file
// diff. It has to settle, and it must not touch a file that is already
// settled - a formatter that rewrites regardless churns git history.
func TestFormattingCommandSettlesAndLeavesASettledFileAlone(t *testing.T) {
	dir := t.TempDir()
	messy := "# a comment\nversion:    1\n\ntargets:\n  - id: a      # inline\n    format: txt\n    size: 1kb\n"
	path := writeRecipe(t, dir, messy)

	if code, _, _ := run(t, "recipe", "fmt", "--check", path); code != cli.ExitRecipe {
		t.Errorf("--check on an unsettled file gave %d, expected %d", code, cli.ExitRecipe)
	}

	if code, _, _ := run(t, "recipe", "fmt", "-w", path); code != cli.ExitOK {
		t.Fatalf("-w gave %d", code)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if !strings.Contains(string(after), "# a comment") || !strings.Contains(string(after), "# inline") {
		t.Errorf("formatting dropped a comment, which is the one reason this format is YAML:\n%s", after)
	}

	if code, _, _ := run(t, "recipe", "fmt", "--check", path); code != cli.ExitOK {
		t.Errorf("--check still refuses the file the formatter just wrote, so the formatter never settles")
	}

	// Writing again must be a no-op, byte for byte.
	if code, _, _ := run(t, "recipe", "fmt", "-w", path); code != cli.ExitOK {
		t.Fatalf("second -w gave %d", code)
	}
	twice, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading back: %v", err)
	}
	if !bytes.Equal(after, twice) {
		t.Error("formatting an already formatted file changed it, so every run would churn the file")
	}
}

// A boundary set is the case this whole tool is pointed at. An application
// claims a limit, and a test needs one file just under it, one exactly on it
// and one just over. The three sizes have to be consecutive, which is why WAV
// pads the way it does.
func TestABoundaryGivesThreeConsecutiveSizesAroundTheLimit(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	path := writeRecipe(t, dir, `version: 1
seed: 7741
targets:
  - id: edges
    format: wav
    boundary: 1mib
`)

	if code, _, errOut := run(t, "generate", path, "--out", out); code != cli.ExitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}

	m := readManifest(t, filepath.Join(out, "manifest.json"))
	if len(m.Files) != 3 {
		t.Fatalf("a boundary set produced %d files, expected 3", len(m.Files))
	}

	const limit = 1 << 20
	want := []int64{limit - 1, limit, limit + 1}
	for i, f := range m.Files {
		if f.Bytes != want[i] {
			t.Errorf("file %d is %d B, expected %d B", i+1, f.Bytes, want[i])
		}
		// The manifest is not evidence on its own. What is on disk is.
		info, err := os.Stat(filepath.Join(out, f.Name))
		if err != nil {
			t.Fatalf("stat %s: %v", f.Name, err)
		}
		if info.Size() != want[i] {
			t.Errorf("%s is %d B on disk and the manifest says %d B", f.Name, info.Size(), want[i])
		}
	}
}

// Two ways of stating the size at once means two different things at once, and
// picking one silently is how somebody ends up testing a limit they did not
// set.
func TestABoundaryBesideASizeOrACountIsRefused(t *testing.T) {
	for _, c := range []struct{ name, extra, word string }{
		{"size", "    size: 2mb\n", "size"},
		{"count", "    count: 5\n", "count"},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeRecipe(t, dir, "version: 1\ntargets:\n  - id: edges\n    format: txt\n    boundary: 1mib\n"+c.extra)

			code, _, errOut := run(t, "validate", path)
			if code != cli.ExitRecipe {
				t.Errorf("exit %d, expected %d for a boundary beside a %s", code, cli.ExitRecipe, c.word)
			}
			if !strings.Contains(errOut, c.word) {
				t.Errorf("the message does not name %s:\n%s", c.word, errOut)
			}
		})
	}
}

// The edge below the limit is the one a format can refuse, and it has to be
// refused before any of the three files exists.
//
// Two ways it can be impossible, and both are real. Measured on WAV with the
// label off: 43 B is below the minimum of 44 B, and everything from 45 to
// 51 B is unreachable because the smallest padding chunk costs 8 B on its own.
// Every format has a gap like that just above its minimum, so a boundary set
// placed there is a case users will meet.
func TestABoundaryTheFormatCannotDeliverIsRefusedBeforeAnyFile(t *testing.T) {
	// The label is off throughout because it needs room of its own, which
	// would move the effective minimum and refuse the run for another reason.
	const head = "version: 1\ndefaults:\n  label: false\ntargets:\n  - id: edges\n    format: wav\n    boundary: "

	for _, c := range []struct {
		name     string
		boundary int
	}{
		{"lower edge below the minimum", 44},     // asks for 43
		{"lower edge in an unreachable gap", 52}, // asks for 51
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			out := filepath.Join(dir, "out")
			path := writeRecipe(t, dir, head+strconv.Itoa(c.boundary)+"\n")

			code, stdout, _ := run(t, "generate", path, "--out", out)
			if code != cli.ExitFormat {
				t.Errorf("exit %d, expected %d for a size the format cannot deliver", code, cli.ExitFormat)
			}
			if stdout != "" {
				t.Errorf("a failed run wrote to standard output: %q", stdout)
			}
			if entries, err := os.ReadDir(out); err == nil && len(entries) > 0 {
				t.Errorf("%d files were written before the impossible size was noticed", len(entries))
			}
		})
	}
}

// A name is a name, not a path.
//
// A recipe travels between teams by design, so a name carrying "../" in a file
// somebody sent over would write outside the directory its reader chose. The
// free space check, the collision check and cleanup all work on that
// directory, so none of them would even be looking in the right place.
func TestANameThatIsAPathIsRefusedAndNothingEscapes(t *testing.T) {
	for _, name := range []string{"../escaped.txt", "sub/dir.txt", `back\slash.txt`, "..", "."} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			out := filepath.Join(dir, "out")
			path := writeRecipe(t, dir,
				"version: 1\ntargets:\n  - id: a\n    format: txt\n    size: 64\n    name: \""+name+"\"\noutput:\n  dir: "+filepath.ToSlash(out)+"\n")

			code, stdout, errOut := run(t, "generate", path)
			if code != cli.ExitRecipe {
				t.Errorf("exit %d, expected %d for the name %q", code, cli.ExitRecipe, name)
			}
			if stdout != "" {
				t.Errorf("a failed run wrote to standard output: %q", stdout)
			}
			if !strings.Contains(errOut, "name") {
				t.Errorf("the message does not explain what is wrong:\n%s", errOut)
			}

			// Nothing anywhere, not just nothing in the output directory.
			var found []string
			_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
				if err == nil && !d.IsDir() && !strings.HasSuffix(p, "recipe.yaml") {
					found = append(found, p)
				}
				return nil
			})
			if len(found) > 0 {
				t.Errorf("a refused name still produced %v", found)
			}
		})
	}
}

// The manifest lands beside the files, so its name is a name too.
func TestAManifestNameThatIsAPathIsRefused(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	path := writeRecipe(t, dir, "version: 1\ntargets:\n  - id: a\n    format: txt\n    size: 64\noutput:\n  dir: "+
		filepath.ToSlash(out)+"\n  manifest: \"../stray.json\"\n")

	code, _, errOut := run(t, "generate", path)
	if code != cli.ExitRecipe {
		t.Errorf("exit %d, expected %d", code, cli.ExitRecipe)
	}
	if !strings.Contains(errOut, "manifest") {
		t.Errorf("the message does not name the manifest:\n%s", errOut)
	}
	if _, err := os.Stat(filepath.Join(dir, "stray.json")); err == nil {
		t.Error("a manifest was written outside the output directory")
	}
}

// A placeholder that does not exist used to end up in the file name, so
// somebody asking for numbering got a file called file_{index}.txt instead.
func TestAnUnknownPlaceholderInANameIsRefused(t *testing.T) {
	dir := t.TempDir()
	path := writeRecipe(t, dir,
		"version: 1\ntargets:\n  - id: a\n    format: txt\n    size: 64\n    name: \"file_{index}.txt\"\noutput:\n  dir: "+
			filepath.ToSlash(filepath.Join(dir, "out"))+"\n")

	code, _, errOut := run(t, "validate", path)
	if code != cli.ExitRecipe {
		t.Errorf("exit %d, expected %d", code, cli.ExitRecipe)
	}
	if !strings.Contains(errOut, indexPlaceholder) {
		t.Errorf("the message does not say which placeholder exists:\n%s", errOut)
	}
}

const indexPlaceholder = "{index:04}"

// The reason is half of an expectation. It used to be read from the recipe and
// dropped on the way, so the manifest said "reject" with nothing to say why.
func TestTheDeclaredReasonReachesTheManifest(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	path := writeRecipe(t, dir, `version: 1
targets:
  - id: oversized
    format: txt
    size: 64
    expected:
      outcome: reject
      reason: size_limit
output:
  dir: `+filepath.ToSlash(out)+"\n")

	if code, _, errOut := run(t, "generate", path); code != cli.ExitOK {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	m := readManifest(t, filepath.Join(out, "manifest.json"))
	if len(m.Files) != 1 {
		t.Fatalf("%d files", len(m.Files))
	}
	if m.Files[0].Expected.Reason != "size_limit" {
		t.Errorf("the manifest carries the reason %q, the recipe said size_limit", m.Files[0].Expected.Reason)
	}
}

// The list of reasons is closed so a report can group by it. A typo would make
// a category of one, and nobody would notice.
func TestAReasonOffTheClosedListIsRefused(t *testing.T) {
	dir := t.TempDir()
	path := writeRecipe(t, dir, "version: 1\ntargets:\n  - id: a\n    format: txt\n    size: 64\n    expected:\n      outcome: reject\n      reason: too_chunky\n")

	code, _, errOut := run(t, "validate", path)
	if code != cli.ExitRecipe {
		t.Errorf("exit %d, expected %d", code, cli.ExitRecipe)
	}
	if !strings.Contains(errOut, "size_limit") {
		t.Errorf("the message does not list what is allowed:\n%s", errOut)
	}
}

// The same recipe has to give the same bytes, which is what makes a recipe in
// a repository a replacement for binary fixtures.
func TestTheSameRecipeTwiceGivesTheSameFiles(t *testing.T) {
	dir := t.TempDir()
	path := writeRecipe(t, dir, sampleRecipe)

	first := filepath.Join(dir, "a")
	second := filepath.Join(dir, "b")
	if code, _, _ := run(t, "generate", path, "--out", first); code != cli.ExitOK {
		t.Fatalf("first run exit %d", code)
	}
	if code, _, _ := run(t, "generate", path, "--out", second); code != cli.ExitOK {
		t.Fatalf("second run exit %d", code)
	}

	a := readManifest(t, filepath.Join(first, "manifest.json"))
	b := readManifest(t, filepath.Join(second, "manifest.json"))
	if len(a.Files) == 0 {
		t.Fatal("the run produced no files")
	}
	for i := range a.Files {
		if a.Files[i].Hashes.SHA256 != b.Files[i].Hashes.SHA256 {
			t.Errorf("%s differs between two runs of the same recipe", a.Files[i].Name)
		}
	}
}

func writeRecipe(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "recipe.yaml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("writing the recipe: %v", err)
	}
	return path
}

func run(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	code := cli.Run(context.Background(), args, &out, &errOut)
	return code, out.String(), errOut.String()
}

// manifestShape is the part of the manifest these guards assert on. Decoding
// into the real type would make the guard pass by construction whenever the
// type changed, so it reads the JSON a consumer actually receives.
type manifestShape struct {
	Run struct {
		Seed       int64  `json:"seed"`
		RecipeHash string `json:"recipe_hash"`
		Overrides  map[string]struct {
			FromRecipe any `json:"from_recipe"`
			FromFlag   any `json:"from_flag"`
		} `json:"overrides"`
	} `json:"run"`
	Summary struct {
		FileCount int `json:"file_count"`
	} `json:"summary"`
	Files []struct {
		Name     string `json:"name"`
		Bytes    int64  `json:"bytes"`
		Expected struct {
			Outcome string `json:"outcome"`
			Reason  string `json:"reason"`
		} `json:"expected"`
		Hashes struct {
			SHA256 string `json:"sha256"`
		} `json:"hashes"`
		Properties map[string]any `json:"properties"`
	} `json:"files"`
}

func readManifest(t *testing.T, path string) manifestShape {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the manifest: %v", err)
	}
	var m manifestShape
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("parsing the manifest: %v", err)
	}
	return m
}

// A recipe arrives from somebody else's repository - it can come in a pull
// request - so its size is chosen by a stranger, and reading time grows with
// it. The cost sits inside the YAML parser, where nothing we do afterwards can
// reduce it, so the only lever is refusing the bytes before they get there.
//
// Measured 2026-08-02: 80 kB of deliberate nesting cost about 1.3 s over the
// baseline, growing faster than linear.
func TestARecipeTooLargeToBeWrittenByHandIsRefused(t *testing.T) {
	dir := t.TempDir()

	// One byte past the limit, so this fails if the boundary moves either way.
	huge := filepath.Join(dir, "huge.yaml")
	body := append([]byte("version: 1\n# "), bytes.Repeat([]byte("x"), recipe.MaxBytes)...)
	if err := os.WriteFile(huge, body, 0o644); err != nil {
		t.Fatalf("writing the oversized recipe: %v", err)
	}

	// Every command that takes a recipe, because the check lives in one helper
	// and the point of the guard is that no command bypasses it.
	for _, args := range [][]string{
		{"validate", huge},
		{"generate", huge},
		{"recipe", "fmt", huge},
		{"validate", huge, "--json"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := cli.Run(context.Background(), args, &out, &errOut)
			if code != cli.ExitRecipe {
				t.Fatalf("exit code %d, expected %d - an oversized recipe is a bad recipe, not an I/O failure",
					code, cli.ExitRecipe)
			}
			said := errOut.String() + out.String()
			if !strings.Contains(said, "the limit is") {
				t.Errorf("the refusal does not say what the limit is: %q", said)
			}
		})
	}

	// A recipe just under the limit still works, or the guard above would be
	// satisfied by refusing everything.
	fits := filepath.Join(dir, "fits.yaml")
	pad := recipe.MaxBytes - 200
	body = append([]byte("version: 1\n# "), bytes.Repeat([]byte("x"), int(pad))...)
	body = append(body, []byte("\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\n")...)
	if err := os.WriteFile(fits, body, 0o644); err != nil {
		t.Fatalf("writing the large but legal recipe: %v", err)
	}
	var out, errOut bytes.Buffer
	if code := cli.Run(context.Background(), []string{"validate", fits}, &out, &errOut); code != cli.ExitOK {
		t.Fatalf("a recipe under the limit was refused with %d: %s", code, errOut.String())
	}
}
