package guard

import (
	"bytes"
	"context"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// Drawing the sizes of a range is planning, and it has to stop when it is told
// to - docs/STABILITY-REVIEW-2026-10-26.md, R05.
//
// The size range with snapping arrived on 2026-09-08, six days after planning
// was made stoppable, and it planned every file of the range with a context of
// its own. TestPlanningStopsBeforeItWalksToTheFarTarget stayed green, because
// it never asks for a range - the guard did not reach the new path. Measured on
// 2026-10-27 with a real Ctrl+Break: three thousand jpg files, --size-range,
// --dry-run, and the run went on for 13.7 to 15.8 s after the signal, where a
// fixed size ended in 0.01 s.
//
// The same two halves as the far target guard, and for the same reason they
// need no clock. The range lies wholly under the smallest pdf, so drawing its
// sizes is where the run is refused - with a live context by the format, with
// a finished one by the stop. A draw that ignores the context answers the
// format's refusal both times.
func TestARangeStopsWhileItsSizesAreDrawn(t *testing.T) {
	cases := []struct {
		name string
		args func(dir string) []string
	}{
		{"generate", func(dir string) []string {
			return []string{"generate", "--format", "pdf", "--size-range", "1-10", "--out", dir, "--dry-run"}
		}},
		{"validate", func(dir string) []string {
			return []string{"validate", writeRecipe(t, dir, rangeUnderTheFloorRecipeBody)}
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			args := c.args(t.TempDir())

			// The live half asserts the state the stopped half depends on: the
			// refusal really comes from drawing the sizes. If the range ever
			// became writable, the stopped half would end with the interrupt
			// code for a reason that has nothing to do with the draw.
			var out, errOut bytes.Buffer
			if code := cli.Run(context.Background(), args, &out, &errOut); code != cli.ExitFormat {
				t.Fatalf("with a live context this ended with %d, expected %d from the range under the floor - the two halves would no longer differ\nstderr: %s",
					code, cli.ExitFormat, errOut.String())
			}

			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			out.Reset()
			errOut.Reset()

			if code := cli.Run(ctx, args, &out, &errOut); code != cli.ExitInterrupted {
				t.Errorf("with a finished context this ended with %d, expected %d - the sizes of the range were drawn without looking at the context\nstderr: %s",
					code, cli.ExitInterrupted, errOut.String())
			}
		})
	}
}

// rangeUnderTheFloorRecipeBody is a pdf range no pdf can be inside.
const rangeUnderTheFloorRecipeBody = "version: 1\nseed: 7\noutput:\n  dir: out\ntargets:\n  - id: a\n    format: pdf\n    size-range: 1-10\n    count: 3\n"
