//go:build !windows

package guard

import (
	"context"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/audit"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// TestVerifyAndCleanupNeverWaitOnAPipe puts a pipe where a run left an empty
// file. The size the manifest records is 0 and a pipe's size is 0, so the size
// check let it through, and os.Open then waited on it until something wrote to
// it - which may be never (docs/STABILITY-REVIEW-2026-10-26.md, R04). Both
// commands read listed files through core.OpenRegular since 2026-10-26, which
// opens without waiting and turns away what is not a file.
//
// An empty file is the case and not a convenience: every preset that probes
// "nothing at all" lists one, and any other size the pipe fails at once.
func TestVerifyAndCleanupNeverWaitOnAPipe(t *testing.T) {
	out := filepath.Join(t.TempDir(), "out")
	if code, _, errOut := run(t, "generate", "--format", "txt", "--size", "0B", "--clean", "--out", out); code != 0 {
		t.Fatalf("generate gave %d:\n%s", code, errOut)
	}
	mf := filepath.Join(out, "manifest.json")
	m, err := manifest.Load(mf)
	if err != nil {
		t.Fatalf("loading the manifest: %v", err)
	}
	if len(m.Files) != 1 || m.Files[0].Bytes != 0 {
		t.Fatalf("the run is not one empty file, so this is not the case it is for: %+v", m.Files)
	}
	full := filepath.Join(out, m.Files[0].Path)
	if err := os.Remove(full); err != nil {
		t.Fatalf("removing the empty file: %v", err)
	}
	if err := syscall.Mkfifo(full, 0o600); err != nil {
		t.Fatalf("making a pipe in its place: %v", err)
	}

	answersWithoutWaiting(t, "verify", func() bool {
		diffs, err := audit.Verify(context.Background(), out, m, filepath.Base(mf))
		return err == nil && len(diffs) == 1 && diffs[0].Kind == audit.Unreadable
	})
	answersWithoutWaiting(t, "cleanup", func() bool {
		cands, err := audit.Inspect(context.Background(), out, m)
		return err == nil && len(cands) == 1 && cands[0].Disposition == audit.Unreachable
	})
}

// answersWithoutWaiting holds one command to an answer within ten seconds, and
// to the right one.
func answersWithoutWaiting(t *testing.T, name string, ask func() bool) {
	t.Helper()
	answered := make(chan bool, 1)
	go func() { answered <- ask() }()
	select {
	case right := <-answered:
		if !right {
			t.Errorf("%s answered about the pipe, but not that it cannot read it", name)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("%s is still waiting on the pipe after ten seconds - it opened it", name)
	}
}
