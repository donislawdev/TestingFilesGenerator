// Package oracle wraps the external reference tools that tests compare our
// output against.
//
// Production code never imports it. A missing tool skips a test loudly - a
// quietly skipped oracle is a green run that checked nothing.
//
// The point of these is that our own tests are written by whoever wrote the
// generator, so they cannot be the only judge of whether a file is correct.
// An independent implementation can.
package oracle

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Result is what a reference tool said about a file.
type Result struct {
	// Available is false when the tool is not installed. The caller reports
	// that rather than passing quietly.
	Available bool
	// Tool is what was run, for the report.
	Tool string
	// Output is what it printed, trimmed.
	Output string
	// Err is set when the tool ran and rejected the file.
	Err error
}

// Checker verifies one file with one external tool.
type Checker struct {
	// Name is what a report calls this oracle.
	Name string
	// find returns the path of the tool, or false when it is missing.
	find func() (string, bool)
	// args builds the command line for a file.
	args func(path string) []string
	// accept decides whether the output means the file is good.
	accept func(stdout, stderr string, exitCode int) error
}

// For returns the oracle a format declares, or false when it declares none.
func For(oracleName string) (Checker, bool) {
	c, ok := checkers[oracleName]
	return c, ok
}

// Check runs the tool against a file.
func (c Checker) Check(path string) Result {
	bin, ok := c.find()
	if !ok {
		return Result{Available: false, Tool: c.Name}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, c.args(path)...)
	var out, errOut strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	runErr := cmd.Run()

	code := 0
	var exitErr *exec.ExitError
	if runErr != nil {
		if ok := asExitError(runErr, &exitErr); ok {
			code = exitErr.ExitCode()
		} else {
			return Result{Available: true, Tool: c.Name, Err: runErr}
		}
	}

	// A tool can be present while the library it needs is not - python is
	// installed on every runner and Pillow is not. A checker says so with
	// this marker, and it counts as unavailable rather than as a rejection.
	// Without this the file looks broken when in fact nothing looked at it.
	if strings.HasPrefix(strings.TrimSpace(out.String()), "SKIP") {
		return Result{Available: false, Tool: c.Name}
	}

	res := Result{
		Available: true,
		Tool:      c.Name,
		Output:    strings.TrimSpace(out.String() + " " + errOut.String()),
	}
	res.Err = c.accept(out.String(), errOut.String(), code)
	return res
}

func asExitError(err error, target **exec.ExitError) bool {
	if e, ok := err.(*exec.ExitError); ok {
		*target = e
		return true
	}
	return false
}

var checkers = map[string]Checker{
	"ffprobe": {
		Name: "ffprobe",
		find: inPath("ffprobe"),
		args: func(p string) []string {
			return []string{"-v", "error", "-show_entries",
				"stream=codec_name,sample_rate,channels,bits_per_sample", "-of", "csv=p=0", p}
		},
		accept: func(stdout, stderr string, code int) error {
			if code != 0 {
				return fmt.Errorf("ffprobe refused the file: %s", strings.TrimSpace(stderr))
			}
			if strings.TrimSpace(stderr) != "" {
				// ffprobe stays silent at this log level unless something is
				// wrong, so anything here is a complaint about our file.
				return fmt.Errorf("ffprobe complained: %s", strings.TrimSpace(stderr))
			}
			if strings.TrimSpace(stdout) == "" {
				return fmt.Errorf("ffprobe found no stream in the file")
			}
			return nil
		},
	},

	"pdftotext": {
		Name: "pdftotext",
		find: inPath("pdftotext"),
		args: func(p string) []string { return []string{p, "-"} },
		accept: func(stdout, stderr string, code int) error {
			if code != 0 {
				return fmt.Errorf("pdftotext refused the file: %s", strings.TrimSpace(stderr))
			}
			// This reader reports a broken cross reference table on standard
			// error while still exiting zero, so the exit code alone would
			// miss it.
			if strings.Contains(stderr, "Error") || strings.Contains(stderr, "error") {
				return fmt.Errorf("pdftotext complained: %s", strings.TrimSpace(stderr))
			}
			if !strings.Contains(stdout, "Page") {
				return fmt.Errorf("pdftotext extracted no page text")
			}
			return nil
		},
	},

	"7z": {
		Name: "7z",
		find: sevenZip,
		args: func(p string) []string { return []string{"t", p} },
		accept: func(stdout, stderr string, code int) error {
			if code != 0 {
				return fmt.Errorf("7z refused the archive: %s", strings.TrimSpace(stdout+stderr))
			}
			if !strings.Contains(stdout, "Everything is Ok") {
				return fmt.Errorf("7z did not report the archive as good: %s", strings.TrimSpace(stdout))
			}
			// A warning still exits zero. For ZIP we expect none, and if one
			// appears it is a change worth seeing.
			if strings.Contains(stdout, "WARNINGS") || strings.Contains(stdout, "Warning") {
				return fmt.Errorf("7z reported a warning: %s", strings.TrimSpace(stdout))
			}
			return nil
		},
	},

	"pillow": {
		Name: "python-pillow",
		find: inPath("python"),
		args: func(p string) []string {
			return []string{"-c", pillowScript, p}
		},
		accept: func(stdout, stderr string, code int) error {
			if code != 0 {
				return fmt.Errorf("Pillow refused the image: %s", strings.TrimSpace(stderr))
			}
			if !strings.HasPrefix(strings.TrimSpace(stdout), "OK") {
				return fmt.Errorf("Pillow did not confirm the image: %s", strings.TrimSpace(stdout+stderr))
			}
			return nil
		},
	},
}

