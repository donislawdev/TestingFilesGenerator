package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// The protection against writing over an earlier run was a check followed by a
// write, which is not the same as claiming the name.
//
// Measured on 2026-08-03, two runs started together into one directory, under
// different ids so that no file name collided:
//
//	alpha exit=0    beta exit=0
//	16 files on the disk, one manifest, describing eight of them
//
// The eight files of the other run were left with nothing able to remove them,
// which is the exact harm the manifest safety guard was written to prevent. It
// holds for runs one after another and said nothing about runs at the same
// time.
//
// This does not start two processes. It reproduces what they meet: a manifest
// that appeared between the preflight and the write. That is the condition, and
// asserting on it directly makes the guard deterministic rather than a race
// somebody has to lose to see fail.
func TestAManifestIsNeverWrittenOverEvenWhenItAppearsMidRun(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	if err := os.WriteFile(path, []byte(`{"manifest_version":"1.0","files":[]}`), 0o644); err != nil {
		t.Fatalf("writing the manifest of the other run: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading it back: %v", err)
	}

	m := manifest.New("testing-files-generator", "0.0.0-dev", "run_x", "tfg generate", 1, "windows", "amd64")
	err = m.Save(path)
	if err == nil {
		t.Error("Save wrote over a manifest that was already there - the record of the run it described is gone and its files can never be cleaned up")
	}

	after, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatalf("the manifest of the other run disappeared: %v", readErr)
	}
	if string(after) != string(before) {
		t.Errorf("the manifest of the other run was modified:\nbefore %s\nafter  %s", before, after)
	}
}

// The name is taken before the run writes anything, not after it writes
// everything.
//
// Claiming at save time already stopped two runs from both writing a manifest,
// and it happened at the end - so the second run wrote its whole set of files
// and only then found out it had nowhere to record them. Measured on
// 2026-08-03, two runs started together under different ids: they ended 0 and
// 5, with sixteen files on the disk and eight of them in nobody's manifest.
// After the claim moved to the front: eight files and one manifest.
//
// Since 2026-09-29 the reservation is the temporary name the manifest is
// written under, not an empty file under the final name (O252). So while a run
// goes there is no manifest at all, and nothing can read an empty one as the
// record of a run that happened.
//
// What is guarded here is the primitive, because the window it closes is
// between two processes and this package keeps concurrency out on purpose.
// The engine calling it is covered by the guards for a refused run writing
// nothing.
func TestAClaimedNameCannotBeClaimedTwice(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	first, err := manifest.Claim(path)
	if err != nil {
		t.Fatalf("claiming an unused name: %v", err)
	}
	defer func() { _ = first.Release() }()
	if second, err := manifest.Claim(path); err == nil {
		_ = second.Release()
		t.Error("the same name was claimed twice, so two runs could both believe it is theirs")
	}

	if _, err := os.Lstat(path); err == nil {
		t.Error("a manifest exists while the run that reserved it is still going - a reader would take it for a record")
	}
	info, err := os.Stat(manifest.ReservationPath(path))
	if err != nil {
		t.Fatalf("the reservation is not there: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("the reservation carries %d B before the save - it has to be empty", info.Size())
	}
}

// And a run that reserves the name and then cannot start gives it back, or the
// next run into that directory is told a run is going when none is.
func TestAClaimIsGivenBackAndLeavesNoManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	r, err := manifest.Claim(path)
	if err != nil {
		t.Fatalf("claiming: %v", err)
	}
	if err := r.Release(); err != nil {
		t.Fatalf("releasing: %v", err)
	}
	for _, name := range []string{path, manifest.ReservationPath(path)} {
		if _, err := os.Lstat(name); err == nil {
			t.Errorf("%s is still there after a run that wrote nothing gave its name back", filepath.Base(name))
		}
	}
	if again, err := manifest.Claim(path); err != nil {
		t.Errorf("the name could not be claimed again after it was given back: %v", err)
	} else {
		_ = again.Release()
	}
}

// A manifest that appears between the reservation and the save is left as it
// is, and the save says so.
//
// This is the second window of O252, measured on 2026-09-29: the save used to
// rename over the empty claim under the final name, and a rename replaces what
// it lands on. So anything put there during the run - by a person, by another
// tool - was destroyed without a word. Reproduced here without a second
// process, by putting the file there at the moment the race would.
func TestAManifestPutThereDuringTheRunIsNotWrittenOver(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	r, err := manifest.Claim(path)
	if err != nil {
		t.Fatalf("claiming: %v", err)
	}
	theirs := []byte(`{"manifest_version":"1.0","files":[]}`)
	if err := os.WriteFile(path, theirs, 0o644); err != nil {
		t.Fatalf("putting the other manifest there: %v", err)
	}

	m := manifest.New("testing-files-generator", "0.0.0-dev", "run_x", "tfg generate", 1, "windows", "amd64")
	if err := r.Save(m); err == nil {
		t.Error("the save reported success over a manifest somebody else put there")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the other manifest is gone: %v", err)
	}
	if string(after) != string(theirs) {
		t.Errorf("the other manifest was written over:\nbefore %s\nafter  %s", theirs, after)
	}
	if _, err := os.Lstat(manifest.ReservationPath(path)); err == nil {
		t.Error("the refused save left its reservation behind, so the next run would be told a run is going")
	}
}

