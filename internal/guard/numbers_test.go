package guard

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// A number in a recipe means what it looks like.
//
// YAML types bare scalars by its own rules, and those rules are not the ones a
// person reading the file applies. Until 2026-08-02 we let it, and every one of
// these was wrong without saying anything:
//
//	seed: 010     the run used 8
//	count: 010    eight files, not ten
//	width: 0100   a 64 by 64 image
//	seed: 0x10    the run used 16
//	seed: 1_000   read as 1000, by a rule nobody wrote down
//
// A leading zero is what somebody types when they number runs 001, 002, 003,
// and a padded width is what somebody types to keep a column straight. Rule 6
// of this project says silence is forbidden, and this was silence with the
// manifest recording the changed value afterwards, so nothing was left to
// notice it by.
//
// The reading path now takes the source text of the node and does its own base
// ten parse, so these guards are on the behaviour rather than on one spelling.
func TestANumberInARecipeMeansWhatItLooksLike(t *testing.T) {
	cases := []struct {
		name   string
		recipe string
		check  func(t *testing.T, dir string, out string)
	}{
		{
			name: "a padded seed is the number it looks like",
			recipe: `version: 1
seed: 010
targets:
  - id: a
    format: txt
    size: 1kb
`,
			check: func(t *testing.T, dir, out string) {
				if got := seedOf(t, dir); got != 10 {
					t.Errorf("seed %d, expected 10 - YAML read the leading zero as octal", got)
				}
			},
		},
		{
			name: "a padded count is the number of files it looks like",
			recipe: `version: 1
targets:
  - id: a
    format: txt
    count: 010
    size: 1kb
`,
			check: func(t *testing.T, dir, out string) {
				if n := len(filesIn(t, dir)); n != 10 {
					t.Errorf("%d files, expected 10 - a padded count changed how much was produced", n)
				}
			},
		},
		{
			name: "a padded property is the number it looks like",
			recipe: `version: 1
targets:
  - id: a
    format: png
    size: 20kb
    properties:
      width: 0100
      height: 0100
`,
			check: func(t *testing.T, dir, out string) {
				// Read back from the manifest rather than the image, because the
				// point is what the recipe asked for, and the format guards
				// already prove the file matches its plan.
				if got := propertyOf(t, dir, "width"); got != "100" {
					t.Errorf("width %q, expected \"100\" - a padded property changed the image", got)
				}
			},
		},
		{
			name: "a plain seed is untouched, so no existing recipe moved",
			recipe: `version: 1
seed: 42
targets:
  - id: a
    format: txt
    size: 1kb
`,
			check: func(t *testing.T, dir, out string) {
				if got := seedOf(t, dir); got != 42 {
					t.Errorf("seed %d, expected 42", got)
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			out := filepath.Join(dir, "out")
			path := writeRecipe(t, dir, c.recipe)
			code, stdout, errOut := run(t, "generate", path, "--out", out)
			if code != cli.ExitOK {
				t.Fatalf("exit %d, expected 0:\n%s", code, errOut)
			}
			c.check(t, out, stdout)
		})
	}
}

// A spelling that only means a number under a rule the reader has to know is
// refused rather than guessed at.
//
// This is the other half of the fix and the more important one. Reading 0x10 as
// 16 would be the same act the guard above exists to stop - deciding on the
// author's behalf what digits mean - so the answer is a message, not a guess.
func TestASpellingThatOnlyYAMLCallsANumberIsRefused(t *testing.T) {
	cases := []struct {
		name   string
		recipe string
		says   string
	}{
		{"a hexadecimal seed", "version: 1\nseed: 0x10\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\n", "seed"},
		{"a seed with a digit separator", "version: 1\nseed: 1_000\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\n", "seed"},
		{"a version with a decimal point", "version: 1.0\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\n", "version"},
		{"a count that is not whole", "version: 1\ntargets:\n  - id: a\n    format: txt\n    count: 1.5\n    size: 1kb\n", "count"},
		// All digits and still not a number we can hold. Added because mutation
		// testing reported NOT CAUGHT and was right: the three cases above are
		// turned away before the parse is even reached, so nothing here covered
		// what happens when the parse itself fails. A seed past the range has to
		// be refused rather than wrapped into a different run than the one asked
		// for - that is rule 6, and it was the one case that could reach it.
		{"a seed past what the type holds", "version: 1\nseed: 99999999999999999999\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\n", "seed"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			out := filepath.Join(dir, "out")
			path := writeRecipe(t, dir, c.recipe)

			code, stdout, errOut := run(t, "generate", path, "--out", out)
			if code != cli.ExitRecipe {
				t.Fatalf("exit %d, expected %d:\n%s", code, cli.ExitRecipe, errOut)
			}
			if stdout != "" {
				t.Errorf("a failed run wrote to stdout:\n%s", stdout)
			}
			if !strings.Contains(errOut, c.says) {
				t.Errorf("the message does not name %s:\n%s", c.says, errOut)
			}
			// Rule 1 of the recipe contract: a recipe that does not validate
			// produces nothing at all, not a partial set.
			if _, err := os.Stat(out); !os.IsNotExist(err) {
				if n := len(filesIn(t, out)); n != 0 {
					t.Errorf("%d files were written by a recipe that was refused", n)
				}
			}
		})
	}
}

// seedOf reads the seed the run actually used, from the manifest it wrote.
//
// Read back out of the JSON a consumer receives rather than off our own types,
// which is the rule manifestShape already states: decoding into the real type
// would make the guard pass by construction on the day the type changed.
func seedOf(t *testing.T, dir string) int64 {
	t.Helper()
	return readManifest(t, filepath.Join(dir, "manifest.json")).Run.Seed
}

// propertyOf reads one format property back off the first file of the manifest.
func propertyOf(t *testing.T, dir, key string) string {
	t.Helper()
	m := readManifest(t, filepath.Join(dir, "manifest.json"))
	if len(m.Files) == 0 {
		t.Fatal("the manifest lists no files")
	}
	v, ok := m.Files[0].Properties[key]
	if !ok {
		t.Fatalf("the manifest carries no property %q, it has %v", key, m.Files[0].Properties)
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		// JSON has one number type, so an integer property comes back as a
		// float. -1 keeps the shortest form that reads back to the same number.
		return strconv.FormatFloat(x, 'f', -1, 64)
	default:
		t.Fatalf("property %q is a %T", key, v)
		return ""
	}
}

// filesIn lists what was written, ignoring the manifest beside it.
func filesIn(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("reading %s: %v", dir, err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || e.Name() == "manifest.json" {
			continue
		}
		out = append(out, e.Name())
	}
	return out
}
