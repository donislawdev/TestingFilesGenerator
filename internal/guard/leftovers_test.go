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

// The other marker, and it went unrecognised for as long as it existed.
//
// A file being produced is called "<final>.tfg-partial-<pid>". A file being
// REPLACED - the manifest written over an earlier one, or the recipe that
// "recipe fmt -w" formats in place - is called "<final>.tfg-writing". Both are
// ours and both outlive a hard kill, and until 2026-09-06 the reading side knew
// only the first. So verify reported our own half written manifest as "extra",
// which is the word that means somebody else put it here.
//
// That is the same defect the comment on core.PartialMarker describes as fixed
// on 2026-08-03, arriving a second time through the other marker - which is the
// argument for the source level guard below rather than for this one alone.
//
// The sentence differs from the one above it deliberately. A half written
// generated file costs nothing: the manifest does not describe it and nothing
// is missing. A half written RECORD is the case where a person has to look,
// because the run was saving the list of what it produced, so the directory can
// hold finished files that nothing lists and that cleanup therefore cannot
// remove - untouchable rule 7.
func TestAHalfWrittenRecordIsNamedForWhatItIs(t *testing.T) {
	out, mf := generated(t)

	leftover := filepath.Join(out, filepath.Base(mf)+".tfg-writing")
	if err := os.WriteFile(leftover, []byte("{\"manifest_v"), 0o644); err != nil {
		t.Fatalf("writing the leftover: %v", err)
	}

	code, _, errOut := run(t, "verify", mf)

	if code != cli.ExitVerify {
		t.Errorf("exit %d, expected %d - a leftover is still a difference:\n%s", code, cli.ExitVerify, errOut)
	}
	if strings.Contains(errOut, "extra     "+filepath.Base(leftover)) {
		t.Errorf("our own half written manifest is reported as an ordinary extra file, which "+
			"tells the reader nothing and reads as a polluted directory:\n%s", errOut)
	}
	// Not the other marker's sentence. Both reach the same kind, so a build
	// that ran them through one branch would pass every check above while
	// telling somebody their record is an ordinary unfinished file.
	if strings.Contains(errOut, "an unfinished file") {
		t.Errorf("the half written record got the sentence written for a half written "+
			"GENERATED file, which does not mention the files that may be unlisted:\n%s", errOut)
	}
	if !strings.Contains(errOut, "record") {
		t.Errorf("the report does not say the file is a run's record:\n%s", errOut)
	}
	if !strings.Contains(errOut, "nothing lists") {
		t.Errorf("the report does not say the directory may hold files nothing lists, which is "+
			"the whole reason this case is worse than the one above:\n%s", errOut)
	}
}

// Each marker is spelled in exactly one place.
//
// This is the guard that would have caught the defect above, and the one above
// would not have caught it coming back. core.PartialMarker carries the argument
// in its own comment - "two parts of the tool have to agree on it: the engine
// writes it, and the reading side has to recognise one that outlived its run. A
// second spelling would mean verify reports our own leftovers as files it knows
// nothing about" - and that argument was written down, applied to one marker,
// and then the other marker was introduced with the literal duplicated and no
// reader taught about it.
//
// A written reason is a claim until something can turn red on it.
//
// Asked of quoted Go literals rather than of the text, so the prose in
// core/createnew.go that names these files while explaining what it defends
// against is not a hit.
func TestEachLeftoverMarkerIsSpelledInOnePlace(t *testing.T) {
	root := repoRoot(t)
	home := filepath.Join("internal", "core", "limits.go")

	for _, marker := range []string{".tfg-partial-", ".tfg-writing"} {
		literal := "\"" + marker + "\""
		var found []string
		for _, dir := range []string{"internal", "cmd"} {
			err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d os.DirEntry, err error) error {
				if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
					return err
				}
				if strings.HasSuffix(path, "_test.go") {
					return nil
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				if strings.Contains(string(body), literal) {
					rel, _ := filepath.Rel(root, path)
					found = append(found, rel)
				}
				return nil
			})
			if err != nil {
				t.Fatalf("walking %s: %v", dir, err)
			}
		}

		// Asserted rather than assumed. Nought occurrences would mean the
		// constant was renamed and this guard is now checking a marker nothing
		// uses, which is a green that proves nothing.
		if len(found) == 0 {
			t.Fatalf("%s is spelled nowhere in the shipped code, so this guard checked nothing. "+
				"If the marker was renamed, rename it here too.", marker)
		}
		if len(found) == 1 && found[0] == home {
			continue
		}
		t.Errorf("%s is spelled in %d place(s): %v\n"+
			"What to do: declare it once in %s and read it everywhere else. A second spelling "+
			"is how the reading side came to not recognise a file this tool wrote itself.",
			marker, len(found), found, home)
	}
}
