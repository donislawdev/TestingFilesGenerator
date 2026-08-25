package guard

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A doc comment sits above the thing it describes, and Go has no way of saying
// otherwise: a blank line between a comment and a declaration detaches it, and
// no blank line between two paragraphs joins them into one. So a paragraph
// about A written immediately above B becomes B's documentation, silently, and
// A is left with none.
//
// Measured on 2026-08-25, from an outside review of internal/engine that named
// two of them. "go doc engine.Plan" and "go doc engine.Run" printed the
// signature and nothing else - the two functions the whole package exists for -
// while the paragraphs about planning before writing and about the manifest of
// a cut short run were showing as the documentation of a helper and of a struct
// of counters. A scan of the tree found two more of the same shape, in packages
// the review never opened: core.BoundarySizes had given its two paragraphs to
// ParseBoundary, and parts.ToggleSaying opened by defining a different type.
//
// The rule is deliberately narrower than "every exported declaration has a
// comment". That wider rule is what revive offers and it does not fit here: the
// window's text catalogue groups scores of one line functions under a single
// heading on purpose - "Field labels on the generate screen, in the order
// somebody fills them in" - and demanding a sentence on each would trade a
// measured defect for seventy of that kind of noise. What is asked instead is
// the thing that actually went wrong: a comment whose first word is the name of
// a DIFFERENT declaration of the same package is a paragraph that has come
// adrift from it.
func TestNoDocCommentOpensByNamingADifferentDeclaration(t *testing.T) {
	root := repoRoot(t)

	type decl struct {
		names []string // what this declaration declares
		first string   // first word of the comment above it
		pos   string
	}

	var problems []string
	for _, tree := range []string{"internal", "cmd"} {
		err := filepath.Walk(filepath.Join(root, tree), func(path string, info os.FileInfo, err error) error {
			if err != nil || !info.IsDir() {
				return err
			}
			fset := token.NewFileSet()
			// One package at a time, because "a different declaration" is a
			// question about a package rather than about a file. Test files are
			// left out: they are where the guards live and they name the things
			// they are about on purpose.
			pkgs, perr := parser.ParseDir(fset, path, func(fi os.FileInfo) bool {
				return !strings.HasSuffix(fi.Name(), "_test.go")
			}, parser.ParseComments)
			if perr != nil {
				return nil // a directory with no Go in it, or one being edited
			}
			for _, pkg := range pkgs {
				exported := map[string]bool{}
				var decls []decl
				for _, file := range pkg.Files {
					for _, d := range file.Decls {
						names, doc := declaredNames(d)
						for _, n := range names {
							if ast.IsExported(n) {
								exported[n] = true
							}
						}
						if len(names) == 0 || doc == nil {
							continue
						}
						at := fset.Position(d.Pos())
						decls = append(decls, decl{
							names: names,
							first: firstWordOf(doc.Text()),
							pos:   fmt.Sprintf("%s:%d", shorten(root, at.Filename), at.Line),
						})
					}
				}
				for _, d := range decls {
					if d.first == "" || !exported[d.first] {
						continue
					}
					if contains(d.names, d.first) {
						continue
					}
					problems = append(problems, "   "+d.pos+
						"  declares "+strings.Join(d.names, ", ")+
						"  but its comment opens by naming "+d.first)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", tree, err)
		}
	}

	if len(problems) > 0 {
		t.Errorf("%d comment(s) sit above a declaration they are not about:\n%s\n\n"+
			"Reason: the paragraph became the documentation of whatever follows it, and the\n"+
			"declaration it was written for was left with none. \"go doc\" is where this shows,\n"+
			"which is not somewhere anybody looks while writing. Move the paragraph to the\n"+
			"declaration it names, or open it with the name of the one it now sits above.",
			len(problems), strings.Join(problems, "\n"))
	}
}

// declaredNames is what one top level declaration declares, and the comment
// block directly above it.
//
// Methods are left out. Every case measured was a package level declaration,
// and a method whose comment opens with the name of its own type reads
// perfectly well - counting those would be inventing a second rule under cover
// of the first.
func declaredNames(d ast.Decl) ([]string, *ast.CommentGroup) {
	switch v := d.(type) {
	case *ast.FuncDecl:
		if v.Recv != nil {
			return nil, nil
		}
		return []string{v.Name.Name}, v.Doc
	case *ast.GenDecl:
		var names []string
		for _, spec := range v.Specs {
			switch s := spec.(type) {
			case *ast.TypeSpec:
				names = append(names, s.Name.Name)
			case *ast.ValueSpec:
				for _, n := range s.Names {
					names = append(names, n.Name)
				}
			}
		}
		return names, v.Doc
	}
	return nil, nil
}

// firstWordOf is the first word of a comment, with the punctuation a sentence
// ends or breaks on taken off, so that "Plan," and "Plan." count as Plan.
func firstWordOf(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	return strings.Trim(fields[0], ".,:;()\"'")
}

// shorten prints a file relative to the module, because the absolute path
// carries the name of whoever built it and this message can reach a log.
func shorten(root, file string) string {
	if rel, err := filepath.Rel(root, file); err == nil && !strings.HasPrefix(rel, "..") {
		return filepath.ToSlash(rel)
	}
	return file
}