// pillowScript opens the image and forces every pixel to be decoded, so a
// truncated or malformed image fails rather than passing on its header alone.
const pillowScript = `
import sys
try:
    from PIL import Image
except ImportError:
    print("SKIP no pillow"); sys.exit(0)
im = Image.open(sys.argv[1])
im.load()
im.convert("RGBA").tobytes()
print("OK", im.format, im.width, im.height)
`

func inPath(name string) func() (string, bool) {
	return func() (string, bool) {
		p, err := exec.LookPath(name)
		if err != nil {
			return "", false
		}
		return p, true
	}
}

// sevenZip looks in the usual place on Windows as well, because the installer
// does not put it on the path.
func sevenZip() (string, bool) {
	if p, err := exec.LookPath("7z"); err == nil {
		return p, true
	}
	if runtime.GOOS == "windows" {
		for _, dir := range []string{
			`C:\Program Files\7-Zip`,
			`C:\Program Files (x86)\7-Zip`,
		} {
			p := filepath.Join(dir, "7z.exe")
			if _, err := os.Stat(p); err == nil {
				return p, true
			}
		}
	}
	return "", false
}

// Strict runs the structural checker written against the specification.
//
// The tolerant readers above answer "would a real viewer accept this". This
// answers "is it actually well formed". Measured, the two are different
// questions: ffprobe ignores a wrong size in the RIFF header, Pillow does not
// verify the checksum of an ancillary PNG chunk, and a PDF reader rebuilds a
// cross reference table whose offset is off by one.
//
// It is written in another language, to the specification, so it is not our
// own code judging our own code.
func Strict(formatID, path string) Result {
	python, ok := inPath("python")()
	if !ok {
		return Result{Available: false, Tool: "strict structural check"}
	}
	script, ok := strictScriptPath()
	if !ok {
		return Result{Available: false, Tool: "strict structural check"}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, python, script, formatID, path)
	var out, errOut strings.Builder
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	_ = cmd.Run()

	text := strings.TrimSpace(out.String())
	res := Result{Available: true, Tool: "strict structural check", Output: text}
	switch {
	case strings.HasPrefix(text, "OK"):
	case strings.HasPrefix(text, "FAIL"):
		res.Err = fmt.Errorf("%s", strings.TrimSpace(strings.TrimPrefix(text, "FAIL")))
	default:
		res.Err = fmt.Errorf("the checker said nothing useful: %s %s", text, strings.TrimSpace(errOut.String()))
	}
	return res
}

// StrictKnows says whether the structural checker covers a format.
func StrictKnows(formatID string) bool {
	switch formatID {
	case "png", "wav", "pdf", "zip":
		return true
	}
	return false
}

// strictScriptPath finds the checker next to this source file, so the tests
// work wherever the repository is checked out.
func strictScriptPath() (string, bool) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", false
	}
	p := filepath.Join(filepath.Dir(thisFile), "strict.py")
	if _, err := os.Stat(p); err != nil {
		return "", false
	}
	return p, true
}
