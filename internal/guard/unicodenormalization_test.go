package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// The same letter can be spelled two ways. "e with an acute accent" is one code
// point, U+00E9, and it is also "e" followed by the combining accent U+0301.
// Both are valid UTF-8, both display identically, and they are different byte
// sequences.
//
// APFS treats them as one name. It normalises what it is given, so a file
// created under one spelling answers to the other. NTFS and ext4 do not - there
// the two spellings are two files. So a recipe asking for both produces two
// files on the machine it was written on and one file on a colleague's Mac.
//
// This is the same defect that changing the case of a letter used to reach, and
// the plan-phase check misses it for the same reason: it compares strings after
// folding case, and case folding does not bring the two spellings together.
//
// Measured on macOS 26.5.1, arm64, APFS, 2026-08-04, a recipe asking for one
// accented name spelled both ways:
//
//	2 file(s) in 2 target(s), 3072 B total     exit 0
//	on disk:      one file, 2048 B
//	the manifest: both spellings, 1024 and 2048
//	tfg verify:   wrong-size, exit 7
//
// The first file was written over by the second without a word, the manifest
// describes a file that is not there, and the tool's own check fails on the
// tool's own output a second later. A second pair measured the same day was
// worse still: a PNG target and a TXT target under the two spellings left one
// TXT on disk and a manifest promising a PNG that was never written.
//
// Refused on every system rather than only where it bites, for the reason the
// case rule and the path separator rule are: a recipe travels between machines
// by design, and one that quietly loses a file on somebody else's is worse than
// one that is refused on both.
//
// The two spellings are written as escapes rather than as literal accented
// text. Typed literally they are indistinguishable on screen, and a pair that
// turned out to be the same bytes would make this test pass without ever
// reaching the check it is aiming at.
func TestTwoNamesDifferingOnlyInUnicodeNormalizationAreRefused(t *testing.T) {
	for _, pair := range []struct {
		label    string
		composed string
		decomp   string
	}{
		// e + combining acute.
		{"cafe", "café.txt", "café.txt"},
		// u + combining diaeresis.
		{"uber", "über.txt", "über.txt"},
		// Polish, three marks in one name. The slashed l, U+0142, has no
		// decomposition at all, so this pair also shows the rule does not need
		// every letter in a name to decompose before it fires.
		{"zazolc", "zażółć.txt", "zażółć.txt"},
	} {
		t.Run(pair.label, func(t *testing.T) {
			// Without this the table could hold two identical strings and
			// every assertion below would still pass, proving nothing.
			if pair.composed == pair.decomp {
				t.Fatalf("the two spellings in this case are the same bytes, so the case tests nothing: %q", pair.composed)
			}

			dir := t.TempDir()
			out := filepath.Join(dir, "out")
			path := writeRecipe(t, dir, `version: 1
targets:
  - id: one
    format: txt
    size: 1kb
    name: "`+pair.composed+`"
  - id: two
    format: txt
    size: 2kb
    name: "`+pair.decomp+`"
output:
  dir: `+filepath.ToSlash(out)+`
`)

			code, _, errOut := run(t, "generate", path)
			if code == cli.ExitOK {
				t.Errorf("two spellings of one name were accepted, so on a filesystem that normalises names one file is written over the other and the manifest still describes both")
			}
			// A refusal that does not name both targets leaves the reader
			// looking at two names that print identically.
			for _, id := range []string{"one", "two"} {
				if !strings.Contains(errOut, id) {
					t.Errorf("the refusal does not name target %q, and the two spellings look the same on screen:\n%s", id, errOut)
				}
			}
			// Nothing was written, which is what makes "the plan refuses it"
			// worth anything.
			if entries, err := os.ReadDir(out); err == nil && len(entries) > 0 {
				t.Errorf("a refused plan still produced %d entry(s)", len(entries))
			}
		})
	}
}

// The other direction. A name carrying an accent is ordinary input on its own -
// only a pair that collides is a problem - and names that differ by more than
// the spelling of one letter are two files everywhere. Without this, "refuse
// the pair" could be satisfied by refusing every accented name, which would be
// a worse tool than the one that has the defect.
func TestAccentedNamesThatDoNotCollideStillWork(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	path := writeRecipe(t, dir, `version: 1
targets:
  - id: one
    format: txt
    size: 1kb
    name: "raport-żyła.txt"
  - id: two
    format: txt
    size: 2kb
    name: "raport-wóz.txt"
output:
  dir: `+filepath.ToSlash(out)+`
`)

	if code, _, errOut := run(t, "generate", path); code != cli.ExitOK {
		t.Fatalf("two distinct accented names were refused: exit %d\n%s", code, errOut)
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
