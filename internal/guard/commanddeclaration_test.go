package guard

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// The command line describes itself from one declaration, and the documents
// that repeat it are held to it.
//
// Three lists said the same thing and nothing compared any two of them, which
// is O201. The block tfg --help prints was a raw string beside the switch that
// dispatched. The website copied that block by hand. The flag table in README
// was typed out separately from the flag set generate builds. Measured on
// 2026-09-09: README was missing --damage, a flag added to the program that
// same day, and nothing said so.
//
// The help is rendered from the declaration now, so the first of those cannot
// drift at all. The other two are documents, which no amount of structure can
// generate, so they get guards - and both are asked with a rule rather than a
// list of exceptions, because a list of exceptions is a guard with a hole.

// commandsTheToolPrints is the command names out of tfg --help, which is the
// text the page is a copy of.
//
// Asked of the help rather than of the router beside it on purpose: the help
// is what a visitor compares the page against, so agreeing with anything else
// would prove the wrong thing.
func commandsTheToolPrints(t *testing.T) []string {
	t.Helper()
	var out, errOut bytes.Buffer
	if code := cli.Run(context.Background(), []string{"--help"}, &out, &errOut); code != cli.ExitOK {
		t.Fatalf("tfg --help ended with %d rather than %d, so there is no list to compare against",
			code, cli.ExitOK)
	}
	names, err := commandNamesIn(out.String())
	if err != nil {
		t.Fatalf("reading the command block out of tfg --help: %v", err)
	}
	return names
}

