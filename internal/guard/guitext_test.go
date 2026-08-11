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

// textCarriers are the calls that put words in front of a person. A literal
// reaching any of these outside the text package is the defect.
// This list is kept by hand, so a new way to put words on screen is invisible
// to this guard until somebody adds it here. That happened on 2026-08-11, three
// times in one day: Toggle and FieldSaying arrived with the layout work and
// NewTabItem with the tabs, and every tab title was unchecked until the mutation
// runner reported this guard staying green on a literal.
var textCarriers = map[string]bool{
	"NewButton":      true,
	"NewLabel":       true,
	"NewCheck":       true,
	"NewTabItem":     true,
	"SetText":        true,
	"SetPlaceHolder": true,
	"Field":          true,
	"FieldSaying":    true,
	"Toggle":         true,
	"Prose":          true,
	"Heading":        true,
	"Screen":         true,
	"Note":           true,
	"Say":            true,
}

// literalArgs returns the non empty string literals handed to a carrier.
//
// Empty ones are allowed and are not an oversight: a box is built empty and
// filled later, and an error area is cleared by setting it to nothing. Neither
// is a word anybody reads.
func literalArgs(call *ast.CallExpr) []string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	name := ""
	switch {
	case ok:
		name = sel.Sel.Name
	default:
		if id, isID := call.Fun.(*ast.Ident); isID {
			name = id.Name
		}
	}
	if !textCarriers[name] {
		return nil
	}
	var found []string
	for _, arg := range call.Args {
		found = append(found, literalsIn(arg)...)
	}
	return found
}

// literalsIn looks inside the argument rather than only at it, because glueing
// a value onto a literal hides it from a check that reads arguments alone -
// which is how "   - " + catch survived the first sweep of this refactor.
//
// It is also the shape that translates worst. Languages disagree about where a
// name goes in a phrase, so a sentence built with + cannot be reordered later
// without finding every place that built it.
func literalsIn(node ast.Expr) []string {
	switch v := node.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING || v.Value == `""` {
			return nil
		}
		return []string{v.Value}
	case *ast.BinaryExpr:
		return append(literalsIn(v.X), literalsIn(v.Y)...)
	}
	return nil
}

func TestTheWindowSaysNothingItDoesNotSayFromTheTextPackage(t *testing.T) {
	root := repoRoot(t)
	gui := filepath.Join(root, "internal", "gui")

	checked, files := 0, 0
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
		ast.Inspect(parsed, func(n ast.Node) bool {
			call, isCall := n.(*ast.CallExpr)
			if !isCall {
				return true
			}
			checked++
			for _, lit := range literalArgs(call) {
				t.Errorf("%s:%d hands %s straight to a person.\n"+
					"Text somebody reads belongs in internal/gui/text, because D9 gives this surface "+
					"translations and the command line never gets them. Put it there and call it from here.",
					rel, fset.Position(call.Pos()).Line, lit)
			}
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
	t.Logf("%d call(s) across %d file(s) carry no literal a person would read", checked, files)
}
