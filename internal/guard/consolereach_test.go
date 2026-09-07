package guard

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// The console belongs to the surface, and to nothing under it.
//
// layers_test.go asks which package may IMPORT which. This asks a different
// question that a layer number cannot answer: a package may import only
// downwards and still write a line to standard output, because fmt is on
// everybody's list and always will be.
//
// Why it matters here rather than as a matter of taste. This tool exists to be
// run by somebody else's CI, and `--json` promises a document a script parses.
// One stray line printed from the engine on the SUCCESS path does not fail a
// run, does not trip "a failed run prints nothing on stdout", and turns a
// machine readable report into text that no parser accepts. The reader finds
// out in their pipeline rather than in ours.
//
// What is allowed and is not a hole: writing to an io.Writer that was handed
// in. cmd/tfg passes os.Stdout into cli.Run and everything below takes a
// writer as an argument, which is the whole point - the caller decides where
// the words go. This guard refuses the console reached DIRECTLY, by name.
//
// 🔴 Read from the syntax tree rather than by searching the text, and that is a
// measurement rather than caution. internal/oracle holds Python scripts inside
// Go raw strings and they call print() thirty times over. A text scan reports
// every one of them, and the honest fix for a guard that shouts at correct code
// is a guard that reads what the compiler reads. oracle is test only and out of
// the layer map anyway, but the next embedded script will not be.
func TestNothingBelowASurfaceReachesTheConsoleDirectly(t *testing.T) {
	// Layer 4 is the surface - internal/cli and the window - and layer 5 is a
	// main package. Those own the console by construction. Everything at 3 or
	// below is a library, and a library that prints has taken a decision that
	// belongs to whoever called it.
	const surface = 4

	var offenders []string
	packagesRead, filesRead := 0, 0

	for _, p := range packages(t) {
		depth, known := layer[p.rel]
		if !known || depth >= surface {
			continue
		}
		packagesRead++

		for _, path := range p.files {
			filesRead++
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("parsing %s: %v", path, err)
			}
			rel, err := filepath.Rel(repoRoot(t), path)
			if err != nil {
				rel = path
			}
			rel = filepath.ToSlash(rel)

			ast.Inspect(file, func(n ast.Node) bool {
				said := consoleReach(n)
				if said == "" {
					return true
				}
				offenders = append(offenders, fmt.Sprintf(
					"%s:%d %s reaches the console from layer %d - take an io.Writer instead and let the caller decide",
					rel, fset.Position(n.Pos()).Line, said, depth))
				return true
			})
		}
	}

	// The canary every guard here carries. A walk that reads nothing finds no
	// offender and looks exactly like a walk that works.
	if packagesRead < 5 || filesRead < 20 {
		t.Fatalf("the console scan read %d package(s) and %d file(s), which is too few to be this tree",
			packagesRead, filesRead)
	}

	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("%d place(s) below the surface write straight to the console:\n  %s",
			len(offenders), strings.Join(offenders, "\n  "))
	}
}

// consoleReach names what a node does to the console, or "" when it does
// nothing. Three shapes, because there are three ways to get there.
func consoleReach(n ast.Node) string {
	switch node := n.(type) {
	case *ast.SelectorExpr:
		// os.Stdout and os.Stderr, wherever they appear - passed, assigned or
		// written to. Holding one is already the decision this refuses.
		pkg, ok := node.X.(*ast.Ident)
		if !ok || pkg.Name != "os" {
			return ""
		}
		if node.Sel.Name == "Stdout" || node.Sel.Name == "Stderr" {
			return "os." + node.Sel.Name
		}
	case *ast.CallExpr:
		switch fn := node.Fun.(type) {
		case *ast.Ident:
			// The builtins. They go to standard error and survive every
			// refactor because nothing has to be imported for them to work,
			// which is exactly what makes a forgotten one hard to see.
			if fn.Name == "print" || fn.Name == "println" {
				return fn.Name + "()"
			}
		case *ast.SelectorExpr:
			pkg, ok := fn.X.(*ast.Ident)
			if !ok || pkg.Name != "fmt" {
				return ""
			}
			// Print, Printf and Println only. Fprint and its family take a
			// writer and are the sanctioned way to say something.
			if strings.HasPrefix(fn.Sel.Name, "Print") {
				return "fmt." + fn.Sel.Name
			}
		}
	}
	return ""
}
