package guard

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// Planning is work somebody may want to stop, and until 2026-09-02 the command
// line could not be stopped while it did it.
//
// engine.PlanContext exists for exactly this and asks ctx.Err() once per file -
// internal/engine/plantarget.go says why it is asked there rather than per
// target, "one target of ten thousand pictures is the case this is for". Only
// the window called it. All three planning call sites in the command line
// called engine.Plan, which is PlanContext(context.Background(), ...), so the
// context the process had already built for the signal reached the writing and
// stopped at the planning.
//
// Measured that day on this machine, --dry-run so nothing is written: 200 pngs
// 1.66 s, 500 4.18 s, 1000 8.20 s, 2000 17.03 s. About 8.5 ms a file, so ten
// thousand pictures is about a minute and a half of a signal being ignored,
// and under SIGTERM in CI the grace period runs out and SIGKILL arrives.
//
// The two halves are the whole design. A finished context on its own would
// pass for a command that never reached planning at all - a typo in a flag
// ends long before any of this - so each surface is asked twice: once with a
// live context, where it has to succeed, and once with a finished one, where
// it has to end with the interrupt code.
//
// generate is deliberately NOT here, and that omission is the finding this
// guard was written by. It passes this shape already, and for the wrong
// reason: planning runs to the end ignoring the context, and preflight then
// notices the context on its way to writing. Green without reaching the thing
// being guarded. It is asked in the test below instead, where the two answers
// differ.
func TestPlanningFromTheCommandLineCanBeStopped(t *testing.T) {
	cases := []struct {
		name string
		args func(recipePath string) []string
	}{
		{"validate", func(p string) []string { return []string{"validate", p} }},
		{"preset show", func(string) []string { return []string{"preset", "show", "size-boundaries"} }},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			args := c.args(writeRecipe(t, t.TempDir(), validRecipeBody))

			// The live half. Without it the finished half below would pass for
			// a command that ends before it plans anything.
			var out, errOut bytes.Buffer
			if code := cli.Run(context.Background(), args, &out, &errOut); code != cli.ExitOK {
				t.Fatalf("with a live context this ended with %d, so the stopped half below would prove nothing about planning\nstderr: %s",
					code, errOut.String())
			}

			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			out.Reset()
			errOut.Reset()

			code := cli.Run(ctx, args, &out, &errOut)
			if code != cli.ExitInterrupted {
				t.Errorf("with a finished context this ended with %d, expected %d - planning ignored the context it was handed\nstderr: %s",
					code, cli.ExitInterrupted, errOut.String())
			}
			// A run that was stopped is a failed run, and a failed run puts
			// nothing on standard output.
			if out.Len() > 0 {
				t.Errorf("a stopped run wrote %d bytes to standard output: %q", out.Len(), out.String())
			}
		})
	}
}

// Stopping has to happen DURING planning, not be noticed after it.
//
// This is the half that tells the two apart, and it needs no clock to do it.
// The recipe has a first target that plans and a second one that cannot: a zip
// far below the smallest archive its contents already need. So a run that
// walks the whole plan reaches the second target and refuses it by format,
// while a run that honours the context stops inside the first target and ends
// with the interrupt code. Two different codes for the same input, decided by
// nothing but whether planning looked at the context.
//
// Without this, generate passed while planning was still uninterruptible,
// because preflight checks the context on the way to writing - the run did end
// with the right code, having first done every second of the work somebody
// asked it to stop.
func TestPlanningStopsBeforeItWalksToTheFarTarget(t *testing.T) {
	cases := []struct {
		name string
		args func(recipePath string) []string
	}{
		{"generate", func(p string) []string { return []string{"generate", p, "--dry-run"} }},
		{"validate", func(p string) []string { return []string{"validate", p} }},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			args := c.args(writeRecipe(t, t.TempDir(), farRefusalRecipeBody))

			// The live half asserts the state this guard depends on: planning
			// really does walk as far as the second target and really does
			// refuse it. If that ever stops being true - the first target
			// starts failing, say - the stopped half below would agree for a
			// reason that has nothing to do with the context.
			var out, errOut bytes.Buffer
			if code := cli.Run(context.Background(), args, &out, &errOut); code != cli.ExitFormat {
				t.Fatalf("with a live context this ended with %d, expected %d from the far target - the two halves would no longer differ\nstderr: %s",
					code, cli.ExitFormat, errOut.String())
			}

			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			out.Reset()
			errOut.Reset()

			code := cli.Run(ctx, args, &out, &errOut)
			if code != cli.ExitInterrupted {
				t.Errorf("with a finished context this ended with %d, expected %d - planning walked all the way to the far target before anything looked at the context\nstderr: %s",
					code, cli.ExitInterrupted, errOut.String())
			}
		})
	}
}

// A validate that was stopped has no verdict about the recipe, and must not
// print one.
//
// This is a shape the change above created rather than one that was already
// there: before it, validate could not be stopped at all, so the machine
// readable report had no way to reach its "valid: false" branch by being
// interrupted. Left alone it would answer a question nobody asked - a consumer
// reading that one field would learn the recipe is bad, when all that happened
// is that somebody pressed Ctrl+C. Untouchable rule 5 in the other surface: do
// not claim a certainty there is none of.
func TestAStoppedValidateDoesNotCallTheRecipeInvalid(t *testing.T) {
	recipePath := writeRecipe(t, t.TempDir(), validRecipeBody)
	args := []string{"validate", recipePath, "--json"}

	// The live half earns its place twice here: it proves the recipe really is
	// valid, so a "valid" field appearing below could only have come from the
	// stopping.
	var out, errOut bytes.Buffer
	if code := cli.Run(context.Background(), args, &out, &errOut); code != cli.ExitOK {
		t.Fatalf("with a live context validate --json ended with %d, expected %d\nstderr: %s",
			code, cli.ExitOK, errOut.String())
	}
	if !strings.Contains(out.String(), "\"valid\": true") {
		t.Fatalf("the live run did not report the recipe as valid, so this guard is not reading what it thinks it is\nstdout: %s", out.String())
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out.Reset()
	errOut.Reset()

	code := cli.Run(ctx, args, &out, &errOut)
	if code != cli.ExitInterrupted {
		t.Errorf("a stopped validate --json ended with %d, expected %d\nstderr: %s",
			code, cli.ExitInterrupted, errOut.String())
	}
	if strings.Contains(errOut.String(), "\"valid\"") || strings.Contains(out.String(), "\"valid\"") {
		t.Errorf("a stopped validate reported a verdict about the recipe, and it has none to report\nstdout: %s\nstderr: %s",
			out.String(), errOut.String())
	}
}

const validRecipeBody = "version: 1\nseed: 7\noutput:\n  dir: out\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\n    count: 4\n"

// A first target that plans and a second one that cannot. The size is not a
// copied number: it is far below anything a zip could be, so it stays a
// refusal whatever the smallest archive turns out to be.
const farRefusalRecipeBody = "version: 1\nseed: 7\noutput:\n  dir: out\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\n    count: 4\n  - id: b\n    format: zip\n    size: 10\n"
