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
)

// contextRoots is every place a context may be made from nothing, and why.
//
// Everywhere else the context arrives from the caller, because a context made
// from nothing is one nobody can stop. That mistake was made twice in the same
// way and found by a report rather than by a guard (docs/STABILITY-REVIEW-
// 2026-10-26.md, R05 and R09): the size range drew its sizes under a context
// of its own and an Office package checksummed four gigabytes of padding under
// another, so Ctrl+C waited for both. Each had a guard of its own after that,
// and neither guard reaches the next place the shape is written. This one does,
// which is pitfall 7 - a mistake that comes back is a missing rule rather than
// a missing fix.
//
// Keyed by file and by the function the call sits in, never by line, so moving
// code inside a function does not make the list lie (pitfall 11).
var contextRoots = map[string]string{
	"cmd/tfg/main.go main":                            "the process's own context, the one the signal cancels",
	"internal/gui/window/run.go runner.onPreview":     "a preview starts here, and Cancel on the screen is what stops it",
	"internal/gui/window/run.go runner.startRun":      "a run started in the window begins here, and Cancel is what stops it",
	"internal/gui/window/tools.go startTool":          "a tool started in the window begins here, and its Cancel is what stops it",
	"internal/oracle/oracle.go Checker.Check":         "the oracle runs somebody else's reader under a time limit of its own - a measuring instrument for the guards, never on the path of a run",
	"internal/oracle/oracle.go PictureChanges":        "the same instrument, a reference reader under a time limit",
	"internal/oracle/oracle.go Strict":                "the same instrument, a reference reader under a time limit",
	"internal/engine/engine.go Plan":                  "the version without a context, for callers that have none - it is PlanContext, so it cannot disagree with it",
	"internal/format/eml/eml.go generator.Plan":       "the Generator interface's Plan for callers without a context - the engine asks PlanContext",
	"internal/format/htmlfile/html.go generator.Plan": "the Generator interface's Plan for callers without a context - the engine asks PlanContext",
	"internal/format/rtf/rtf.go generator.Plan":       "the Generator interface's Plan for callers without a context - the engine asks PlanContext",
	"internal/format/targz/targz.go generator.Plan":   "the Generator interface's Plan for callers without a context - the engine asks PlanContext",
	"internal/format/zip/zip.go generator.Plan":       "the Generator interface's Plan for callers without a context - the engine asks PlanContext",
	"internal/format/opc/opc.go size":                 "measures a package with the padding left out, so there is nothing long to stop",
	"internal/format/zip/zip.go archiveSize":          "measures an archive with the contents left out, so there is nothing long to stop",
}

func TestAContextIsMadeFromNothingOnlyWhereItIsMeantToStart(t *testing.T) {
	root := repoRoot(t)
	found := map[string]bool{}
	var stray []string
	files := 0

	for _, p := range packages(t) {
		if p.rel != "cmd" && !strings.HasPrefix(p.rel, "cmd/") && !strings.HasPrefix(p.rel, "internal/") {
			continue
		}
		for _, path := range p.files {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				t.Fatal(err)
			}
			rel = filepath.ToSlash(rel)
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("parsing %s: %v", path, err)
			}
			files++
			for _, at := range contextsFromNothing(fset, file) {
				key := rel + " " + at.function
				if _, allowed := contextRoots[key]; allowed {
					found[key] = true
					continue
				}
				stray = append(stray, rel+":"+strconv.Itoa(at.line)+" in "+at.function)
			}
		}
	}

	// The state this depends on: the walk read the tree it is about. A walk
	// that found no files would find no stray context and say so in green.
	if files < 100 {
		t.Fatalf("read %d files under cmd/ and internal/ - the walk is not reaching the source", files)
	}
	sort.Strings(stray)
	for _, s := range stray {
		t.Errorf("%s makes a context from nothing, so whatever runs under it cannot be stopped. Take the caller's context, or add the place to contextRoots with the reason it is where a context begins", s)
	}
	// And the list is closed the other way, or it fills with places that
	// stopped existing and keeps excusing the next one written there.
	var idle []string
	for key := range contextRoots {
		if !found[key] {
			idle = append(idle, key)
		}
	}
	sort.Strings(idle)
	for _, key := range idle {
		t.Errorf("contextRoots names %q and no context is made there any more - take it off the list", key)
	}
}

// contextFromNothing is one call to context.Background or context.TODO.
type contextFromNothing struct {
	function string
	line     int
}

// contextsFromNothing finds every call to Background or TODO of the context
// package, under whatever name the file imports it, with the function around
// it - "Type.Method" for a method.
func contextsFromNothing(fset *token.FileSet, file *ast.File) []contextFromNothing {
	name := ""
	for _, imp := range file.Imports {
		if path, _ := strconv.Unquote(imp.Path.Value); path == "context" {
			name = "context"
			if imp.Name != nil {
				name = imp.Name.Name
			}
		}
	}
	if name == "" || name == "_" {
		return nil
	}
	var out []contextFromNothing
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		where := fn.Name.Name
		if fn.Recv != nil && len(fn.Recv.List) == 1 {
			where = receiverName(fn.Recv.List[0].Type) + "." + where
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || (sel.Sel.Name != "Background" && sel.Sel.Name != "TODO") {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == name {
				out = append(out, contextFromNothing{function: where, line: fset.Position(call.Pos()).Line})
			}
			return true
		})
	}
	return out
}

// receiverName is the type a method hangs on, without the star.
func receiverName(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.StarExpr:
		return receiverName(v.X)
	case *ast.Ident:
		return v.Name
	case *ast.IndexExpr:
		return receiverName(v.X)
	}
	return "?"
}

// The finder itself, on a file written to have one of each shape: the plain
// import, a call inside a method, a call inside a function literal, and a
// selector that only looks like one.
func TestTheContextFinderSeesEveryShapeOfTheCall(t *testing.T) {
	const src = `package a

import ctxpkg "context"

type r struct{}

func (*r) m() { _ = ctxpkg.Background() }

func f() {
	go func() { _ = ctxpkg.TODO() }()
	var other struct{ Background func() int }
	_ = other.Background
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "a.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, at := range contextsFromNothing(fset, file) {
		got = append(got, at.function)
	}
	want := "r.m f"
	if strings.Join(got, " ") != want {
		t.Errorf("found the calls in %q, expected %q", strings.Join(got, " "), want)
	}
}
