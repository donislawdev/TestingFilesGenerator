package guard

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// validate has to answer about the run that generate would actually make.
//
// It exists to sit in a pre-commit hook - that is what CLI.md gives as the
// reason for it - so both of its ways of being wrong cost something real. A
// miss lets a broken recipe through to the run it was meant to catch. A false
// alarm blocks a commit that was never wrong, and that is the one that gets a
// check switched off.
//
// Measured on 2026-08-04, and it was the false alarm: the two commands each
// built their own engine targets from the recipe, and the one behind validate
// left out the limit a boundary set is built around. That limit decides what
// the three files are called - under_1b, at_limit, over_1b rather than
// 0001, 0002, 0003 - so validate ran its collision check against names nothing
// would ever produce. A recipe holding a boundary set beside a target named
// cap_0001.txt was refused with exit 3 while generate wrote four files with no
// collision at all.
//
// The two conversions are one function now. This guards the behaviour rather
// than the shape, because the same drift could come back through any field
// that reaches naming.

func boundaryRecipe(t *testing.T, dir, out, otherName string) string {
	t.Helper()
	return writeRecipe(t, dir, `version: 1
seed: 1
targets:
  - id: cap
    format: txt
    boundary: 4096
  - id: other
    format: txt
    count: 1
    size: 100
    name: `+otherName+`
output:
  dir: `+filepath.ToSlash(out)+`
`)
}

func TestValidateAgreesWithGenerateAboutBoundaryNames(t *testing.T) {
	// A name that only collides with the numbered form validate used to
	// imagine. Generate never produces it, so refusing this is a false alarm.
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	path := boundaryRecipe(t, dir, out, "cap_0001.txt")

	if code, _, errOut := run(t, "validate", path); code != cli.ExitOK {
		t.Errorf("validate refused a recipe generate accepts, exit %d:\n%s", code, errOut)
	}
	if code, _, errOut := run(t, "generate", path); code != cli.ExitOK {
		t.Fatalf("generate gave %d:\n%s", code, errOut)
	}

	var got []string
	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "manifest.json" {
			got = append(got, e.Name())
		}
	}
	sort.Strings(got)
	want := []string{"cap_0001.txt", "cap_at_limit.txt", "cap_over_1b.txt", "cap_under_1b.txt"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("the run produced %v and the names a boundary set takes are %v", got, want)
	}
}

// The other direction, so the repair is not "accept everything". A name that
// really does collide with what a boundary set produces still has to be
// refused, and refused by validate rather than only at the run.
func TestValidateStillCatchesARealBoundaryCollision(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	path := boundaryRecipe(t, dir, out, "cap_at_limit.txt")

	code, _, errOut := run(t, "validate", path)
	if code == cli.ExitOK {
		t.Fatal("validate accepted a recipe whose boundary set collides with another target")
	}
	if !strings.Contains(errOut, "cap_at_limit.txt") {
		t.Errorf("the refusal does not name the file both targets produce:\n%s", errOut)
	}
	if entries, err := os.ReadDir(out); err == nil && len(entries) > 0 {
		t.Errorf("validate wrote %d entries and it must write none", len(entries))
	}
}
