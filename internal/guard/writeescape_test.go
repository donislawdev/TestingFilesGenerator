package guard

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// Every file this tool writes is created under a name nobody else holds.
//
// This is one rule that was spelled in four places and complete in none of
// them, and what that cost was measured on 2026-09-06 against the shipped
// binary, in a scratch directory outside the repository:
//
//	a link at <manifest>.tfg-writing    the manifest landed on a file outside
//	                                    the output directory, exit 0, and then
//	                                    verify said "matches" and cleanup said
//	                                    "2 files removed" - both exit 0
//	a link at <recipe>.tfg-writing      "recipe fmt -w" wrote the recipe onto
//	                                    somebody else's file and left the
//	                                    recipe itself as a link, exit 0
//	a link at <manifest>                an empty file appeared outside the
//	                                    output directory, exit 0
//
// The first of those needs no race and no guessing: the name is fixed, and
// nothing anywhere looked at it. On Windows it needs no privilege either,
// because a HARD link is enough and an ordinary user creates one - measured on
// this machine, the victim file came back holding the manifest.
//
// SECURITY.md puts "a way to make the tool write outside the directory it was
// given" first in scope, and says in its own words that a path leaving the
// directory through a symbolic link is refused. It was true of the two reading
// commands and of the files a run produces. It was not true of the names those
// files are written under first.
//
// So the rule became one function, and this asks that it stays one. A fifth
// writer added next year inherits the answer instead of having to know the
// question.
func TestEveryFileThisToolWritesIsCreatedThroughOneClaim(t *testing.T) {
	// Every place allowed to create a file for itself, and why. An entry that
	// stops naming real code is a failure below, not a comment nobody reads.
	allowed := map[string]string{
		"internal/core/createnew.go": "the one claim - this is where the rule lives",
		"internal/legal/cmd/sbom/main.go": "a development command that neither shipped binary links, " +
			"measured with go list -deps: zero",
	}

	// The calls that bring a file into being. os.Open and os.ReadFile are not
	// here on purpose: reading through somebody's link is a different question,
	// and the two reading commands already answer it with core.Boundary.
	creators := map[string]bool{"Create": true, "WriteFile": true, "OpenFile": true}

	seen := map[string]bool{}
	root := repoRoot(t)

	for _, start := range []string{filepath.Join(root, "internal"), filepath.Join(root, "cmd")} {
		err := filepath.WalkDir(start, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
			if d.IsDir() {
				// The guards themselves plant files to test with, and two
				// packages exist only for guards to reach - neither is in a
				// binary anybody downloads.
				switch rel {
				case "internal/guard", "internal/oracle", "internal/site":
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}

			fset := token.NewFileSet()
			file, perr := parser.ParseFile(fset, path, nil, 0)
			if perr != nil {
				return perr
			}
			ast.Inspect(file, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				pkg, ok := sel.X.(*ast.Ident)
				if !ok || pkg.Name != "os" || !creators[sel.Sel.Name] {
					return true
				}
				if _, granted := allowed[rel]; granted {
					seen[rel] = true
					return true
				}
				t.Errorf("%s:%d creates a file with os.%s.\n"+
					"Files are created through core.CreateNew, which refuses a name something else "+
					"already holds - a leftover, a symbolic link, or a hard link somebody planted. "+
					"A create that is not exclusive follows whatever is at the name, and on "+
					"2026-09-06 three of them did exactly that and wrote outside the output "+
					"directory with exit 0. Use core.CreateNew, or add this file to the allowed "+
					"map above with the reason.",
					rel, fset.Position(call.Pos()).Line, sel.Sel.Name)
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", start, err)
		}
	}

	// An exception that outlived its code is an exception nobody granted.
	for rel, why := range allowed {
		if !seen[rel] {
			t.Errorf("%s is listed as allowed to create a file (%s), but it does not create one.\n"+
				"Delete the entry. A standing exception for code that has gone will quietly cover "+
				"the next thing that lands in that file.", rel, why)
		}
	}
}

// core.CreateNew creates only when the name is free, and what it refuses covers
// every way a name can be held.
//
// The three shapes matter for different reasons and no one of them proves the
// others:
//
//	a plain file        O_EXCL alone answers this
//	a hard link         O_EXCL answers it, and NOTHING ELSE CAN - os.Lstat
//	                    reports a hard link as an ordinary file, because that
//	                    is what it is. This is the shape that needs no
//	                    privilege on Windows
//	a link to nothing   O_EXCL says "it exists" and the fallback has to agree.
//	                    os.Stat follows the link and says the name is free,
//	                    which is exactly how the manifest escaped
//
// The last one is why the fallback asks os.Lstat. The fallback exists because
// O_EXCL lies on Windows when the path runs through a reparse point - measured
// 2026-08-03 - so it cannot simply be taken away.
func TestCreateNewCreatesOnlyWhenTheNameIsFree(t *testing.T) {
	t.Run("a free name is created", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "fresh.txt")
		f, err := core.CreateNew(path, 0o644)
		if err != nil {
			t.Fatalf("a free name was refused: %v", err)
		}
		if _, err := f.WriteString("ours"); err != nil {
			t.Fatalf("writing: %v", err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("closing: %v", err)
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != "ours" {
			t.Fatalf("the file did not come back as it was written: %q, %v", got, err)
		}
	})

	held := []struct {
		what  string
		plant func(t *testing.T, dir, name, victim string)
	}{
		{"a plain file", func(t *testing.T, dir, name, victim string) {
			if err := os.WriteFile(name, []byte("SOMEBODY ELSE"), 0o644); err != nil {
				t.Fatalf("planting a file: %v", err)
			}
		}},
		{"a hard link to a file outside the directory", func(t *testing.T, dir, name, victim string) {
			if err := os.Link(victim, name); err != nil {
				t.Fatalf("planting a hard link: %v", err)
			}
		}},
		{"a symbolic link to a file outside the directory", func(t *testing.T, dir, name, victim string) {
			if err := os.Symlink(victim, name); err != nil {
				skipIfLinksAreNotAllowed(t, err)
			}
		}},
		{"a symbolic link to nothing at all", func(t *testing.T, dir, name, victim string) {
			if err := os.Symlink(filepath.Join(dir, "nothing-is-here"), name); err != nil {
				skipIfLinksAreNotAllowed(t, err)
			}
		}},
	}

	for _, c := range held {
		t.Run(c.what, func(t *testing.T) {
			dir := t.TempDir()
			victim := filepath.Join(t.TempDir(), "victim.txt")
			if err := os.WriteFile(victim, []byte("ORIGINAL"), 0o644); err != nil {
				t.Fatalf("writing the victim: %v", err)
			}
			name := filepath.Join(dir, "taken.txt")
			c.plant(t, dir, name, victim)

			f, err := core.CreateNew(name, 0o644)
			if err == nil {
				_ = f.Close()
				t.Fatalf("%s was written through rather than refused", c.what)
			}
			if !errors.Is(err, fs.ErrExist) {
				t.Errorf("the refusal for %s does not read as \"already there\": %v.\n"+
					"Callers tell this apart from a disk failure with errors.Is(err, fs.ErrExist), "+
					"and the engine turns it into its own wording that way.", c.what, err)
			}
			if got, rerr := os.ReadFile(victim); rerr != nil || string(got) != "ORIGINAL" {
				t.Errorf("the file outside the directory changed: %q, %v", got, rerr)
			}
			// What somebody else put there stays there. Untouchable rule 7 is
			// that this tool removes only what a manifest lists, and a refusal
			// is not a licence to tidy.
			if _, lerr := os.Lstat(name); lerr != nil {
				t.Errorf("%s was removed by the refusal: %v", c.what, lerr)
			}
		})
	}
}

// The two writers that put a file beside somebody else's refuse a held
// temporary name rather than writing through it.
//
// Asked through the packages rather than of core.CreateNew again, because what
// broke was not the primitive - it did not exist. What broke is that these two
// call sites did their own create. A guard on the primitive alone would stay
// green if either of them stopped calling it.
func TestAHeldTemporaryNameStopsTheWriteRatherThanGoingThroughIt(t *testing.T) {
	t.Run("the manifest", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "manifest.json")
		victim := filepath.Join(t.TempDir(), "notes.txt")
		if err := os.WriteFile(victim, []byte("ORIGINAL"), 0o644); err != nil {
			t.Fatalf("writing the victim: %v", err)
		}
		if err := os.Link(victim, path+".tfg-writing"); err != nil {
			t.Fatalf("planting a hard link: %v", err)
		}

		m := manifest.New("testing-files-generator", "0.0.0-test", "run_x", "tfg generate", 1, "windows", "amd64")
		m.Add(manifest.File{ID: "files", Path: "files_0001.txt", Name: "files_0001.txt", Bytes: 1024})
		if err := m.Save(path); err == nil {
			t.Fatal("the manifest was saved through a name somebody else held")
		}
		if got, err := os.ReadFile(victim); err != nil || string(got) != "ORIGINAL" {
			t.Errorf("the manifest landed on a file outside the directory: %q, %v", got, err)
		}
	})

	t.Run("the manifest name itself, held by a link to nothing", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "manifest.json")
		target := filepath.Join(t.TempDir(), "created-by-escape.json")
		if err := os.Symlink(target, path); err != nil {
			skipIfLinksAreNotAllowed(t, err)
		}

		if err := manifest.Claim(path); err == nil {
			t.Fatal("the name was claimed through a link pointing at nothing")
		}
		if _, err := os.Stat(target); err == nil {
			t.Error("a file was created outside the directory. os.Stat follows a link, so a link " +
				"pointing at nothing answers \"the name is free\" - the claim has to ask os.Lstat")
		}
	})

	t.Run("recipe fmt -w", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "r.yaml")
		if err := os.WriteFile(path, []byte("version: 1\n"), 0o644); err != nil {
			t.Fatalf("writing the recipe: %v", err)
		}
		victim := filepath.Join(t.TempDir(), "passwd-ish.txt")
		if err := os.WriteFile(victim, []byte("ORIGINAL"), 0o644); err != nil {
			t.Fatalf("writing the victim: %v", err)
		}
		if err := os.Link(victim, path+".tfg-writing"); err != nil {
			t.Fatalf("planting a hard link: %v", err)
		}

		if err := core.ReplaceFile(path, []byte("version: 1\nreplaced: true\n")); err == nil {
			t.Fatal("the recipe was replaced through a name somebody else held")
		}
		if got, err := os.ReadFile(victim); err != nil || string(got) != "ORIGINAL" {
			t.Errorf("the recipe landed on a file outside the directory: %q, %v", got, err)
		}
		// The recipe is still the file it was, rather than a link to the
		// victim. That is what the rename did with the planted link on
		// 2026-09-06, and it is the half that loses the user's own work.
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			t.Errorf("the recipe is no longer an ordinary file of its own: %v, %v", info, err)
		}
		if got, err := os.ReadFile(path); err != nil || string(got) != "version: 1\n" {
			t.Errorf("the recipe changed even though the write was refused: %q, %v", got, err)
		}
	})
}

// skipIfLinksAreNotAllowed says out loud when a case did not run.
//
// Creating a symbolic link needs a privilege on Windows that an ordinary
// account does not have, and a case that quietly passes because it never ran is
// the failure this project has recorded more than any other. The hard link
// cases above need no privilege anywhere, so the shape that matters most is
// never the one being skipped.
func skipIfLinksAreNotAllowed(t *testing.T, err error) {
	t.Helper()
	if errors.Is(err, fs.ErrPermission) || strings.Contains(err.Error(), "privilege") {
		t.Skipf("this host does not allow creating a symbolic link (%v), so this case did not run. "+
			"The hard link cases beside it did.", err)
	}
	t.Fatalf("planting a symbolic link: %v", err)
}
