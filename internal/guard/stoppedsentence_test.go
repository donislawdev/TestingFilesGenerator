package guard

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
)

// A stopped run says so in our words, not Go's.
//
// The exit codes were already right and are not what this is about: 130 for a
// cancel and 143 for a signal that says time is up, told apart deliberately
// because "every stop used to be reported as Ctrl+C, so a CI job that timed out
// looked like somebody had cancelled it". The SENTENCE never got the same
// treatment. Both endings printed "tfg: context canceled" - six characters of
// Go runtime vocabulary handed to somebody who asked the tool to stop and now
// wants to know what is on their disk.
//
// Written up as O175 on 2026-09-02 from the code rather than from a run,
// because a signal could not be delivered to the process from the shell on this
// machine. This guard needs no signal: cli.Run takes the context, so handing it
// a finished one reaches the same path.
//
// The four part shape of a refusal (D6) does not apply. A stop is not a fault
// to correct, it is the answer to what was asked for.
func TestAStoppedRunSaysSoInOurOwnWords(t *testing.T) {
	body := "version: 1\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\n    count: 3\n"

	for _, c := range []struct {
		name string
		args func(recipe, out string) []string
	}{
		{"generate", func(r, o string) []string { return []string{"generate", r, "--out", o} }},
		{"validate", func(r, _ string) []string { return []string{"validate", r} }},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			args := c.args(writeRecipe(t, dir, body), t.TempDir())

			// The live half. Without it the stopped half below would pass for a
			// command that was refused for some other reason entirely.
			var out, errOut bytes.Buffer
			if code := cli.Run(context.Background(), args, &out, &errOut); code != cli.ExitOK {
				t.Fatalf("with a live context this ended with %d, so the stopped half proves nothing\nstderr: %s",
					code, errOut.String())
			}

			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			out.Reset()
			errOut.Reset()

			if code := cli.Run(ctx, args, &out, &errOut); code != cli.ExitInterrupted {
				t.Fatalf("with a finished context this ended with %d, expected %d\nstderr: %s",
					code, cli.ExitInterrupted, errOut.String())
			}

			said := errOut.String()
			if strings.Contains(said, "context canceled") {
				t.Errorf("a stopped run still prints Go's own words:\n%s\n"+
					"Somebody reading this wants to know what happened to their directory, and "+
					"\"context canceled\" is vocabulary from the language this is written in.", said)
			}
			if !strings.Contains(said, "stopped") {
				t.Errorf("the sentence does not say the run was stopped:\n%s", said)
			}
		})
	}
}

// A deadline is a different ending and says so.
//
// Free, because the error itself distinguishes it - unlike Ctrl+C and SIGTERM,
// which both arrive as context.Canceled and are told apart by the exit code
// instead. Nothing on the command line sets a deadline today, so this is
// reachable only through a caller that does, which the window will be. It is
// here before it is needed rather than after somebody has reported the tool
// crashing.
func TestARunThatRanOutOfTimeSaysThatRatherThanThatItWasCancelled(t *testing.T) {
	dir := t.TempDir()
	args := []string{"validate", writeRecipe(t, dir,
		"version: 1\ntargets:\n  - id: a\n    format: txt\n    size: 1kb\n")}

	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	// Waited for rather than assumed. A deadline that has not passed yet would
	// let the run succeed and this would prove nothing.
	<-ctx.Done()

	var out, errOut bytes.Buffer
	code := cli.Run(ctx, args, &out, &errOut)
	if code != cli.ExitTerminated {
		t.Fatalf("a run whose deadline had passed ended with %d, expected %d\nstderr: %s",
			code, cli.ExitTerminated, errOut.String())
	}

	said := errOut.String()
	if strings.Contains(said, "context deadline exceeded") {
		t.Errorf("a run that ran out of time prints Go's own words:\n%s", said)
	}
	if !strings.Contains(said, "time") {
		t.Errorf("the sentence does not say that time was what ran out, so it reads the same "+
			"as somebody pressing Ctrl+C:\n%s", said)
	}
}
