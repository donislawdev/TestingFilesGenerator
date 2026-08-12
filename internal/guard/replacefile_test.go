package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// Replacing somebody's file keeps the mode it had.
//
// The defect this closes was in the tool before the window existed, and it is
// the reason the whole write path got looked at. "tfg recipe fmt -w" replaced
// a recipe through a rename, and a rename moves the file it renames - mode and
// all - so what survived was the temporary file's mode rather than the
// original's. Measured on Linux with tools/probes/atomic-replace: a recipe
// somebody had made private at 0600 came back at 0644, readable by everyone
// on the machine.
//
// It skips on Windows and says so, rather than passing there. Windows has no
// permission bits, so the question this asks cannot be put to it - and a guard
// that reports success for a question it did not ask is worse than one that
// stays away.
func TestReplacingAFileKeepsTheModeItHad(t *testing.T) {
	if os.Getenv("GOOS") == "windows" || filepath.Separator == '\\' {
		t.Skip("Windows has no permission bits, so there is no mode to keep - measured on Linux instead")
	}

	path := filepath.Join(t.TempDir(), "recipe.yaml")
	if err := os.WriteFile(path, []byte("version: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := core.ReplaceFile(path, []byte("version: 2\n")); err != nil {
		t.Fatalf("replacing a writable file failed: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("the file was %v before and is %v after, so a private recipe came back readable by others",
			os.FileMode(0o600), got)
	}
}

// A file marked read only is left alone, on every system.
//
// Measured on both, because they disagree: a rename asks for permission on the
// DIRECTORY rather than on the file, so read only stops the replacement on
// Windows and does not stop it on Linux. Refusing everywhere is the rule this
// project already applies to file names - a recipe that behaves differently on
// a colleague's machine is worse than one refused on all of them.
//
// Both halves are checked, because refusing while having already written is
// the failure that looks like success: the error is reported, and the file is
// gone anyway.
func TestAReadOnlyFileIsLeftAlone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "recipe.yaml")
	const was = "version: 1\n"
	if err := os.WriteFile(path, []byte(was), 0o444); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(path, 0o644)

	err := core.ReplaceFile(path, []byte("version: 2\n"))
	if err == nil {
		t.Fatal("a read only file was replaced without complaint")
	}

	// The refusal says what to do about it, which is D6 rather than politeness.
	for _, want := range []string{"read only", "writable"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %q. It says: %s", want, err)
		}
	}

	now, readErr := os.ReadFile(path)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(now) != was {
		t.Errorf("the file was refused and changed anyway: %q", string(now))
	}
}

// Nothing is left beside the file, whatever happened.
//
// The half written copy sits next to the target rather than in the system
// temporary directory, because a rename across volumes is not one operation.
// That puts it in somebody's repository, so it has to go - on the way out of
// every path, not only the happy one. The version this replaced cleaned up
// after a failed rename and not after a failed write.
func TestReplacingLeavesNoHalfWrittenCopyBehind(t *testing.T) {
	for _, c := range []struct {
		what  string
		build func(t *testing.T, dir string) string
	}{
		{"a replacement that worked", func(t *testing.T, dir string) string {
			path := filepath.Join(dir, "fine.yaml")
			if err := os.WriteFile(path, []byte("version: 1\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			return path
		}},
		{"one refused before anything was created", func(t *testing.T, dir string) string {
			path := filepath.Join(dir, "locked.yaml")
			if err := os.WriteFile(path, []byte("version: 1\n"), 0o444); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { os.Chmod(path, 0o644) })
			return path
		}},
		// The one that matters, and the one the first version of this guard
		// did not have: the copy is written and THEN the rename fails. Without
		// a case that gets that far, this test passed on a writer that never
		// cleaned up, because the only failure it tried refuses before there
		// is anything to clean.
		//
		// A directory with something in it is the portable way to make a
		// rename fail. Holding the target open does it on Windows and not on
		// Linux, and a full disk cannot be arranged in a test.
		{"one where the rename failed after the copy was written", func(t *testing.T, dir string) string {
			path := filepath.Join(dir, "occupied")
			if err := os.MkdirAll(filepath.Join(path, "child"), 0o755); err != nil {
				t.Fatal(err)
			}
			return path
		}},
	} {
		t.Run(c.what, func(t *testing.T) {
			dir := t.TempDir()
			path := c.build(t, dir)

			_ = core.ReplaceFile(path, []byte("version: 2\n"))

			leftovers, err := filepath.Glob(filepath.Join(dir, "*"+".tfg-writing"))
			if err != nil {
				t.Fatal(err)
			}
			if len(leftovers) != 0 {
				t.Errorf("after %s there is still %v beside the file", c.what, leftovers)
			}
		})
	}
}

// The case above is only worth having if the middle of it is really reached.
//
// A test that arranges a failure it never triggers reports a clean directory
// because nothing ever happened in it - which is the same shape as a guard
// that passes without reaching the code. So this asks the arrangement itself:
// does replacing onto a non empty directory actually fail.
func TestTheRenameFailureThatGuardIsBuiltOnReallyHappens(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "occupied")
	if err := os.MkdirAll(filepath.Join(path, "child"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := core.ReplaceFile(path, []byte("version: 2\n")); err == nil {
		t.Fatal("replacing onto a non empty directory succeeded, so the guard above never reaches the cleanup it exists for")
	}
}

// The content that arrives is the content that was asked for.
//
// The plain case, and it is here because everything else in this file is about
// a refusal - a writer that refuses correctly and writes the wrong bytes would
// pass all of them.
func TestReplacingPutsTheContentThatWasAskedFor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "recipe.yaml")
	if err := os.WriteFile(path, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	const want = "version: 2\ntargets: []\n"
	if err := core.ReplaceFile(path, []byte(want)); err != nil {
		t.Fatalf("replacing failed: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("the file holds %q and should hold %q", string(got), want)
	}
}
