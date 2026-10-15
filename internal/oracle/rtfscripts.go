package oracle

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// pandocRTF is pandoc's RTF reader, writing the text it read. Chosen for CI on
// 2026-10-15 from five readers measured (docs/RTF-2026-10-15.md section 6.1):
// it is one apt install on the runner, it refuses a document without its
// closing brace and a \' without two hex digits, and it is neither our code nor
// our language.
//
// It is also the strictest reader in a way that is not a virtue - it shows
// codepage text in the wrong letters and drops a character after every letter
// written literal. Those are its readings of good files, which the guard of the
// format holds as such, so this checker asks only whether it read the file and
// said nothing about it. A warning on standard error counts as a complaint,
// the way it does for ffprobe.
var pandocRTF = Checker{
	Name: "pandoc",
	find: inPath("pandoc"),
	args: func(p string) []string { return []string{"-f", "rtf", "-t", "plain", "--wrap=none", p} },
	accept: func(stdout, stderr string, code int) error {
		if code != 0 {
			return fmt.Errorf("pandoc refused the document: %s", strings.TrimSpace(stderr))
		}
		if s := strings.TrimSpace(stderr); s != "" {
			return fmt.Errorf("pandoc complained: %s", s)
		}
		return nil
	},
}

// PandocRTF is the same reader with what it printed kept apart from what it
// complained of, for a guard that reads the text: standard output as it was,
// and the warnings, which codepage is measured to bring.
func PandocRTF(path string) (text, warnings string, res Result) {
	c := pandocRTF
	var out, errOut string
	c.accept = func(stdout, stderr string, code int) error {
		out, errOut = stdout, stderr
		if code != 0 {
			return fmt.Errorf("pandoc refused the document: %s", strings.TrimSpace(stderr))
		}
		return nil
	}
	res = c.Check(path)
	return out, strings.TrimSpace(errOut), res
}

// pandocPicture is how pandoc's markdown points at a picture it wrote out.
var pandocPicture = regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)

// PandocRTFPictures is the same reader asked for the pictures: the files it
// wrote out into dir, in the order the document shows them, with the warnings.
// A document it refused comes back with the refusal in res.Err and no
// pictures - which for a binary picture was measured to happen, and is a
// reading rather than a fault of the check (docs/RTF-2-2026-10-15.md 1.1).
func PandocRTFPictures(path, dir string) (pictures [][]byte, warnings string, res Result) {
	c := pandocRTF
	var out, errOut string
	c.args = func(p string) []string {
		return []string{"-f", "rtf", "-t", "markdown", "--wrap=none", "--extract-media=" + dir, p}
	}
	c.accept = func(stdout, stderr string, code int) error {
		out, errOut = stdout, stderr
		if code != 0 {
			return fmt.Errorf("pandoc refused the document: %s", strings.TrimSpace(stderr))
		}
		return nil
	}
	res = c.Check(path)
	if res.Err != nil || !res.Available {
		return nil, strings.TrimSpace(errOut), res
	}
	for _, m := range pandocPicture.FindAllStringSubmatch(out, -1) {
		b, err := os.ReadFile(m[1])
		if err != nil {
			res.Err = fmt.Errorf("pandoc points at a picture it did not write, %s: %w", m[1], err)
			return nil, strings.TrimSpace(errOut), res
		}
		pictures = append(pictures, b)
	}
	return pictures, strings.TrimSpace(errOut), res
}
