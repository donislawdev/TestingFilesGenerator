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

// The owner of this project does not read the code. That makes "keep it tidy"
// a rule with one reader and one judge, and they are the same person - which is
// the same shape as a guard claimed to be proven by mutation with no mutation
// behind it. So the part of tidiness a machine can hold is held here, and the
// rest is not written down as a rule at all.
//
// These two numbers are a ratchet. They came from measuring the tree, not from
// a style guide, and they move in one direction: down, when work makes them
// lowerable. Raising one to turn a run green is the same act as editing the
// golden values to turn a run green.
//
// What this cannot see: whether the design is right, whether a primitive was
// well chosen, whether a format is faithful. Two defects found by the owner on
// 2026-08-02 had exact sizes and passed every guard in this package. A short
// function can be wrong. This only stops the tree from drifting into something
// nobody can follow.
const (
	// Counted in lines that carry code - blank lines and comments do not
	// count. This codebase explains itself at length on purpose, and a limit
	// that counted comments would be a limit on explaining. Measured before
	// choosing: comments and blanks run 17 to 45 lines in the longest
	// functions, so counting them would have punished the wrong thing.
	longestFunction = 80
	longestFile     = 550

	// Depth answers a different question than length, and it is the better
	// question of the two. A hundred line function that is flat reads top to
	// bottom. A thirty line function nested five deep has to be held in the
	// head all at once, and that is where a case gets missed.
	//
	// Set from measurement like the two above, and a ratchet like them.
	deepestNesting = 4
)

// nesting reports how many blocks deep the deepest statement of a function sits.
//
// A function body is depth zero. Each if, for, switch, select, range and
// function literal inside it adds one. else-if is deliberately not counted as
// deeper than its if - it reads as one chain, and counting it would push toward
// a switch that says less.
func nesting(fn *ast.FuncDecl) int {
	deepest := 0
	var walk func(n ast.Node, depth int)
	walk = func(n ast.Node, depth int) {
		if depth > deepest {
			deepest = depth
		}
		switch node := n.(type) {
		case *ast.IfStmt:
			walk(node.Body, depth+1)
			// An else-if chain stays at the same depth as the if it follows.
			if el, ok := node.Else.(*ast.IfStmt); ok {
				walk(el, depth)
			} else if node.Else != nil {
				walk(node.Else, depth+1)
			}
			return
		case *ast.ForStmt:
			walk(node.Body, depth+1)
			return
		case *ast.RangeStmt:
			walk(node.Body, depth+1)
			return
		case *ast.SwitchStmt:
			walk(node.Body, depth+1)
			return
		case *ast.TypeSwitchStmt:
			walk(node.Body, depth+1)
			return
		case *ast.SelectStmt:
			walk(node.Body, depth+1)
			return
		case *ast.FuncLit:
			walk(node.Body, depth+1)
			return
		}
		ast.Inspect(n, func(c ast.Node) bool {
			if c == nil || c == n {
				return true
			}
			switch c.(type) {
			case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt,
				*ast.TypeSwitchStmt, *ast.SelectStmt, *ast.FuncLit:
				walk(c, depth)
				return false
			}
			return true
		})
	}
	walk(fn.Body, 0)
	return deepest
}

// codeLines reports, for the given 1-based inclusive line range, how many lines
// carry code. A line is code when it is not blank and is not inside a comment.
//
// Comments are located through the parsed file rather than by looking for a
// leading slash, because this tree embeds a Python script in a raw string and
// scanning for "//" would read parts of it as comments.
func codeLines(src []string, comments map[int]bool, from, to int) int {
	n := 0
	for line := from; line <= to && line <= len(src); line++ {
		if comments[line] {
			continue
		}
		if strings.TrimSpace(src[line-1]) == "" {
			continue
		}
		n++
	}
	return n
}

// commentLines marks every line covered by a comment group.
func commentLines(fset *token.FileSet, f *ast.File) map[int]bool {
	out := map[int]bool{}
	for _, g := range f.Comments {
		from := fset.Position(g.Pos()).Line
		to := fset.Position(g.End()).Line
		for line := from; line <= to; line++ {
			out[line] = true
		}
	}
	return out
}

