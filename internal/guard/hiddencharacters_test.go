package guard

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// No file in this repository carries a character nobody can see.
//
// The class a reviewer reading a diff cannot catch by reading: a right to left
// override that makes code say one thing on screen and another to the
// compiler - the attack published as Trojan Source - a zero width space that
// makes two identifiers look like one, a byte order mark or a line separator
// inside a string. The preset of unusual file names holds exactly these
// characters as data, so its source is the first place in this tree that
// wants them, and it writes every one of them as an escape. Measured before
// this guard existed, on 2026-09-24: not one tracked file carried such a
// character, so the list of exceptions starts empty.
//
// And the tool that writes this code is the reason it is a guard rather than a
// habit. The editor this project is written with turns a typed backslash-u
// escape into the character itself, silently - it did so twice on the day this
// guard was written, once in a document about this very problem.
//
// A file that is not UTF-8 is not text and is left out, counted. The tab, the
// line feed and the carriage return are what text is laid out with.
func TestNoTrackedFileCarriesACharacterNobodyCanSee(t *testing.T) {
	root := repoRoot(t)
	listed := strings.Split(gitOutput(t, "ls-files", "-z", "--cached", "--others", "--exclude-standard"), "\x00")

	read, goFiles, binary := 0, 0, 0
	var faults []string
	for _, f := range listed {
		if f == "" {
			continue
		}
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(f)))
		if err != nil {
			continue
		}
		if !utf8.Valid(body) {
			binary++
			continue
		}
		read++
		if strings.HasSuffix(f, ".go") {
			goFiles++
		}
		if fault := firstHidden(string(body)); fault != "" {
			faults = append(faults, f+":"+fault)
		}
	}

	// The state this is about: the Go source was actually read. A listing
	// that came back empty or from the wrong directory would pass below.
	if goFiles < 100 {
		t.Fatalf("only %d Go files were read (%d text files, %d not UTF-8), so this guard is not looking at this repository", goFiles, read, binary)
	}
	if len(faults) > 0 {
		t.Errorf("%d file(s) carry a character nobody can see. Write it as an escape instead - in Go, a backslash, u and four hex digits:\n  %s",
			len(faults), strings.Join(faults, "\n  "))
	}
	t.Logf("%d text files read, %d of them Go, %d not UTF-8 left out", read, goFiles, binary)
}

// firstHidden is where the first such character of text is, as "line: U+XXXX",
// or "" when there is none.
func firstHidden(text string) string {
	line := 1
	for _, r := range text {
		switch {
		case r == '\n':
			line++
		case r == '\t' || r == '\r':
		case unseenHere(r):
			return fmt.Sprintf("%d: U+%04X", line, r)
		}
	}
	return ""
}

// The detector finds what it is for, or the guard above passes by finding
// nothing anywhere. Built from rune numbers, since this file is one of the
// files being scanned.
func TestTheHiddenCharacterDetectorFindsWhatItIsFor(t *testing.T) {
	for _, r := range []rune{0x202E, 0x200B, 0xFEFF, 0x2028, 0x00A0, 0x3000, 0xE0041} {
		if got := firstHidden("ok\nx := \"a" + string(r) + "b\"\n"); got != fmt.Sprintf("2: U+%04X", r) {
			t.Errorf("U+%04X on the second line was reported as %q", r, got)
		}
	}
	if got := firstHidden("tab\there, crlf\r\nand a plain space\n"); got != "" {
		t.Errorf("ordinary layout was reported as %q", got)
	}
}