// commandNamesIn takes the names out of the Commands: block of a help text.
//
// Split on two spaces rather than on the first one, because "recipe fmt" is a
// command whose name has a space in it - splitting on the first would name a
// command the router does not have.
//
// Finding nothing is an error rather than an empty list, and that is the whole
// reason this is a function of its own. A guard that quietly stopped finding
// the block would compare the page against nothing and stay green while
// proving it - which is the failure this file exists to make impossible.
func commandNamesIn(help string) ([]string, error) {
	const header = "Commands:"
	_, rest, found := strings.Cut(help, header+"\n")
	if !found {
		return nil, fmt.Errorf("no %q line in the help text", header)
	}
	var names []string
	for _, line := range strings.Split(rest, "\n") {
		if !strings.HasPrefix(line, "  ") {
			break
		}
		name, _, split := strings.Cut(strings.TrimPrefix(line, "  "), "  ")
		if !split {
			return nil, fmt.Errorf("the line %q under %s has no summary beside the name", line, header)
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("the %s block is empty", header)
	}
	return names, nil
}

// Every command the help lists can actually be run.
//
// The pair to it below is what makes this worth having: on its own this would
// pass for a declaration listing one command and hiding ten.
func TestEveryCommandTheHelpListsIsOneTheToolAnswersTo(t *testing.T) {
	listed := 0
	for _, c := range cli.Commands() {
		if c.Summary == "" {
			continue
		}
		listed++
		// The first word of the PRINTED name, not the verb beside it in the
		// declaration. That difference is the whole point: a reader types what
		// the help shows them, so asking the declaration about its own verb
		// would pass for an entry printing one word and dispatching on
		// another. Mutation said so - the first version of this asked c.Verb
		// and stayed green against exactly that swap.
		//
		// First word, because "recipe fmt" is printed as two and the dispatch
		// branches on the first.
		typed, _, _ := strings.Cut(c.Name, " ")
		var out, errOut bytes.Buffer
		// The word alone. Some need an operation or a file after it and will
		// say so, which is an answer - what would mean the help is lying is
		// "unknown command".
		cli.Run(context.Background(), []string{typed}, &out, &errOut)
		if strings.Contains(errOut.String(), "unknown command") {
			t.Errorf("the help lists %q and the tool does not know %q", c.Name, typed)
		}
	}
	if listed == 0 {
		t.Fatal("the help lists nothing, so this guard would pass against any tree")
	}
	t.Logf("%d command(s) listed in the help", listed)
}

// A command the help does not list does nothing except print that help.
//
// This is the other half, and it is a rule rather than an exception for the
// one command that has no summary today. Anything unlisted is either help
// itself - which a reader of the help has already found - or a verb somebody
// forgot to describe, and only the second is a defect. Comparing the output
// against the help tells them apart without naming anybody.
func TestACommandTheHelpDoesNotListOnlyPrintsTheHelp(t *testing.T) {
	var wanted bytes.Buffer
	cli.Run(context.Background(), []string{"--help"}, &wanted, &bytes.Buffer{})
	if wanted.Len() == 0 {
		t.Fatal("the help printed nothing, so there is nothing to compare against")
	}

	unlisted := 0
	for _, c := range cli.Commands() {
		if c.Summary != "" {
			continue
		}
		unlisted++
		var out, errOut bytes.Buffer
		if code := cli.Run(context.Background(), []string{c.Verb}, &out, &errOut); code != cli.ExitOK {
			t.Errorf("%q is not listed in the help and does not end with %d, so it is a command nobody is told about",
				c.Verb, cli.ExitOK)
		}
		if out.String() != wanted.String() {
			t.Errorf("%q is not listed in the help and does something other than print it, so it is a command nobody is told about",
				c.Verb)
		}
	}
	t.Logf("%d command(s) reachable but unlisted", unlisted)
}

// The flag table in README names every flag generate has, and no others.
//
// README is what somebody reads before they have the binary, so a flag missing
// from it is a feature that does not exist for that reader. Measured on
// 2026-09-09: --damage was in the program, in the help and on the website, and
// not in this table - the same class as the site listing nine commands out of
// ten, one document further out.
//
// Both directions, because a row for a flag that was removed sends somebody to
// type something the tool will refuse.
func TestTheReadmeFlagTableNamesEveryFlagGenerateTakes(t *testing.T) {
	declared := flagsOfGenerate(t)
	if len(declared) == 0 {
		t.Fatal("generate declared no flags, so this guard would pass against any tree")
	}
	documented := flagsInTheReadmeTable(t)

	for name := range declared {
		if !documented[name] {
			t.Errorf("generate takes --%s and the README flag table does not mention it", name)
		}
	}
	for name := range documented {
		if !declared[name] {
			t.Errorf("the README flag table offers --%s and generate does not take it", name)
		}
	}
	t.Logf("%d flag(s) declared, %d documented", len(declared), len(documented))
}

// flagsOfGenerate asks the command for its own flag set rather than reading
// the source, so a flag added by any route is covered.
//
// It runs generate with --help, which builds the set and prints it, and reads
// the names back out of what it printed. The alternative was exporting the set
// itself, which would put a seam in the command line for a test to hold.
func flagsOfGenerate(t *testing.T) map[string]bool {
	t.Helper()
	var out, errOut bytes.Buffer
	if code := cli.Run(context.Background(), []string{"generate", "--help"}, &out, &errOut); code != cli.ExitOK {
		t.Fatalf("generate --help ended with %d: %s", code, errOut.String())
	}
	// The flag package prints "  -name" at the head of each entry, one per
	// flag, whatever its type.
	found := map[string]bool{}
	for _, line := range strings.Split(out.String(), "\n") {
		if !strings.HasPrefix(line, "  -") {
			continue
		}
		name, _, _ := strings.Cut(strings.TrimPrefix(line, "  -"), " ")
		if name != "" {
			found[name] = true
		}
	}
	// A sanity check on the shape rather than on the count: flag.PrintDefaults
	// is what this reads, and a change in its format would leave the map empty
	// and every comparison below trivially true.
	if !found["format"] {
		t.Fatalf("no --format among %d name(s) read back, so the help format changed and this reads nothing", len(found))
	}
	return found
}

var readmeFlagRow = regexp.MustCompile(`(?m)^\| ` + "`" + `--([a-z-]+)`)

// flagsInTheReadmeTable reads the flag names out of the generate table.
//
// Bounded to the table under the generate heading rather than the whole file,
// because every other command documents its own flags further down and those
// are not this table's business.
func flagsInTheReadmeTable(t *testing.T) map[string]bool {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(repoRoot(t), "README.md"))
	if err != nil {
		t.Skipf("no README here: %v", err)
	}
	text := string(body)
	const first = "| `--format <id>`"
	start := strings.Index(text, first)
	if start < 0 {
		t.Fatalf("the generate flag table does not start with %q any more, so this guard reads nothing", first)
	}
	end := strings.Index(text[start:], "\n\n")
	if end < 0 {
		t.Fatal("the generate flag table does not end")
	}
	found := map[string]bool{}
	for _, m := range readmeFlagRow.FindAllStringSubmatch(text[start:start+end], -1) {
		found[m[1]] = true
	}
	return found
}
