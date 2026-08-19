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
			// A crash is not a verdict on the file. It is worth reporting in
			// its own words, because "the tool says your file is wrong" and
			// "the tool fell over" call for different answers.
			if code < 0 || strings.Contains(exitErr.String(), "signal") {
				return Result{
					Available: true, Tool: c.Name,
					Output: strings.TrimSpace(out.String() + " " + errOut.String()),
					Err: fmt.Errorf("%s crashed rather than answering (%v) - that is a defect in the tool, and a file that crashes it is still a file nobody will trust",
						c.Name, exitErr),
				}
			}
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
			if strings.Contains(stdout, "Segmentation fault") || strings.Contains(stderr, "Segmentation fault") {
				return fmt.Errorf("7z crashed reading the archive: %s", strings.TrimSpace(stdout+stderr))
			}
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

	// V8's parser, which is neither our code nor our language. Measured present
	// on this machine as node v26.5.0 on 2026-08-01.
	"node-json": {
		Name:   "node",
		find:   inPath("node"),
		args:   func(p string) []string { return []string{"-e", nodeJSONScript, p} },
		accept: expectOK("node"),
	},

	// The Python csv module at its DEFAULT settings, on purpose. Measured on
	// 2026-08-01 it refuses a field above 131 072 B, and that is the reader a
	// tester is most likely to have. Raising the limit here would turn the
	// measurement that shaped the CSV generator into something nothing checks.
	"python-csv": {
		Name:   "python-csv",
		find:   inPath("python"),
		args:   func(p string) []string { return []string{"-c", pythonCSVScript, p} },
		accept: expectOK("the Python csv module"),
	},

	// expat through xml.sax, which is a C parser rather than anything written
	// here. The structural check beside it is hand written to the specification,
	// so the two are genuinely different implementations.
	"python-xml": {
		Name:   "python-xml",
		find:   inPath("python"),
		args:   func(p string) []string { return []string{"-c", pythonXMLScript, p} },
		accept: expectOK("the Python XML parser"),
	},

	// The strongest reference tool in this project. Everything else answers
	// "did it parse" - this one answers "did it draw", by rendering the file to
	// a bitmap and then looking at the pixels. A drawing that parses and paints
	// nothing passes every other check here.
	//
	// Driven from a script rather than directly, because two things had to be
	// measured before this could be trusted, and both surprised. Inkscape 1.4.4
	// exits ZERO on a malformed file and only says so on standard error - the
	// exit code alone would bless a broken drawing. And it will not write the
	// bitmap to standard output, so the only way to know a bitmap exists is to
	// open the file it wrote.
	"inkscape": {
		Name:   "inkscape-render",
		find:   inPath("python"),
		args:   func(p string) []string { return []string{"-c", inkscapeScript, p} },
		accept: expectOK("the Inkscape renderer"),
	},

	// The only HTML reader on this machine, and lenient by design - the format
	// requires a parser to recover from almost anything, so this says less than
	// the readers beside it. The structural check carries more of the weight.
	"python-html": {
		Name:   "python-html",
		find:   inPath("python"),
		args:   func(p string) []string { return []string{"-c", pythonHTMLScript, p} },
		accept: expectOK("the Python HTML parser"),
	},
}

// inkscapeScript renders the drawing and then looks at what came out.
//
// The blank check is the point. An SVG that parses but paints nothing renders
// to a single flat colour, and that is a defect no parser and no size guard can
// see. Two colours is a low bar on purpose - it says something was drawn, not
// that it was drawn well.
const inkscapeScript = `
import os, pathlib, shutil, subprocess, sys, tempfile

exe = shutil.which("inkscape")
if exe is None:
    for candidate in (r"C:\ProgramData\chocolatey\bin\inkscape.exe",
                      r"C:\Program Files\Inkscape\bin\inkscape.exe"):
        if os.path.exists(candidate):
            exe = candidate
            break
if exe is None:
    print("SKIP no inkscape"); sys.exit(0)
try:
    from PIL import Image
except ImportError:
    print("SKIP no pillow"); sys.exit(0)

out = pathlib.Path(tempfile.mkdtemp()) / "render.png"
run = subprocess.run([exe, "--export-type=png", "--export-filename=" + str(out), sys.argv[1]],
                     capture_output=True, text=True, timeout=120)

# Measured 2026-08-01: a malformed drawing still exits zero here and says so
# only on standard error, so the exit code alone would bless a broken file.
#
# Any word on standard error was therefore treated as a complaint - and that
# made this guard flaky. Seen three times on 2026-08-02: GLib, which Inkscape
# links, writes warnings about the machine rather than about the drawing, and
# one of them named an unrelated Windows Store application. A guard that
# reddens on somebody else's software is a guard that gets switched off.
#
# So lines from the GLib subsystems are dropped and everything else is judged
# as before. The filter is narrow on purpose: Inkscape's own objections to a
# file do not carry these prefixes, and the check below is proven by feeding
# this script a malformed drawing.
# Widened on 2026-08-04, after this reddened CI on every run for a day. The
# prefixes below were not enough: the runner also produces
#
#   ** (inkscape:3815): WARNING **: Failed to wrap object of type
#   'GtkRecentManager'. Hint: this error is commonly caused by failing to
#   call a library init() function.
#
# which carries none of them, because the type name has no hyphen after Gtk.
# It is the same kind of complaint - about how the machine is set up, not
# about the file - and it took the SVG oracle down in seven places at once.
#
# Matched by its own sentence rather than by relaxing the rule to "ignore
# warnings", which would throw away the thing this checker is for. Anything
# Inkscape says about the drawing itself still counts, and the test below
# feeds it a drawing that paints nothing to prove that is still true.
NOT_ABOUT_THE_FILE = ("GLib-", "Gtk-", "GdkPixbuf-", "Failed to wrap object of type")


def about_the_drawing(line):
    return not any(marker in line for marker in NOT_ABOUT_THE_FILE)

noise = "\n".join(l for l in (run.stderr or "").splitlines() if about_the_drawing(l))
for word in ("error", "Error", "ERROR", "unsupported", "WARNING"):
    if word in noise:
        print("FAIL inkscape complained:", noise.strip()[:200]); sys.exit(1)
if run.returncode != 0:
    print("FAIL inkscape refused the drawing:", noise.strip()[:200]); sys.exit(1)
if not out.exists() or out.stat().st_size == 0:
    print("FAIL inkscape wrote no bitmap, so nothing was rendered"); sys.exit(1)

im = Image.open(out)
im.load()
colours = im.convert("RGB").getcolors(maxcolors=1 << 20)
if colours is None:
    count = 1 << 20
else:
    count = len(colours)
if count < 2:
    print("FAIL the rendered bitmap is one flat colour, so the drawing paints nothing"); sys.exit(1)
print("OK rendered", im.width, "x", im.height, "with", count, "colours")
`

