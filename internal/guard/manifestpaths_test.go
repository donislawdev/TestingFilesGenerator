package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// A manifest is an input, not a record we can trust.
//
// The writing side has been careful about this from early on: checkFileName
// refuses a name carrying a path, FuzzNameTemplate throws generated templates
// at it, and docs/SECURITY.md section 2.4 writes the rule down. All of it is
// about the path a run writes to.
//
// The reading side had none of it. "verify" and "cleanup" take a manifest
// somebody hands them - it travels with a fixture set, it arrives in a pull
// request, it gets edited by hand - and they resolved every path in it against
// the output directory without asking whether the result was still inside.
//
// Measured on 2026-08-03, before this guard existed:
//
//	"path": "../VICTIM.txt"  +  tfg cleanup m.json --yes --force
//	-> 1 file(s) removed from ...\outdir      exit 0      VICTIM.txt was gone
//
// Exit code zero, and a sentence naming the directory the file was not in.
// That is untouchable rule 7 broken by the one input nobody was checking.

// escapingManifest writes a manifest whose single entry points outside dir.
// The hash is deliberately wrong, which is the harder case: without --force it
// reads as "changed", and --force is documented as the way to remove exactly
// those. So the file is reachable by an ordinary invocation.
func escapingManifest(t *testing.T, dir, escapePath string, bytes int64) string {
	t.Helper()
	mf := filepath.Join(dir, "manifest.json")
	doc := fmt.Sprintf(`{
  "manifest_version": "1.0",
  "generated_at": "2026-08-03T00:00:00Z",
  "tool": {"name":"testing-files-generator","version":"0.0.0-dev","generators":{}},
  "run": {"id":"run_x","seed":0,"command":"tfg generate",
          "platform":{"os":"windows","arch":"amd64"},"complete":true},
  "summary": {"file_count":1,"total_bytes":%d,"materialized":1,
              "by_format":{},"by_expected":{}},
  "files": [
    {"id":"f_0001","path":%q,"name":"VICTIM.txt","materialized":true,
     "bytes":%d,"format":"txt","fidelity":"full",
     "hashes":{"sha256":"0000000000000000000000000000000000000000000000000000000000000000"},
     "seed":"00000000","generator":{"name":"txt","version":"1"},
     "determinism":"byte","expected":{"outcome":"unspecified"},
     "label_embedded":false}
  ]
}`, bytes, escapePath, bytes)
	if err := os.WriteFile(mf, []byte(doc), 0o644); err != nil {
		t.Fatalf("writing the manifest: %v", err)
	}
	return mf
}

// outsideVictim puts a file one level above the output directory and returns
// its path. It stands in for whatever the user keeps next to their fixtures.
func outsideVictim(t *testing.T) (outDir, victim string) {
	t.Helper()
	root := t.TempDir()
	outDir = filepath.Join(root, "out")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("making the output directory: %v", err)
	}
	victim = filepath.Join(root, "VICTIM.txt")
	if err := os.WriteFile(victim, []byte("the owner's own work\n"), 0o644); err != nil {
		t.Fatalf("writing the victim: %v", err)
	}
	return outDir, victim
}

func victimSurvives(t *testing.T, victim string) {
	t.Helper()
	body, err := os.ReadFile(victim)
	if err != nil {
		t.Fatalf("a file outside the output directory was removed: %v", err)
	}
	if string(body) != "the owner's own work\n" {
		t.Errorf("a file outside the output directory was modified: %q", body)
	}
}

// Untouchable rule 7 reaches the reading side too. The list is the authority
// for what may go, and a list that points outside the directory is not a list
// this tool acts on at all.
func TestCleanupNeverReachesOutsideTheDirectory(t *testing.T) {
	for _, escape := range []string{
		"../VICTIM.txt",
		"../../out/../VICTIM.txt",
		`..\VICTIM.txt`,
		"a/../../VICTIM.txt",
	} {
		t.Run(escape, func(t *testing.T) {
			out, victim := outsideVictim(t)
			mf := escapingManifest(t, out, escape, 21)

			// The preview first. It is what a person reads before deciding, so
			// offering to remove something outside the directory is already the
			// defect even though nothing has gone yet.
			code, stdout, errOut := run(t, "cleanup", mf)
			if code == cli.ExitOK {
				t.Errorf("the preview accepted a manifest pointing outside the directory: exit %d\n%s%s", code, stdout, errOut)
			}
			if strings.Contains(stdout, "remove") {
				t.Errorf("the preview offered to remove a path that leaves the directory:\n%s", stdout)
			}
			victimSurvives(t, victim)

			// Then the real thing, with the flag that covers a file whose
			// content we do not recognise.
			code, _, errOut = run(t, "cleanup", mf, "--yes", "--force")
			victimSurvives(t, victim)

			if code != cli.ExitIO {
				t.Errorf("exit %d, expected %d - a manifest we cannot trust is a reading failure:\n%s", code, cli.ExitIO, errOut)
			}
			// Quoted the way the tool renders a path, so a backslash reads the
			// same here as it does on the screen.
			if !strings.Contains(errOut, strconv.Quote(escape)) {
				t.Errorf("the refusal does not name the entry that caused it:\n%s", errOut)
			}
			if !strings.Contains(errOut, "outside") {
				t.Errorf("the refusal does not say what is wrong with it:\n%s", errOut)
			}
		})
	}
}

