package guard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

// Every sentence the text package holds goes through the catalogue.
//
// There were two questions to ask about the window's words and only one was
// being asked. TestNoWordAPersonReadsIsBornOutsideTheTextPackage asks whether a
// literal was written OUTSIDE this package, which is about where a sentence
// lives. This asks whether a sentence inside it ever reaches a translator, and
// measured on 2026-08-25 the answer was no for eighteen of them - the section
// heading over a format's settings, the line saying where the files will go, the
// progress line, the outcome of a run. O130.
//
// The failure is quiet in the way this project keeps finding. gen-locale.py
// writes the catalogue from the calls it can see, so a sentence that never calls
// one is simply absent from en.json - and a translator copying that file gets an
// incomplete one with nothing to say so. They would translate every line in it
// and ship a window still speaking English in eighteen places.
//
// The rule, decided with the owner on 2026-08-26: anything that reads as
// language goes through the catalogue. A separator does not, and nothing that
// carries no letter at all is language - which is a line a program can draw, so
// the punctuation looks after itself and the list below only holds the awkward
// cases.
var notThroughTheCatalogue = map[string]string{
	"WindowTitle":             "the name of the program, which is not translated. It is the same three words in every language, and a catalogue entry for it would be an invitation to change them.",
	"HeadingAbout":            "the same name again, at the top of the About screen. Same reason.",
	"CatalogueNotLoaded":      "the sentence saying the catalogue could not be read. It cannot come from the catalogue - that is what it is about - so this one is English wherever it appears, and it is written to a terminal rather than to the window.",
	"NoWindowInThisBuild":     "written to standard error by a window binary with no window in it, so it is terminal text and D9 keeps the terminal English forever.",
	"ExactBytes":              "the byte symbol, which follows what the command line prints rather than the language of the window - the comment above it says so. Translating one and not the other would make two numbers on one screen disagree about their unit.",
	"PlaceholderNameTemplate": "an example of a file name template, so it is a value somebody could type rather than a sentence. Translating it would produce an example that does not work.",
	"SupportURL":              "an address.",
}

// wordsIn is every string literal a declaration states for itself, ignoring the
// ones handed to the catalogue.
//
// Only literals carrying a letter. A separator, a mark and a piece of spacing
// are punctuation in every language, and a rule that demanded catalogue entries
// for them would fill a translator's file with lines to copy unchanged - which
// is the opposite of what an inventory is for.
func wordsIn(node ast.Node) []string {
	var found []string
	ast.Inspect(node, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if fn, ok := call.Fun.(*ast.Ident); ok {
				switch fn.Name {
				case "say", "sayf", "sayN":
					// The English inside a catalogue call is the entry itself.
					// Its other arguments are still walked, because a sentence
					// built out of a second literal on the way in would be a
					// sentence the catalogue never sees.
					for i, arg := range call.Args {
						if i == 0 {
							continue
						}
						if _, isLit := arg.(*ast.BasicLit); isLit {
							continue
						}
						found = append(found, wordsIn(arg)...)
					}
					return false
				}
			}
		}
		// The key of a map entry is a placeholder name, not language. It has to
		// match the spelling inside the sentence exactly, so translating it
		// would break the sentence rather than translate it - the values beside
		// it are still walked.
		if kv, ok := n.(*ast.KeyValueExpr); ok {
			found = append(found, wordsIn(kv.Value)...)
			return false
		}
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(lit.Value)
		if err != nil || !strings.ContainsFunc(value, unicode.IsLetter) {
			return true
		}
		found = append(found, value)
		return true
	})
	return found
}

func TestEverySentenceInTheTextPackageGoesThroughTheCatalogue(t *testing.T) {
	checked, offenders := 0, map[string][]string{}
	for _, name := range []string{"screens.go", "text.go"} {
		path := filepath.Join("../gui/text", name)
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				checked++
				if words := wordsIn(d.Body); len(words) > 0 {
					offenders[d.Name.Name] = words
				}
			case *ast.GenDecl:
				// Constants and variables as well as functions. A sentence held
				// as a constant is read by exactly the same person.
				for _, spec := range d.Specs {
					value, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					for i, ident := range value.Names {
						checked++
						if i >= len(value.Values) {
							continue
						}
						if words := wordsIn(value.Values[i]); len(words) > 0 {
							offenders[ident.Name] = words
						}
					}
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no declaration was read in the text package, so this guard checked nothing")
	}

	var unexplained, stale []string
	for name := range offenders {
		if _, allowed := notThroughTheCatalogue[name]; !allowed {
			unexplained = append(unexplained, name)
		}
	}
	// An allowance for something that now goes through the catalogue is an
	// allowance nobody needs, and a list nobody prunes is where this drift
	// would hide next - the same rule the regression table guard keeps.
	for name := range notThroughTheCatalogue {
		if _, still := offenders[name]; !still {
			stale = append(stale, name)
		}
	}
	sort.Strings(unexplained)
	sort.Strings(stale)

	for _, name := range unexplained {
		t.Errorf("%s states %q itself, so it never reaches a translator.\n"+
			"Reason: gen-locale.py writes the catalogue from the calls it can see, so this sentence\n"+
			"is absent from en.json and a translator copying that file is handed an incomplete one\n"+
			"with nothing to say so.\n"+
			"What to do: state it with say, sayf or sayN - or name it in notThroughTheCatalogue and\n"+
			"say why it is not language.", name, offenders[name])
	}
	for _, name := range stale {
		t.Errorf("%s is excused from the catalogue and no longer needs to be.\n"+
			"What to do: take it off notThroughTheCatalogue.", name)
	}
	t.Logf("%d declaration(s) read, %d hold words of their own, all of them named", checked, len(offenders))
}
