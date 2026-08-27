package guard

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The second knob of the ratchet: how many things are CROWDING the ceiling.
//
// The ceiling in codeshape_test.go answers one question - is anything over the
// line - and it is blind to the shape that actually happens: nothing over the
// line, and everything creeping towards it. A tree where thirty functions sit
// at seventy nine lines passes that guard and is exactly the tree the guard
// exists to prevent.
//
// So the ratchet needs two knobs rather than one, which is the general form of
// "the threshold only goes down" applied to any metric that decays slowly:
//
//	a ceiling on the worst single case  - catches one thing growing to a record
//	a count of things near the ceiling  - catches everything drifting at once
//
// Measured on 2026-08-05 before the numbers below were chosen, because a
// threshold picked out of the air is a guess, and a guess written into a gate
// is a guess nobody can argue with later.
const (
	// What counts as crowding. Three quarters of the ceiling is far enough from
	// it that ordinary code does not trip the count, and close enough that
	// something arriving there is on its way.
	crowdingFileLines     = 413
	crowdingFunctionLines = 60

	// Caps measured on the tree of 2026-08-05, then frozen - eleven functions and
	// two files were already crowding, against a first guess of four. That gap is
	// the whole argument for measuring rather than choosing: a cap of four would
	// have gone in red and been raised to make it pass, which is how a ratchet
	// becomes a rubber band. Like the ceilings
	// themselves these only go down. Raising one to turn a run green is the
	// same act as editing a golden value for the same reason.
	crowdedFunctions = 10
	crowdedFiles     = 2
)

func TestNothingIsQuietlyCreepingTowardsTheCeiling(t *testing.T) {
	var functions, files []string

	for _, p := range packages(t) {
		for _, path := range p.files {
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}
			src := strings.Split(strings.ReplaceAll(string(body), "\r\n", "\n"), "\n")

			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, body, parser.ParseComments)
			if err != nil {
				t.Fatalf("parsing %s: %v", path, err)
			}
			comments := commentLines(fset, file)

			rel, err := filepath.Rel(repoRoot(t), path)
			if err != nil {
				rel = path
			}
			rel = filepath.ToSlash(rel)

			if n := codeLines(src, comments, 1, len(src)); crowding(n, crowdingFileLines) {
				files = append(files, fmt.Sprintf("%s %d/%d", rel, n, longestFile))
			}

			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				from := fset.Position(fn.Pos()).Line
				to := fset.Position(fn.End()).Line
				if n := codeLines(src, comments, from, to); crowding(n, crowdingFunctionLines) {
					functions = append(functions, fmt.Sprintf("%s:%d %s %d/%d",
						rel, from, name(fn), n, longestFunction))
				}
			}
		}
	}

	sort.Strings(functions)
	sort.Strings(files)

	if len(functions) > crowdedFunctions {
		t.Errorf("%d function(s) are %d lines of code or longer and the cap is %d:\n  %s\n\n"+
			"Nothing is over the line, which is the point - this is the drift the other guard "+
			"cannot see. Split one of these rather than raising the cap.",
			len(functions), crowdingFunctionLines, crowdedFunctions, strings.Join(functions, "\n  "))
	}
	if len(files) > crowdedFiles {
		t.Errorf("%d file(s) are %d lines of code or longer and the cap is %d:\n  %s",
			len(files), crowdingFileLines, crowdedFiles, strings.Join(files, "\n  "))
	}

	t.Logf("crowding the ceiling: %d function(s) of %d allowed, %d file(s) of %d allowed",
		len(functions), crowdedFunctions, len(files), crowdedFiles)
	for _, f := range functions {
		t.Logf("  function %s", f)
	}
	for _, f := range files {
		t.Logf("  file %s", f)
	}
}

// crowding reports whether a measurement has reached the share of the ceiling
// at which it counts as on its way there.
func crowding(n, band int) bool {
	return n >= band
}
