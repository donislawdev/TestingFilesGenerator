// The scripts the reference tools are driven by.
//
// Split out of oracle.go on 2026-08-19, when adding LibreOffice pushed that
// file past the size a guard allows. They belong together anyway: each one is
// a small program in another language, and none of them is Go worth reading
// beside the checker table.
package oracle

import (
	"fmt"
	"strings"
)

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

// libreOfficeScript converts an Office package to PDF and reports whether one
// came out.
//
// The profile directory is a fixed name rather than a fresh one per file, and
// that is a measured trade rather than carelessness: building a profile costs
// about ten seconds and reusing one costs about three, so a fresh profile per
// check would add a minute to every run of the suite. The cost of the fixed
// name is that two of these cannot run at the same moment, which is why it
// carries the process id of whatever launched it.
const libreOfficeScript = `
import os, shutil, subprocess, sys, tempfile

candidates = [
    r"C:\Program Files\LibreOffice\program\soffice.exe",
    r"C:\Program Files (x86)\LibreOffice\program\soffice.exe",
    "/usr/bin/soffice", "/usr/local/bin/soffice",
    "/Applications/LibreOffice.app/Contents/MacOS/soffice",
]
soffice = next((c for c in candidates if os.path.exists(c)), None) or shutil.which("soffice")
if not soffice:
    print("SKIP no libreoffice"); sys.exit(0)

root = os.path.join(tempfile.gettempdir(), "tfg-libreoffice-%d" % os.getppid())
profile = os.path.join(root, "profile").replace("\\", "/")
out = os.path.join(root, "out")
shutil.rmtree(out, ignore_errors=True)
os.makedirs(out, exist_ok=True)
try:
    subprocess.run([soffice, "-env:UserInstallation=file:///" + profile,
                    "--headless", "--norestore", "--convert-to", "pdf",
                    "--outdir", out, sys.argv[1]],
                   capture_output=True, text=True, timeout=180)
    made = [f for f in os.listdir(out) if f.endswith(".pdf")]
    if not made:
        print("FAIL LibreOffice would not open the package"); sys.exit(1)
    size = os.path.getsize(os.path.join(out, made[0]))
    if size < 400:
        print("FAIL LibreOffice opened the package and rendered nothing"); sys.exit(1)
    print("OK rendered to %d B of PDF" % size)
finally:
    shutil.rmtree(out, ignore_errors=True)
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
