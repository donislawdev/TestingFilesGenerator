package guard

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/gui/text"
	"github.com/donislawdev/TestingFilesGenerator/internal/gui/window"
)

// What this defends. The window never offers to write into a directory it was
// not meant to write into: its own program directory or the root of a disk.
//
// Why it needed a guard of its own. destination_test.go and the first start in
// remembered_test.go stay green whatever this rule does, because under go test
// the working directory is the package directory and the program is a test
// binary somewhere under the temporary directory - the two are never the same
// directory and neither is a root, so those guards never reach the branch this
// is about. Measured on 2026-09-28 (docs/STARTING-DIRECTORY-2026-09-28.md):
// macOS starts an application from Finder in "/", which is read only, and a
// double click or an installer's shortcut starts it in its own directory.
func TestTheWindowOffersTheHomeDirectoryWhenStartedWhereItShouldNotWrite(t *testing.T) {
	home := t.TempDir()
	program := t.TempDir()
	elsewhere := t.TempDir()
	atHome := filepath.Join(home, window.OutputFolderName)

	spelled, how := anotherSpelling(t, program)
	if spelled == program {
		t.Fatalf("the second spelling of %s is the same text, so the case below would test nothing", program)
	}
	if !sameDirectoryHere(t, spelled, program) {
		t.Fatalf("%s (%s) is not the same directory as %s, so the case below would test the wrong thing", spelled, how, program)
	}
	root := rootOf(home)
	if filepath.Dir(root) != root {
		t.Fatalf("%s is not the root of a disk, so the case below would test the wrong thing", root)
	}
	if sameDirectoryHere(t, elsewhere, program) {
		t.Fatalf("%s and %s are one directory, so the ordinary case would test the wrong thing", elsewhere, program)
	}

	for _, c := range []struct{ what, working, home, want string }{
		{"started in its own directory, spelled as " + how, spelled, home, atHome},
		{"started in its own directory, spelled the same", program, home, atHome},
		{"started in the root of a disk, as macOS does from Finder", root, home, atHome},
		{"started anywhere else, as from a terminal", elsewhere, home, filepath.Join(elsewhere, window.OutputFolderName)},
		{"no home directory to go to", program, "", filepath.Join(program, window.OutputFolderName)},
	} {
		t.Run(c.what, func(t *testing.T) {
			got := window.OfferedDirectory(c.working, program, c.home)
			if got != c.want {
				t.Errorf("started in %s with the program in %s and the home in %q, the window offers %s.\n"+
					"Want %s: a program directory or the root of a disk is not a place to write ten "+
					"thousand files into, and anywhere else is where the person chose to stand.",
					c.working, program, c.home, got, c.want)
			}
		})
	}
}

// A folder the old offer left in the remembered settings is not offered again.
//
// Closing the window writes down whatever the box held, chosen or not, so a
// window once started from Finder remembers "/tfg-out" and would offer it at
// every start after the offer itself was fixed. The owner decided on
// 2026-09-28 that such a value counts as nothing remembered.
func TestAFolderTheOldOfferLeftBehindIsNotOfferedAgain(t *testing.T) {
	program := t.TempDir()
	elsewhere := t.TempDir()
	spelled, how := anotherSpelling(t, program)
	if !sameDirectoryHere(t, spelled, program) {
		t.Fatalf("%s (%s) is not the same directory as %s", spelled, how, program)
	}
	root := rootOf(program)

	for _, c := range []struct {
		what, remembered string
		leftBehind       bool
	}{
		{"the folder under the root of a disk", filepath.Join(root, window.OutputFolderName), true},
		{"the folder under this program's directory, spelled as " + how, filepath.Join(spelled, window.OutputFolderName), true},
		{"another folder under this program's directory", filepath.Join(program, "results"), false},
		{"the folder under a directory somebody chose", filepath.Join(elsewhere, window.OutputFolderName), false},
	} {
		t.Run(c.what, func(t *testing.T) {
			if got := window.LeftByTheOldOffer(c.remembered, program); got != c.leftBehind {
				t.Errorf("LeftByTheOldOffer(%s) = %v, want %v", c.remembered, got, c.leftBehind)
			}
		})
	}
}

// And the window really does pass over it at start. The rule above is only
// half of the fix: without the call where the remembered folder is handed to
// the screens, the value left by the old offer comes back regardless.
func TestTheWindowDoesNotOfferTheFolderTheOldOfferLeftBehind(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("this system has no home directory, so the fixed offer has nowhere to go: %v", err)
	}
	stale := filepath.Join(rootOf(home), window.OutputFolderName)
	if !window.LeftByTheOldOffer(stale, "") {
		t.Fatalf("%s does not count as left by the old offer, so this would test the wrong thing", stale)
	}

	host := newFakeHost(t)
	host.Remembered().RememberDirectory(stale)
	window.Open(host)
	if host.content == nil {
		t.Fatal("opening the window put no screen in it")
	}
	for _, tab := range []string{text.TabOneTarget(), text.TabPresets(), text.TabRecipe()} {
		screen := selectTab(t, host.content, tab)
		box := entryUnder(t, screen, text.FieldOutputDir())
		if box == nil {
			t.Fatalf("the %s screen has no output directory box", tab)
		}
		if box.Text == stale {
			t.Errorf("the %s screen offers %s, which the old offer left in the remembered settings "+
				"and which can never be written into", tab, stale)
		}
		if !hasSuffix(box.Text, window.OutputFolderName) {
			t.Errorf("the %s screen offers %q instead of the folder of our own", tab, box.Text)
		}
	}
}

// anotherSpelling is the same directory under a different text, and says how
// it was made. A link where the system allows one, letter case where the file
// system ignores it. The guard asks the file system that the two really are one
// directory before it trusts the case - a spelling that turned out to be a
// second directory would make the "own directory" case pass for the wrong
// reason.
func anotherSpelling(t *testing.T, dir string) (string, string) {
	t.Helper()
	link := filepath.Join(t.TempDir(), "same-directory")
	if err := os.Symlink(dir, link); err == nil {
		return link, "a link"
	}
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.ToUpper(dir), "other letter case"
	}
	t.Skip("this system allows neither a link nor a second letter case, so there is no second spelling to try")
	return "", ""
}

// sameDirectoryHere is the precondition every case above asserts rather than
// assumes: a guard that only believes it reached a state is green for the
// wrong reason the day something else changes.
func sameDirectoryHere(t *testing.T, a, b string) bool {
	t.Helper()
	ia, errA := os.Stat(a)
	ib, errB := os.Stat(b)
	return errA == nil && errB == nil && os.SameFile(ia, ib)
}

// rootOf is the root of the disk a directory is on: "C:\" or "/".
func rootOf(dir string) string {
	return filepath.VolumeName(dir) + string(filepath.Separator)
}
