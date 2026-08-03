package guard

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// What the machine underneath is allowed to change about a run, and what it is
// not. Every case here was measured on 2026-08-04 while going through the
// environment questions the owner raised.

// A directory somebody cannot write in is a permission problem and says so.
//
// It used to say "manifest.json already exists ... it is the only record of
// what an earlier run wrote" about an empty directory, because every failure
// to claim the manifest name was reported as a collision. That sends a person
// looking for a run that never happened. A second message about failing to
// write the manifest followed it, for one fault.
func TestADirectoryWeCannotWriteInIsAPermissionProblem(t *testing.T) {
	if runtime.GOOS == "windows" {
		// os.Chmod on Windows only toggles the read only bit, which does not
		// stop a file being created in a directory. Denying it needs an ACL,
		// which is not something a test should be installing. Measured by hand
		// there instead, and this runs on the other two systems in the matrix.
		t.Skip("a directory that refuses writes needs an ACL on Windows")
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "ro")
	if err := os.Mkdir(out, 0o555); err != nil {
		t.Fatalf("making a directory nobody can write in: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(out, 0o755) })

	code, stdout, errOut := run(t, "generate", "--format", "txt", "--size", "1kb", "--out", out)

	if code == cli.ExitOK {
		t.Fatal("a run into a directory it cannot write in reported success")
	}
	if strings.Contains(errOut, "already exists") {
		t.Errorf("a permission problem is reported as a name that is taken:\n%s", errOut)
	}
	if n := strings.Count(errOut, "tfg: "); n != 1 {
		t.Errorf("one fault produced %d messages:\n%s", n, errOut)
	}
	if stdout != "" {
		t.Errorf("a failed run wrote to stdout:\n%s", stdout)
	}
}

// The time the manifest records does not move with the clock of the machine.
//
// It is written as UTC with the Z on it, so a run in Tokyo and a run in
// California describe the same instant the same way. A local time would make
// two manifests of the same run look like different runs, and a reader
// comparing them has no way to tell which.
func TestTheRecordedTimeDoesNotDependOnTheLocalZone(t *testing.T) {
	dir := t.TempDir()
	code, stdout, errOut := run(t,
		"generate", "--format", "txt", "--size", "1kb", "--json", "--out", dir)
	if code != cli.ExitOK {
		t.Fatalf("exit %d:\n%s", code, errOut)
	}
	var m struct {
		GeneratedAt string `json:"generated_at"`
	}
	if err := json.Unmarshal([]byte(stdout), &m); err != nil {
		t.Fatalf("the manifest did not parse: %v", err)
	}
	if !strings.HasSuffix(m.GeneratedAt, "Z") {
		t.Errorf("generated_at is %q - it has to be UTC, or two machines describe one run differently", m.GeneratedAt)
	}
	if _, err := time.Parse(time.RFC3339, m.GeneratedAt); err != nil {
		t.Errorf("generated_at is not a time anything can read: %v", err)
	}
}

// A recipe is UTF-8 and anything else is refused rather than guessed at.
//
// Measured on 2026-08-04: a recipe saved as cp1250 with Polish letters in a
// name was accepted with exit 0, and the file arrived called "za" followed by
// four replacement characters. The manifest agreed with the disk, so nothing
// downstream noticed - the tool had simply produced a different name from the
// one that was asked for, silently. Notepad on Windows still offers ANSI, and
// this tool is aimed at testers on Windows.
func TestARecipeThatIsNotUTF8IsRefused(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "r.yaml")

	// "zażółć.txt" in cp1250 - valid text in that encoding, not valid UTF-8.
	body := []byte("version: 1\ntargets:\n  - id: t\n    format: txt\n    size: 1kb\n    name: za")
	body = append(body, 0xbf, 0xf3, 0xb3, 0xe6) // ż ó ł ć
	body = append(body, []byte(".txt\n")...)
	if utf8.Valid(body) {
		t.Fatal("the fixture is valid UTF-8, so it cannot show what it was written to show")
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("writing: %v", err)
	}

	code, _, errOut := run(t, "validate", path)
	if code == cli.ExitOK {
		t.Fatal("a recipe that is not UTF-8 was accepted, so a name comes back with replacement characters in it")
	}
	if !strings.Contains(errOut, "UTF-8") {
		t.Errorf("the refusal does not say what is wrong with the file:\n%s", errOut)
	}
	if !strings.Contains(strings.ToLower(errOut), "save") && !strings.Contains(errOut, "encoding") {
		t.Errorf("the refusal does not say what to do about it:\n%s", errOut)
	}
}

// A name is judged the same way on every system.
//
// The rule about separators says so in as many words - a recipe that only
// works on the machine it was written on is not portable - and the rule beside
// it about absolute paths was asking the local machine. Measured on
// 2026-08-04: "a:b.txt" was accepted on Linux and refused on Windows from one
// recipe, because filepath.VolumeName answers for the system it runs on.
//
// The name below is the same on every run of this test, so a disagreement
// between two systems shows up as one of them failing rather than as nobody
// noticing.
func TestANameIsJudgedTheSameWayOnEverySystem(t *testing.T) {
	for _, name := range []string{"a:b.txt", "C:x.txt", "z:", "AB:c.txt"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			code, _, errOut := run(t,
				"generate", "--format", "txt", "--size", "1kb",
				"--name", name, "--dry-run", "--out", dir)
			if code == cli.ExitOK {
				t.Errorf("the name %q was accepted here, and a machine with drive letters refuses it - so one recipe gives two answers", name)
			}
			_ = errOut
		})
	}

	// The other direction: a colon is not the only thing with a letter in
	// front of it, and an ordinary name has to survive.
	for _, name := range []string{"ab.txt", "a-b.txt", "a.b.txt"} {
		t.Run("still works "+name, func(t *testing.T) {
			dir := t.TempDir()
			if code, _, errOut := run(t,
				"generate", "--format", "txt", "--size", "1kb",
				"--name", name, "--dry-run", "--out", dir); code != cli.ExitOK {
				t.Errorf("an ordinary name was refused: exit %d\n%s", code, errOut)
			}
		})
	}
}

// And a recipe that is UTF-8 keeps working, with the letters intact all the
// way to the name on the disk.
func TestARecipeInUTF8KeepsItsLetters(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out")
	path := writeRecipe(t, dir, "version: 1\ntargets:\n  - id: t\n    format: txt\n    size: 1kb\n    name: za\u017c\u00f3\u0142\u0107.txt\noutput:\n  dir: "+filepath.ToSlash(out)+"\n")

	if code, _, errOut := run(t, "generate", path); code != cli.ExitOK {
		t.Fatalf("exit %d:\n%s", code, errOut)
	}
	entries, err := os.ReadDir(out)
	if err != nil {
		t.Fatalf("reading the output: %v", err)
	}
	found := false
	for _, e := range entries {
		if e.Name() == "za\u017c\u00f3\u0142\u0107.txt" {
			found = true
		}
		if strings.ContainsRune(e.Name(), '\uFFFD') {
			t.Errorf("the name came back with a replacement character in it: %q", e.Name())
		}
	}
	if !found {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("the file is not there under the name that was asked for: %v", names)
	}
}
