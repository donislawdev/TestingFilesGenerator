package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// One file under two spellings is named for what it is, and the destructive
// command and the reporting one agree about it.
//
// Measured on 2026-08-27, and what was found was a contradiction rather than an
// inconsistency of style. Rename a generated file to the other case on Windows
// and verify said "extra REPORT_0001.TXT" - on its own, without the "missing"
// that would have given it away, because os.Stat had already found the entry
// under the name the manifest gives. So one difference where a mismatch usually
// shows two, reading as a directory somebody had polluted. Meanwhile cleanup,
// on the same directory and the same manifest, deleted that same file and said
// "1 file removed". The reporting command called it somebody else's, and the
// destructive one treated it as ours.
//
// Untouchable rule 7 is what settles the second half: cleanup removes what the
// manifest lists and nothing else, and REPORT_0001.TXT is not what it lists.
// The filesystem was bending that rule quietly, because os.Remove is given the
// manifest's spelling and a case-insensitive filesystem finds whatever is
// there.
//
// Verify still compares literally, which is the older decision and stands - it
// asks whether the file described is the file that is here, on THIS machine,
// where the filesystem has already answered by storing what it stored. What
// changed is that the disagreement is now named.
func TestOneFileUnderTwoSpellingsIsNamedAndNotDeleted(t *testing.T) {
	dir := t.TempDir()
	if !foldsNames(t, dir) {
		t.Skip("this filesystem keeps the two spellings apart, so they are two files here and " +
			"both commands already agree - the disagreement this guard is about cannot arise")
	}

	out := filepath.Join(dir, "out")
	code, _, errOut := run(t, "generate", "--format", "txt", "--size", "1kb", "--count", "2",
		"--name", "report_{index:04}.txt", "--out", out)
	if code != 0 {
		t.Fatalf("the run ended with %d: %s", code, errOut)
	}

	from := filepath.Join(out, "report_0001.txt")
	to := filepath.Join(out, "REPORT_0001.TXT")
	if err := os.Rename(from, to); err != nil {
		t.Fatalf("renaming to the other spelling: %v", err)
	}

	manifestPath := filepath.Join(out, "manifest.json")

	// The reporting half. It has to say this is a spelling rather than
	// somebody else's file, and it has to name what the manifest calls it -
	// without that the reader has nothing to act on.
	code, _, errOut = run(t, "verify", manifestPath)
	if code == 0 {
		t.Fatal("verify is content with a directory whose file is not spelled the way the manifest says")
	}
	if strings.Contains(errOut, "extra ") {
		t.Errorf("verify calls one of our own files extra, which reads as a polluted directory:\n%s", errOut)
	}
	if !strings.Contains(errOut, "respelled") || !strings.Contains(errOut, "report_0001.txt") {
		t.Errorf("verify does not say this is the same file under another spelling, "+
			"or does not say what the manifest calls it:\n%s", errOut)
	}

	// The destructive half, and this is the one untouchable rule 7 is about.
	code, _, errOut = run(t, "cleanup", manifestPath, "--yes")
	if code == 0 {
		t.Error("cleanup reports a clean sweep over a file whose name the manifest does not list")
	}
	if _, err := os.Stat(to); err != nil {
		t.Errorf("cleanup deleted %s, and the manifest lists report_0001.txt. "+
			"Rule 7 says it removes what the manifest lists and nothing else: %v", to, err)
	}

	// And the file that IS spelled the way the manifest says is still removed,
	// so this did not turn cleanup into a no-op.
	if _, err := os.Stat(filepath.Join(out, "report_0002.txt")); err == nil {
		t.Errorf("cleanup left the file it was asked for as well, so it stopped doing its job:\n%s", errOut)
	}
}

// foldsNames asks the filesystem under dir whether it treats two spellings of
// one name as one file.
//
// Asked rather than assumed from runtime.GOOS, because the answer belongs to
// the filesystem and not to the operating system: a case sensitive volume on
// Windows and a case sensitive APFS volume both exist, and this guard would
// otherwise fail on them for a defect that cannot happen there.
func foldsNames(t *testing.T, dir string) bool {
	t.Helper()
	probe := filepath.Join(dir, "spellingprobe.txt")
	if err := os.WriteFile(probe, []byte("x"), 0o644); err != nil {
		t.Fatalf("writing the probe: %v", err)
	}
	defer os.Remove(probe)

	_, err := os.Stat(filepath.Join(dir, "SPELLINGPROBE.TXT"))
	return err == nil
}
