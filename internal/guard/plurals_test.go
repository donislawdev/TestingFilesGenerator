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

// What this defends. A message that counts something says it in the right
// number: "1 file", "7 files". Not "1 file(s)".
//
// Why it needed a guard rather than care. The window stopped writing brackets
// on 2026-08-12 and the command line kept writing them, so on 2026-08-13 the
// same run was described two ways and the owner read it off the screen. The
// fix was fourteen lines across seven files - and fourteen is exactly the size
// at which "I will remember next time" stops being true. Counting them was the
// cheap part. Nothing stopped the fifteenth.
//
// Why the whole tree and not the command line. The dodge is not a property of
// a surface, it is a property of writing English against a number, and both
// surfaces do that. A rule that watched internal/cli would have been green on
// the day the window drifted, which is the drift it exists to catch.
//
// Why string literals and not the file text. Comments discuss "file(s)" on
// purpose - the doc above core.Count does, and so does the one above package
// text, because explaining a dodge means naming it. A comment is not a message
// and reading the raw file could not tell them apart. The parser can.
//
// What this does NOT check. That the two words handed to core.Count are the
// singular and the plural of the same noun. Count("file", "targets") passes
// here and is nonsense, and no test written from the outside can see it - the
// call sites are the only place that knows, which is why they name both words
// instead of trusting a rule that adds an "s".
func TestNoMessageDodgesThePluralWithBrackets(t *testing.T) {
	root := repoRoot(t)

	// Both spellings of the same dodge. "(s)" is the one this project wrote,
	// "(es)" is the one it would have written the first time somebody counted
	// matches or boxes.
	dodges := []string{"(s)", "(es)"}

	files, checked := 0, 0
	for _, dir := range []string{"internal", "cmd"} {
		start := filepath.Join(root, dir)
		err := filepath.WalkDir(start, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			// Tests are not messages. They quote the output they assert on, and
			// a guard that read them would fail on its own evidence.
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
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
			skip := map[any]bool{}
			for _, imported := range parsed.Imports {
				skip[imported.Path] = true
			}
			ast.Inspect(parsed, func(n ast.Node) bool {
				lit, isLit := n.(*ast.BasicLit)
				if !isLit || lit.Kind != token.STRING || skip[lit] {
					return true
				}
				checked++
				for _, dodge := range dodges {
					if !strings.Contains(lit.Value, dodge) {
						continue
					}
					t.Errorf("%s:%d writes %s, which dodges the plural.\n"+
						"A person reading a run should be told \"1 file\" or \"7 files\", never \"1 file(s)\".\n"+
						"Call core.Count(n, \"file\", \"files\") and build the sentence so no verb agrees with "+
						"the number - see the note above core.Count for why that part matters.",
						rel, fset.Position(lit.Pos()).Line, lit.Value)
					return true
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", start, err)
		}
	}

	// A floor, because every check above is inside a walk. A walk that finds
	// nothing reports nothing, and a guard that passes on an empty tree is the
	// exact shape this project has already been bitten by.
	if files < 30 {
		t.Fatalf("only %d files were read, so this guard would pass on an empty tree", files)
	}
	t.Logf("%d strings across %d files, none of them dodging a plural", checked, files)
}
