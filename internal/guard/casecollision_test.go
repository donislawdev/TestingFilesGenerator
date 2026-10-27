package guard

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// A file whose name differs from a planned one only in case is refused before
// the first byte on a system that treats the two as one name, and left alone
// beside it on a system that does not - docs/STABILITY-REVIEW-2026-10-26.md,
// R07.
//
// Until 2026-10-27 preflight compared names letter for letter. With
// FILES_0001.TXT in the directory, --dry-run said nothing was in the way and
// ended with 0, and the run itself wrote files_0002.txt and its manifest and
// only then met the protection at the rename, ending with 8 and a directory
// half written. A name that is truly taken is refused with 5 before anything
// exists, in --dry-run as well, and this is the same case.
//
// The guard ASKS the system which kind it is rather than assuming from the
// platform, because a Windows directory can be case sensitive and a Linux one
// can fold. Both answers have a branch, and CI runs this on Linux, Windows and
// macOS, so both are reached.
func TestAFileWhoseNameDiffersOnlyInCaseIsNotWrittenOver(t *testing.T) {
	for _, c := range []struct {
		name string
		// inTheWay is the name already in the directory, from the name this
		// run gives its first file.
		inTheWay func(first string) string
	}{
		{"a planned file", strings.ToUpper},
		// The temporary name a file is written under before the rename is
		// guarded by the same check, and the run here is in this process, so
		// it carries this process's id.
		{"the temporary name of one", func(first string) string {
			return strings.ToUpper(core.SiblingName(first, fmt.Sprintf("%s%d", core.PartialMarker, os.Getpid())))
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			const first = "files_0001.txt"
			dir := t.TempDir()
			theirs := c.inTheWay(first)
			const content = "somebody else's work\n"
			if err := os.WriteFile(filepath.Join(dir, theirs), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := os.Lstat(filepath.Join(dir, strings.ToLower(theirs)))
			folds := err == nil
			t.Logf("this directory treats %s and %s as one name: %v", theirs, strings.ToLower(theirs), folds)

			for _, dry := range []bool{true, false} {
				args := []string{"generate", "--format", "txt", "--size", "10B", "--count", "2", "--out", dir}
				if dry {
					args = append(args, "--dry-run")
				}
				var out, errOut bytes.Buffer
				code := cli.Run(context.Background(), args, &out, &errOut)
				if folds {
					if code != cli.ExitIO {
						t.Errorf("dry run %v: ended with %d, expected %d - %s is %s on this system and the run went ahead\nstderr: %s",
							dry, code, cli.ExitIO, theirs, strings.ToLower(theirs), errOut.String())
					}
					if !strings.Contains(errOut.String(), theirs) {
						t.Errorf("dry run %v: the refusal does not name %s, the file that is in the directory\nstderr: %s", dry, theirs, errOut.String())
					}
					if got := namesIn(t, dir); strings.Join(got, " ") != theirs {
						t.Errorf("dry run %v: the directory holds %q after the refusal, expected %s alone - something was written before it", dry, got, theirs)
					}
				} else if code != cli.ExitOK {
					t.Errorf("dry run %v: ended with %d on a system where %s and %s are two names\nstderr: %s",
						dry, code, theirs, strings.ToLower(theirs), errOut.String())
				}
			}
			if got, err := os.ReadFile(filepath.Join(dir, theirs)); err != nil || string(got) != content {
				t.Errorf("%s was changed: %q, %v", theirs, got, err)
			}
		})
	}
}

// The refusal names the file that is really in the way, not another one that
// folds to the same letters.
//
// core.FoldName joins pairs a filesystem keeps apart. On NTFS STRASSE.txt and
// Stra\u00dfe.txt are two files side by side, both fold to strasse.txt, and a run
// writing stra\u00dfe.txt would write over the second - measured on 2026-10-27.
// Until CodeRabbit asked on #207, preflight kept the first spelling of a fold
// and named STRASSE.txt, which is the file it would have told somebody to
// remove. A system that joins the two (APFS does) cannot hold this directory at
// all, and the guard says so rather than passing.
func TestARefusalNamesTheFileThatIsReallyInTheWay(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "STRASSE.txt"), []byte("upper case ss\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sharp := "Stra\u00dfe.txt"
	f, err := os.OpenFile(filepath.Join(dir, sharp), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		t.Skipf("this filesystem will not hold %s beside STRASSE.txt (%v), so the pair this guard is about cannot exist here", sharp, err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	planned := strings.ToLower(sharp)
	_, err = os.Lstat(filepath.Join(dir, planned))
	folds := err == nil
	t.Logf("this directory treats %s and %s as one name: %v", sharp, planned, folds)

	var out, errOut bytes.Buffer
	code := cli.Run(context.Background(), []string{"generate", "--format", "txt", "--size", "10B", "--name", planned, "--out", dir, "--dry-run"}, &out, &errOut)
	if !folds {
		if code != cli.ExitOK {
			t.Errorf("ended with %d where %s and %s are two names\nstderr: %s", code, sharp, planned, errOut.String())
		}
		return
	}
	if code != cli.ExitIO {
		t.Fatalf("ended with %d, expected %d - %s is in the way\nstderr: %s", code, cli.ExitIO, sharp, errOut.String())
	}
	if !strings.Contains(errOut.String(), "as "+sharp) || strings.Contains(errOut.String(), "STRASSE") {
		t.Errorf("the refusal has to name %s, the file %s would write over, and not STRASSE.txt\nstderr: %s", sharp, planned, errOut.String())
	}
}
