package guard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The command line is English only, error messages included. Only the window
// gets translations.
//
// This catches accented characters. It does not catch another language
// written in plain ASCII, and no automated check ever will - that part stays
// with reading. See docs/QUALITY.md section 8.
//
// Test files are exempt on purpose. Other languages are legitimate there as
// test data.
func asciiRequired(rel string) bool {
	switch rel {
	case "internal/cli", "cmd/tfg":
		return true
	}
	return false
}

// Text that lives in the repository is English, whatever its audience. The
// criterion is the place, not the reader - the technical changelog is read
// internally and still sits on the public surface of the project.
//
// The internal documents are absent from this list because they are absent
// from the repository. See .gitignore.
// CHANGELOG-DEV.md was on this list until 2026-08-02 and is deliberately off
// it. The owner took the technical changelog out of the repository that day, so
// by the criterion this comment states - the place, not the reader - it stopped
// being repository text. Leaving it listed would fail every fresh clone and
// every CI run, because the guard would look for a file that is not there.
//
// Do not put it back without moving the file back first. The two go together.
var englishFiles = []string{
	"README.md",
	"CHANGELOG.md",
}

func TestTextInTheRepositoryIsAsciiOnly(t *testing.T) {
	root := repoRoot(t)
	for _, name := range englishFiles {
		b, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Errorf("reading %s: %v - it is listed as English text but is not there", name, err)
			continue
		}
		for n, line := range strings.Split(string(b), "\n") {
			for col, r := range line {
				if r > 127 {
					t.Errorf("%s:%d:%d holds %q - text in the repository is English", name, n+1, col+1, r)
					break
				}
			}
		}
	}
}

func TestCommandLineIsAsciiOnly(t *testing.T) {
	checked := 0
	for _, p := range packages(t) {
		if !asciiRequired(p.rel) {
			continue
		}
		for _, f := range p.files {
			checked++
			b, err := os.ReadFile(f)
			if err != nil {
				t.Fatalf("reading %s: %v", f, err)
			}
			for n, line := range strings.Split(string(b), "\n") {
				for col, r := range line {
					if r > 127 {
						t.Errorf("%s:%d:%d holds %q - the command line is English only",
							f, n+1, col+1, r)
						break
					}
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no file was examined - this guard would pass without checking anything")
	}
}
