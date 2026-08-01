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
		if p != root && (name == ".git" || name == ".github" || name == "docs" || name == "testdata") {
			return filepath.SkipDir
		}

		bp, err := build.ImportDir(p, 0)
		if err != nil {
			// No Go files here. Not an error for us.
			return nil //nolint
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

		out = append(out, pkg{
			rel:     rel,
			dir:     p,
			imports: internalOnly(bp.Imports),
			all:     internalOnly(concat(bp.Imports, bp.TestImports, bp.XTestImports)),
			files:   files,
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
// prefix, so "…/internal/core" becomes "internal/core".
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
