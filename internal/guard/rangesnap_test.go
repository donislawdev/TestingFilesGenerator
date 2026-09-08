package guard

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// A range asks for SOME size between two ends, not for a number, so a size the
// format cannot write is moved to the nearest one it can rather than refused.
//
// This is O190, and it was worth fixing because it took whole runs down for a
// reason nobody could see coming. Two shapes cause it. A UTF-16 file is a whole
// number of sixteen bit units, so half of every range is unwritable and
// `--size-range 1000-1010` failed about half the time. Four formats have
// unreachable BANDS instead - PNG cannot use the eleven byte counts above a
// picture's encoded size, because the smallest padding chunk costs twelve.
//
// The line between snapping and refusing is the point of these guards, and
// getting it wrong in either direction is a real defect:
//
//   - the low end UNDER the format's floor stays a refusal. Asking PDF for 10 B
//     to 8 kB says a spread was wanted and most of it does not exist, so the
//     answer is the format's own refusal, not forty files piled on the floor.
//     TestARangeIsJudgedByItsLowEndRatherThanByWhatWasDrawn holds that half.
//   - a size unwritable INSIDE what the format can do gets snapped. Nobody can
//     be expected to enumerate parities and bands.
//
// See docs/OBSERVATIONS.md O190.

// pictureFloor is the smallest PNG this recipe can be asked for, worked out
// rather than written down.
//
// It has to be worked out, and the first version of this file learned that the
// expensive way: the number moves with the SEED, and the seed a file gets is
// derived through the target's id, so the floor under a recipe is not the floor
// under the same seed on the command line. A hardcoded 143 passed by hand and
// failed here, which is the guard reaching a state it assumed rather than
// asserted - O118, from the inside.
//
// The band sits immediately above it, so a range has to START here to hold one.
func pictureFloor(t *testing.T, seed int64, id string) int64 {
	t.Helper()
	d, err := format.Get("png")
	if err != nil {
		t.Fatal(err)
	}
	return d.SmallestAccepted(format.Request{
		Seed:       core.FileSeed(core.TargetSeed(seed, id), 0),
		Label:      true,
		Properties: map[string]string{"width": "64", "height": "64"},
	})
}

// snapCases are the two shapes, named rather than derived so a third arriving
// without a case here is a gap somebody has to notice.
func snapCases(t *testing.T) []struct {
	name  string
	body  string
	step  int64 // sizes have to be a multiple of this, 1 when anything goes
	count int
} {
	t.Helper()
	floor := pictureFloor(t, 2, "a")

	return []struct {
		name  string
		body  string
		step  int64
		count int
	}{
		{
			// Parity. Every size in this range is legal for the format and half
			// of them are unwritable in this encoding.
			name: "a wide encoding makes half the range unwritable",
			body: `version: 1
seed: 3
targets:
  - id: a
    format: xml
    count: 6
    size-range: 1001-1200
    properties:
      encoding: utf-16le
      bom: true
`,
			step:  2,
			count: 6,
		},
		{
			// A band. The floor itself is writable and the eleven byte counts
			// above it are not, because the padding chunk that makes up any
			// difference costs twelve. The range starts at the floor so the band
			// is inside it, and reaches well past the band so there is something
			// to snap TO.
			name: "a band above the encoded picture is unwritable",
			body: fmt.Sprintf(`version: 1
seed: 2
targets:
  - id: a
    format: png
    count: 12
    size-range: %d-%d
    properties:
      width: 64
      height: 64
`, floor, floor+57),
			step:  1,
			count: 12,
		},
	}
}

// TestASizeTheFormatCannotWriteIsMovedInsideTheRange is the claim itself.
func TestASizeTheFormatCannotWriteIsMovedInsideTheRange(t *testing.T) {
	for _, c := range snapCases(t) {
		t.Run(c.name, func(t *testing.T) {
			files, notes := generateAndRead(t, c.body)
			if len(files) != c.count {
				t.Fatalf("produced %d files, expected %d", len(files), c.count)
			}

			// Asserted rather than assumed, and this is the half that makes the
			// rest mean anything. A range whose draws all happened to miss the
			// unwritable sizes would pass every check below without one byte
			// ever being snapped - green, and about nothing.
			if !notes["size_moved"] {
				t.Fatalf("no size had to move in this run, so it is not exercising the thing it names")
			}

			lo, hi := rangeEnds(t, c.body)
			for _, f := range files {
				if f.bytes < lo || f.bytes > hi {
					t.Errorf("%s came out at %d B, outside the %d to %d that was asked for",
						f.name, f.bytes, lo, hi)
				}
				if c.step > 1 && f.bytes%c.step != 0 {
					t.Errorf("%s came out at %d B, which this encoding cannot write",
						f.name, f.bytes)
				}
			}

			// The control, and without it a build that answered every range
			// with one size would pass everything above. The whole point of a
			// range is that the sizes differ.
			seen := map[int64]bool{}
			for _, f := range files {
				seen[f.bytes] = true
			}
			if len(seen) < 2 {
				t.Errorf("all %d files came out the same size, so snapping has flattened the range",
					len(files))
			}
		})
	}
}