// The same manifest, read by the other command. It removes nothing, so the
// harm is smaller and real: it reports the size of a file outside the
// directory, and it would call a directory sound on the strength of files that
// are not in it.
func TestVerifyNeverReachesOutsideTheDirectory(t *testing.T) {
	out, victim := outsideVictim(t)
	mf := escapingManifest(t, out, "../VICTIM.txt", 21)

	code, stdout, errOut := run(t, "verify", mf)
	if code != cli.ExitIO {
		t.Errorf("exit %d, expected %d:\n%s%s", code, cli.ExitIO, stdout, errOut)
	}
	if strings.Contains(stdout, "matches") {
		t.Errorf("verify called the directory sound using a file outside it:\n%s", stdout)
	}
	// The size of a file outside the directory is not ours to report. Finding
	// it in the output means the entry was resolved and read.
	if strings.Contains(stdout+errOut, "21 B") {
		t.Errorf("verify reported the size of a file outside the directory:\n%s%s", stdout, errOut)
	}
	victimSurvives(t, victim)
}

// An absolute path is refused for the same reason and does not rely on
// filepath.Join happening to mangle it. Measured on 2026-08-03: on Windows it
// came out as "unreachable" rather than as a refusal, which is the right
// outcome arrived at by accident - and an accident holds only until the day
// somebody points the tool at a directory on the same volume.
// Asserted on the reason rather than on the exit code, and that distinction is
// the whole of this guard. The first version checked for exit 5 alone and
// mutation reported NOT CAUGHT: with the volume check disabled the path becomes
// a name no disk has, the file is reported unreachable, cleanup leaves it and
// ends with 5 anyway. The code was right by accident while the manifest was
// being resolved exactly as before.
func TestAManifestPathIsNeverAbsolute(t *testing.T) {
	out, victim := outsideVictim(t)
	mf := escapingManifest(t, out, filepath.ToSlash(victim), 21)

	code, _, errOut := run(t, "cleanup", mf, "--yes", "--force")
	victimSurvives(t, victim)

	if code != cli.ExitIO {
		t.Errorf("exit %d, expected %d - an absolute path in a manifest is refused, not resolved:\n%s", code, cli.ExitIO, errOut)
	}
	if !strings.Contains(errOut, "cannot be read as a manifest") {
		t.Errorf("the manifest was acted on rather than turned away - an absolute path has to be refused at the door, not discovered to be unreachable later:\n%s", errOut)
	}
	if !strings.Contains(errOut, "outside") {
		t.Errorf("the refusal does not say what is wrong with the path:\n%s", errOut)
	}
}

// A path with folders in it is legitimate and has to keep working. A run that
// groups its output into subdirectories writes exactly this, and audit walks
// the tree recursively to match it.
//
// Without this case the guard above would be satisfied by refusing every path
// containing a separator, which would break grouped output and nobody would
// find out until somebody used it.
func TestAManifestPathMayStillNameASubdirectory(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "out")
	if err := os.MkdirAll(filepath.Join(out, "invoices"), 0o755); err != nil {
		t.Fatalf("making the tree: %v", err)
	}
	inner := filepath.Join(out, "invoices", "a.txt")
	if err := os.WriteFile(inner, []byte("x\n"), 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}

	// Wrong hash on purpose, so this reaches the same --force path as the
	// escaping cases above and the only difference between them is the path.
	mf := escapingManifest(t, out, "invoices/a.txt", 2)

	code, _, errOut := run(t, "cleanup", mf, "--yes", "--force")
	if code != cli.ExitOK {
		t.Fatalf("a path inside the directory was refused: exit %d\n%s", code, errOut)
	}
	if _, err := os.Stat(inner); err == nil {
		t.Error("the file inside a subdirectory was not removed, so the check refuses more than it should")
	}
}
