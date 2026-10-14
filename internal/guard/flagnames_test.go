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

	// Anything above the surfaces is a binary, anything below is what both of
	// them show.
	const surfaces = surfaceLayer

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

// flagLike is a dash pair followed by a letter. "|---|---|" in generated
// markdown is three dashes and does not match, and neither does a range or an
// em dash written as two.
var flagLike = regexp.MustCompile(`--[a-z]`)

// spellsAFlag says whether a text holds a dash pair followed by a letter other
// than the "xn--" every international domain name opens a label with in ASCII
// (RFC 3490) - an html page links to one (docs/HTML-5-2026-10-14.md).
//
// Forgiven by where the two dashes stand, not by taking the prefix out of the
// text first: taken out, "xn---limit" became "-limit" and the flag in it was
// gone (review of #185). Two matches cannot overlap, because a match ends on a
// letter and the next one would have to start on a dash, so the matches found
// are every dash pair there is.
func spellsAFlag(value string) bool {
	for _, at := range flagLike.FindAllStringIndex(value, -1) {
		if !opensAnInternationalLabel(value, at[0]) {
			return true
		}
	}
	return false
}

// opensAnInternationalLabel says whether the dash pair at i is the one of an
// "xn--" starting a label - at the start of the text, or after a character
// that cannot be part of a name. "max-xn--limit" is a name with a flag in it.
func opensAnInternationalLabel(value string, i int) bool {
	if i < 2 || value[i-2:i] != "xn" {
		return false
	}
	if i == 2 {
		return true
	}
	c := value[i-3]
	inName := c == '-' || '0' <= c && c <= '9' || 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
	return !inName
}

func TestNoMessageBelowTheSurfacesIsWrittenInFlagSpelling(t *testing.T) {
	// The canary: what the reading must find and what it must leave alone, so
	// a change to what is forgiven shows here rather than as a quiet pass.
	for value, flag := range map[string]bool{
		"Raise --limit above it":                    true,
		"https://xn--bcher-kva.example/":            false,
		"xn---limit":                                true,
		"max-xn--limit":                             true,
		"https://xn--bcher-kva.example/ or --limit": true,
	} {
		if spellsAFlag(value) != flag {
			t.Fatalf("%q was read as a flag: %v. It is %v, so this guard reads text wrongly", value, !flag, flag)
		}
	}

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
				if spellsAFlag(value) {
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
