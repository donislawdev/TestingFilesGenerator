package guard

import (
	"os"
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