func TestNoFunctionOrFileHasGrownPastWhatAPersonCanFollow(t *testing.T) {
	var tooLong []string

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

			if n := codeLines(src, comments, 1, len(src)); n > longestFile {
				tooLong = append(tooLong, fmt.Sprintf(
					"%s holds %d lines of code and the ceiling is %d - split it by what the parts do",
					rel, n, longestFile))
			}

			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				from := fset.Position(fn.Pos()).Line
				to := fset.Position(fn.End()).Line
				if d := nesting(fn); d > deepestNesting {
					tooLong = append(tooLong, fmt.Sprintf(
						"%s:%d %s nests %d deep and the ceiling is %d",
						rel, from, name(fn), d, deepestNesting))
				}

				n := codeLines(src, comments, from, to)
				if n <= longestFunction {
					continue
				}
				tooLong = append(tooLong, fmt.Sprintf(
					"%s:%d %s is %d lines of code and the ceiling is %d",
					rel, from, name(fn), n, longestFunction))
			}
		}
	}

	// Every one at once rather than the first. Being sent back seven times for
	// seven answers is the failure this project already named for recipes
	// (RC7), and it is no better here.
	if len(tooLong) > 0 {
		sort.Strings(tooLong)
		t.Errorf("%d things have grown past the ceiling:\n  %s",
			len(tooLong), strings.Join(tooLong, "\n  "))
	}
}

// name renders a function or method name the way a person would say it.
func name(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	var recv strings.Builder
	switch e := fn.Recv.List[0].Type.(type) {
	case *ast.StarExpr:
		if id, ok := e.X.(*ast.Ident); ok {
			recv.WriteString(id.Name)
		}
	case *ast.Ident:
		recv.WriteString(e.Name)
	}
	if recv.Len() == 0 {
		return fn.Name.Name
	}
	return recv.String() + "." + fn.Name.Name
}

// The ceiling counts code and not explanation, and this is what proves it.
//
// The exclusion is not a detail. This project asks for comments that say WHY,
// and a size limit counting them would be a limit on explaining - so the two
// rules would pull against each other, and the one with a guard would win.
// That is the shape docs/QUALITY.md names when it says the ceiling is measured
// without comments.
//
// This does not discover the rule - it makes it permanent. The exclusion was
// already proven on 2026-08-02 by tools/probes/probe-shape.py, which padded a
// function with ninety lines of comment and watched the gate stay green, and
// that is written down in the provenByProbe entry beside this package and in
// docs/QUALITY.md section 8.1.
//
// What changes here is when the question gets asked. A probe is run by hand,
// once, by somebody who remembered. This runs on every suite. If the exclusion
// broke, it would not surface as a wrong number - it would surface as a
// function suddenly over the ceiling for having been explained, which reads
// like a real finding and would be repaired in the wrong direction.
//
// Written the way the rule asks for: add N lines of comment and N lines of
// code to the same sample, and require that only the second moves the number.
func TestTheSizeCeilingCountsCodeAndNotExplanation(t *testing.T) {
	measure := func(t *testing.T, source string) int {
		t.Helper()
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, "measured.go", source, parser.ParseComments)
		if err != nil {
			t.Fatalf("parsing the sample: %v", err)
		}
		return codeLines(strings.Split(source, "\n"), commentLines(fset, file), 1, len(strings.Split(source, "\n")))
	}

	const bare = `package sample

func f() {
	a := 1
	b := 2
	_ = a + b
}
`
	// Six lines of comment and a blank line, none of which is code.
	const explained = `package sample

// One.
// Two.
// Three.
/* Four.
   Five.
   Six. */

func f() {
	a := 1
	b := 2
	_ = a + b
}
`
	// The same as bare, plus three lines that really are code.
	const bigger = `package sample

func f() {
	a := 1
	b := 2
	c := 3
	d := 4
	e := 5
	_ = a + b + c + d + e
}
`

	base := measure(t, bare)
	if base == 0 {
		t.Fatal("the sample measured zero lines of code, so this guard would pass on anything")
	}

	if n := measure(t, explained); n != base {
		t.Errorf("adding six lines of comment moved the measure from %d to %d.\n"+
			"The ceiling would then be a limit on explaining, which is the other rule this project runs on.",
			base, n)
	}
	if n := measure(t, bigger); n != base+3 {
		t.Errorf("adding three lines of code moved the measure from %d to %d, and %d was expected.\n"+
			"A measure that does not move for code is not measuring size at all.",
			base, n, base+3)
	}
	t.Logf("%d lines of code, unchanged by six lines of comment, plus three for three lines of code", base)
}
