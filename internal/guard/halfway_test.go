package guard

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/audit"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool/checksum"
)

// Things that happen to a folder half way through a run - a Ctrl+C, a link
// pointed somewhere else - found by the review of #158
// (docs/REVIEW-158-2026-10-05.md). Each guard stops or changes the run at a
// point it can name rather than at a moment on a clock.

// cancelledFromLook is a context cancelled from its n-th look on - nought is
// never - so a walk can be stopped half way without a clock. It counts the
// looks as well, so a guard can first ask how many a whole walk takes.
type cancelledFromLook struct {
	context.Context
	from, looks int
}

func (c *cancelledFromLook) Err() error {
	c.looks++
	if c.from > 0 && c.looks >= c.from {
		return context.Canceled
	}
	return nil
}

// TestAWalkStoppedHalfWayHandsBackNothingItFound stops a walk after it met a
// folder it could not list, and holds it to saying only that it was stopped.
//
// Until the review of #158 it handed back what it had found by then, and
// verify looks at a folder it could not list before anything else - so a
// Ctrl+C there ended as a refusal of the folder, code 5, rather than as an
// interruption.
func TestAWalkStoppedHalfWayHandsBackNothingItFound(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("SKIPPED: no way to make a folder unreadable to its own user here")
	}
	dir := folderOf(t, map[string]string{"a/x": "x", "b/y": "y"})
	locked := filepath.Join(dir, "a")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o755) })

	// The whole walk first, asserting the state the guard is about: "a" could
	// not be listed and "b/y" is the last thing met, after it - a walk goes in
	// the order of the names.
	whole := &cancelledFromLook{Context: context.Background()}
	found, err := audit.Walk(whole, dir)
	if err != nil || len(found.Unreadable) != 1 || len(found.Entries) != 1 || found.Entries[0].Path != "b/y" {
		t.Fatalf("the whole walk was to meet one folder it could not list and then b/y, and came to %v and %+v", err, found)
	}

	half := &cancelledFromLook{Context: context.Background(), from: whole.looks}
	found, err = audit.Walk(half, dir)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("a walk cancelled at its last look ended with %v", err)
	}
	if len(found.Unreadable) != 0 || len(found.Entries) != 0 {
		t.Errorf("a walk cancelled half way handed back what it had found by then - %d folders it could not list "+
			"and %d entries - which a caller looking at those first takes for the answer", len(found.Unreadable), len(found.Entries))
	}
}

// TestAFolderSwappedHalfWayGetsNoChecksumFileOfAnother runs checksum-write on
// a folder named through a link, and points the link at another folder the
// moment the tool says it is reading.
//
// Until the review of #158 the walk followed the link, and the reading and the
// writing each followed it again by name - so the checksum file of the first
// folder landed in the second, describing files that are not there. Now the
// folder is looked up once and everything after works in what that look found.
func TestAFolderSwappedHalfWayGetsNoChecksumFileOfAnother(t *testing.T) {
	first := folderOf(t, map[string]string{"f.txt": "abc"})
	second := folderOf(t, map[string]string{"f.txt": "xyz"})
	link := filepath.Join(t.TempDir(), "link")
	plantLinkWith(t, os.Symlink, os.Getenv("CI"), first, link)
	d, err := tool.Get(checksum.WriteID)
	if err != nil {
		t.Fatal(err)
	}

	// Called from a worker of the tool, so a failure here is reported rather
	// than fatal - a test may not stop itself from another goroutine.
	swapped := false
	_, err = d.Start(context.Background(), tool.Request{Inputs: map[string]string{checksum.InputFolder: link}}, func(int64, int64) {
		if swapped {
			return
		}
		swapped = true
		if err := os.Remove(link); err != nil {
			t.Errorf("taking the link away: %v", err)
		}
		if err := os.Symlink(second, link); err != nil {
			t.Errorf("pointing the link at the second folder: %v", err)
		}
	})
	if !swapped {
		t.Fatal("the tool never said it was reading, so nothing was swapped and this guard asked nothing")
	}
	if err != nil {
		t.Fatalf("checksum-write of a folder swapped half way answered %v", err)
	}
	if _, err := os.Lstat(filepath.Join(second, "SHA256SUMS")); err == nil {
		t.Error("the link was pointed at another folder half way and the checksum file landed there, about files it did not read")
	}
	got, err := os.ReadFile(filepath.Join(first, "SHA256SUMS"))
	if want := abcSHA256 + "  f.txt\n"; err != nil || string(got) != want {
		t.Errorf("the folder whose file was read holds %q (%v) and should hold %q", got, err, want)
	}
}
