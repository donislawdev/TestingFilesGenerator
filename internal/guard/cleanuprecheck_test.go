package guard

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/audit"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// Cleanup judges every file before it removes any, so the first file judged
// ours then waited for the hashing of all the others - and a change made to it
// in that wait was deleted without --force, under exit 0. Reproduced by an
// outside review on 2026-10-21 with 257 files and a change 400 ms in
// (docs/STABILITY-REVIEW-2026-10-26.md, R01).
//
// These ask it at the seam that wait lives in: Inspect has answered, Remove
// has not begun. No clock is involved, so the answer does not depend on how
// fast the machine hashes.

// inspected runs a small generate and judges it for cleanup, and hands back the
// directory, the candidates and the name of the first file.
func inspected(t *testing.T) (string, []audit.Candidate, string) {
	t.Helper()
	out, mf := generated(t)
	m, err := manifest.Load(mf)
	if err != nil {
		t.Fatalf("loading the manifest: %v", err)
	}
	victim := firstGenerated(t, out)
	// Pushed an hour back before it is judged, so the write below lands on a
	// later time even where the clock moves in steps of several milliseconds -
	// a write in the same step is the limit changedSince names, and a guard
	// that sometimes landed in it would be red for no fault.
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(out, victim), past, past); err != nil {
		t.Fatalf("moving the time of %s back: %v", victim, err)
	}
	cands, err := audit.Inspect(context.Background(), out, m)
	if err != nil {
		t.Fatalf("inspecting: %v", err)
	}
	// Asserted rather than assumed: a candidate that is not ready is never
	// asked again, so the question below would be answered by the old rule.
	for _, c := range cands {
		if c.Disposition != audit.Ready {
			t.Fatalf("%s was judged %s before anything was changed, so the guard is not where it means to be", c.Path, c.Disposition)
		}
	}
	return out, cands, victim
}

// keptAsChanged asks that Remove left the victim on the disk with a reason that
// says why, and took the rest - the rest is what shows Remove ran at all.
func keptAsChanged(t *testing.T, out, victim string, outcomes []audit.Outcome) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(out, victim)); err != nil {
		t.Fatalf("%s changed after it was checked and cleanup removed it anyway: %v", victim, err)
	}
	removed := 0
	for _, o := range outcomes {
		switch {
		case o.Path == victim:
			if !o.Blocked || !strings.Contains(o.Reason, "changed after it was checked") {
				t.Errorf("%s was kept but the report does not say so and why: %+v", victim, o)
			}
		case o.Removed:
			removed++
		}
	}
	if removed != len(outcomes)-1 {
		t.Errorf("%d of the %d files nobody touched were removed", removed, len(outcomes)-1)
	}
}

// A file written over in place, at its own length, after it was judged ours.
func TestCleanupLeavesAFileThatChangedAfterItWasChecked(t *testing.T) {
	out, cands, victim := inspected(t)
	full := filepath.Join(out, victim)

	body, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	body[0] ^= 0xFF
	if err := os.WriteFile(full, body, 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}

	outcomes, err := audit.Remove(context.Background(), out, cands, false)
	if err != nil {
		t.Fatalf("removing: %v", err)
	}
	keptAsChanged(t, out, victim, outcomes)
}

// A file put in the place of the one judged ours, with its size and its time.
// Only the identity tells the two apart, and on Windows only an identity read
// from the handle does - os.Stat there reads it lazily, by name, so two looks
// by name agree about whatever is there at the end (the review of #157,
// point 4).
func TestCleanupLeavesAFileReplacedAfterItWasChecked(t *testing.T) {
	out, cands, victim := inspected(t)
	full := filepath.Join(out, victim)

	before, err := os.Lstat(full)
	if err != nil {
		t.Fatalf("looking at %s: %v", victim, err)
	}
	body, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	body[0] ^= 0xFF
	// Made before the original goes, so the filesystem cannot hand the new
	// file the number the old one had.
	swap := filepath.Join(out, "swap.tmp")
	if err := os.WriteFile(swap, body, 0o644); err != nil {
		t.Fatalf("writing the replacement: %v", err)
	}
	if err := os.Chtimes(swap, before.ModTime(), before.ModTime()); err != nil {
		t.Fatalf("giving the replacement the old time: %v", err)
	}
	if err := os.Remove(full); err != nil {
		t.Fatalf("removing the original: %v", err)
	}
	if err := os.Rename(swap, full); err != nil {
		t.Fatalf("putting the replacement in its place: %v", err)
	}
	// Asserted, so a replacement that differed in size or time cannot make
	// this green for a reason other than the identity.
	after, err := os.Lstat(full)
	if err != nil {
		t.Fatalf("looking at the replacement: %v", err)
	}
	if after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("the replacement is %d B at %v and the original was %d B at %v, so this is not the case it is for",
			after.Size(), after.ModTime(), before.Size(), before.ModTime())
	}

	outcomes, err := audit.Remove(context.Background(), out, cands, false)
	if err != nil {
		t.Fatalf("removing: %v", err)
	}
	keptAsChanged(t, out, victim, outcomes)
}

