package guard

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// Every ending has an exit code from the frozen table, because CI can tell
// situations apart by nothing else. A failure mode that quietly falls through
// to the generic code makes "give me a bigger runner" and "fix the directory
// permissions" look identical.
//
// See docs/CLI.md section 3. Changing what a code means is a breaking change.

// A stop by signal and a stop by Ctrl+C are different endings in the table,
// and the difference is the whole point: 130 says a person cancelled, 143 says
// the job ran out of time. CI reads that to know whether to retry.
//
// They used to be the same. signal.NotifyContext does not report which signal
// arrived, so every stop came out as 130 and the 143 in the table was
// unreachable - a documented ending nothing could produce.
func TestAStopBySignalIsToldApartFromCtrlC(t *testing.T) {
	if got := cli.ExitForSignal(syscall.SIGTERM); got != cli.ExitTerminated {
		t.Errorf("SIGTERM gave %d, expected %d - a CI timeout would read as somebody cancelling", got, cli.ExitTerminated)
	}
	if got := cli.ExitForSignal(os.Interrupt); got != cli.ExitInterrupted {
		t.Errorf("Ctrl+C gave %d, expected %d", got, cli.ExitInterrupted)
	}
	// The two must not collapse into one value, whatever those values are.
	if cli.ExitTerminated == cli.ExitInterrupted {
		t.Fatal("the two endings share a code, so nothing can tell them apart")
	}
}

// The constants and docs/CLI.md section 3 describe one frozen table. A code
// present in one and not the other means somebody changed a public contract in
// half the places.
func TestTheExitCodeConstantsMatchTheFrozenTable(t *testing.T) {
	// Straight from docs/CLI.md section 3. Written out here rather than parsed
	// from the document, because the document lives outside the repository and
	// a guard that skips when it is absent guards nothing.
	table := map[string]int{
		"OK": 0, "RUNTIME": 1, "USAGE": 2, "RECIPE": 3, "FORMAT": 4,
		"IO": 5, "SPACE": 6, "VERIFY": 7, "PARTIAL": 8,
		"INTERRUPTED": 130, "TERMINATED": 143,
	}
	code := map[string]int{
		"OK": cli.ExitOK, "RUNTIME": cli.ExitRuntime, "USAGE": cli.ExitUsage,
		"RECIPE": cli.ExitRecipe, "FORMAT": cli.ExitFormat, "IO": cli.ExitIO,
		"SPACE": cli.ExitSpace, "VERIFY": cli.ExitVerify, "PARTIAL": cli.ExitPartial,
		"INTERRUPTED": cli.ExitInterrupted, "TERMINATED": cli.ExitTerminated,
	}
	if len(table) != len(code) {
		t.Fatalf("the table names %d endings and the constants %d", len(table), len(code))
	}
	for name, want := range table {
		if got, ok := code[name]; !ok {
			t.Errorf("%s is in the table and has no constant", name)
		} else if got != want {
			t.Errorf("%s is %d in the table and %d in the code", name, want, got)
		}
	}
	// Two endings sharing a number would make them indistinguishable to CI.
	seen := map[int]string{}
	for name, v := range code {
		if other, dup := seen[v]; dup {
			t.Errorf("%s and %s both end with %d", name, other, v)
		}
		seen[v] = name
	}
}

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
