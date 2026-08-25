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
//
// Widened again the same evening, and by a blind spot rather than by a hunch.
// An outside review of the whole tree listed eleven of these. Ten were the shape
// above and had been fixed that morning, and the eleventh went on passing: the
// end of internal/cli/errors.go was a paragraph about propertyFlag with nothing
// at all under it, while propertyFlag itself sat undocumented in cli.go. A
// comment attached to no declaration is not a comment about a different
// declaration, so the question above never reached it. So a comment standing
// below the last declaration of its file is asked the same thing.
//
// Widened on 2026-08-25 from exported names to every name a package declares,
// after the narrow version let one through: a function inserted between a
// comment and the unexported declaration it belonged to. The wider rule found
// five more of the same shape and no noise at all, and four of the five were
// left behind when cli.go was split into seven files - the paragraphs stayed
// and the functions went.
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
			entries, derr := os.ReadDir(path)
			if derr != nil {
				return nil // a directory being edited underneath us
			}
			fset := token.NewFileSet()
			// One package at a time, because "a different declaration" is a
			// question about a package rather than about a file. Test files are
			// left out: they are where the guards live and they name the things
			// they are about on purpose.
			//
			// File by file rather than parser.ParseDir, which is deprecated for
			// ignoring build tags. Ignoring them is what this wants: a comment
			// adrift behind a tag is still adrift, and the file that holds the
			// window's wiring is behind one.
			byPackage := map[string][]*ast.File{}
			for _, e := range entries {
				name := e.Name()
				if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
					continue
				}
				parsed, ferr := parser.ParseFile(fset, filepath.Join(path, name), nil, parser.ParseComments)
				if ferr != nil {
					continue // not Go we can read, and not this guard's business
				}
				byPackage[parsed.Name.Name] = append(byPackage[parsed.Name.Name], parsed)
			}
			for _, pkgFiles := range byPackage {
				declared := map[string]bool{}
				var decls []decl
				// Comments standing below the last declaration of their file.
				// Nothing follows them, so they are not the doc of anything and
				// the check above never sees them - it asks about comments
				// attached to a declaration.
				var orphans []decl
				for _, file := range pkgFiles {
					if len(file.Decls) > 0 {
						below := file.Decls[len(file.Decls)-1].End()
						for _, group := range file.Comments {
							if group.Pos() <= below {
								continue
							}
							at := fset.Position(group.Pos())
							orphans = append(orphans, decl{
								first: firstWordOf(group.Text()),
								pos:   fmt.Sprintf("%s:%d", shorten(root, at.Filename), at.Line),
							})
						}
					}
					for _, d := range file.Decls {
						names, doc := declaredNames(d)
						for _, n := range names {
							declared[n] = true
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
					if d.first == "" || !declared[d.first] {
						continue
					}
					if contains(d.names, d.first) {
						continue
					}
					problems = append(problems, "   "+d.pos+
						"  declares "+strings.Join(d.names, ", ")+
						"  but its comment opens by naming "+d.first)
				}
				for _, o := range orphans {
					if o.first == "" || !declared[o.first] {
						continue
					}
					problems = append(problems, "   "+o.pos+
						"  stands below the last declaration of its file, so it documents nothing"+
						"  and it opens by naming "+o.first)
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
