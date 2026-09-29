package guard

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
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
// without the privilege, and plantLink says so rather than passing quietly -
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
	plantLink(t, root, filepath.Join(out, "jn"))
	return out, victim
}

// plantLink makes link lead to target, or says out loud why the case cannot
// run.
//
// Creating a symbolic link needs a privilege on Windows that an ordinary
// account does not have. Off CI that is a skip, which -v prints. On CI it is a
// failure, because the Windows job runs go test without -v, and there a skip
// reads exactly like a pass. On 2026-09-29 the guard asked to prove that a
// directory reached through a link still works after O252 was green on all
// three systems, and nothing said whether it had run on Windows at all. The
// same guard skipped on the machine that wrote it, with "A required privilege
// is not held by the client". The first CI run after this helper was green on
// Windows, and GitHub sets CI on every job, so the runner makes links.
func plantLink(t *testing.T, target, link string) {
	t.Helper()
	plantLinkWith(t, os.Symlink, os.Getenv("CI"), target, link)
}

// linkReporter is the part of testing.T that plantLinkWith speaks to, so that
// its decision can be asked of a recorder rather than of a test that really
// stops.
type linkReporter interface {
	Helper()
	Fatalf(format string, args ...any)
	Skipf(format string, args ...any)
}

// plantLinkWith is plantLink with the two things it reads from the world
// passed in. A real testing.T stops at Fatalf and Skipf and a recorder does
// not, so every branch returns on its own.
func plantLinkWith(t linkReporter, symlink func(oldname, newname string) error, ci, target, link string) {
	t.Helper()
	err := symlink(target, link)
	if err == nil {
		return
	}
	if !linkWantsAPrivilege(err) {
		t.Fatalf("planting a symbolic link %s to %s: %v", link, target, err)
		return
	}
	if !linkCasesMaySkip(ci) {
		t.Fatalf("this host does not allow creating a symbolic link (%v), so this case did not run.\n"+
			"On CI a case that did not run must not look like one that passed. "+
			"Give the job the privilege, or build this case with something that needs none.", err)
		return
	}
	t.Skipf("this host does not allow creating a symbolic link (%v), so this case did not run", err)
}

// errPrivilegeNotHeld is what Windows answers an account that may not create a
// symbolic link. By number, because the syscall package does not name it and
// its text is written in the system's own language.
const errPrivilegeNotHeld = syscall.Errno(1314) // ERROR_PRIVILEGE_NOT_HELD

// linkWantsAPrivilege says whether a link was refused for want of a privilege.
// Asked of the error inside rather than of the text: the text of an
// os.LinkError carries both paths, and a path is not evidence.
func linkWantsAPrivilege(err error) bool {
	return errors.Is(err, errPrivilegeNotHeld) || errors.Is(err, fs.ErrPermission)
}

// linkCasesMaySkip says whether a case built on a symbolic link may skip when
// the host refuses to make one.
//
// A function of its input for the reason screensAreCompared is one: a skipped
// test is a green test, so a condition widened by accident stops the case from
// being checked without one thing going red. Asked this way, the CI answer is
// tested on a machine that is not CI.
func linkCasesMaySkip(ci string) bool {
	return ci == ""
}

// linkRecorder answers for testing.T in TestACaseBuiltOnALinkSkipsOnlyOffCI.
type linkRecorder struct{ failed, skipped bool }

func (r *linkRecorder) Helper()               {}
func (r *linkRecorder) Fatalf(string, ...any) { r.failed = true }
func (r *linkRecorder) Skipf(string, ...any)  { r.skipped = true }

func (r *linkRecorder) outcome() string {
	switch {
	case r.failed && r.skipped:
		return "failed and skipped"
	case r.failed:
		return "failed"
	case r.skipped:
		return "skipped"
	}
	return "made"
}

// What this defends. A case built on a symbolic link runs on every CI system
// or fails there. It never skips there, because nothing would show it did.
// And only a refused privilege is a skip anywhere - any other refusal is a
// failure, whatever the paths in it say.
//
// Asked of plantLinkWith rather than of the predicate alone (outside review of
// #151): a guard on linkCasesMaySkip stays green if plantLink stops asking it.
func TestACaseBuiltOnALinkSkipsOnlyOffCI(t *testing.T) {
	refused := &os.LinkError{Op: "symlink", Old: "target", New: "link", Err: errPrivilegeNotHeld}
	denied := &os.LinkError{Op: "symlink", Old: "target", New: "link", Err: fs.ErrPermission}
	// The paths say "privilege" and the error inside does not.
	other := &os.LinkError{Op: "symlink", Old: "privilege", New: "privilege", Err: fs.ErrExist}

	cases := []struct {
		err  error
		ci   string
		want string
		why  string
	}{
		{nil, "true", "made", "a link that was made needs nothing said about it"},
		{refused, "", "skipped", "off CI a Windows account without the privilege is not a defect, and -v prints the skip"},
		{refused, "true", "failed", "on CI the Windows job runs without -v, so a skip there reads as a pass"},
		{denied, "", "skipped", "a Unix permission refusal is the same case as the Windows privilege"},
		{denied, "true", "failed", "on CI no refusal may hide a case"},
		{other, "", "failed", "a refusal that is not about the privilege is a defect, whatever the paths say"},
		{other, "true", "failed", "a refusal that is not about the privilege is a defect on CI too"},
	}
	for _, c := range cases {
		r := &linkRecorder{}
		plantLinkWith(r, func(string, string) error { return c.err }, c.ci, "target", "link")
		if got := r.outcome(); got != c.want {
			t.Errorf("with %v and CI=%q the case %s, want %s.\n"+
				"Reason: %s.\n"+
				"What to do: a skip is green, so a wider skip stops the link cases from being checked "+
				"without anything going red. Narrow it back.",
				c.err, c.ci, got, c.want, c.why)
		}
	}
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

	// Only what the preview printed matters here. Its exit code is checked by
	// the run below, which is the one that would do the damage.
	_, stdout, _ := run(t, "cleanup", mf)
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
	_, stdout, _ := run(t, "cleanup", mf)
	if strings.Contains(stdout, "remove") {
		t.Errorf("the preview offered to remove a path that leaves the directory through a link:\n%s", stdout)
	}
	victimSurvives(t, victim)

	code, _, errOut := run(t, "cleanup", mf, "--yes", "--force")
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
	plantLink(t, real, linked)

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
