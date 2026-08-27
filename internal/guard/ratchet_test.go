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

// The other half of every ceiling in this package: the number has to BE the
// measurement, not merely be above it.
//
// "Down is routine, up is a decision" only holds if down actually happens. A
// ceiling parked above the truth grants headroom nobody decided to grant, and
// the next arrival slips in under it in silence - which is the failure the
// ceiling exists to prevent, wearing its own badge. Nothing was watching that
// gap until 2026-08-27, and the measurement that day found 47 lines of it:
// the largest file was 503 against a ceiling of 550.
//
// So shrinking something comes with a two line chore, and it is meant to be
// routine: bring the ceiling down with it.
//
// The crowd counts are held to the same rule for the same reason. Frozen one
// above the truth, the cap would let the next arrival through without a word.
func TestTheCeilingsAreTodaysMeasurementAndNotALooserNumber(t *testing.T) {
	m := measureTree(t)

	if m.files == 0 {
		t.Fatal("the scan read no file at all - this guard would pass against any number ever set")
	}

	type axis struct {
		what    string
		ceiling int
		actual  int
		where   string
	}
	for _, a := range []axis{
		{"longestFile", longestFile, m.biggestFile, m.biggestFileName},
		{"longestFunction", longestFunction, m.biggestFunc, m.biggestFuncName},
		{"deepestNesting", deepestNesting, m.deepest, m.deepestName},
	} {
		if a.actual == a.ceiling {
			continue
		}
		verb := "above"
		if a.actual > a.ceiling {
			verb = "below"
		}
		t.Errorf("%s is %d and the biggest thing it measures is %d (%s), so the ceiling stands %s the truth.\n"+
			"  Move it to %d. Headroom nobody decided to grant is how the next arrival gets in without being noticed.",
			a.what, a.ceiling, a.actual, a.where, verb, a.actual)
	}

	if m.crowdedFiles != crowdedFiles {
		t.Errorf("crowdedFiles is %d and %d file(s) reach %d lines - lower it to %d.",
			crowdedFiles, m.crowdedFiles, crowdingFileLines, m.crowdedFiles)
	}
	if m.crowdedFuncs != crowdedFunctions {
		t.Errorf("crowdedFunctions is %d and %d function(s) reach %d lines - lower it to %d.",
			crowdedFunctions, m.crowdedFuncs, crowdingFunctionLines, m.crowdedFuncs)
	}

	t.Logf("file %d, function %d, depth %d, crowding %d file(s) and %d function(s)",
		m.biggestFile, m.biggestFunc, m.deepest, m.crowdedFiles, m.crowdedFuncs)
}

// crowdBands is where each axis counts as on its way to the ceiling. Passed
// as one value rather than as five parameters, because the argument ceiling
// this same change introduces would otherwise be tripped by the change itself.
type crowdBands struct {
	file, function, cyclo, args, depth int
}

var productionBands = crowdBands{
	file:     crowdingFileLines,
	function: crowdingFunctionLines,
	cyclo:    crowdingComplexity,
	args:     crowdingArguments,
	depth:    crowdingDepth,
}

// shape is what the tree measures on every axis a ceiling stands on.
type shape struct {
	files                             int
	biggestFile, biggestFunc, deepest int
	biggestFileName, biggestFuncName  string
	deepestName                       string
	crowdedFiles, crowdedFuncs        int

	// The two axes added 2026-08-27. Length and branching do not move
	// together: a hundred lines of straight setup reads top to bottom, forty
	// with eight nested branches does not. And a nine argument signature can
	// be four lines of assignment, which passes both ceilings above.
	biggestCyclo, biggestArgs         int
	biggestCycloName, biggestArgsName string
	crowdedCyclo, crowdedArgs         int
	crowdedDepth                      int
}

// measureTree applies the same metric the ceilings use, so the two cannot
// disagree about what they are measuring.
func measureTree(t *testing.T) shape {
	t.Helper()
	var m shape
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
			m.files++
			rel, err := filepath.Rel(repoRoot(t), path)
			if err != nil {
				rel = path
			}
			measureFile(&m, fset, file, src, filepath.ToSlash(rel), productionBands)
		}
	}
	return m
}

func measureFile(m *shape, fset *token.FileSet, file *ast.File, src []string, rel string, bands crowdBands) {
	comments := commentLines(fset, file)

	if n := codeLines(src, comments, 1, len(src)); n > 0 {
		if n > m.biggestFile {
			m.biggestFile, m.biggestFileName = n, rel
		}
		if crowding(n, bands.file) {
			m.crowdedFiles++
		}
	}

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		from := fset.Position(fn.Pos()).Line
		to := fset.Position(fn.End()).Line
		n := codeLines(src, comments, from, to)
		if n > m.biggestFunc {
			m.biggestFunc, m.biggestFuncName = n, rel+" "+name(fn)
		}
		if crowding(n, bands.function) {
			m.crowdedFuncs++
		}
		if d := nesting(fn); d > m.deepest {
			m.deepest, m.deepestName = d, rel+" "+name(fn)
		}
		measureBranching(m, fn, rel, bands)
	}
}

// measureBranching is the two axes no ceiling watched before 2026-08-27.
func measureBranching(m *shape, fn *ast.FuncDecl, rel string, bands crowdBands) {
	if c := complexity(fn); c > 0 {
		if c > m.biggestCyclo {
			m.biggestCyclo, m.biggestCycloName = c, rel+" "+name(fn)
		}
		if crowding(c, bands.cyclo) {
			m.crowdedCyclo++
		}
	}
	if a := params(fn); a >= 0 {
		if a > m.biggestArgs {
			m.biggestArgs, m.biggestArgsName = a, rel+" "+name(fn)
		}
		if crowding(a, bands.args) {
			m.crowdedArgs++
		}
	}
	if crowding(nesting(fn), bands.depth) {
		m.crowdedDepth++
	}
}

// complexity counts decision points the way gocyclo does: one plus every
// branch, loop, case and short circuit operator.
func complexity(fn *ast.FuncDecl) int {
	n := 1
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		switch v := node.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.CommClause:
			n++
		case *ast.BinaryExpr:
			if v.Op.String() == "&&" || v.Op.String() == "||" {
				n++
			}
		}
		return true
	})
	return n
}

// params counts what a caller has to line up, naming each one separately.
func params(fn *ast.FuncDecl) int {
	if fn.Type.Params == nil {
		return 0
	}
	n := 0
	for _, f := range fn.Type.Params.List {
		if len(f.Names) == 0 {
			n++
			continue
		}
		n += len(f.Names)
	}
	return n
}
