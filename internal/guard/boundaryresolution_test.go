package guard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// Verify and cleanup ask, for every entry a manifest lists, whether that entry
// still lands inside the directory once the links on the way have been
// followed. Until 2026-08-20 asking that question resolved the whole directory
// from the root down, and then resolved the file from the root down, once per
// entry - so a run of 3000 entries walked the same ancestors 6000 times.
//
// Measured that day on Windows, 3000 files of 1 kB (observation O117):
//
//	resolving the directory again, per entry   5665 ms
//	resolving each file from the root down     8703 ms
//	opening and hashing all 3000 files          626 ms
//
// The cost of filepath.EvalSymlinks grows with the depth of the path on
// Windows, about 0.24 ms per component, which is why two thirds of a verify
// went into path arithmetic rather than into reading anything.
//
// The two guards below are the two ways the fix could have gone wrong, and one
// of them nearly did. Neither case is covered by the containment guards that
// were already here.

// A link is not refused for being a link - it is refused for where it lands.
//
// This is the case the first version of the fix got wrong. Taking "a link below
// the boundary" as the answer is quick and it is not the rule: a link inside
// the output directory that points at another file inside the same output
// directory has not left anything, and the tool accepted it before. Refusing it
// would turn an ordinary directory into a hard refusal with exit 5, which is a
// change to what this tool accepts rather than to how fast it answers.
// The output directory is itself reached through a link here, and that is not
// decoration. It is the only shape left in which the boundary has to be
// resolved rather than merely made absolute: the quick reading declines as soon
// as it sees the redirection below, so the thorough reading answers, and the
// thorough reading compares against the resolved boundary. Take the resolving
// away and this directory - which is entirely contained - reads as an escape.
// Before the fast path existed, TestADirectoryReachedThroughALinkStillWorks
// held that line. It cannot any more, because the fast path now answers that
// case without ever consulting the resolved boundary.
func TestALinkInsideTheDirectoryPointingBackInsideItIsNotAnEscape(t *testing.T) {
	root := t.TempDir()
	actual := filepath.Join(root, "actual")
	real := filepath.Join(actual, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatalf("making the directories: %v", err)
	}
	out := filepath.Join(root, "out")
	if err := os.Symlink(actual, out); err != nil {
		t.Skipf("this system will not create a link here: %v", err)
	}

	if code, _, errOut := run(t,
		"generate", "--format", "txt", "--size", "1kb", "--count", "1",
		"--name", "f.txt", "--out", out); code != cli.ExitOK {
		t.Fatalf("generating gave %d:\n%s", code, errOut)
	}

	// Move the file one level down and put a link where it used to be. The
	// manifest still names "f.txt", and "f.txt" now redirects - inside.
	// Built on the real path rather than through the link above, because
	// Windows refuses to create a link whose parent is reached through one.
	// What is under test is the path verify is handed, and that is still the
	// link.
	named := filepath.Join(actual, "f.txt")
	target := filepath.Join(real, "f.txt")
	if err := os.Rename(named, target); err != nil {
		t.Fatalf("moving the file: %v", err)
	}
	if err := os.Symlink(target, named); err != nil {
		t.Skipf("this system will not create a link here: %v", err)
	}

	mf := filepath.Join(out, "manifest.json")
	code, stdout, errOut := run(t, "verify", mf)
	if code == cli.ExitIO {
		t.Errorf("verify called a link that points back inside the directory an escape (exit %d):\n%s%s",
			code, stdout, errOut)
	}
	if strings.Contains(stdout+errOut, "lands outside") {
		t.Errorf("verify reported an escape for a path that never leaves the directory:\n%s%s", stdout, errOut)
	}
	// The bytes behind the link are the bytes the manifest recorded, so the
	// entry itself has to come back clean. Anything else means the link was not
	// followed at all.
	if strings.Contains(stdout+errOut, "wrong-hash") || strings.Contains(stdout+errOut, "missing   f.txt") {
		t.Errorf("the entry behind the link was not read through it:\n%s%s", stdout, errOut)
	}
}