// The ordinary case, so the claim above cannot be satisfied by refusing to
// write a manifest at all.
func TestAManifestIsStillWrittenWhenNothingIsThere(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	m := manifest.New("testing-files-generator", "0.0.0-dev", "run_x", "tfg generate", 1, "windows", "amd64")
	if err := m.Save(path); err != nil {
		t.Fatalf("Save refused an empty directory: %v", err)
	}

	loaded, err := manifest.Load(path)
	if err != nil {
		t.Fatalf("the manifest it wrote cannot be read back: %v", err)
	}
	if loaded.Run.ID != "run_x" {
		t.Errorf("the manifest came back as %q", loaded.Run.ID)
	}
	// Nothing of the write is left beside it.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading the directory: %v", err)
	}
	for _, e := range entries {
		if e.Name() != "manifest.json" {
			t.Errorf("the write left %s behind", e.Name())
		}
	}
}

// A name the host system stores under a different name than the one it was
// given. The file lands, the manifest describes the name that was asked for,
// and the two are not the same string.
//
// Measured on 2026-08-03: "--name trailing." ended with exit code 0, the file
// arrived as "trailing", and "tfg verify" on that directory failed a second
// later with exit code 7 reporting "extra trailing". The tool produced output
// its own command rejects, and nothing said the name had been changed.
func TestANameTheSystemWouldNotStoreVerbatimIsRefused(t *testing.T) {
	for _, name := range []string{"trailing.", "sp ace ", "dots..", "two  "} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			code, stdout, errOut := run(t,
				"generate", "--format", "txt", "--size", "1kb",
				"--name", name, "--out", dir)

			if code == cli.ExitOK {
				t.Fatalf("the name %q was accepted:\n%s", name, stdout)
			}
			if code != cli.ExitRecipe {
				t.Errorf("exit %d, expected %d:\n%s", code, cli.ExitRecipe, errOut)
			}
			if !strings.Contains(errOut, "dot or a space") {
				t.Errorf("the refusal does not say what is wrong with the name:\n%s", errOut)
			}
			// Nothing was written under either spelling.
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("reading the directory: %v", err)
			}
			if len(entries) != 0 {
				t.Errorf("a refused name still produced %d entry(s)", len(entries))
			}
		})
	}
}

// The other direction, so the rule cannot creep into refusing ordinary names
// that merely contain a dot or a space.
func TestAnOrdinaryNameWithADotOrASpaceStillWorks(t *testing.T) {
	for _, name := range []string{"invoice.txt", "my file.txt", "a.b.txt"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if code, _, errOut := run(t,
				"generate", "--format", "txt", "--size", "1kb",
				"--name", name, "--out", dir); code != cli.ExitOK {
				t.Fatalf("the name %q was refused: exit %d\n%s", name, code, errOut)
			}
			if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
				t.Errorf("the file did not land under the name that was asked for: %v", err)
			}
		})
	}
}

// A manifest is read into memory to be compared against a directory, so its
// size is a lever somebody else holds. The recipe has had a ceiling since
// 2026-08-02 and this had none, which was an asymmetry rather than a decision.
func TestAManifestTooLargeToReadIsRefusedBeforeItIsRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")

	// Just over the ceiling, written as valid JSON so that the refusal cannot
	// be mistaken for a parse failure.
	padding := strings.Repeat("x", manifest.MaxBytes)
	doc := `{"manifest_version":"1.0","files":[],"run":{"command":"` + padding + `"}}`
	if err := os.WriteFile(path, []byte(doc), 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}

	code, stdout, errOut := run(t, "verify", path)
	if code != cli.ExitIO {
		t.Errorf("exit %d, expected %d:\n%s", code, cli.ExitIO, errOut)
	}
	if !strings.Contains(errOut, "the limit is") {
		t.Errorf("the refusal does not say there is a limit:\n%s", errOut)
	}
	if stdout != "" {
		t.Errorf("a failed run wrote to stdout:\n%s", stdout)
	}
}
