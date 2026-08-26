package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// A manifest that could not be written leaves no manifest at all.
//
// The run claims the manifest name before it writes its first file, and the
// claim is an empty file. So a save that fails after the files are on disk used
// to leave a nought byte manifest.json sitting beside a complete set of files,
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

	// The claim, exactly as a run makes it before writing anything.
	if err := manifest.Claim(path); err != nil {
		t.Fatalf("claiming the name: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the claim did not create the file: %v", err)
	}

	// Block the temporary name with a directory, which is how this was
	// reproduced against the real binary. Any other way of making the write
	// fail would do - this one needs no permissions and works on every host.
	if err := os.Mkdir(path+".tfg-writing", 0o755); err != nil {
		t.Fatalf("blocking the temporary name: %v", err)
	}

	m := manifest.New("testing-files-generator", "0.0.0-test", "run_x", "tfg generate", 1, "windows", "amd64")
	m.Add(manifest.File{ID: "files", Path: "files_0001.txt", Name: "files_0001.txt", Bytes: 1024})

	if err := m.Save(path); err == nil {
		t.Fatal("saving over a blocked temporary name reported success, so this test is not reaching the failure it is about")
	}

	if _, err := os.Stat(path); err == nil {
		t.Error("a manifest is still there after the save failed. It is empty, so cleanup cannot read it and " +
			"the next run into this directory is refused for a record that records nothing")
	}
}

// Giving a claimed name back never takes away a manifest with something in it.
//
// This is what makes the rule above safe to have. A failed save now removes the
// name it was writing to, and the one file this tool promises never to destroy
// is a manifest - so the removal has to be able to tell its own empty claim
// from somebody's record of a thousand files.
//
// Asked of Release directly rather than through a failed run, and that is a
// correction rather than a shortcut. The first version of this drove a real
// save over a real manifest, passed, and could not be reddened by any single
// mutation - because in that path the file is protected TWICE: Save refuses at
// the size check before it ever writes, and Release refuses again afterwards.
// A guard nothing can break is not a guard, and this project has removed six
// pieces of code for exactly that reason. The end to end half of the question
// already has a guard of its own in manifestsafety_test.go.
func TestGivingAClaimedNameBackSparesAManifestWithContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	const theirs = `{"manifest_version":"1.0","files":[]}`
	if err := os.WriteFile(path, []byte(theirs), 0o644); err != nil {
		t.Fatalf("writing the earlier manifest: %v", err)
	}

	if err := manifest.Release(path); err != nil {
		t.Fatalf("releasing a name somebody else holds reported an error: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the earlier manifest is gone: %v", err)
	}
	if string(got) != theirs {
		t.Errorf("the earlier manifest changed.\n got: %s\nwant: %s", got, theirs)
	}

	// And the empty claim it IS meant to take away still goes, or the check
	// above would pass against a Release that never removes anything.
	claim := filepath.Join(dir, "claim.json")
	if err := manifest.Claim(claim); err != nil {
		t.Fatalf("claiming a name: %v", err)
	}
	if err := manifest.Release(claim); err != nil {
		t.Fatalf("releasing our own claim: %v", err)
	}
	if _, err := os.Stat(claim); err == nil {
		t.Error("an empty claim survived being given back, so a refused run leaves the name taken")
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
		if err := os.MkdirAll(filepath.Join(out, "manifest.json.tfg-writing"), 0o755); err != nil {
			t.Fatalf("blocking the temporary name: %v", err)
		}

		code, _, errOut := run(t, "generate",
			"--format", "txt", "--size", "1kb",
			"--count", itoa(c.count), "--out", out)

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
