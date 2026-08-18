package guard

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// A size drawn from a range is still derived, never consumed from a stream.
//
// This is the guard that matters most for size-range and the reason the draw
// takes a file index rather than reading a running source. Rule 2 says raising
// a count leaves the earlier files byte for byte identical. A stream would give
// file three a different size the moment file eleven was asked for, every hash
// in somebody's CI would go red at once, and the cause would look like nothing
// at all - the recipe would be unchanged apart from one number that is supposed
// to only add.
func TestRaisingTheCountOfARangeLeavesTheEarlierFilesAlone(t *testing.T) {
	const body = `version: 1
seed: 7741
targets:
  - id: a
    format: txt
    count: %s
    size-range: 1kb-8kb
`
	five := generateInto(t, strings.Replace(body, "%s", "5", 1))
	ten := generateInto(t, strings.Replace(body, "%s", "10", 1))

	if len(five) != 5 || len(ten) != 10 {
		t.Fatalf("produced %d and %d files, expected 5 and 10", len(five), len(ten))
	}
	for i := range five {
		if five[i].bytes != ten[i].bytes || five[i].sha != ten[i].sha {
			t.Errorf("file %d differs between a count of 5 and a count of 10: %d B %s against %d B %s - "+
				"sizes are being drawn from a stream rather than derived per file",
				i+1, five[i].bytes, five[i].sha[:12], ten[i].bytes, ten[i].sha[:12])
		}
	}
}

// The same recipe twice gives the same sizes, and every size lands inside the
// range that was asked for.
//
// The second half is the cheap one and still worth holding: a draw that fell
// outside would produce a file nobody asked for, at a size the recipe forbids,
// and the exact size guard would not notice because the file matches its plan.
func TestARangeIsRepeatableAndStaysInsideItsEnds(t *testing.T) {
	const body = `version: 1
seed: 20260802
targets:
  - id: a
    format: txt
    count: 12
    size-range: 2kb-5kb
`
	first := generateInto(t, body)
	second := generateInto(t, body)

	if len(first) != 12 {
		t.Fatalf("produced %d files, expected 12", len(first))
	}
	for i := range first {
		if first[i].sha != second[i].sha {
			t.Errorf("file %d is not repeatable", i+1)
		}
		if first[i].bytes < 2048 || first[i].bytes > 5120 {
			t.Errorf("file %d is %d B, outside the range 2048 to 5120", i+1, first[i].bytes)
		}
	}

	// A range that produced one size over and over would pass everything above
	// while being useless. Twelve files from a span of three thousand bytes
	// landing on one value is not something a working draw does.
	distinct := map[int64]bool{}
	for _, f := range first {
		distinct[f.bytes] = true
	}
	if len(distinct) < 2 {
		t.Errorf("all twelve files came out the same size, so nothing is being drawn")
	}
}

// The refusal comes from the low end of the range, not from a file that
// happened to be drawn too small.
//
// This is the sharper half and the one worth having. PDF cannot go below
// 3255 B, the range starts at 3000, and the single file this asks for draws a
// size well above that minimum - so a build that judged after the draw, or at
// the wrong end of the range, would sail through and produce a file. The recipe
// is still wrong: raise the count and it starts failing on some runs and not on
// others, which is the one thing a deterministic tool must not do.
func TestARangeIsJudgedByItsLowEndRatherThanByWhatWasDrawn(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		// The obvious one: nearly every draw is below what PDF can deliver.
		{"far below the minimum", "version: 1\ntargets:\n  - id: a\n    format: pdf\n    count: 40\n    size-range: 10-8kb\n"},
		// The sharp one, and the reason this test is worth having. One file,
		// and it draws a size well above the minimum, so a build that judged
		// after the draw would sail through and produce it.
		{"only the low end is out of reach", "version: 1\nseed: 7741\ntargets:\n  - id: a\n    format: pdf\n    count: 1\n    size-range: 3000-8kb\n"},
		// The same shape one level down: at the low end the container cannot
		// hold what it was told to hold.
		{"a container that cannot hold its contents", "version: 1\ntargets:\n  - id: a\n    format: zip\n    count: 20\n    size-range: 1kb-80kb\n    contains:\n      - format: pdf\n        count: 2\n        size: 8kb\n"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			out := filepath.Join(dir, "out")
			path := writeRecipe(t, dir, c.body)

			code, stdout, errOut := run(t, "generate", path, "--out", out)
			if code != cli.ExitFormat {
				t.Fatalf("exit %d, expected %d - the low end of the range is out of reach, "+
					"so this is refused whatever the draw came out as:\n%s", code, cli.ExitFormat, errOut)
			}
			if stdout != "" {
				t.Errorf("a failed run wrote to stdout:\n%s", stdout)
			}
			if n := len(filesIn(t, out)); n != 0 {
				t.Errorf("%d files were written by a run that was refused", n)
			}
		})
	}
}

// Two ways of saying how big the files are is two answers to one question.
func TestTwoSizeDeclarationsInOneTargetAreRefused(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"size and size-range", "version: 1\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\n    size-range: 1kb-8kb\n"},
		{"boundary and size-range", "version: 1\ntargets:\n  - id: a\n    format: txt\n    boundary: 4kib\n    size-range: 1kb-8kb\n"},
		{"a range running backwards", "version: 1\ntargets:\n  - id: a\n    format: txt\n    size-range: 8kb-1kb\n"},
		{"a range with one end", "version: 1\ntargets:\n  - id: a\n    format: txt\n    size-range: 1kb\n"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			out := filepath.Join(dir, "out")
			path := writeRecipe(t, dir, c.body)

			code, stdout, errOut := run(t, "generate", path, "--out", out)
			if code != cli.ExitRecipe {
				t.Fatalf("exit %d, expected %d:\n%s", code, cli.ExitRecipe, errOut)
			}
			if stdout != "" {
				t.Errorf("a failed run wrote to stdout:\n%s", stdout)
			}
			if n := len(filesIn(t, out)); n != 0 {
				t.Errorf("%d files were written by a recipe that was refused", n)
			}
		})
	}
}

