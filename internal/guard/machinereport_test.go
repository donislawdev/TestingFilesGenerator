package guard

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// This tool is pointed at CI, and the first thing its own documentation says
// about an ending is that one CI cannot tell apart is a defect. A machine
// report that arrives cut in half under a zero exit code is exactly that: the
// script parses what it got, fails on the syntax, and blames itself.
//
// Measured on the built binary on 2026-08-25, Windows: a cleanup report of
// 300 kB through "| head -2" ended with code 0 and half a document. The same
// command with a 40 kB report ended cleanly, because that much fits in the pipe
// and the reader closing it is never felt - which is why the shell is not where
// this is asked. A writer that refuses is the same failure without the buffer.
//
// Found by an outside review of the whole tree, and it named the tell as well:
// formats.go had been checking this all along, so two ways out of one tool
// behaved differently. docs/CODE-REVIEW-2026-08-23.md section 2.

// refusingWriter takes limit bytes and then stops, the way a pipe does when the
// reader has gone.
type refusingWriter struct {
	limit int
	wrote int
}

func (w *refusingWriter) Write(p []byte) (int, error) {
	if w.wrote >= w.limit {
		return 0, errors.New("the reader went away")
	}
	w.wrote += len(p)
	return len(p), nil
}

func TestAReportThatCouldNotBeWrittenWholeIsNotASuccess(t *testing.T) {
	_, mf := generated(t)

	t.Run("a run that would have been fine ends with an IO failure", func(t *testing.T) {
		var errOut bytes.Buffer
		out := &refusingWriter{}
		code := cli.Run(context.Background(), []string{"verify", mf, "--json"}, out, &errOut)
		if code != cli.ExitIO {
			t.Errorf("exit code %d, expected %d - the report never arrived and the run said nothing was wrong",
				code, cli.ExitIO)
		}
		if !strings.Contains(errOut.String(), "report") {
			t.Errorf("nothing on stderr says the report is the thing that failed:\n%s", errOut.String())
		}
	})

	// Without this the guard above is satisfied by a verify that always ends
	// with an IO failure.
	t.Run("the same report on a writer that takes it ends with zero", func(t *testing.T) {
		var out, errOut bytes.Buffer
		if code := cli.Run(context.Background(), []string{"verify", mf, "--json"}, &out, &errOut); code != cli.ExitOK {
			t.Fatalf("exit code %d on a directory that matches: %s", code, errOut.String())
		}
		if !strings.Contains(out.String(), "\"differences\"") && !strings.Contains(out.String(), "{") {
			t.Errorf("nothing that looks like a report reached the writer:\n%s", out.String())
		}
	})

	// A run that already failed keeps the reason it failed. Replacing it with
	// the IO code would take away the one thing the report was going to say,
	// and a broken pipe is not why the recipe was wrong.
	t.Run("a failure keeps its own code", func(t *testing.T) {
		dir := t.TempDir()
		bad := writeRecipe(t, dir, "version: 1\ntargets:\n  - id: a\n    format: nosuchformat\n    count: 1\n    size: 1kb\n")

		var errOut bytes.Buffer
		code := cli.Run(context.Background(), []string{"validate", bad, "--json"}, &bytes.Buffer{}, &errOut)
		if code == cli.ExitOK {
			t.Fatal("a recipe naming a format nobody registered was called valid")
		}
		want := code

		// The same command, with the report going nowhere. Both reports of this
		// command go to the same writer, so this is the one that fails.
		refusing := &refusingWriter{}
		if got := cli.Run(context.Background(), []string{"validate", bad, "--json"}, &bytes.Buffer{}, refusing); got != want {
			t.Errorf("exit code %d when the report could not be written, expected %d - "+
				"the reason the recipe was refused is the news, not the writer", got, want)
		}
	})
}
