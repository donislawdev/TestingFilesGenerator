package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// Two files heading for one name is caught in the plan, and "one name" was
// being read as "the same string".
//
// Most filesystems people run this on do not agree. NTFS and APFS keep the
// case somebody typed and match without it, exFAT the same, while ext4 matches
// exactly. So "report.txt" and "REPORT.TXT" are two files on one machine and
// one file on the next.
//
// Measured on 2026-08-03, Windows, a recipe asking for both:
//
//	2 file(s) in 2 target(s), 3072 B total     exit 0
//	on disk:      REPORT.TXT, 2048 B           one file
//	the manifest: report.txt and REPORT.TXT    two entries
//	tfg verify:   wrong-size report.txt        exit 7
//
// One file silently written over by the other, a manifest describing something
// that is not there, and the tool's own check failing on its own output a
// second later. That is the defect the collision check exists to stop, reached
// by changing the case of a letter.
//
// Refused on every system rather than only where it bites, for the same reason
// a path separator is: a recipe travels between machines by design, and one
// that quietly loses a file on somebody else's is worse than one that is
// refused on both.
func TestTwoFilesDifferingOnlyInCaseAreRefused(t *testing.T) {
	for _, pair := range [][2]string{
		{"report.txt", "REPORT.TXT"},
		{"Invoice.txt", "invoice.txt"},
		{"a.txt", "A.txt"},
	} {
		t.Run(pair[0]+" vs "+pair[1], func(t *testing.T) {
			dir := t.TempDir()
			out := filepath.Join(dir, "out")
			path := writeRecipe(t, dir, `version: 1
targets:
  - id: one
    format: txt
    size: 1kb
    name: `+pair[0]+`
  - id: two
    format: txt
    size: 2kb
    name: `+pair[1]+`
output:
  dir: `+filepath.ToSlash(out)+`
`)

			code, _, errOut := run(t, "generate", path)
			if code == cli.ExitOK {
				t.Errorf("two names differing only in case were accepted, so one file is written over the other on any filesystem that matches without case")
			}
			if !strings.Contains(errOut, "case") {
				t.Errorf("the refusal does not say what the two names have in common:\n%s", errOut)
			}
			// Nothing was written, which is what makes "the plan refuses it"
			// worth anything.
			if entries, err := os.ReadDir(out); err == nil && len(entries) > 0 {
				t.Errorf("a refused plan still produced %d entry(s)", len(entries))
			}
		})
	}
}

// The other direction. Names that differ by more than case are two files
// everywhere and have to keep working, or the rule above would refuse ordinary
// output.
func TestNamesThatDifferByMoreThanCaseStillWork(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	path := writeRecipe(t, dir, `version: 1
targets:
  - id: one
    format: txt
    size: 1kb
    name: report_a.txt
  - id: two
    format: txt
    size: 2kb
    name: REPORT_B.TXT
output:
  dir: `+filepath.ToSlash(out)+`
`)

	if code, _, errOut := run(t, "generate", path); code != cli.ExitOK {
		t.Fatalf("two distinct names were refused: exit %d\n%s", code, errOut)
	}
	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatalf("reading the output: %v", err)
	}
	// Two files and the manifest.
	if len(entries) != 3 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("expected two files and a manifest, found %v", names)
	}
	if code, _, errOut := run(t, "verify", filepath.Join(out, "manifest.json")); code != cli.ExitOK {
		t.Errorf("the tool does not agree with its own output: exit %d\n%s", code, errOut)
	}
}
