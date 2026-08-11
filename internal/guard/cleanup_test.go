package guard

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// "cleanup" is the only command in this tool that destroys data, and it runs
// in directories that belong to somebody else. A person generates fixtures
// into a folder where they already keep something of their own, and the tool
// cannot tell the two apart except by the list it was handed.
//
// So the guard that matters most is not "does it delete" - it is "does it
// leave everything else alone". That one is asserted on every path below.

// bystander is a file nobody's manifest mentions. It goes into every case
// here, and its survival is checked every time.
const bystander = "not_ours.txt"

func withBystander(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, bystander), []byte("somebody's own work\n"), 0o644); err != nil {
		t.Fatalf("writing the bystander: %v", err)
	}
}

func bystanderSurvives(t *testing.T, dir string) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(dir, bystander))
	if err != nil {
		t.Fatalf("cleanup removed a file that is in nobody's manifest: %v", err)
	}
	if string(body) != "somebody's own work\n" {
		t.Errorf("a file outside the manifest was modified: %q", body)
	}
}

// The default run prints and deletes nothing. A tool that removes files on the
// strength of one argument is the wrong shape here, and asking interactively
// is ruled out by docs/CLI.md section 9.
func TestCleanupWithoutYesRemovesNothing(t *testing.T) {
	out, mf := generated(t)
	withBystander(t, out)
	before := entryNames(t, out)

	code, stdout, errOut := run(t, "cleanup", mf)
	if code != cli.ExitOK {
		t.Fatalf("exit %d:\n%s", code, errOut)
	}
	if !strings.Contains(stdout, "would be removed") {
		t.Errorf("the list does not say it is a preview:\n%s", stdout)
	}
	if !strings.Contains(errOut, "--yes") {
		t.Errorf("nothing tells the reader how to actually remove them:\n%s", errOut)
	}

	if after := entryNames(t, out); len(after) != len(before) {
		t.Errorf("a run without --yes removed something: %v -> %v", before, after)
	}
	bystanderSurvives(t, out)
}

// Untouchable rule 7. The list is the whole authority.
func TestCleanupRemovesWhatTheManifestListsAndNothingElse(t *testing.T) {
	out, mf := generated(t)
	withBystander(t, out)

	code, stdout, errOut := run(t, "cleanup", mf, "--yes")
	if code != cli.ExitOK {
		t.Fatalf("exit %d:\n%s", code, errOut)
	}
	if !strings.Contains(stdout, "3 file(s) removed") {
		t.Errorf("the report does not say what went:\n%s", stdout)
	}

	bystanderSurvives(t, out)

	// The manifest is not in its own list of files, so it stays by default.
	if _, err := os.Stat(mf); err != nil {
		t.Errorf("the manifest was removed without being asked for: %v", err)
	}

	left := entryNames(t, out)
	for _, n := range left {
		if n != bystander && n != "manifest.json" {
			t.Errorf("a generated file survived cleanup: %s", n)
		}
	}
}

// Running it twice has to be quiet the second time, or people stop putting it
// in scripts.
func TestCleanupRunTwiceIsNotAnError(t *testing.T) {
	out, mf := generated(t)
	if code, _, errOut := run(t, "cleanup", mf, "--yes"); code != cli.ExitOK {
		t.Fatalf("first run gave %d:\n%s", code, errOut)
	}
	code, _, errOut := run(t, "cleanup", mf, "--yes")
	if code != cli.ExitOK {
		t.Errorf("the second run gave %d, expected %d - a file already gone is the state that was asked for:\n%s", code, cli.ExitOK, errOut)
	}
	if !strings.Contains(errOut, "already gone") {
		t.Errorf("the second run does not say why it removed nothing:\n%s", errOut)
	}
	bystanderSurvivesAbsent(t, out)
}

func bystanderSurvivesAbsent(t *testing.T, dir string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, bystander)); err == nil {
		t.Error("this case did not write a bystander, so finding one means the fixture drifted")
	}
}

