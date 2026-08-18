package guard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The window says everything it says from one package, and this is what keeps
// it that way.
//
// D9 gives the window translations and keeps the command line English forever,
// so the window is the one surface here that will ever have a second language.
// Until 2026-08-10 every label, hint and message was a literal spread across
// six files and nobody could say how many there were - measured then: about
// forty, among a hundred and fifty other literals that are keys and format ids
// and never reach a person.
//
// A rule in prose would not have held. This project has the receipts: the
// palette in docs/UX.md section 8 was measured before the window existed
// precisely so that wiring it would be free, and three screens later it is
// still not wired (O70). The difference between that and this is a test.
//
// What it does NOT claim. It is not i18n, and passing it does not mean the
// window can be translated - there is no catalogue and no lookup. It means the
// inventory is in one place and stays there, so the day a catalogue arrives it
// goes underneath one package instead of being chased through six.

// notWords are the string literals this window may hand to a call, each with
// the reason it is not something a person reads as prose.
//
// The rule is the other way round as of 2026-08-13, and the inversion is the
// whole point. It used to be a list of CARRIERS - the calls that put words on
// screen - which is open ended by nature: a new way to show text is invisible
// to the guard until somebody remembers to add it. That happened three times in
// one day on 2026-08-11, and again on 2026-08-12 with Section, Title and
// Screen, which meant every section heading and every screen heading had never
// been looked at. O80, and the same shape as the field registry: an audit that
// walks a list somebody wrote cannot find what they left out of it.
//
// Now every literal handed to any call is suspect and these are the exceptions.
// The list can only be argued down, never grown by forgetting - a new one has
// to be written here with its reason, in front of somebody.
//
// Two things fell out of the inversion immediately and both were real: the
// window passed "how many" and "seed" into its own refusal about a number that
// is not a number, so two sentences a person reads were built outside the text
// package and no carrier list would ever have named them.
var notWords = map[string]string{
	`"10mb"`:          "the size a fresh screen starts at, a value rather than prose",
	`"1"`:             "how many files a fresh screen starts at",
	`"0"`:             "the seed a fresh screen starts at",
	`"files"`:         "the group name a fresh screen starts at, and a recipe value",
	`"tfg-gui"`:       "recorded in the manifest as the command that ran, a contract value",
	`"chickpea.png"`:  "the name the toolkit files the icon resource under, never shown",
	`"preset"`:        "the key the preset field is registered under, not a label",
	`"."`:             "the working directory, when the system will not say which one it is",
	`". "`:            "what joins two sentences the declaration already carries",
	`"png"`:           "a format id in a comment example rather than a label",
	`"windows"`:       "a system name asked of the compiler",
	`"linux"`:         "a system name asked of the compiler",
	`"darwin"`:        "a system name asked of the compiler",
	`"GOOS="`:         "an environment variable handed to the compiler",
	`"CGO_ENABLED=1"`: "an environment variable handed to the compiler",
	`"true"`:          "compared against a declared default, never shown",
	`"\n"`:            "what joins the lines of the status area, not a word",
	`"\n\n"`:          "the blank line between two refusals the form could not place, not a word",
	`"Ag"`:            "a sample measured to find how tall a row must be - an ascender and a descender, never drawn",
	`"%s: %w"`:        "how one error is wrapped around another, both already worded",
	`"•"`:             "the marker in front of a list item, a shape rather than a word",
	`"panel"`:         "our name for a colour, in the palette the toolkit asks by name",
	`"fyneDo"`:        "a migration flag the toolkit reads, never shown",
	// The application's own name and id are its identity rather than prose.
	// An application is not renamed in another language, and the desktop uses
	// both to decide which running program this is. What a person READS in the
	// title bar is text.WindowTitle, which is built from this plus the version.
	`"dev.donislaw.tfg"`:        "the id the desktop knows this program by",
	`"Testing Files Generator"`: "the name the desktop knows this program by",
}

// literalArgs and literalsIn are gone as of 2026-08-13. They read the arguments
// of a call, which was the whole of what this guard could see, and a sentence
// assigned to a struct field is not an argument to anything - the mutation
// runner said so by reporting this guard green on exactly that. What replaced
// them is every string in the file, which needs no helper.
//
// The empty string is still allowed and is not an oversight: a box is built
// empty and filled later, and an error area is cleared by setting it to
// nothing. Neither is a word anybody reads.

func TestTheWindowSaysNothingItDoesNotSayFromTheTextPackage(t *testing.T) {
	root := repoRoot(t)
	gui := filepath.Join(root, "internal", "gui")

	checked, files := 0, 0
	// The import paths of the file being read, which are strings and are not
	// words. Collected per file, just before the walk that uses them.
	skip := map[interface{}]bool{}
	err := filepath.WalkDir(gui, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		// The one package allowed to hold the words is the one whose whole
		// purpose is holding them.
		if strings.Contains(filepath.ToSlash(path), "/gui/text/") {
			return nil
		}
		files++

		fset := token.NewFileSet()
		parsed, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			t.Errorf("parsing %s: %v", path, perr)
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		// Every string in the file rather than every argument of a call, which
		// is a widening made on 2026-08-13 because the mutation runner reported
		// this guard green on a literal. A sentence assigned to a struct field
		// - which is how this window carries a refusal it worded itself - is
		// not an argument to anything, so reading arguments could never see it.
		//
		// The import block is skipped because those are paths, and a path is
		// the one string in a Go file that is never read by anybody.
		for _, imported := range parsed.Imports {
			skip[imported.Path] = true
		}
		ast.Inspect(parsed, func(n ast.Node) bool {
			lit, isLit := n.(*ast.BasicLit)
			if !isLit || lit.Kind != token.STRING || skip[lit] {
				return true
			}
			checked++
			if _, excused := notWords[lit.Value]; excused || lit.Value == `""` {
				return true
			}
			t.Errorf("%s:%d holds %s where a person can read it.\n"+
				"Text somebody reads belongs in internal/gui/text, because D9 gives this surface "+
				"translations and the command line never gets them. Put it there and call it from here.\n"+
				"If it is not something anybody reads, say so in notWords with the reason.",
				rel, fset.Position(lit.Pos()).Line, lit.Value)
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", gui, err)
	}
	if files < 5 {
		t.Fatalf("only %d file(s) were read, so this guard would pass on an empty tree", files)
	}
	t.Logf("%d string(s) across %d file(s), none of them a word a person reads", checked, files)
}
