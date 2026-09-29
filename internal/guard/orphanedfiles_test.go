package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// A manifest that could not be written leaves no manifest at all.
//
// The run claims the manifest name before it writes its first file, and until
// 2026-09-29 the claim was an empty file under the final name. So a save that
// failed after the files were on disk used to leave a nought byte
// manifest.json sitting beside a complete set of files,
// and that one empty file is worse than nothing three separate ways. Measured
// on 2026-08-27 by putting a directory under the temporary name the writer uses
// and running an ordinary generate:
//
//	cleanup   exit 5, "unexpected end of JSON input"
//	verify    exit 5, the same
//	generate  refused, and the refusal called that file "the only record of
//	          what an earlier run wrote"
//
// The third is the one that makes this worth a guard rather than a note. That
// sentence is true every other time it is printed, so somebody reads it and
// goes looking for a run whose files it cannot name - and the files it should
// have named are right there, unrecorded. Review item S2, which the review
// itself marked as read from the code and never reproduced.
//
// What this asks is the outcome rather than the mechanism: after a failed save,
// is there a manifest? Asking whether Release was called would pass against a
// build that called it on a file it had already replaced.
func TestAManifestThatCouldNotBeWrittenLeavesNoManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	// The reservation, exactly as a run makes it before writing anything.
	r, err := manifest.Claim(path)
	if err != nil {
		t.Fatalf("reserving the name: %v", err)
	}
	if _, err := os.Stat(manifest.ReservationPath(path)); err != nil {
		t.Fatalf("the reservation did not create its file: %v", err)
	}

	// Block the final name with a directory, so the save fails at the very
	// last step. Since 2026-09-29 the reservation is the temporary name
	// itself, so blocking that one - how this was first reproduced - now
	// refuses the reservation instead, before any file is written.
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("blocking the final name: %v", err)
	}

	m := manifest.New("testing-files-generator", "0.0.0-test", "run_x", "tfg generate", 1, "windows", "amd64")
	m.Add(manifest.File{ID: "files", Path: "files_0001.txt", Name: "files_0001.txt", Bytes: 1024})

	if err := r.Save(m); err == nil {
		t.Fatal("saving over a blocked final name reported success, so this test is not reaching the failure it is about")
	}

	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
		t.Error("a manifest file is there after the save failed, so the next run into this directory " +
			"is refused for a record that may record nothing")
	}
	if _, err := os.Lstat(manifest.ReservationPath(path)); err == nil {
		t.Error("the failed save left its reservation behind, so the next run is told a run is going when none is")
	}
}

// A clean-up removes the file this tool wrote and nothing that took its name.
//
// This is what makes the rule above safe to have. A failed save removes the
// name it was writing to, and the one thing this tool promises never to
// destroy is a file somebody else put there - so the removal has to be able to
// tell its own file from one that took the name while it worked (O252, the
// third window: until 2026-09-29 every clean-up removed by name).
//
// Asked of core.RemoveOwn directly, because every clean-up of this kind goes
// through it - the reservation, the run lock, the instructions after a failed
// manifest. The replacement is made the way the measurement made it: ours
// removed and theirs created at once, which on ext4 hands theirs our inode
// number every time.
func TestGivingAClaimedNameBackSparesWhatTookItsName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json.tfg-writing")

	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatalf("writing our file: %v", err)
	}
	ours, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("asking what our file is: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("taking our file away: %v", err)
	}
	const theirs = `{"manifest_version":"1.0","files":[]}`
	if err := os.WriteFile(path, []byte(theirs), 0o644); err != nil {
		t.Fatalf("putting their file under the name: %v", err)
	}

	if err := core.RemoveOwn(path, ours); err == nil {
		t.Error("removing our file under a name that now holds theirs reported success")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("their file is gone: %v", err)
	}
	if string(got) != theirs {
		t.Errorf("their file changed.\n got: %s\nwant: %s", got, theirs)
	}

	// And our own file IS removed, or the check above would pass against a
	// clean-up that never removes anything.
	mine, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("asking what the file is: %v", err)
	}
	if err := core.RemoveOwn(path, mine); err != nil {
		t.Fatalf("removing a file by its own identity: %v", err)
	}
	if _, err := os.Lstat(path); err == nil {
		t.Error("a file survived being removed by its own identity, so a failed save would leave its name taken")
	}
}

// A run whose manifest could not be saved says what that leaves behind.
//
// The line about the manifest is about the manifest, and the person's problem
// is the files: they are on the disk, nothing records them, and cleanup works
// from a manifest. Rule 6 says that has to be said rather than discovered by
// running cleanup and being told the file will not parse.
//
// Asked of the words the command prints rather than of the code that prints
// them, and the number is checked at one as well as at several - a sentence
// with a verb agreeing with the count reads wrong at exactly one of those, and
// core.Count carries a paragraph about that mistake.
//
// How the save is made to fail changed on 2026-09-29 (O252). This used to put a
// directory under the temporary name before the run. That name is now the
// run's reservation, taken before the first file, so the same block refuses the
// run at the start and it writes nothing - and nothing else a person can do
// before a run makes the save fail at the end, which is the point of the
// change. So the block is put in place during the run: the moment the
// reservation appears, a directory goes under the manifest's final name. The
// files are large enough that the run is still writing them by then, and the
// guard asserts that it got there rather than assuming.
func TestARunThatCannotSaveItsManifestSaysWhatItLeftBehind(t *testing.T) {
	for _, c := range []struct {
		count int
		want  string
	}{
		{1, "1 file written"},
		{3, "3 files written"},
	} {
		dir := t.TempDir()
		out := filepath.Join(dir, "out")
		final := filepath.Join(out, "manifest.json")
		blocked := blockWhenReserved(final)

		code, _, errOut := run(t, "generate",
			"--format", "txt", "--size", "16mb",
			"--count", itoa(c.count), "--out", out)

		if !<-blocked {
			t.Fatalf("count %d: the run finished before its manifest name could be blocked, so nothing here was tested", c.count)
		}
		if code == 0 {
			t.Fatalf("count %d: the run ended with 0 although its manifest could not be written", c.count)
		}
		if !strings.Contains(errOut, c.want) {
			t.Errorf("count %d: nothing says how many files were left unrecorded.\nwanted to see: %s\ngot:\n%s",
				c.count, c.want, errOut)
		}
		if !strings.Contains(errOut, "Cleanup works from a manifest") {
			t.Errorf("count %d: nothing says why cleanup cannot remove them.\ngot:\n%s", c.count, errOut)
		}
		if !strings.Contains(errOut, out) {
			t.Errorf("count %d: nothing names the directory the files are in.\ngot:\n%s", c.count, errOut)
		}
	}
}

// blockWhenReserved puts a directory under a manifest's final name the moment
// the run reserves it, and says on the channel whether it did so while the run
// still had the reservation - that is, before the save.
func blockWhenReserved(final string) <-chan bool {
	done := make(chan bool, 1)
	reservation := manifest.ReservationPath(final)
	go func() {
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Lstat(reservation); err == nil {
				if err := os.Mkdir(final, 0o755); err != nil {
					done <- false
					return
				}
				_, stillReserved := os.Lstat(reservation)
				done <- stillReserved == nil
				return
			}
		}
		done <- false
	}()
	return done
}
