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

// Two names that are one file on macOS are refused as well, even where the
// letters are different ones.
//
// The rule above is about case. This is about the other way a filesystem folds:
// APFS reads the sharp s as ss, the long s as s and a ligature as the letters
// inside it. Measured 2026-08-26 on a Mac Mini running macOS 26.6.2, nine pairs
// written twice each with different lengths so one file and two files could be
// told apart by content - tools/probes/apfs-case.py. The same recipe writes two
// files on Windows and one on macOS, which is exactly the split D10 exists to
// close, and the same reason a path separator is refused everywhere rather than
// only where it bites.
//
// The pairs the tool must NOT refuse are here too, and they matter as much: a
// dotted capital I and a dotless one are separate letters on both filesystems,
// and a rule that folded them would invent a refusal. Lowercasing invented that
// one until this changed, because Go maps the dotted capital onto a plain i in
// a single rune.
func TestTwoNamesThatFoldTogetherAreRefused(t *testing.T) {
	for _, c := range []struct {
		what    string
		a, b    string
		oneFile bool
	}{
		{"the sharp s against ss", "straße.txt", "STRASSE.txt", true},
		{"the long s against s", "maſs.txt", "mass.txt", true},
		{"an ff ligature against ff", "oﬀer.txt", "offer.txt", true},
		{"a dotted capital I against a plain i", "İstanbul.txt", "istanbul.txt", false},
		{"a dotless i against a plain i", "ırmak.txt", "irmak.txt", false},
	} {
		t.Run(c.what, func(t *testing.T) {
			dir := t.TempDir()
			out := filepath.Join(dir, "out")
			path := writeRecipe(t, dir, `version: 1
targets:
  - id: one
    format: txt
    size: 1kb
    name: `+c.a+`
  - id: two
    format: txt
    size: 2kb
    name: `+c.b+`
output:
  dir: `+filepath.ToSlash(out)+`
`)

			code, _, errOut := run(t, "generate", path)
			if c.oneFile {
				if code == cli.ExitOK {
					t.Fatalf("%s was accepted, so macOS writes one file where the manifest describes two", c.what)
				}
				// The sentence has to be about the right thing. Before this
				// class existed the refusal fell through to the one about an
				// accent written as a separate character, which would have told
				// somebody their names differ by an accent when they do not.
				if strings.Contains(errOut, "accent") {
					t.Errorf("%s is refused with the sentence about accents:\n%s", c.what, errOut)
				}
				if !strings.Contains(errOut, "mean the same one") {
					t.Errorf("%s is refused without saying what the two names have in common:\n%s", c.what, errOut)
				}
				if entries, err := os.ReadDir(out); err == nil && len(entries) > 0 {
					t.Errorf("a refused plan still produced %d entry(s)", len(entries))
				}
				return
			}
			if code != cli.ExitOK {
				t.Fatalf("%s was refused, and both filesystems keep those apart: exit %d\n%s",
					c.what, code, errOut)
			}
			entries, err := os.ReadDir(out)
			if err != nil {
				t.Fatalf("reading the output: %v", err)
			}
			if len(entries) != 3 {
				t.Errorf("expected two files and a manifest, found %d entries", len(entries))
			}
		})
	}
}
