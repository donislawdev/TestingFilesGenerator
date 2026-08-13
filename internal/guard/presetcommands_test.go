package guard

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/preset"
)

// The preset commands, guarded where they can go wrong quietly.
//
// The engine behind them was already watched - a preset expands into a recipe
// this build accepts, the flip sits exactly at the limit, an impossible set is
// refused whole. What arrived with the commands is a surface, and a surface
// fails in ways an expansion cannot: a number reported that nothing computed, a
// channel that puts prose into a file somebody is about to commit, an ending CI
// cannot tell apart from a typo.

// The budget is the number somebody plans disk space around, and until
// 2026-08-04 it was a figure typed into docs/PRESETS.md beside the code. It was
// wrong by a factor of three and a half for three days, because the distances of
// a boundary set cancel in pairs: the set is seven times the limit, not about
// twice it. Nobody noticed, because nothing compared the two.
//
// So the guard is not that the number is 7 times - that would be the same typed
// figure moved into a test. It is that the number "preset show" prints is the
// number a run of the same preset actually writes.
func TestTheBudgetShownIsTheBudgetWritten(t *testing.T) {
	dir := t.TempDir()

	var shown, runOut bytes.Buffer
	if code := cli.Run(context.Background(), []string{
		"preset", "show", "size-boundaries", "--limit", "4mb", "--format", "txt", "--json",
	}, &shown, &runOut); code != cli.ExitOK {
		t.Fatalf("preset show ended with %d: %s", code, runOut.String())
	}

	var entry struct {
		Budget struct {
			Targets int   `json:"targets"`
			Files   int   `json:"files"`
			Bytes   int64 `json:"total_bytes"`
		} `json:"budget"`
	}
	if err := json.Unmarshal(shown.Bytes(), &entry); err != nil {
		t.Fatalf("the machine readable form does not parse: %v\n%s", err, shown.String())
	}

	runOut.Reset()
	var runErr bytes.Buffer
	code := cli.Run(context.Background(), []string{
		"generate", "--preset", "size-boundaries", "--limit", "4mb",
		"--format", "txt", "--out", dir,
	}, &runOut, &runErr)
	if code != cli.ExitOK {
		t.Fatalf("the run ended with %d: %s", code, runErr.String())
	}

	written, total := 0, int64(0)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() == "manifest.json" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			t.Fatal(err)
		}
		written++
		total += info.Size()
	}

	if entry.Budget.Files != written {
		t.Errorf("show promised %d file(s) and the run wrote %d", entry.Budget.Files, written)
	}
	if entry.Budget.Bytes != total {
		t.Errorf("show promised %d B and the run wrote %d B - a budget nobody can plan around is worse than none",
			entry.Budget.Bytes, total)
	}
	if entry.Budget.Targets == 0 {
		t.Error("show reported no targets at all")
	}
}

