package guard

import (
	"os"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// forbiddenNames are names the host filesystem will not store on Windows, one
// per character the rule covers, plus a control character.
//
// A tab rather than a null, because a null cannot travel through a command
// line argument and this guard drives the tool the way somebody does.
var forbiddenNames = []string{
	"a<b.txt", "a>b.txt", `a"b.txt`, "a|b.txt", "a?b.txt", "a*b.txt", "a\tb.txt",
}

// A name the host cannot store is refused while planning, not discovered while
// writing - and the dry run says the same thing the run does.
//
// Measured on 2026-08-25, before the rule existed. Every one of these names
// planned cleanly: "--dry-run" answered "1 file in 1 target, 200 B total" and
// exit 0, and the run that followed failed that one file with exit 8 and the
// operating system's own sentence in the manifest, carrying our internal
// temporary name into it. So the preview answered for a run that could not
// happen, which is the fault preflight exists to stop, and the same recipe
// writes the file on Linux where all of these are legal.
//
// The two halves are asserted together on purpose. Asking only "is it refused"
// would stay green if the refusal moved back to write time, which is the half
// that was broken - the run always did refuse the file, it just refused it too
// late and in somebody else's words.
func TestANameTheHostCannotStoreIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	for _, name := range forbiddenNames {
		t.Run(name, func(t *testing.T) {
			preview := t.TempDir()
			code, out, errOut := run(t,
				"generate", "--format", "txt", "--size", "200",
				"--name", name, "--dry-run", "--out", preview)
			if code != cli.ExitRecipe {
				t.Errorf("the preview of %q ended with %d and the run refuses it - a preview that answers for a run that cannot happen is the thing --dry-run exists to prevent.\nWhat it said:\n%s%s",
					name, code, out, errOut)
			}

			// The same name again, for real. Same verdict, same code, and an
			// empty directory - not even a manifest, because a run refused
			// before it starts has nothing to record and taking the manifest
			// name would leave an earlier run's files with nothing to remove
			// them by.
			real := t.TempDir()
			code, _, errOut = run(t,
				"generate", "--format", "txt", "--size", "200",
				"--name", name, "--out", real)
			if code != cli.ExitRecipe {
				t.Errorf("the run with %q ended with %d rather than %d, so the fault is being found while writing rather than while planning.\nWhat it said:\n%s",
					name, code, cli.ExitRecipe, errOut)
			}
			left, err := os.ReadDir(real)
			if err != nil {
				t.Fatalf("reading the output directory: %v", err)
			}
			if len(left) != 0 {
				t.Errorf("a refused run left %d entries in the output directory, and a refusal writes nothing at all", len(left))
			}

			// A refusal owes the reader the character to look for and
			// something to do about it. The name is quoted back either way,
			// so asking whether the name appears would pass on anything.
			if !strings.Contains(errOut, "archive") {
				t.Errorf("the refusal about %q does not say what to do instead:\n%s", name, errOut)
			}
		})
	}
}

// And the other direction, which is the half a rule about characters gets
// wrong by being too wide. These names hold nothing forbidden and have to go
// on working - a template with the placeholder in it most of all, since that
// is what every target uses when nobody names one.
func TestAnOrdinaryNameIsStillAccepted(t *testing.T) {
	for _, name := range []string{"ok.txt", "a-b.txt", "a.b.txt", "report_{index:04}.txt", "faktura ąćę.txt"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if code, _, errOut := run(t,
				"generate", "--format", "txt", "--size", "200",
				"--name", name, "--out", dir); code != cli.ExitOK {
				t.Errorf("an ordinary name was refused: exit %d\n%s", code, errOut)
			}
		})
	}
}
