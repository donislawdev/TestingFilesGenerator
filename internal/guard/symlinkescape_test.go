package guard

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// The containment check on a manifest path is textual, and a link is how a
// textual check is got around.
//
// "jn/VICTIM.txt" holds no climb, so it passes every reading of the string.
// Resolved against a directory holding a link called "jn", it lands wherever
// that link points. Measured on 2026-08-03, after the textual check was in
// place:
//
//	out\jn -> the directory above out
//	"path": "jn/VICTIM.txt"  +  tfg cleanup --yes --force
//	-> 1 file(s) removed from ...\out      exit 0      VICTIM.txt was gone
//
// docs/SECURITY.md section 2.4 already carried the rule this breaks - "check
// after resolving the path, not before" - marked as brought in from another
// project and NOT VERIFIED here. It was right and it was not verified.
//
// The fixture is built with os.Symlink rather than a junction so that the same
// guard runs on all three systems in the matrix. Windows refuses to create one
// without the privilege, and the guard says so rather than passing quietly -
// a skip that looks like a pass is how this class of defect survives.
func linkedEscape(t *testing.T) (out, victim string) {
	t.Helper()
	root := t.TempDir()
	out = filepath.Join(root, "out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatalf("making the output directory: %v", err)
	}
	victim = filepath.Join(root, "VICTIM.txt")
	if err := os.WriteFile(victim, []byte("the owner's own work\n"), 0o644); err != nil {
		t.Fatalf("writing the victim: %v", err)
	}
	if err := os.Symlink(root, filepath.Join(out, "jn")); err != nil {
		t.Skipf("this system will not create a link here, so the escape cannot be built: %v", err)
	}
	return out, victim
}

// A junction is the other redirection Windows offers, and it is the one that
// matters most here: creating it needs no privilege at all, while a symbolic
// link does.
//
// It had to be its own case because the two are not interchangeable to the
// code that judges them. Measured on 2026-08-03, after the resolving check was
// already in place and the symbolic link guard above was green:
//
//	symbolic link  Lstat says ModeSymlink, EvalSymlinks resolves it
//	junction       Lstat says neither, EvalSymlinks returns it unchanged
//	               and reports no error
//
// So cleanup --yes --force still removed a file above the output directory
// through a junction, with exit code 0, while the test suite said the escape
// was closed. The guard was passing without reaching the case.
func junctionEscape(t *testing.T) (out, victim string) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip("a junction is a Windows reparse point")
	}
	root := t.TempDir()
	out = filepath.Join(root, "out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatalf("making the output directory: %v", err)
	}
	victim = filepath.Join(root, "VICTIM.txt")
	if err := os.WriteFile(victim, []byte("the owner's own work\n"), 0o644); err != nil {
		t.Fatalf("writing the victim: %v", err)
	}
	if err := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(out, "jn"), root).Run(); err != nil {
		t.Skipf("this system will not make a junction here: %v", err)
	}
	return out, victim
}

func TestCleanupDoesNotFollowAJunctionOutOfTheDirectory(t *testing.T) {
	out, victim := junctionEscape(t)
	mf := escapingManifest(t, out, "jn/VICTIM.txt", 21)

	code, stdout, _ := run(t, "cleanup", mf)
	if strings.Contains(stdout, "remove") {
		t.Errorf("the preview offered to remove a path that leaves through a junction:\n%s", stdout)
	}
	victimSurvives(t, victim)

	code, _, errOut := run(t, "cleanup", mf, "--yes", "--force")
	victimSurvives(t, victim)
	if code != cli.ExitIO {
		t.Errorf("exit %d, expected %d:\n%s", code, cli.ExitIO, errOut)
	}
}

func TestVerifyDoesNotFollowAJunctionOutOfTheDirectory(t *testing.T) {
	out, victim := junctionEscape(t)
	mf := escapingManifest(t, out, "jn/VICTIM.txt", 21)

	code, stdout, errOut := run(t, "verify", mf)
	if code != cli.ExitIO {
		t.Errorf("exit %d, expected %d:\n%s%s", code, cli.ExitIO, stdout, errOut)
	}
	victimSurvives(t, victim)
}

func TestCleanupDoesNotFollowALinkOutOfTheDirectory(t *testing.T) {
	out, victim := linkedEscape(t)
	mf := escapingManifest(t, out, "jn/VICTIM.txt", 21)

	// The preview first. Offering to remove it is already the defect.
	code, stdout, errOut := run(t, "cleanup", mf)
	if strings.Contains(stdout, "remove") {
		t.Errorf("the preview offered to remove a path that leaves the directory through a link:\n%s", stdout)
	}
	victimSurvives(t, victim)

	code, _, errOut = run(t, "cleanup", mf, "--yes", "--force")
	victimSurvives(t, victim)

	if code != cli.ExitIO {
		t.Errorf("exit %d, expected %d:\n%s", code, cli.ExitIO, errOut)
	}
	if !strings.Contains(errOut, "outside") {
		t.Errorf("the refusal does not say what is wrong:\n%s", errOut)
	}
}

func TestVerifyDoesNotFollowALinkOutOfTheDirectory(t *testing.T) {
	out, victim := linkedEscape(t)
	mf := escapingManifest(t, out, "jn/VICTIM.txt", 21)

	code, stdout, errOut := run(t, "verify", mf)
	if code != cli.ExitIO {
		t.Errorf("exit %d, expected %d:\n%s%s", code, cli.ExitIO, stdout, errOut)
	}
	if strings.Contains(stdout, "matches") {
		t.Errorf("verify called the directory sound using a file reached through a link out of it:\n%s", stdout)
	}
	victimSurvives(t, victim)
}

// The other direction, and it matters more here than anywhere else in this
// project: people put fixtures on a linked path on purpose. A workspace
// mounted somewhere else, a scratch disk, a home directory that is itself a
// link. Refusing every link would break all of that.
//
// So the question is never "is a link involved" but "does the path still land
// inside the directory once the links have been followed".
func TestADirectoryReachedThroughALinkStillWorks(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatalf("making the directory: %v", err)
	}
	linked := filepath.Join(root, "linked")
	if err := os.Symlink(real, linked); err != nil {
		t.Skipf("this system will not create a link here: %v", err)
	}

	// Generate through the link, then verify and clean up through it. Every
	// path involved resolves inside, so all three have to behave normally.
	if code, _, errOut := run(t,
		"generate", "--format", "txt", "--size", "1kb", "--count", "3",
		"--out", linked); code != cli.ExitOK {
		t.Fatalf("generating into a linked directory gave %d:\n%s", code, errOut)
	}

	mf := filepath.Join(linked, "manifest.json")
	if code, _, errOut := run(t, "verify", mf); code != cli.ExitOK {
		t.Errorf("verify through a linked directory gave %d:\n%s", code, errOut)
	}
	if code, _, errOut := run(t, "cleanup", mf, "--yes"); code != cli.ExitOK {
		t.Errorf("cleanup through a linked directory gave %d:\n%s", code, errOut)
	}
	left, err := os.ReadDir(real)
	if err != nil {
		t.Fatalf("reading the real directory: %v", err)
	}
	for _, e := range left {
		if e.Name() != "manifest.json" {
			t.Errorf("a generated file survived cleanup through the link: %s", e.Name())
		}
	}
}
