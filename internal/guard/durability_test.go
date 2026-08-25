package guard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// Two files in this tool are written beside their target and then renamed into
// place, and both are the only copy of something.
//
// The manifest is the only record able to remove a run's files. ReplaceFile
// writes over a recipe somebody wrote by hand, in a repository of theirs, and
// "tfg recipe fmt -w" is the one command that does that. A rename can reach the
// disk before the bytes it renames, so without a flush the thing that survives
// a power cut is an empty file under the right name - which is worse than the
// old content and worse than no file, because both commands then report success
// and the loss is found later by somebody else.
//
// Generated files are deliberately not flushed and that is not this guard's
// business: the reason is written on engine.Run and it is about ten thousand
// flushes against one. Owner's call on 2026-08-25, after an outside review
// raised it. docs/CODE-REVIEW-2026-08-23.md section 3.4.
//
// This reads the source, which is the weaker kind of guard and is used here
// because there is nothing else to ask. A test cannot pull the plug, and both
// functions open their own file, so there is no seam to hand a fake one
// through. Adding a seam only a test would use would be inventing a structure
// to make a check possible rather than checking the structure there is. The
// same choice was made, for the same reason, in boundaryresolution_test.go.
func TestWhatIsRenamedIntoPlaceIsOnTheDiskFirst(t *testing.T) {
	for _, c := range []struct {
		file     string
		function string
		// order is what has to appear, in this order, inside the function.
		order []string
	}{
		{
			file:     "internal/manifest/manifest.go",
			function: "Save",
			order:    []string{"m.Encode(f)", "f.Sync()", "f.Close()", "os.Rename(tmp, path)"},
		},
		{
			file:     "internal/core/replace.go",
			function: "writeWhole",
			order:    []string{"f.Write(content)", "f.Sync()", "f.Close()"},
		},
	} {
		t.Run(c.function, func(t *testing.T) {
			steps := successPath(t, c.file, c.function)
			at := -1
			for _, want := range c.order {
				found := -1
				for i, step := range steps {
					if strings.Contains(step, want) {
						found = i
						break
					}
				}
				if found < 0 {
					t.Fatalf("%s in %s does not call %s on the path where nothing goes wrong - "+
						"what it renames into place can then be a name with nothing behind it",
						c.function, c.file, want)
				}
				if found < at {
					t.Errorf("%s in %s calls %s out of order, and the order is the whole point: "+
						"a flush after the close or after the rename protects nothing",
						c.function, c.file, want)
				}
				at = found
			}
		})
	}
}

// successPath is the source of each top level step of one function, in order,
// with the bodies of its if statements left out.
//
// Left out because both of these functions close the file inside every error
// branch, so a plain search of the text finds an f.Close() that runs instead of
// the flush rather than after it. The first version of this guard did exactly
// that and reported both functions as wrong when both were right - the check
// was reading a path nothing takes when things go well.
//
// Found by name through the parser rather than by line numbers, because a cut
// by line number drifts the moment anything above it moves - measured on
// 2026-08-25, when splitting a file that way produced one that did not compile.
func successPath(t *testing.T, file, function string) []string {
	t.Helper()
	fn, fset, source := function0f(t, file, function)

	text := func(from, to token.Pos) string {
		return source[fset.Position(from).Offset:fset.Position(to).Offset]
	}
	var steps []string
	for _, stmt := range fn.Body.List {
		if branch, ok := stmt.(*ast.IfStmt); ok {
			step := ""
			if branch.Init != nil {
				step += text(branch.Init.Pos(), branch.Init.End())
			}
			steps = append(steps, step+text(branch.Cond.Pos(), branch.Cond.End()))
			continue
		}
		steps = append(steps, text(stmt.Pos(), stmt.End()))
	}
	if len(steps) == 0 {
		t.Fatalf("%s in %s has an empty body, so this guard would prove nothing", function, file)
	}
	return steps
}

// function0f finds one function by name and hands back its declaration, the
// positions it was parsed with and the source it came from.
func function0f(t *testing.T, file, function string) (*ast.FuncDecl, *token.FileSet, string) {
	t.Helper()
	path := filepath.Join(repoRoot(t), filepath.FromSlash(file))
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", file, err)
	}
	source := readFile(t, path)
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != function || fn.Body == nil {
			continue
		}
		return fn, fset, source
	}
	t.Fatalf("%s has no function called %s, so this guard would pass without reading anything",
		file, function)
	return nil, nil, ""
}

// functionSource is the whole body of one function, found by name.
func functionSource(t *testing.T, file, function string) string {
	t.Helper()
	fn, fset, source := function0f(t, file, function)
	from := fset.Position(fn.Body.Pos()).Offset
	to := fset.Position(fn.Body.End()).Offset
	return source[from:to]
}

// Saving a manifest tells "nothing is there" apart from "I could not look".
//
// The switch read every failure of os.Stat as an empty slot and went on to
// claim the name, so a path it could not examine was answered in words about a
// manifest that already existed - a sentence about the wrong thing, and the one
// somebody would act on.
//
// Source again, and this time because the branch is nearly unreachable through
// Save itself: MkdirAll runs first, so a directory that cannot be reached fails
// before the Stat, and a file needs only its directory to be traversable to be
// stat-ed. What is left is a name the host rejects outright. Building a case
// for that on every operating system would be a guard about the host rather
// than about this rule, so what is asked is the rule.
//
// Found by an outside review of the whole tree, docs/CODE-REVIEW-2026-08-23.md
// section 3.7c.
func TestSavingAManifestTellsAnEmptySlotFromAnUnreadableOne(t *testing.T) {
	body := functionSource(t, "internal/manifest/manifest.go", "Save")
	if !strings.Contains(body, "errors.Is(err, fs.ErrNotExist)") {
		t.Error("manifest.Save does not tell a missing file from a failure to look at one, " +
			"so a path it cannot examine is answered in words about a manifest that is already there")
	}
}