// pythonHTMLScript walks the document and counts what it found, so a file that
// merely opened still has to carry a body with blocks in it.
const pythonHTMLScript = `
import sys
from html.parser import HTMLParser

class Walk(HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.open = []
        self.blocks = 0
        self.bad = []
    def handle_starttag(self, tag, attrs):
        if tag in ("meta", "br", "img", "hr", "input", "link"):
            return
        self.open.append(tag)
        if tag in ("p", "h1", "h2", "ul", "table", "blockquote"):
            self.blocks += 1
    def handle_endtag(self, tag):
        if tag in self.open:
            while self.open and self.open.pop() != tag:
                pass
        else:
            self.bad.append(tag)

walk = Walk()
walk.feed(open(sys.argv[1], encoding="utf-8").read())
if walk.bad:
    print("FAIL closing tag with nothing open:", walk.bad[:3]); sys.exit(1)
if walk.open:
    print("FAIL still open at the end:", walk.open[:3]); sys.exit(1)
if walk.blocks < 1:
    print("FAIL the body holds no blocks"); sys.exit(1)
print("OK", walk.blocks, "blocks")
`

// expectOK is the acceptance rule shared by the script based checkers: a zero
// exit and a line starting with OK. Anything else is a rejection reported in
// the tool's own words.
func expectOK(tool string) func(stdout, stderr string, code int) error {
	return func(stdout, stderr string, code int) error {
		if code != 0 {
			return fmt.Errorf("%s refused the file: %s", tool, strings.TrimSpace(stdout+" "+stderr))
		}
		if !strings.HasPrefix(strings.TrimSpace(stdout), "OK") {
			return fmt.Errorf("%s did not confirm the file: %s", tool, strings.TrimSpace(stdout+" "+stderr))
		}
		return nil
	}
}

// nodeJSONScript parses the document and walks it, so a file that is merely
// well formed at the top level still has to hold records.
const nodeJSONScript = `
const fs = require("fs");
const data = JSON.parse(fs.readFileSync(process.argv[1], "utf8"));
if (!Array.isArray(data)) { console.log("FAIL the document is not an array"); process.exit(1); }
if (data.length === 0) { console.log("FAIL the array is empty"); process.exit(1); }
for (const r of data) {
  if (typeof r !== "object" || r === null || Array.isArray(r)) {
    console.log("FAIL an element is not an object"); process.exit(1);
  }
}
console.log("OK " + data.length + " records");
`

// pythonCSVScript reads every row at the module's default settings.
const pythonCSVScript = `
import csv, sys
with open(sys.argv[1], newline="", encoding="utf-8") as handle:
    rows = list(csv.reader(handle))
if len(rows) < 2:
    print("FAIL the table holds no data rows"); sys.exit(1)
print("OK", len(rows) - 1, "rows,", len(rows[0]), "columns")
`

// pythonXMLScript parses with expat, which refuses anything that is not well
// formed rather than repairing it.
const pythonXMLScript = `
import sys
from xml.sax import make_parser, handler, SAXException

class Count(handler.ContentHandler):
    def __init__(self):
        self.elements = 0
    def startElement(self, name, attrs):
        self.elements += 1

counter = Count()
parser = make_parser()
parser.setContentHandler(counter)
try:
    parser.parse(sys.argv[1])
except SAXException as exc:
    print("FAIL", exc); sys.exit(1)
if counter.elements < 2:
    print("FAIL the document holds no elements below the root"); sys.exit(1)
print("OK", counter.elements, "elements")
`

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
	case "png", "wav", "pdf", "zip", "targz", "log", "csv", "json", "xml", "svg", "html",
		"bmp", "gif", "ico":
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
