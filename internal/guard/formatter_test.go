package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// "recipe fmt -w" is the only command in this tool that writes over a file
// somebody wrote by hand, and it was writing files it could not read back.
//
// Found by fuzzing on 2026-08-03, in under a second, by a target that did not
// exist until that day: FuzzParseRecipe stops at the decoder, and the formatter
// parses to a tree and prints it back, which is different code in the same
// dependency. The printer is not faithful for everything the parser accepts.
//
// What it cost, following the tool's own instructions:
//
//	tfg recipe fmt --check r.yaml  ->  3, "not in its settled shape, run -w"
//	tfg recipe fmt -w r.yaml       ->  0, "rewritten"
//	tfg recipe fmt --check r.yaml  ->  3, cannot be read as a recipe
//
// The original content is gone by the third line, and the command that
// destroyed it reported success.
//
// The guard is on the outcome rather than on the input, deliberately. Listing
// the spellings that break the printer would only restate what was found, and
// the next one is a spelling nobody has thought of. What has to hold is that
// whatever comes out can be read back and does not move again.
func TestTheFormatterNeverWritesAFileItCannotReadBack(t *testing.T) {
	for _, source := range []string{
		// An alias where a key belongs. Parses, prints, and the print does not
		// parse.
		"*0000000 : 000",
		// Two byte order marks. Settles to one, then to none, so -w changed the
		// file every time it ran and --check never went green. Written as
		// escapes because Go source may not carry the character itself.
		"\ufeff\ufeffversion: 1\n",
		// A mark that is not at the front until the parser drops what is in
		// front of it.
		" \ufeff",
	} {
		t.Run(strings.ToValidUTF8(source, "?"), func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "r.yaml")
			if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
				t.Fatalf("writing: %v", err)
			}

			code, _, errOut := run(t, "recipe", "fmt", path, "-w")

			// Whatever the verdict, the file is not left in a state this tool
			// cannot read. Either it was settled properly or it was not touched.
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("the file is gone: %v", err)
			}
			if code != cli.ExitOK {
				if string(after) != source {
					t.Errorf("the run failed and still changed the file:\nbefore %q\nafter  %q", source, after)
				}
				if !strings.Contains(errOut, "settled") {
					t.Errorf("the refusal does not say what is wrong:\n%s", errOut)
				}
				return
			}

			// It said it settled the file, so the file has to be settled: a
			// second run must find nothing to do.
			if code, _, errOut := run(t, "recipe", "fmt", path, "--check"); code != cli.ExitOK {
				t.Errorf("the formatter wrote a file its own check refuses: exit %d\n%s\nfile is now %q", code, errOut, after)
			}
		})
	}
}

// The ordinary case, so the check above cannot be satisfied by refusing to
// format anything. A recipe with comments and blank lines has to keep settling,
// and settling has to be something a second run agrees with.
func TestAnOrdinaryRecipeStillSettlesAndStaysSettled(t *testing.T) {
	dir := t.TempDir()
	path := writeRecipe(t, dir, `version: 1

# the comment has to survive, because the diff is the product
targets:
  - id: invoices
    format: pdf
    size:   2mb
`)

	if code, _, errOut := run(t, "recipe", "fmt", path, "-w"); code != cli.ExitOK {
		t.Fatalf("an ordinary recipe was refused: exit %d\n%s", code, errOut)
	}
	settled, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	if !strings.Contains(string(settled), "# the comment has to survive") {
		t.Errorf("the comment did not survive settling:\n%s", settled)
	}
	if code, _, errOut := run(t, "recipe", "fmt", path, "--check"); code != cli.ExitOK {
		t.Errorf("the settled file does not pass its own check: exit %d\n%s", code, errOut)
	}
	// And it still generates, so the hash taken from this shape is usable.
	if code, _, errOut := run(t, "validate", path); code != cli.ExitOK {
		t.Errorf("the settled file no longer validates: exit %d\n%s", code, errOut)
	}
}
