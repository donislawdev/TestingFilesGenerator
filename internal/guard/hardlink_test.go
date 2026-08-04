package guard

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
)

// A hard link is the redirection the path checks cannot see. A symbolic link
// and a junction are reparse points, so resolving a path walks through them and
// the containment check catches what leaves the directory. A hard link is not a
// reparse point at all - it is a second name for one file, indistinguishable
// from the first, and nothing about the path says so.
//
// It was the one redirection nobody here had measured, and the consequence was
// written down as a question: removing one name does not remove the contents.
//
// Measured on 2026-08-04, WSL2 Ubuntu on ext4, all four shapes:
//
//	a file outside, hard linked in, claimed by the manifest
//	  -> cleanup removed the name inside, exit 0
//	  -> the file outside kept its contents, link count 2 to 1
//	a generated file, hard linked out, then cleanup
//	  -> our name went, the name outside kept the contents
//	generate onto a name that is already a hard link      -> refused, exit 5
//	generate where manifest.json is already a hard link   -> refused, exit 5
//
// So a hard link is not an escape and the answer to the question is the good
// one: unlinking a name is not destroying a file, and the two protections that
// already exist reach it without knowing it is there.
//
// This guard holds that answer in place. The property depends on cleanup
// removing a name and nothing else - a cleanup that emptied a file before
// unlinking it, or wrote to it, would destroy somebody's data through a name
// the tool never looked at, and every existing test would stay green because
// the file inside the directory would still be gone.
func TestCleanupUnlinksOurNameWithoutDestroyingWhatIsBehindAHardLink(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "out")

	code, _, errOut := run(t, "generate", "--format", "txt", "--size", "1kb", "--count", "1", "--out", out)
	if code != cli.ExitOK {
		t.Fatalf("the run this guard is built on failed: exit %d\n%s", code, errOut)
	}
	produced := filepath.Join(out, "files_0001.txt")

	kept := filepath.Join(root, "kept.txt")
	if err := os.Link(produced, kept); err != nil {
		t.Skipf("this filesystem will not make a hard link, so the case cannot be built: %v", err)
	}
	before, err := os.ReadFile(kept)
	if err != nil {
		t.Fatalf("reading the second name: %v", err)
	}
	if len(before) == 0 {
		t.Fatal("the second name is empty before cleanup runs, so this guard would pass on a file that never had contents")
	}

	code, _, errOut = run(t, "cleanup", filepath.Join(out, "manifest.json"), "--yes")
	if code != cli.ExitOK {
		t.Fatalf("cleanup refused: exit %d\n%s", code, errOut)
	}

	// Our name is gone, which is what cleanup was asked to do.
	if _, err := os.Stat(produced); !os.IsNotExist(err) {
		t.Errorf("the file inside the output directory is still there after cleanup")
	}
	// The contents are not, because they were never cleanup's to destroy.
	after, err := os.ReadFile(kept)
	if err != nil {
		t.Fatalf("cleanup destroyed a file outside the output directory: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("cleanup changed a file outside the output directory: %d B before, %d B after", len(before), len(after))
	}
}