// PR5: there are no closed presets. The strongest form of that is not a
// promise, it is that eject and a run consume the same bytes - so a recipe
// ejected to a file and run produces the same files as the preset it came from,
// and the manifest says so with the same hash.
//
// The hash is the sharper half. It is computed from the canonical form, so it
// survives a shell rewriting the line endings on the way into the file, and two
// runs agreeing on it means the recipe really was the same recipe.
func TestEjectingAPresetAndRunningItGivesTheSameRunBackDefaultsIncluded(t *testing.T) {
	root := t.TempDir()
	fromPreset := filepath.Join(root, "preset")
	fromFile := filepath.Join(root, "file")

	var source, notes bytes.Buffer
	if code := cli.Run(context.Background(), []string{
		"preset", "eject", "size-boundaries", "--limit", "4mb", "--format", "txt",
	}, &source, &notes); code != cli.ExitOK {
		t.Fatalf("eject ended with %d: %s", code, notes.String())
	}
	recipePath := filepath.Join(root, "ejected.yaml")
	if err := os.WriteFile(recipePath, source.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	presetHash := runAndReadHash(t, fromPreset, []string{
		"generate", "--preset", "size-boundaries", "--limit", "4mb",
		"--format", "txt", "--out", fromPreset,
	})
	fileHash := runAndReadHash(t, fromFile, []string{
		"generate", recipePath, "--out", fromFile,
	})

	if presetHash != fileHash {
		t.Errorf("the preset run recorded %s and the ejected recipe recorded %s - "+
			"then ejecting changed the recipe and PR5 is a promise rather than a fact",
			presetHash, fileHash)
	}

	// The bytes themselves, not only the record of them.
	names, err := os.ReadDir(fromPreset)
	if err != nil {
		t.Fatal(err)
	}
	compared := 0
	for _, e := range names {
		if e.Name() == "manifest.json" {
			continue
		}
		a, err := os.ReadFile(filepath.Join(fromPreset, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(fromFile, e.Name()))
		if err != nil {
			t.Errorf("%s came from the preset and not from the ejected recipe", e.Name())
			continue
		}
		if !bytes.Equal(a, b) {
			t.Errorf("%s differs between the preset and the recipe ejected from it", e.Name())
		}
		compared++
	}
	if compared == 0 {
		t.Fatal("nothing was compared, so this guard would pass on an empty directory")
	}
}

func runAndReadHash(t *testing.T, dir string, args []string) string {
	t.Helper()
	var out, errOut bytes.Buffer
	if code := cli.Run(context.Background(), args, &out, &errOut); code != cli.ExitOK {
		t.Fatalf("%v ended with %d: %s", args, code, errOut.String())
	}
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Run struct {
			RecipeHash string `json:"recipe_hash"`
		} `json:"run"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m.Run.RecipeHash == "" {
		t.Fatal("the manifest carries no recipe hash, so there is nothing to compare")
	}
	return m.Run.RecipeHash
}

// Untouchable rule 5: the manifest does not claim certainty it does not have.
//
// Some parameters describe somebody else's system. The size limit of an upload
// form is theirs, and when nobody gives it we stand a number of our own in its
// place - and then the files carry expectations reading exactly like a set built
// around the real limit. The run says so while it runs and that sentence scrolls
// away. This is the part that stays beside the files.
func TestTheManifestSaysWhichNumbersWereOursRatherThanTheCallers(t *testing.T) {
	dir := t.TempDir()
	var out, errOut bytes.Buffer
	code := cli.Run(context.Background(), []string{
		"generate", "--preset", "size-boundaries", "--format", "txt", "--out", dir,
	}, &out, &errOut)
	if code != cli.ExitOK {
		t.Fatalf("the run ended with %d: %s", code, errOut.String())
	}

	// Said out loud while it runs.
	if !strings.Contains(errOut.String(), "placeholder") {
		t.Errorf("the run never said the limit was a number of ours:\n%s", errOut.String())
	}

	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Run struct {
			Preset *struct {
				ID         string            `json:"id"`
				Parameters map[string]string `json:"parameters"`
				Defaulted  []string          `json:"defaulted"`
			} `json:"preset"`
		} `json:"run"`
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if m.Run.Preset == nil {
		t.Fatal("the manifest does not record that this run came from a preset")
	}
	if m.Run.Preset.ID != "size-boundaries" {
		t.Errorf("the manifest names the preset %q", m.Run.Preset.ID)
	}
	if m.Run.Preset.Parameters["limit"] == "" {
		t.Error("the manifest records no limit, so the set cannot be read back")
	}
	if !contains(m.Run.Preset.Defaulted, "limit") {
		t.Errorf("the limit was never given and the manifest does not say so: defaulted %v",
			m.Run.Preset.Defaulted)
	}

	// And the other way round, so this cannot pass by marking everything.
	other := t.TempDir()
	out.Reset()
	errOut.Reset()
	if code := cli.Run(context.Background(), []string{
		"generate", "--preset", "size-boundaries", "--limit", "4mb",
		"--format", "txt", "--out", other,
	}, &out, &errOut); code != cli.ExitOK {
		t.Fatalf("the run ended with %d: %s", code, errOut.String())
	}
	raw, err = os.ReadFile(filepath.Join(other, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if contains(m.Run.Preset.Defaulted, "limit") {
		t.Error("the limit was given on the command line and the manifest calls it defaulted")
	}
	if strings.Contains(errOut.String(), "placeholder") {
		t.Errorf("the limit was given and the run still called it a placeholder:\n%s", errOut.String())
	}
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

// "tfg preset eject size-boundaries > my.yaml" has to give a clean file.
//
// The note about a limit we invented is worth saying and has no business inside
// a recipe somebody is about to commit. Nothing else in this tool writes prose
// to standard output, and this is the one command whose output is a file.
func TestEjectPutsTheRecipeOnStandardOutputAndTheRestBesideIt(t *testing.T) {
	var out, errOut bytes.Buffer
	code := cli.Run(context.Background(), []string{"preset", "eject", "size-boundaries"}, &out, &errOut)
	if code != cli.ExitOK {
		t.Fatalf("eject ended with %d: %s", code, errOut.String())
	}
	if !strings.Contains(errOut.String(), "placeholder") {
		t.Errorf("nothing said the limit was a number of ours:\n%s", errOut.String())
	}
	for _, unwanted := range []string{"placeholder", "note:"} {
		if strings.Contains(out.String(), unwanted) {
			t.Errorf("the recipe on standard output carries %q, so redirecting it gives a file that is not a recipe:\n%s",
				unwanted, out.String())
		}
	}
}

// docs/CLI.md section 6 calls a parameter clashing with a global flag a mistake
// in the definition of the preset, caught at startup.
//
// It has to be caught rather than merely written down, and the reason is what
// the flag package does about it: registering one name twice panics. A panic
// reaches somebody as a stack trace under the exit code that means they mistyped
// something - the same class this project closed for YAML and for a file count
// past the ceiling. There is a message for it in the command line, and this is
// what keeps that message unreachable.
func TestNoPresetDeclaresAParameterThatIsAlreadyAFlag(t *testing.T) {
	// Taken from the command line rather than listed here, so a flag added to
	// generate tomorrow is compared against too.
	var out, errOut bytes.Buffer
	cli.Run(context.Background(), []string{"generate", "--help"}, &out, &errOut)

	taken := map[string]bool{}
	for _, line := range strings.Split(out.String(), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "-") {
			continue
		}
		name, _, _ := strings.Cut(strings.TrimLeft(trimmed, "-"), " ")
		if name != "" {
			taken[name] = true
		}
	}
	if len(taken) < 5 {
		t.Fatalf("only %d flags were read out of the help, so this guard is comparing against nothing", len(taken))
	}

	presets := preset.All()
	if len(presets) == 0 {
		t.Fatal("no preset is registered - this guard would pass without checking anything")
	}
	for _, p := range presets {
		for _, param := range p.Parameters {
			// A parameter the preset declares AND names in Reads would be two
			// answers about one name. Reads is the way to share one.
			if taken[param.Name] && !namedInReads(p, param.Name) {
				t.Errorf("the preset %s declares a parameter %q and generate already has a flag of that name. "+
					"docs/CLI.md section 6 says the preset supplies a default for the existing flag through Reads instead",
					p.ID, param.Name)
			}
		}
		for _, name := range p.Reads {
			if !taken[name] {
				t.Errorf("the preset %s reads a global flag %q and generate has no such flag, "+
					"so the value it supplies reaches nothing", p.ID, name)
			}
		}
	}
}

// budgetLine is the line carrying the counts, so a comparison is about the
// numbers rather than about the rest of the page.
func budgetLine(shown string) string {
	for _, line := range strings.Split(shown, "\n") {
		if strings.Contains(line, "target") && strings.Contains(line, "B total") {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

func namedInReads(p preset.Preset, name string) bool {
	for _, r := range p.Reads {
		if r == name {
			return true
		}
	}
	return false
}

// Every ending of the preset commands, against the frozen table.
//
// The exit code guard beside this one asks whether a code is in the table, not
// whether it is the right one - it was green while six subcommands answered
// --help with the code that means a typo. So these are named one at a time.
func TestEveryPresetEndingHasTheCodeItShould(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		what string
		args []string
		want int
	}{
		{"listing what there is", []string{"preset", "list"}, cli.ExitOK},
		{"showing one", []string{"preset", "show", "size-boundaries"}, cli.ExitOK},
		{"ejecting one", []string{"preset", "eject", "size-boundaries"}, cli.ExitOK},
		{"a preset nobody registered", []string{"preset", "show", "nosuch"}, cli.ExitUsage},
		{"a preset nobody registered, on a run", []string{"generate", "--preset", "nosuch", "--out", dir}, cli.ExitUsage},
		{"no operation at all", []string{"preset"}, cli.ExitUsage},
		{"an operation that is not one", []string{"preset", "wibble"}, cli.ExitUsage},
		{"no preset named", []string{"preset", "show"}, cli.ExitUsage},
		{"a parameter value the declaration refuses", []string{"preset", "show", "size-boundaries", "--limit", "wibble"}, cli.ExitFormat},
		// Four shapes of a bad spread, all found by fuzzing on 2026-08-05 and
		// all ending with 1 (RUNTIME) before it - which told CI this program
		// had a bug when somebody had mistyped a flag.
		{"a distance that is not a size", []string{"preset", "show", "size-boundaries", "--spread", "notasize"}, cli.ExitFormat},
		{"the same distance twice", []string{"preset", "show", "size-boundaries", "--spread", "1B,1B"}, cli.ExitFormat},
		{"the same distance spelled two ways", []string{"preset", "show", "size-boundaries", "--spread", "1024,1kb"}, cli.ExitFormat},
		{"a distance carrying a character a name cannot", []string{"preset", "show", "size-boundaries", "--spread", "1\rB"}, cli.ExitFormat},
		// A set no format here can build. FORMAT rather than RECIPE, because
		// the request is well formed and it is the floor of a format that puts
		// the smallest file of the set out of reach.
		{"a set that cannot be built", []string{"preset", "show", "size-boundaries", "--limit", "1kb", "--format", "pdf"}, cli.ExitFormat},
		{"a set that cannot be built, on a run", []string{"generate", "--preset", "size-boundaries", "--limit", "1kb", "--format", "pdf", "--out", dir}, cli.ExitFormat},
		// A flag describing one file beside a preset laying out a whole set.
		{"a size beside a preset", []string{"generate", "--preset", "size-boundaries", "--size", "1kb", "--out", dir}, cli.ExitUsage},
		{"a parameter without its preset", []string{"generate", "--format", "txt", "--size", "1kb", "--limit", "5mb", "--out", dir}, cli.ExitUsage},
		{"nothing saying what to produce", []string{"generate", "--out", dir}, cli.ExitUsage},
	}

	for _, c := range cases {
		t.Run(c.what, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := cli.Run(context.Background(), c.args, &out, &errOut)
			if code != c.want {
				t.Errorf("ended with %d and the table says %d\nstdout: %s\nstderr: %s",
					code, c.want, out.String(), errOut.String())
			}
			// A failed run puts nothing on standard output, so a consumer of a
			// pipe never receives half an answer.
			if c.want != cli.ExitOK && out.Len() != 0 {
				t.Errorf("ended with %d and still wrote %d bytes to standard output: %q",
					code, out.Len(), out.String())
			}
		})
	}
}

// The message naming the ways out has to name all of them.
//
// It named two for as long as there were two. A preset is the third and it is
// the one somebody reaching for this tool for the first time wants, so leaving
// it out sends them the long way round - and nothing about the message would
// look wrong while it did.
func TestNothingToProduceNamesAllThreeWaysOut(t *testing.T) {
	var out, errOut bytes.Buffer
	code := cli.Run(context.Background(), []string{"generate", "--out", t.TempDir()}, &out, &errOut)
	if code != cli.ExitUsage {
		t.Fatalf("ended with %d, expected %d", code, cli.ExitUsage)
	}
	said := errOut.String()
	for _, want := range []string{"--format", "recipe", "--preset"} {
		if !strings.Contains(said, want) {
			t.Errorf("the message does not mention %q, so one of the three ways to say what to produce is invisible:\n%s",
				want, said)
		}
	}
}

// The scan that decides which flags exist runs before parsing, and it has one
// job: never to be the reason a legitimate invocation is refused. What actually
// runs is the parsed value.
func TestReadingThePresetOutOfTheArgumentsHandlesBothSpellings(t *testing.T) {
	dir := t.TempDir()
	for _, args := range [][]string{
		{"generate", "--preset", "size-boundaries", "--limit", "4mb", "--format", "txt"},
		{"generate", "--preset=size-boundaries", "--limit=4mb", "--format=txt"},
		{"generate", "-preset", "size-boundaries", "-limit", "4mb", "-format", "txt"},
	} {
		t.Run(strings.Join(args[1:2], " "), func(t *testing.T) {
			out := filepath.Join(dir, strings.ReplaceAll(strings.Join(args[1:], ""), "=", "_"))
			var stdout, stderr bytes.Buffer
			code := cli.Run(context.Background(), append(args, "--out", out), &stdout, &stderr)
			if code != cli.ExitOK {
				t.Fatalf("ended with %d: %s", code, stderr.String())
			}
			entries, err := os.ReadDir(out)
			if err != nil {
				t.Fatal(err)
			}
			// Seven files of the set plus the manifest.
			if len(entries) != 8 {
				t.Errorf("wrote %d entries and the set is seven files and a manifest", len(entries))
			}
		})
	}
}

// A name a preset declares is registered as a flag, and nothing else is. The
// registration is what makes "--preset x --limit 1mb" parse at all, and a
// silent failure of it would look like a parameter nobody gave - which is the
// case that ends with a number we invented and a set that says nothing.
func TestAPresetParameterBecomesAFlagAndAnUnknownOneDoesNot(t *testing.T) {
	p, err := preset.Get("size-boundaries")
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Parameters) == 0 {
		t.Fatal("the preset declares no parameters, so this guard checks nothing")
	}

	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	fs.SetOutput(&bytes.Buffer{})
	for _, param := range p.Parameters {
		if fs.Lookup(param.Name) != nil {
			t.Fatalf("%q was already there before anything registered it", param.Name)
		}
	}

	// Two budgets rather than one number checked against a formula. A formula
	// written here would be the same figure typed twice, which is the mistake
	// the budget was moved out of the document to stop. What has to be true is
	// that giving the parameter changes the answer.
	var out, errOut bytes.Buffer
	code := cli.Run(context.Background(), []string{
		"preset", "show", "size-boundaries", "--limit", "4mb", "--format", "txt",
	}, &out, &errOut)
	if code != cli.ExitOK {
		t.Fatalf("a declared parameter was not accepted as a flag: %d %s", code, errOut.String())
	}
	given := out.String()

	out.Reset()
	errOut.Reset()
	if code := cli.Run(context.Background(), []string{
		"preset", "show", "size-boundaries", "--format", "txt",
	}, &out, &errOut); code != cli.ExitOK {
		t.Fatalf("showing it with the declared default ended with %d: %s", code, errOut.String())
	}
	if budgetLine(given) == budgetLine(out.String()) {
		t.Errorf("--limit 4mb and the declared default give the same budget, so the parameter parsed and never reached the plan:\n%s",
			budgetLine(given))
	}

	out.Reset()
	errOut.Reset()
	if code := cli.Run(context.Background(), []string{
		"preset", "show", "size-boundaries", "--wibble", "1",
	}, &out, &errOut); code != cli.ExitUsage {
		t.Errorf("a parameter nobody declared ended with %d rather than %d", code, cli.ExitUsage)
	}
}
