package guard

import (
	"go/build"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

// modulePath must match go.mod. A mismatch makes every import look external
// and would silently switch all four guards off.
const modulePath = "github.com/donislawdev/TestingFilesGenerator"

// pkg is one package of this module, described by its module relative path.
type pkg struct {
	rel     string   // for example "internal/engine"
	dir     string   // absolute directory
	imports []string // module relative imports, non test files only
	all     []string // module relative imports, tests included
	files   []string // absolute paths of non test .go files
	// tests holds the _test.go files, kept SEPARATE rather than folded
	// into files above. Every guard reading files expects production code
	// only, and widening that field would change what all of them measure
	// in one edit nobody would see. Added 2026-08-27 for the ceiling on
	// test files, which is half the tree and had no ceiling at all.
	tests []string // absolute paths of _test.go files
}

// repoRoot walks up from this test file until it finds go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test directory")
		}
		dir = parent
	}
}

// packages lists every package of this module.
//
// It fails when it finds none. A walk that quietly matches nothing is the
// classic way a guard test turns into decoration.
func packages(t *testing.T) []pkg {
	t.Helper()
	root := repoRoot(t)

	var out []pkg
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		// tools/ holds internal scripts and measurement probes. It is excluded
		// from the repository and from every rule that governs shipped code -
		// any language, no tests required. Walking into it would judge a probe
		// by the rules of the tool it measures.
		if p != root && (name == ".git" || name == ".github" || name == "docs" ||
			name == "testdata" || name == "tools") {
			return filepath.SkipDir
		}

		bp, err := build.ImportDir(p, 0)
		if err != nil {
			// No Go files here. Not an error for us. The //nolint that used to
			// sit on this line was removed on 2026-08-27: nolintlint reported it
			// as unused, so it had been silencing nothing for as long as it was
			// there and read as if it were.
			return nil
		}

		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			rel = ""
		}

		var files []string
		for _, f := range bp.GoFiles {
			files = append(files, filepath.Join(p, f))
		}
		var tests []string
		for _, f := range concat(bp.TestGoFiles, bp.XTestGoFiles) {
			tests = append(tests, filepath.Join(p, f))
		}

		out = append(out, pkg{
			rel:     rel,
			dir:     p,
			imports: internalOnly(bp.Imports),
			all:     internalOnly(concat(bp.Imports, bp.TestImports, bp.XTestImports)),
			files:   files,
			tests:   tests,
		})
		return nil
	})
	if err != nil {
		t.Fatalf("walking the repository: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("no packages found - this guard would pass without checking anything")
	}
	return out
}

// rawImports returns every import of a package including standard library
// ones, tests included.
func rawImports(t *testing.T, p pkg) []string {
	t.Helper()
	bp, err := build.ImportDir(p.dir, 0)
	if err != nil {
		t.Fatalf("reading %s: %v", p.rel, err)
	}
	return concat(bp.Imports, bp.TestImports, bp.XTestImports)
}

// internalOnly keeps imports that belong to this module and strips the module
// prefix, so ".../internal/core" becomes "internal/core".
func internalOnly(imports []string) []string {
	var out []string
	for _, imp := range imports {
		if imp == modulePath {
			out = append(out, "")
			continue
		}
		if strings.HasPrefix(imp, modulePath+"/") {
			out = append(out, path.Clean(strings.TrimPrefix(imp, modulePath+"/")))
		}
	}
	return out
}

func concat(groups ...[]string) []string {
	var out []string
	for _, g := range groups {
		out = append(out, g...)
	}
	return out
}