// The flag and the recipe key mean the same thing down to the byte.
//
// They are two surfaces onto one engine, and the moment they disagree the
// button in the window that copies a command line stops being trustworthy -
// which is the failure D1 exists to prevent.
func TestTheRangeFlagAndTheRangeKeyAgree(t *testing.T) {
	fromRecipe := generateInto(t, `version: 1
seed: 4242
targets:
  - id: files
    format: txt
    count: 7
    size-range: 1kb-9kb
`)

	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	code, _, errOut := run(t, "generate", "--format", "txt", "--size-range", "1kb-9kb",
		"--count", "7", "--seed", "4242", "--out", out)
	if code != cli.ExitOK {
		t.Fatalf("exit %d:\n%s", code, errOut)
	}
	fromFlags := describeFiles(t, out)

	if len(fromRecipe) != len(fromFlags) {
		t.Fatalf("%d files from the recipe and %d from the flags", len(fromRecipe), len(fromFlags))
	}
	for i := range fromRecipe {
		if fromRecipe[i].sha != fromFlags[i].sha {
			t.Errorf("file %d differs between the recipe and the flags: %d B against %d B",
				i+1, fromRecipe[i].bytes, fromFlags[i].bytes)
		}
	}
}

// A boundary set from the flag is the same three sizes the recipe key gives.
func TestTheBoundaryFlagGivesTheThreeSizesAroundTheLimit(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	code, _, errOut := run(t, "generate", "--format", "txt", "--boundary", "4kib", "--out", out)
	if code != cli.ExitOK {
		t.Fatalf("exit %d:\n%s", code, errOut)
	}

	// By name rather than by position. This used to compare the files in the
	// order the directory listed them, which worked only because files_0001
	// through files_0003 happened to sort into size order. A boundary set now
	// names each file for the part it plays, so directory order and size order
	// are no longer the same thing - and the name is the stronger assertion
	// anyway, because it says which file is which rather than only that the
	// three sizes are present somewhere.
	want := map[string]int64{
		"files_under_1b.txt": 4095,
		"files_at_limit.txt": 4096,
		"files_over_1b.txt":  4097,
	}
	got := describeFiles(t, out)
	if len(got) != len(want) {
		t.Fatalf("%d files, expected 3", len(got))
	}
	for _, f := range got {
		size, known := want[f.name]
		if !known {
			t.Errorf("unexpected file %s in a boundary set", f.name)
			continue
		}
		if f.bytes != size {
			t.Errorf("%s is %d B, expected %d B", f.name, f.bytes, size)
		}
	}
}

// fileFact is what these guards assert on: the size on disk and the hash.
type fileFact struct {
	name  string
	bytes int64
	sha   string
}

// generateInto runs a recipe into a fresh directory and describes what landed.
func generateInto(t *testing.T, body string) []fileFact {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	path := writeRecipe(t, dir, body)

	code, _, errOut := run(t, "generate", path, "--out", out)
	if code != cli.ExitOK {
		t.Fatalf("exit %d:\n%s", code, errOut)
	}
	return describeFiles(t, out)
}

// describeFiles reads the size and hash of everything written, in name order.
//
// Read off the disk rather than out of the manifest, because the manifest is
// this tool describing its own work and the question here is what a person
// actually got.
func describeFiles(t *testing.T, dir string) []fileFact {
	t.Helper()
	var out []fileFact
	for _, name := range filesIn(t, dir) {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		out = append(out, fileFact{name: name, bytes: int64(len(b)), sha: hashOf(b)})
	}
	return out
}

// hashOf is the same sum the manifest carries, rendered the same way.
func hashOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// A container can be given a range, and the archive still holds what it was
// told to hold.
//
// This is the pairing that needed a decision rather than a default. A range
// beside contains means the size is stated and varies, so the contents go in
// and padding closes the difference - the same rule an exact size already
// followed. The refusal that makes it safe is judged at the low end of the
// range, so an archive that cannot hold its contents is refused for every file
// or for none, never for the ones the seed happened to make small.
func TestAContainerTakesARangeAndStillHoldsItsContents(t *testing.T) {
	files := generateInto(t, `version: 1
seed: 20260802
targets:
  - id: parcels
    format: zip
    count: 6
    size-range: 40kb-80kb
    contains:
      - format: pdf
        count: 2
        size: 8kb
      - format: png
        count: 1
        size: 4kb
`)

	if len(files) != 6 {
		t.Fatalf("produced %d archives, expected 6", len(files))
	}
	distinct := map[int64]bool{}
	for i, f := range files {
		if f.bytes < 40*1024 || f.bytes > 80*1024 {
			t.Errorf("archive %d is %d B, outside the range 40960 to 81920", i+1, f.bytes)
		}
		distinct[f.bytes] = true
	}
	if len(distinct) < 2 {
		t.Errorf("all six archives came out the same size, so nothing is being drawn")
	}
}
