package guard

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// Every ending has an exit code from the frozen table, because CI can tell
// situations apart by nothing else. A failure mode that quietly falls through
// to the generic code makes "give me a bigger runner" and "fix the directory
// permissions" look identical.
//
// See docs/CLI.md section 3. Changing what a code means is a breaking change.

func TestEveryEndingUsesACodeFromTheTable(t *testing.T) {
	dir := t.TempDir()

	occupied := filepath.Join(dir, "occupied")
	if err := os.MkdirAll(occupied, 0o755); err != nil {
		t.Fatalf("preparing: %v", err)
	}
	if err := os.WriteFile(filepath.Join(occupied, "files_0001.txt"), []byte("theirs"), 0o644); err != nil {
		t.Fatalf("preparing: %v", err)
	}

	cases := []struct {
		name string
		args []string
		want int
	}{
		{"a normal run", []string{"generate", "--format", "txt", "--size", "2kb", "--out", filepath.Join(dir, "ok")}, cli.ExitOK},
		{"no arguments", nil, cli.ExitUsage},
		{"an unknown command", []string{"nonsense"}, cli.ExitUsage},
		{"no format", []string{"generate", "--size", "1kb"}, cli.ExitUsage},
		{"no size", []string{"generate", "--format", "txt"}, cli.ExitUsage},
		{"a size that is not a number", []string{"generate", "--format", "txt", "--size", "banana"}, cli.ExitUsage},
		{"a size that is not a whole byte", []string{"generate", "--format", "txt", "--size", "1.5"}, cli.ExitUsage},
		{"an unknown expectation", []string{"generate", "--format", "txt", "--size", "1kb", "--expected", "maybe", "--out", filepath.Join(dir, "e1")}, cli.ExitUsage},
		{"a count of zero", []string{"generate", "--format", "txt", "--size", "1kb", "--count", "0", "--out", filepath.Join(dir, "e2")}, cli.ExitRecipe},
		{"two files heading for one name", []string{"generate", "--format", "txt", "--size", "100", "--count", "3", "--name", "same.txt", "--out", filepath.Join(dir, "e3")}, cli.ExitRecipe},
		{"an unknown format", []string{"generate", "--format", "nope", "--size", "1kb", "--out", filepath.Join(dir, "e4")}, cli.ExitFormat},
		{"writing over something", []string{"generate", "--format", "txt", "--size", "500", "--out", occupied}, cli.ExitIO},
		{"listing the formats", []string{"formats"}, cli.ExitOK},
		{"the version", []string{"version"}, cli.ExitOK},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			got := cli.Run(context.Background(), c.args, &out, &errOut)
			if got != c.want {
				t.Errorf("ended with %d, expected %d\nstderr: %s", got, c.want, errOut.String())
			}

			// A failed run puts nothing on standard output. A consumer of a
			// pipe must never receive half an answer and have to guess
			// whether that was all of it.
			if got != cli.ExitOK && out.Len() > 0 {
				t.Errorf("a run ending with %d wrote %d bytes to standard output: %q",
					got, out.Len(), out.String())
			}
		})
	}
}

func TestTheFreeSpaceGuardEndsWithItsOwnCode(t *testing.T) {
	dir := t.TempDir()
	var out, errOut bytes.Buffer

	// A run far larger than any disk. The guard refuses before writing, so
	// this costs nothing even though the number is enormous.
	got := cli.Run(context.Background(), []string{
		"generate", "--format", "txt", "--size", "900gib", "--count", "40",
		"--out", filepath.Join(dir, "huge"),
	}, &out, &errOut)

	if got != cli.ExitSpace {
		t.Errorf("ended with %d, expected %d - a full disk is the most common failure of this tool and CI has to tell it apart from a permissions problem\nstderr: %s",
			got, cli.ExitSpace, errOut.String())
	}
	if out.Len() > 0 {
		t.Errorf("a failed run wrote to standard output: %q", out.String())
	}
	if !strings.Contains(errOut.String(), "free") {
		t.Errorf("the message does not say what was wrong: %s", errOut.String())
	}
}
