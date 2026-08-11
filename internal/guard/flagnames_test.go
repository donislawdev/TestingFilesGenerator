package guard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Nothing below the two surfaces spells a setting the way only one of them
// takes it.
//
// O79, seen on a screenshot of the refused preset screen on 2026-08-11: the
// window said "Raise --limit above 1051991 B, narrow --spread", and the window
// labels those fields "limit" and "spread" and has no flags on it anywhere. It
// was not one sentence - twelve messages across six files were written in
// command line spelling, and every one of them can be shown in the window,
// because that is the point of writing them once in the engine.
//
// The command line is where flags live and is not scanned. Neither is the
// window, which quotes nothing of the sort. What is scanned is everything both
// of them show: the engine, the core, the formats, the presets and the recipe.
//
// The pattern is deliberately about the SYNTAX rather than about a list of
// flag names. A list would need a line adding every time a flag is, which is
// the kind of guard somebody forgets to extend - and the defect is the dashes,
// not which word follows them.
func TestNoMessageBelowTheSurfacesIsWrittenInFlagSpelling(t *testing.T) {
	// A dash pair followed by a letter. "|---|---|" in generated markdown is
	// three dashes and does not match, and neither does a range or an em dash
	// written as two.
	flagLike := regexp.MustCompile(`--[a-z]`)

	dirs := []string{
		"../engine", "../core", "../preset", "../recipe", "../manifest",
		"../audit", "../format",
	}
	scanned, found := 0, 0
	for _, dir := range dirs {
		matches, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		nested, err := filepath.Glob(filepath.Join(dir, "*", "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, path := range append(matches, nested...) {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			scanned++
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			// Literals only. A comment may say --limit while explaining why the
			// message does not, and that comment is the record of the decision.
			ast.Inspect(file, func(n ast.Node) bool {
				lit, ok := n.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					return true
				}
				value, err := strconv.Unquote(lit.Value)
				if err != nil {
					return true
				}
				if flagLike.MatchString(value) {
					found++
					t.Errorf("%s writes a setting in command line spelling:\n  %q\n"+
						"This text is shown by the window too, and there are no flags on it.",
						path, value)
				}
				return true
			})
		}
	}
	if scanned == 0 {
		t.Fatal("no files were read, so this guard proved nothing")
	}
	t.Logf("read %d files below the surfaces, %d wrote a flag", scanned, found)
}