// TestAMovedSizeIsReported holds rule 6 against the fix itself.
//
// Snapping is the right answer and it is still a thing the tool did that nobody
// asked for by name. Silence is banned, so it is said - and the control beside
// it is the half that makes this mean something: a range where nothing moves
// has to stay quiet, or the note would be noise on every run.
func TestAMovedSizeIsReported(t *testing.T) {
	moved := notesFromRun(t, `version: 1
seed: 3
targets:
  - id: a
    format: xml
    count: 4
    size-range: 1001-1200
    properties:
      encoding: utf-16le
      bom: true
`)
	if !moved["size_moved"] {
		t.Error("sizes were moved to fit the format and nothing said so - silence is banned")
	}

	quiet := notesFromRun(t, `version: 1
seed: 3
targets:
  - id: a
    format: xml
    count: 4
    size-range: 1000-1200
    properties:
      encoding: utf-8
      bom: false
`)
	if quiet["size_moved"] {
		t.Error("nothing had to move in this run and it was reported anyway, which would make the note noise")
	}
}

// TestARangeHoldingNoWritableSizeIsRefusedBeforeAnyFile is the other end.
//
// Snapping cannot invent a size that is not there. Both shapes get a case,
// because a range sitting entirely inside a band and a range holding only odd
// numbers fail for different reasons in the format and have to come out the
// same way here.
func TestARangeHoldingNoWritableSizeIsRefusedBeforeAnyFile(t *testing.T) {
	cases := []struct{ name, body string }{
		{"only odd sizes in a wide encoding", `version: 1
seed: 3
targets:
  - id: a
    format: xml
    count: 2
    size-range: 1001-1001
    properties:
      encoding: utf-16le
      bom: true
`},
		{"entirely inside a band", `version: 1
seed: 2
targets:
  - id: a
    format: png
    count: 4
    size-range: 143-153
    properties:
      width: 64
      height: 64
`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			out := filepath.Join(dir, "out")
			path := writeRecipe(t, dir, c.body)

			code, stdout, errOut := run(t, "generate", path, "--out", out)
			if code != cli.ExitFormat {
				t.Fatalf("exit %d, expected %d - no size in this range can be written:\n%s",
					code, cli.ExitFormat, errOut)
			}
			if stdout != "" {
				t.Errorf("a failed run wrote to stdout:\n%s", stdout)
			}
			if n := len(filesIn(t, out)); n != 0 {
				t.Errorf("%d file(s) were written by a run that was refused", n)
			}
		})
	}
}

// TestSnappingStillLeavesTheEarlierFilesAlone is rule 2 applied to the fix.
//
// Snapping is a pure function of the drawn size, and the drawn size comes from
// the index - so raising the count cannot move a file that was already there.
// A build that snapped by walking forward from wherever the last file landed
// would pass every other guard here and break this one.
func TestSnappingStillLeavesTheEarlierFilesAlone(t *testing.T) {
	const body = `version: 1
seed: 3
targets:
  - id: a
    format: xml
    count: %s
    size-range: 1001-1200
    properties:
      encoding: utf-16le
      bom: true
`
	three := generateInto(t, strings.Replace(body, "%s", "3", 1))
	nine := generateInto(t, strings.Replace(body, "%s", "9", 1))

	if len(three) != 3 || len(nine) != 9 {
		t.Fatalf("produced %d and %d files, expected 3 and 9", len(three), len(nine))
	}
	for i := range three {
		if three[i].bytes != nine[i].bytes || three[i].sha != nine[i].sha {
			t.Errorf("file %d differs between a count of 3 and a count of 9: %d B %s against %d B %s",
				i+1, three[i].bytes, three[i].sha[:12], nine[i].bytes, nine[i].sha[:12])
		}
	}
}

// rangeEnds reads the two ends back out of the recipe, so the numbers this
// checks against are the ones that were asked for rather than a second copy.
func rangeEnds(t *testing.T, body string) (int64, int64) {
	t.Helper()
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "size-range:") {
			continue
		}
		lo, hi, err := core.ParseSizeRange(strings.TrimSpace(strings.TrimPrefix(line, "size-range:")))
		if err != nil {
			t.Fatalf("reading the range out of the recipe: %v", err)
		}
		return lo, hi
	}
	t.Fatal("this recipe carries no size-range, so this check is reading the wrong thing")
	return 0, 0
}

// notesFromRun generates and hands back the note codes the manifest carries.
func notesFromRun(t *testing.T, body string) map[string]bool {
	t.Helper()
	_, notes := generateAndRead(t, body)
	return notes
}

// generateAndRead runs one recipe and reads back both what landed on the disk
// and what the manifest says about it.
//
// The files come off the DISK rather than out of the manifest, because the
// manifest is this tool describing its own work and half of these checks ask
// what a person actually got. The notes have to come from the manifest, since
// that is the only place they are written down per file.
func generateAndRead(t *testing.T, body string) ([]fileFact, map[string]bool) {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	path := writeRecipe(t, dir, body)

	code, _, errOut := run(t, "generate", path, "--out", out)
	if code != cli.ExitOK {
		t.Fatalf("exit %d:\n%s", code, errOut)
	}

	raw, err := os.ReadFile(filepath.Join(out, "manifest.json"))
	if err != nil {
		t.Fatalf("reading the manifest: %v", err)
	}
	var doc struct {
		Files []struct {
			Notes []struct {
				Code string `json:"code"`
			} `json:"notes"`
		} `json:"files"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parsing the manifest: %v", err)
	}
	codes := map[string]bool{}
	for _, f := range doc.Files {
		for _, n := range f.Notes {
			codes[n.Code] = true
		}
	}
	return describeFiles(t, out), codes
}
