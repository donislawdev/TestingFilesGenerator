package guard

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// requireNewNamesThroughLink stops a case that writes into a directory reached
// through a link, on a host whose file system refuses every new name there.
//
// Measured on 2026-11-18 (O286, tools/probes/linklock): on Windows, a process
// that runs under the file system write virtualization of a packaged app asks
// for a new name in a directory under AppData reached through a symbolic link
// and is told ERROR_FILE_EXISTS, while nothing holds that name and the same
// name in the real directory is free. Every run claims its lock that way, so a
// run through the link is refused with "another run is already writing" and
// the case reads as a regression of ours. The same request succeeds outside
// that virtualization, and inside it when the directory is outside AppData.
//
// The system is asked directly rather than through core.CreateNew, so that a
// refusal of ours still reaches the case and goes red there. A refusal of a
// name nothing holds is the host: a skip off CI and a failure on it, by the
// rule plantLink follows. Any other refusal is a failure everywhere.
func requireNewNamesThroughLink(t *testing.T, dir string) {
	t.Helper()
	requireNewNamesThroughLinkWith(t, claimAndGiveBack, os.Getenv("CI"), dir)
}

// claimAndGiveBack takes a new name the way a run's lock is taken, then gives
// it back, so the case finds the directory as it left it.
func claimAndGiveBack(path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return err
	}
	return errors.Join(f.Close(), os.Remove(path))
}

func requireNewNamesThroughLinkWith(t linkReporter, claim func(string) error, ci, dir string) {
	t.Helper()
	path := filepath.Join(dir, "tfg-new-name-probe")
	err := claim(path)
	if err == nil {
		return
	}
	if !hostRefusesAFreeName(err, path) {
		t.Fatalf("taking a new name through the link at %s: %v", path, err)
		return
	}
	why := "this host refuses a new name in a directory reached through a link (%v) while nothing holds it, " +
		"so no run can write there and this case did not run. Measured under the file system write " +
		"virtualization of a packaged app (O286) - run it from a terminal, or with TMP outside AppData"
	if !linkCasesMaySkip(ci) {
		t.Fatalf(why+".\nOn CI a case that did not run must not look like one that passed.", err)
		return
	}
	t.Skipf(why, err)
}

// hostRefusesAFreeName says whether the system answered "already there" for a
// name that nothing holds. A name somebody does hold is a real refusal.
func hostRefusesAFreeName(err error, path string) bool {
	if !errors.Is(err, fs.ErrExist) {
		return false
	}
	_, look := os.Lstat(path)
	return errors.Is(look, fs.ErrNotExist)
}

// What this defends. A host that refuses a free name through a link skips the
// two cases that write through one off CI and fails them on CI. A name that is
// really held, or any other refusal, fails everywhere - a skip there would
// hide a defect of ours behind the host.
func TestACaseWritingThroughALinkSkipsOnlyWhereTheHostRefusesAFreeName(t *testing.T) {
	free, taken := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(taken, "tfg-new-name-probe"), nil, 0o600); err != nil {
		t.Fatalf("taking the name: %v", err)
	}
	refused := &fs.PathError{Op: "open", Path: "x", Err: fs.ErrExist}
	denied := &fs.PathError{Op: "open", Path: "x", Err: fs.ErrPermission}

	cases := []struct {
		held bool
		err  error
		ci   string
		want string
		why  string
	}{
		{false, nil, "true", "made", "a name the host gives needs nothing said about it"},
		{false, refused, "", "skipped", "off CI a host that refuses a free name is not a defect of ours"},
		{false, refused, "true", "failed", "on CI no refusal may hide a case"},
		{true, refused, "", "failed", "a name that is really held is a real refusal, whatever the host"},
		{false, denied, "", "failed", "a refusal that is not about the name is a defect"},
	}
	for _, c := range cases {
		dir := free
		if c.held {
			dir = taken
		}
		r := &linkRecorder{}
		requireNewNamesThroughLinkWith(r, func(string) error { return c.err }, c.ci, dir)
		if got := r.outcome(); got != c.want {
			t.Errorf("with %v, the name held %v and CI=%q the case %s, want %s.\n"+
				"Reason: %s.\n"+
				"What to do: a skip is green, so a wider skip stops the cases that write through a link "+
				"from being checked without anything going red. Narrow it back.",
				c.err, c.held, c.ci, got, c.want, c.why)
		}
	}
}