// The directory arrives however the person typed it, and "tfg verify
// ./out/manifest.json" is the ordinary way to type it.
//
// This guards the seam the fix introduced. Judging an entry now compares it
// against the directory as a piece of text before asking the filesystem
// anything, and filepath.Rel refuses to compare a relative path against an
// absolute one. Get that wrong and every verdict is still correct - the slow
// reading catches it - so nothing here would have gone red, while everybody who
// types the shorter path keeps paying the cost the fix was written to remove.
// Every other guard in this package builds its directory with t.TempDir, which
// is absolute, so none of them ever asks this.
func TestTheDirectoryMayBeNamedWithARelativePath(t *testing.T) {
	out := t.TempDir()
	if code, _, errOut := run(t,
		"generate", "--format", "txt", "--size", "1kb", "--count", "3",
		"--out", out); code != cli.ExitOK {
		t.Fatalf("generating gave %d:\n%s", code, errOut)
	}

	absolute, _, _ := run(t, "verify", filepath.Join(out, "manifest.json"))

	func() {
		t.Chdir(out)
		relative, stdout, errOut := run(t, "verify", filepath.Join(".", "manifest.json"))
		if relative != absolute {
			t.Errorf("verify answered %d for a relative path and %d for the same directory named absolutely:\n%s%s",
				relative, absolute, stdout, errOut)
		}
		if relative != cli.ExitOK {
			t.Errorf("verify on an untouched run gave %d, expected %d:\n%s%s",
				relative, cli.ExitOK, stdout, errOut)
		}
	}()

	// The same directory, named the same short way, with an entry that leaves
	// through a link. Refusing it is the half that matters, and nothing else in
	// this package ever asks for it: every containment guard here names its
	// directory absolutely, so the relative spelling reaches the containment
	// check untested. It is also the mutation that kills this guard - the quick
	// reading accepting a redirection would report this directory as sound.
	escape, victim := linkedEscape(t)
	escapeManifest := escapingManifest(t, escape, "jn/VICTIM.txt", 21)
	t.Chdir(filepath.Dir(escape))

	code, stdout, errOut := run(t, "verify", filepath.Join(filepath.Base(escape), filepath.Base(escapeManifest)))
	if code != cli.ExitIO {
		t.Errorf("verify gave %d for an entry that leaves the directory, expected %d, when the directory was named relatively:\n%s%s",
			code, cli.ExitIO, stdout, errOut)
	}
	victimSurvives(t, victim)
}

// The saving is in asking once per pass rather than once per entry, so a call
// that drifts back inside the loop puts the cost straight back without changing
// a single answer.
//
// That is the shape a test cannot see. Every verdict stays right, the suite
// stays green, and the run is twenty times slower again on the machine of
// somebody who is not measuring. So this reads the source instead: working the
// boundary out is allowed anywhere a pass begins and nowhere inside a loop.
func TestTheBoundaryIsWorkedOutOncePerPassAndNotOncePerEntry(t *testing.T) {
	dir := filepath.Join(repoRoot(t), "internal", "audit")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading internal/audit: %v", err)
	}

	seen := 0
	var inLoops []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}

		ast.Inspect(file, func(n ast.Node) bool {
			var body *ast.BlockStmt
			switch loop := n.(type) {
			case *ast.ForStmt:
				body = loop.Body
			case *ast.RangeStmt:
				body = loop.Body
			default:
				return true
			}
			ast.Inspect(body, func(inner ast.Node) bool {
				call, ok := inner.(*ast.CallExpr)
				if !ok {
					return true
				}
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					if id, ok := sel.X.(*ast.Ident); ok && id.Name == "core" && sel.Sel.Name == "NewBoundary" {
						inLoops = append(inLoops, fileLine(fset, e.Name(), inner.Pos()))
					}
				}
				return true
			})
			return true
		})

		seen += strings.Count(readFile(t, path), "core.NewBoundary(")
	}

	// A guard that matched nothing would pass for the wrong reason - the whole
	// point is that these calls exist and sit outside the loops.
	if seen == 0 {
		t.Fatal("no call to core.NewBoundary in internal/audit, so this guard checked nothing")
	}
	if len(inLoops) > 0 {
		t.Errorf("the boundary is worked out inside a loop in %d place(s):\n  %s\n\n"+
			"Resolving it per entry is what observation O117 measured at two thirds of a verify on Windows.\n"+
			"Work it out where the pass begins and hand it in.",
			len(inLoops), strings.Join(inLoops, "\n  "))
	}
}

// The guard above says the comparison happens once per pass. This one says it
// happens against something it can actually be compared with.
//
// filepath.Rel refuses a relative path against an absolute one, and the
// directory arrives as the person typed it. Comparing against the name instead
// of its absolute spelling costs nothing in correctness - the thorough reading
// still answers, and answers the same - so the test above stays green and every
// verdict stays right while the saving quietly disappears for everybody who
// types "./out". That is the whole reason this reads the source: there is no
// behaviour to assert on, only a cost, and a cost is invisible to a suite that
// builds every directory with t.TempDir.
func TestTheCheapComparisonUsesTheAbsoluteSpellingOfTheDirectory(t *testing.T) {
	path := filepath.Join(repoRoot(t), "internal", "core", "filename.go")
	src := readFile(t, path)

	if !strings.Contains(src, "filepath.Rel(b.abs,") {
		t.Errorf("Boundary does not compare against the absolute spelling of the directory.\n" +
			"filepath.Rel refuses a relative path against an absolute one, so comparing against the\n" +
			"name as typed sends every \"./out\" run down the expensive reading - with the right answer,\n" +
			"which is why nothing else here goes red. Observation O117 has the measurement.")
	}
}

func fileLine(fset *token.FileSet, name string, pos token.Pos) string {
	return name + ":" + itoa(fset.Position(pos).Line)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(b)
}
