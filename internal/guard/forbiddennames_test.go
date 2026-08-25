package guard

import (
	"os"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// forbiddenNames are names the host filesystem will not store on Windows, one
// per character the rule covers, plus a control character and the one reserved
// device name that is still a device.
//
// A tab rather than a null, because a null cannot travel through a command
// line argument and this guard drives the tool the way somebody does.
//
// Each carries the word its own refusal has to reach for. A refusal owes the
// reader something to do instead, and the two families of name have different
// answers - take the character out and ask for it inside an archive, or give
// the device name an extension. One word for both would either be too vague to
// fail or would pass on a sentence that says the wrong thing.
var forbiddenNames = []struct{ name, remedy string }{
	{"a<b.txt", "archive"},
	{"a>b.txt", "archive"},
	{`a"b.txt`, "archive"},
	{"a|b.txt", "archive"},
	{"a?b.txt", "archive"},
	{"a*b.txt", "archive"},
	{"a\tb.txt", "archive"},
	{"nul", "extension"},
	{"NUL", "extension"},
}

// A name the host cannot store is refused while planning, not discovered while
// writing - and the dry run says the same thing the run does.
//
// Measured on 2026-08-25, before the rule existed. Every one of the character
// names planned cleanly: "--dry-run" answered "1 file in 1 target, 200 B total"
// and exit 0, and the run that followed failed that one file with exit 8 and
// the operating system's own sentence in the manifest, carrying our internal
// temporary name into it. So the preview answered for a run that could not
// happen, which is the fault preflight exists to stop, and the same recipe
// writes the file on Linux where all of these are legal.
//
// NUL got there by a different road and is worse. The write to it succeeds and
// the bytes go nowhere, so nothing fails at all - the run was only saved by the
// collision check finding something at the path, and the sentence that came out
// told the reader to remove a file nobody can remove.
//
// The two halves are asserted together on purpose. Asking only "is it refused"
// would stay green if the refusal moved back to write time, which is the half
// that was broken - the run always did refuse the character names, it just
// refused them too late and in somebody else's words.
func TestANameTheHostCannotStoreIsRefusedBeforeAnythingIsWritten(t *testing.T) {
	for _, bad := range forbiddenNames {
		t.Run(bad.name, func(t *testing.T) {
			preview := t.TempDir()
			code, out, errOut := run(t,
				"generate", "--format", "txt", "--size", "200",
				"--name", bad.name, "--dry-run", "--out", preview)
			if code != cli.ExitRecipe {
				t.Errorf("the preview of %q ended with %d and the run refuses it - a preview that answers for a run that cannot happen is the thing --dry-run exists to prevent.\nWhat it said:\n%s%s",
					bad.name, code, out, errOut)
			}

			// The same name again, for real. Same verdict, same code, and an
			// empty directory - not even a manifest, because a run refused
			// before it starts has nothing to record and taking the manifest
			// name would leave an earlier run's files with nothing to remove
			// them by.
			real := t.TempDir()
			code, _, errOut = run(t,
				"generate", "--format", "txt", "--size", "200",
				"--name", bad.name, "--out", real)
			if code != cli.ExitRecipe {
				t.Errorf("the run with %q ended with %d rather than %d, so the fault is being found while writing rather than while planning.\nWhat it said:\n%s",
					bad.name, code, cli.ExitRecipe, errOut)
			}
			left, err := os.ReadDir(real)
			if err != nil {
				t.Fatalf("reading the output directory: %v", err)
			}
			if len(left) != 0 {
				t.Errorf("a refused run left %d entries in the output directory, and a refusal writes nothing at all", len(left))
			}

			// A refusal owes the reader something to do about it. The name is
			// quoted back either way, so asking whether the name appears would
			// pass on anything.
			if !strings.Contains(errOut, bad.remedy) {
				t.Errorf("the refusal about %q does not say what to do instead - no mention of %q:\n%s",
					bad.name, bad.remedy, errOut)
			}
		})
	}
}

// And the other direction, which is the half a rule about names gets wrong by
// being too wide. These hold nothing forbidden and have to go on working.
//
// nul.txt, con.txt and prn are there on purpose. An extension makes the null
// device an ordinary file, and the rest of the reserved names are ordinary
// files with or without one - measured on Windows 11 Pro 26200 and on Windows
// Server 2025 build 26100, both. A rule written from the folklore rather than
// from that measurement would refuse all three, and con.pdf with them.
//
// A template with the placeholder in it matters most, since that is what every
// target uses when nobody names one.
func TestAnOrdinaryNameIsStillAccepted(t *testing.T) {
	for _, name := range []string{
		"ok.txt", "a-b.txt", "a.b.txt", "report_{index:04}.txt",
		"faktura ąćę.txt", "nul.txt", "con.txt", "prn",
	} {
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