// A name the system would not even look up is not a file that is missing.
// Every failure to look was called "missing" until 2026-10-26, so a file
// somebody had taken the permissions off read as gone (R04, the third of the
// further points). A name longer than any filesystem holds is the one way to
// that failure that is the same on every system: ENAMETOOLONG on Linux and
// macOS, "the syntax is incorrect" on Windows - measured on Windows the same
// day, where the build before this said missing.
func TestVerifyCallsANameItCouldNotLookUpUnreadableRatherThanMissing(t *testing.T) {
	out, mf := generated(t)
	m, err := manifest.Load(mf)
	if err != nil {
		t.Fatalf("loading the manifest: %v", err)
	}
	long := strings.Repeat("a", 300) + ".txt"
	m.Files[0].Path, m.Files[0].Name = long, long
	if err := os.Remove(mf); err != nil {
		t.Fatalf("making room for the edited manifest: %v", err)
	}
	if err := m.Save(mf); err != nil {
		t.Fatalf("saving the edited manifest: %v", err)
	}

	diffs, err := audit.Verify(context.Background(), out, m, filepath.Base(mf))
	if err != nil {
		t.Fatalf("verifying: %v", err)
	}
	for _, d := range diffs {
		if d.Path != long {
			continue
		}
		if d.Kind != audit.Unreadable || d.Got == "" {
			t.Errorf("a name the system would not look up was reported as %s with %q, not as unreadable with the reason", d.Kind, d.Got)
		}
		return
	}
	t.Fatalf("the edited entry was not reported at all: %v", diffs)
}

// cancellingReader stands for a large file. It hands out bytes for ever and
// calls the cancellation the first time it is read from.
type cancellingReader struct {
	cancel func()
	reads  int
}

func (r *cancellingReader) Read(p []byte) (int, error) {
	r.reads++
	if r.reads == 1 {
		r.cancel()
	}
	return len(p), nil
}

// Ctrl+C in the middle of a large file stops the reading of it. The pool asked
// only between files, so stopping verify meant waiting out the largest one
// (R04). One read after the cancellation and no more: the question is put
// before every buffer.
func TestHashingAListedFileStopsPartWayWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := &cancellingReader{cancel: cancel}

	_, err := audit.HashAtMost(ctx, r, 64<<20, sha256.New(), make([]byte, 256<<10))
	if !errors.Is(err, context.Canceled) {
		t.Errorf("a cancelled read came back with %v, not the cancellation", err)
	}
	if r.reads != 1 {
		t.Errorf("the file was read %d times after Ctrl+C had landed on the first read, and 256 is all of it", r.reads)
	}
}

// endlessReader stands for a file somebody keeps appending to. It refuses to go
// past a point no reasonable limit reaches, so a read with no limit ends with
// that refusal rather than never.
type endlessReader struct{ read int64 }

func (r *endlessReader) Read(p []byte) (int, error) {
	if r.read > 64<<20 {
		return 0, errors.New("read past any limit this guard sets")
	}
	r.read += int64(len(p))
	return len(p), nil
}

// No more is read than asked. A file that went on growing was read until it
// stopped (R04), and a listed file is asked for its recorded size and one byte
// more - the byte that says it is longer.
func TestHashingAListedFileReadsNoMoreThanItIsAskedFor(t *testing.T) {
	const limit = 1000 + 1
	r := &endlessReader{}
	read, err := audit.HashAtMost(context.Background(), r, limit, io.Discard, make([]byte, 256<<10))
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	if read != limit || r.read != limit {
		t.Errorf("asked for %d B, it took %d and the file handed out %d", limit, read, r.read)
	}
}
