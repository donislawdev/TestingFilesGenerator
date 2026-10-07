package oracle

import (
	"os"
	"strings"
)

// pythonEmail is Python's email package with policy.default - the parser a
// Python service meets first, which is neither our code nor our language.
//
// It reads nearly anything, measured 2026-10-07 (docs/EML-2026-10-07.md
// section 5.1): of ten broken messages it refused none. What it does instead is
// list defects, and it listed one for every broken structure in the sample - a
// file cut short, a missing closing boundary, a stray character in base64, a
// boundary inside the text. So a defect is a refusal here, the way a warning is
// for GDAL and ffmpeg. What it says nothing about - a missing From line, a line
// past 998 characters - the format's guard asks strict.py.
var pythonEmail = Checker{
	Name:   "python email",
	find:   inPath("python"),
	args:   func(p string) []string { return []string{"-c", pythonEmailScript, p, ""} },
	accept: expectOK("python email"),
}

// PythonEmail is the same reader with defects it is told to expect.
//
// One exists today: a header written in UTF-8 as RFC 6532 allows, which the
// package reads correctly and still reports as UndecodableBytesDefect in the
// From and To lines (measured on the program's own messages, 2026-10-07). headers=utf8 asks for exactly that, so its guard passes the name
// rather than the checker passing every defect.
func PythonEmail(path string, allowed ...string) Result {
	c := pythonEmail
	c.args = func(p string) []string {
		return []string{"-c", pythonEmailScript, p, strings.Join(allowed, ",")}
	}
	return c.Check(path)
}

const pythonEmailScript = `
import email, hashlib, json, sys
from email import policy

# The names printed below are UTF-8 whatever the console's code page is.
sys.stdout.reconfigure(encoding="utf-8")
path, allowed = sys.argv[1], set(a for a in sys.argv[2].split(",") if a)
with open(path, "rb") as f:
    msg = email.message_from_bytes(f.read(), policy=policy.default)

defects = []
for key in ("subject", "from", "to", "date", "message-id"):
    value = msg.get(key)
    if value is not None:
        defects += ["%s:%s" % (key, type(d).__name__) for d in getattr(value, "defects", ())]

parts = []
for part in msg.walk():
    if part.is_multipart():
        continue
    if part.get_content_disposition() == "attachment":
        data = part.get_payload(decode=True) or b""
        parts.append({"name": part.get_filename(), "bytes": len(data), "sha256": hashlib.sha256(data).hexdigest()})
    else:
        part.get_content()
for part in msg.walk():
    defects += [type(d).__name__ for d in part.defects]

unexpected = sorted(set(d for d in defects if d not in allowed))
if unexpected:
    print("FAIL defects: " + ", ".join(unexpected))
    sys.exit(1)
print("OK " + json.dumps({"subject": str(msg.get("subject")), "attachments": parts}, ensure_ascii=False))
`

// MailParser is mailparser, the parser behind nodemailer and much of the mail
// handling written in Node - the third language this format is read in.
//
// It lists no defects, measured on the same day: it read every broken message
// in silence, two of them with the wrong bytes. What it is here for is what it
// refuses that nobody else does - a message of more than a thousand MIME
// entities - and the names and bytes it reads back. Where it lives is the one
// thing node cannot find on its own: TFG_MAILPARSER names the installed
// package, and without it the guard skips loudly.
func MailParser(path string) Result {
	module := os.Getenv("TFG_MAILPARSER")
	return Checker{
		Name: "mailparser",
		find: func() (string, bool) {
			if module == "" {
				return "", false
			}
			return inPath("node")()
		},
		args:   func(p string) []string { return []string{"-e", mailParserScript, module, p} },
		accept: expectOK("mailparser"),
	}.Check(path)
}

const mailParserScript = `
const crypto = require("crypto");
const fs = require("fs");
const [module, path] = process.argv.slice(1);
const { simpleParser } = require(module);
simpleParser(fs.readFileSync(path)).then((m) => {
  const attachments = m.attachments.map((a) => ({
    name: a.filename === undefined ? null : a.filename,
    bytes: a.content.length,
    sha256: crypto.createHash("sha256").update(a.content).digest("hex"),
  }));
  console.log("OK " + JSON.stringify({ subject: m.subject, attachments }));
}).catch((e) => {
  console.log("FAIL " + e.message);
  process.exit(1);
});
`
