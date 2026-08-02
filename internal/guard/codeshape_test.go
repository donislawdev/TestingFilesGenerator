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
)

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
