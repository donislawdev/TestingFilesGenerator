package guard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"regexp"
	"sort"
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
// packagesBelowTheSurfaces is every package the layer map puts under the
// command line and the window, as a path from this directory.
//
// Derived rather than written out, and that is the whole of O197. The list
// here used to name seven directories by hand, and internal/damage arrived on
// 2026-09-08 without joining it - a package on layer 2, under BOTH surfaces,
// whose refusals the window and the command line both show. Measured when the
// gap was found: zero flag spellings in it, so the hole was empty. It was still
// a hole, and the next package below the surfaces would have fallen in it too.
//
// The layer map is the right source rather than a walk of internal, because
// TestLayeringHoldsForEveryPackage already refuses a package that is neither on
// the ladder nor declared test only. So this covers a package added tomorrow
// with nothing to remember, and it leaves out the two test only packages on
// purpose: internal/oracle carries the flags of other people's programs in raw
// strings - inkscape and ffprobe are called with real command lines - and
// measured on 2026-09-09 it holds three of them. A walk of internal would
// redden on those, which is how a guard gets switched off inside a week.
func packagesBelowTheSurfaces(t *testing.T) []string {
	t.Helper()

	// The surfaces are layer 4. Anything above them is a binary, anything
	// below is what both of them show.
	const surfaces = 4

	var out []string
	for pkg, n := range layer {
		if n >= surfaces {
			continue
		}
		rest, inside := strings.CutPrefix(pkg, "internal/")
		if !inside {
			// Nothing below the surfaces lives outside internal today.
			// Saying so rather than silently skipping it, because a
			// package that did would go unscanned and look scanned.
			t.Errorf("%s sits below the surfaces and outside internal, so this guard does not know where to read it", pkg)
			continue
		}
		out = append(out, filepath.Join("..", rest))
	}
	if len(out) == 0 {
		t.Fatal("the layer map put no package below the surfaces, so this guard would read nothing")
	}
	sort.Strings(out)
	return out
}

func TestNoMessageBelowTheSurfacesIsWrittenInFlagSpelling(t *testing.T) {
	// A dash pair followed by a letter. "|---|---|" in generated markdown is
	// three dashes and does not match, and neither does a range or an em dash
	// written as two.
	flagLike := regexp.MustCompile(`--[a-z]`)

	dirs := packagesBelowTheSurfaces(t)
	scanned, found := 0, 0
	for _, dir := range dirs {
		matches, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) == 0 {
			t.Errorf("%s is on the ladder and holds no Go file, so the layer map names a package that is not there", dir)
		}
		for _, path := range matches {
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
