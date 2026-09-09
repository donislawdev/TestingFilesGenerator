package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/oracle"
)

// The structural checker is found by a binary that was built somewhere else.
//
// What this defends against was measured on 2026-09-09 and had been true for as
// long as this project has run its suite in a container. oracle.Strict looks for
// its script beside its own source file, and that path is compiled in - so a
// binary cross compiled on Windows and run on Linux looked for a Windows
// directory, found nothing, and answered "not available" for every structural
// check. Nothing went red, because an unavailable checker is a skip and a skip
// reads like a check that ran.
//
// It surfaced only when two guards started refusing to pass on zero checks, and
// the blind spot was older than both of them. The first diagnosis was wrong in
// a way worth recording: the message said "0 file(s) decoded strictly by
// Python", so the container was given an image carrying Python, and nothing
// changed. The message named the outcome. A probe printing what the function
// returned named the cause.
//
// This asks the second way directly, because the compiled in path is chosen
// first on any machine that built the binary - including the one running this
// guard - so asking through Strict would prove the first way and never reach
// the second.
func TestTheStructuralCheckerIsFoundByABinaryBuiltSomewhereElse(t *testing.T) {
	root := t.TempDir()
	want := filepath.Join(root, "internal", "oracle", "strict.py")
	if err := os.MkdirAll(filepath.Dir(want), 0o755); err != nil {
		t.Fatalf("laying out a tree to search: %v", err)
	}
	if err := os.WriteFile(want, []byte("print('OK')\n"), 0o600); err != nil {
		t.Fatalf("writing a stand in for the checker: %v", err)
	}

	// Where a test binary actually runs from: a package directory, some way
	// down the tree from the script.
	from := filepath.Join(root, "internal", "guard")
	if err := os.MkdirAll(from, 0o755); err != nil {
		t.Fatalf("laying out the directory to search from: %v", err)
	}

	got, ok := oracle.StrictScriptUnder(from)
	if !ok {
		t.Fatalf("the checker was not found by walking up from %s.\n"+
			"Reason: a binary built on one machine and run on another cannot use the path\n"+
			"compiled into it, so this is the only way left. Without it every structural\n"+
			"check in a container is skipped and the suite still reports success.", from)
	}
	if got != want {
		t.Errorf("the walk found %q and the checker is at %q", got, want)
	}

	// The other half, and it is not decoration: a walk with no stopping
	// condition climbs to the root of the disk and answers about a file that
	// belongs to something else.
	if p, ok := oracle.StrictScriptUnder(t.TempDir()); ok {
		t.Errorf("a tree with no checker in it answered %q.\n"+
			"Reason: the walk has to give up at the top rather than keep going and pick up\n"+
			"whatever it finds outside the tree it was asked about.", p)
	}
}

// The way that works on the machine that built the binary still works.
//
// A control, and the reason it is here: the guard above lays out its own tree,
// so it would pass unchanged if the real lookup were broken outright. This one
// asks the real thing, from the real repository.
//
// It steps aside when there is no interpreter to find, because then there is
// nothing for the lookup to be available for and a red result would be a
// sentence about the machine rather than about this code. The name looked for
// is "python" and not "python3", which is the name oracle.Strict itself looks
// for - asking a different question here would make this guard agree with a
// machine the checker cannot actually use.
func TestTheStructuralCheckerIsAvailableWhereItWasBuilt(t *testing.T) {
	if _, err := exec.LookPath("python"); err != nil {
		t.Skip("no python on this machine, so there is nothing for the checker to be")
	}
	res := oracle.Strict("txt", filepath.Join(t.TempDir(), "nothing.txt"))
	if !res.Available {
		t.Errorf("python is on this machine and the structural checker is still not available.\n" +
			"Reason: the script sits beside its own source here, so the compiled in path should\n" +
			"find it. If this is red, the lookup is broken rather than the machine unusual.")
	}
}
