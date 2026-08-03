package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// What a run leaves behind when it is killed outright.
//
// The graceful stop is covered: Ctrl+C reaches the generator, the file being
// written is thrown away, the manifest is written, and the run ends with 130.
// A hard kill runs no code at all, so the temporary file stays.
//
// Measured on 2026-08-03 with tools/probes/hard-kill-probe.py, three runs of
// eight files killed with taskkill /F after two had finished:
//
//	files finished          3
//	leftover .tfg-partial   3 of 3
//	manifest written        0 of 3
//
// Two separate consequences come out of that, and only one of them is fixable
// here. The finished files having no manifest is the same cost as power loss
// and the engine says so already. The leftover is different: it is ours, it is
// named after us, and it turned verify into a failure that says nothing.
//
//	tfg verify -> exit 7,  "extra  files_0004.txt.tfg-partial-99999"
//
// A person reading that has no way to know what the file is, whether their
// fixtures are damaged, or what to do. cleanup will not remove it either, and
// that is correct - untouchable rule 7 says the manifest is the whole authority
// over what may be deleted. So it sits there and verify stays red for good.
func TestALeftoverFromAKilledRunIsNamedForWhatItIs(t *testing.T) {
	out, mf := generated(t)

	leftover := filepath.Join(out, "a_0004.txt.tfg-partial-99999")
	if err := os.WriteFile(leftover, []byte("half a file\n"), 0o644); err != nil {
		t.Fatalf("writing the leftover: %v", err)
	}

	code, _, errOut := run(t, "verify", mf)

	// It is still a difference. The directory holds something the manifest does
	// not describe, and a run was interrupted - saying nothing about either
	// would be the silence rule broken to make a number go green.
	if code != cli.ExitVerify {
		t.Errorf("exit %d, expected %d - a leftover is still a difference:\n%s", code, cli.ExitVerify, errOut)
	}
	// What changes is that it says what the file is.
	if strings.Contains(errOut, "extra     "+filepath.Base(leftover)) {
		t.Errorf("the leftover is reported as an ordinary extra file, which tells the reader nothing:\n%s", errOut)
	}
	if !strings.Contains(errOut, "unfinished") {
		t.Errorf("the report does not say the file is unfinished:\n%s", errOut)
	}
	if !strings.Contains(errOut, "stopped") && !strings.Contains(errOut, "interrupted") {
		t.Errorf("the report does not say where it came from:\n%s", errOut)
	}
	if !strings.Contains(errOut, "delete") && !strings.Contains(errOut, "remove") {
		t.Errorf("the report does not say what to do about it:\n%s", errOut)
	}
}

// A file somebody else put there is still an ordinary extra. The rule above
// must not spread to anything with a dot in the name.
func TestAFileNobodyAskedForIsStillReportedAsExtra(t *testing.T) {
	out, mf := generated(t)

	stranger := filepath.Join(out, "notes.txt")
	if err := os.WriteFile(stranger, []byte("somebody's own work\n"), 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}

	code, _, errOut := run(t, "verify", mf)
	if code != cli.ExitVerify {
		t.Fatalf("exit %d, expected %d:\n%s", code, cli.ExitVerify, errOut)
	}
	if !strings.Contains(errOut, "extra") || !strings.Contains(errOut, "notes.txt") {
		t.Errorf("an ordinary extra file is no longer reported as one:\n%s", errOut)
	}
	if strings.Contains(errOut, "unfinished") {
		t.Errorf("a file that is not ours was described as our leftover:\n%s", errOut)
	}
}

// Deliberately not guarded here: that cleanup leaves the leftover alone.
//
// It does, and it is the right answer - untouchable rule 7 makes the manifest
// the whole authority over what may be removed. But cleanup_test.go already
// puts a file nobody's manifest mentions into every one of its cases and checks
// it survives each time, so a guard here would re-prove that with a different
// file name and nothing else.
//
// Mutation is what settled it rather than judgement. The only swap that could
// reach the behaviour makes cleanup walk more manifest entries, and a leftover
// is in no entry at all, so the runner reported NOT CAUGHT: the guard was
// asserting against a capability the code does not have.