// A file whose content changed since it was written may not be ours. Deleting
// it would be the tool guessing whose work it is, which is the one thing rule
// 7 exists to stop.
func TestCleanupLeavesAChangedFileAloneUnlessForced(t *testing.T) {
	out, mf := generated(t)
	victim := firstGenerated(t, out)
	full := filepath.Join(out, victim)

	body, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	body[0] ^= 0xFF
	if err := os.WriteFile(full, body, 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}

	code, _, errOut := run(t, "cleanup", mf, "--yes")
	if code != cli.ExitIO {
		t.Errorf("exit %d, expected %d - a file left behind has to reach the exit code or a script never learns", code, cli.ExitIO)
	}
	if !strings.Contains(errOut, victim) || !strings.Contains(errOut, "changed") {
		t.Errorf("the report does not say which file was kept and why:\n%s", errOut)
	}
	// Named as the setting rather than as the flag, since 2026-08-11: this
	// sentence is written below both surfaces and shown by both, and the window
	// has no flags on it - so a spelling only the command line takes sends the
	// other reader translating. O79. What has to survive is that the report
	// says there is a way through, which is what somebody stuck needs.
	if !strings.Contains(strings.ToLower(errOut), "force") {
		t.Errorf("the report does not say how to remove it anyway:\n%s", errOut)
	}
	if _, err := os.Stat(full); err != nil {
		t.Fatalf("the changed file was removed without --force: %v", err)
	}

	// --force is the explicit statement that it is ours after all.
	if code, _, errOut := run(t, "cleanup", mf, "--yes", "--force"); code != cli.ExitOK {
		t.Errorf("--force gave %d:\n%s", code, errOut)
	}
	if _, err := os.Stat(full); err == nil {
		t.Error("--force did not remove the changed file")
	}
}

// The manifest goes only when asked, and only once it describes nothing that
// is still there. It is the sole record of the files it lists.
func TestCleanupRemovesTheManifestOnlyWhenAskedAndOnlyWhenEverythingElseWent(t *testing.T) {
	_, mf := generated(t)
	if code, _, errOut := run(t, "cleanup", mf, "--yes", "--with-manifest"); code != cli.ExitOK {
		t.Fatalf("exit %d:\n%s", code, errOut)
	}
	if _, err := os.Stat(mf); err == nil {
		t.Error("--with-manifest did not remove the manifest")
	}

	// Now the case where something was left behind. The manifest has to stay.
	out2, mf2 := generated(t)
	victim := firstGenerated(t, out2)
	full := filepath.Join(out2, victim)
	body, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	body[0] ^= 0xFF
	if err := os.WriteFile(full, body, 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}

	code, _, errOut := run(t, "cleanup", mf2, "--yes", "--with-manifest")
	if code != cli.ExitIO {
		t.Errorf("exit %d, expected %d", code, cli.ExitIO)
	}
	if _, err := os.Stat(mf2); err != nil {
		t.Error("the manifest was removed while a file it lists is still on the disk, so the only record of that file is gone")
	}
	if !strings.Contains(errOut, "only record") {
		t.Errorf("the message does not explain why the manifest was kept:\n%s", errOut)
	}
}

// An entry the manifest describes but never put on the disk is not a file to
// go removing, even when something of that name is sitting there.
//
// The scenario is real: a run in archive or manifest-only mode describes an
// artefact it deliberately did not write (D10), and later somebody's own file
// turns up under that name. The manifest even carries the right hash for the
// artefact, so every other check in cleanup would wave it through. Which entry
// the manifest actually claims is the only thing standing between that file
// and deletion.
//
// Built this way on purpose. The first version of this guard used an entry
// that no check could have passed anyway, so it proved nothing - mutation is
// what said so.
func TestCleanupDoesNotRemoveAFileTheRunNeverPutOnTheDisk(t *testing.T) {
	dir := t.TempDir()
	withBystander(t, dir)

	body, err := os.ReadFile(filepath.Join(dir, bystander))
	if err != nil {
		t.Fatalf("reading the bystander: %v", err)
	}
	sum := sha256.Sum256(body)

	mf := filepath.Join(dir, "manifest.json")
	doc := fmt.Sprintf(`{
  "manifest_version": "1.0",
  "files": [
    {"path":%q,"name":%q,"materialized":false,"bytes":%d,"hashes":{"sha256":%q}},
    {"path":"never_written.txt","name":"never_written.txt","materialized":true,"bytes":10,
     "hashes":{"sha256":""},"failed":true,"error":"the disk filled up"}
  ]
}`, bystander, bystander, len(body), hex.EncodeToString(sum[:]))
	if err := os.WriteFile(mf, []byte(doc), 0o644); err != nil {
		t.Fatalf("writing the manifest: %v", err)
	}

	code, _, errOut := run(t, "cleanup", mf, "--yes")
	if code != cli.ExitOK {
		t.Errorf("exit %d, expected %d:\n%s", code, cli.ExitOK, errOut)
	}
	bystanderSurvives(t, dir)

	// --force is a statement about content, not about what the run claimed.
	// It must not reach a file the run never wrote either.
	if code, _, errOut := run(t, "cleanup", mf, "--yes", "--force"); code != cli.ExitOK {
		t.Errorf("--force gave %d:\n%s", code, errOut)
	}
	bystanderSurvives(t, dir)
}

func entryNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}
