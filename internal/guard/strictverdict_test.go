package guard

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/oracle"
)

// strictCase is one file handed to the structural checker, with what it has
// to answer.
type strictCase struct {
	name   string
	format string
	body   []byte
	// verdict is the answer wanted, and reason a piece of what it has to say.
	// The reason is asked for because a refusal for some other cause would
	// pass a guard that read the verdict alone.
	verdict string
	reason  string
}

// TestTheStructuralCheckerRefusesBrokenBytesInsteadOfStopping holds the three
// faults of the checker found when it became the judge of what a damage may be
// offered for (docs/DAMAGE-ETAP-2-2026-11-09.md section 3):
//
//   - an archive whose first bytes are zeroed was accepted, because the check
//     read the end records and never followed them to a local header
//   - an Office package with bytes after its end record was accepted, because
//     the ZIP reader underneath finds that record by searching for it
//   - a PNG cut short and a package whose part cannot be read stopped the
//     check with an exception, and an exception was counted as a refusal
//
// Each broken file is made from a healthy one this tool writes, and the
// healthy ones are asked first: a checker that refused everything would pass
// the rest of this guard.
func TestTheStructuralCheckerRefusesBrokenBytesInsteadOfStopping(t *testing.T) {
	png := writeEncoded(t, "png", 2048, nil)
	zip := writeEncoded(t, "zip", 16384, nil)
	docx := writeEncoded(t, "docx", 4096, nil)
	xlsx := writeEncoded(t, "xlsx", 4096, nil)
	pptx := writeEncoded(t, "pptx", 8192, nil)
	pdf := writeEncoded(t, "pdf", 4096, nil)
	html := writeEncoded(t, "html", 2048, nil)

	cases := []strictCase{
		{"healthy png", "png", png, oracle.StrictOK, ""},
		{"healthy zip", "zip", zip, oracle.StrictOK, ""},
		{"healthy docx", "docx", docx, oracle.StrictOK, ""},
		{"healthy xlsx", "xlsx", xlsx, oracle.StrictOK, ""},
		{"healthy pptx", "pptx", pptx, oracle.StrictOK, ""},
		{"healthy pdf", "pdf", pdf, oracle.StrictOK, ""},
		{"healthy html", "html", html, oracle.StrictOK, ""},

		{"png with nothing after the signature", "png", png[:8], oracle.StrictFail, "no chunk at all"},
		{"png one byte short", "png", png[:len(png)-1], oracle.StrictFail, "before its 4 B checksum"},
		{"zip with a zeroed head", "zip", zeroed(zip, 4), oracle.StrictFail, "no local file header"},
		{"zip whose local header names another file", "zip", renamed(t, zip), oracle.StrictFail, "and the central directory names"},
		{"docx with a zeroed head", "docx", zeroed(docx, 4), oracle.StrictFail, "no local file header"},
		{"xlsx with a zeroed head", "xlsx", zeroed(xlsx, 4), oracle.StrictFail, "no local file header"},
		{"pptx with a zeroed head", "pptx", zeroed(pptx, 4), oracle.StrictFail, "no local file header"},
		{"docx with bytes after its end", "docx", appended(docx, 64), oracle.StrictFail, "the comment declares 0 B and 64 follow"},
		{"docx with a part that does not match its checksum", "docx", flipped(t, docx), oracle.StrictFail, "cannot be read from the archive"},
		{"docx with a compression method nobody knows", "docx", unknownMethod(t, docx), oracle.StrictFail, "compression method"},
		{"pdf whose cross reference names an offset that is not a number", "pdf", garbledOffset(t, pdf), oracle.StrictFail, "which is not a number"},
		{"html whose label heading is never closed", "html", unclosedLabel(t, html), oracle.StrictFail, "never closed on its line"},
	}

	dir := t.TempDir()
	reqs := make([]oracle.StrictRequest, len(cases))
	for i, c := range cases {
		path := filepath.Join(dir, strings.ReplaceAll(c.name, " ", "_")+"."+c.format)
		if err := os.WriteFile(path, c.body, 0o600); err != nil {
			t.Fatal(err)
		}
		reqs[i] = oracle.StrictRequest{Format: c.format, Path: path}
	}

	got, available, err := oracle.StrictBatch(t.Context(), reqs)
	if !available {
		t.Skip("the structural checker needs python, and there is none on this machine")
	}
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range cases {
		j := got[i]
		if j.Verdict != c.verdict || !strings.Contains(j.Reason, c.reason) {
			t.Errorf("%s: the checker answered %s %q, and %s saying %q was wanted.\n"+
				"Reason: the checker decides which damage is offered for which format, so a\n"+
				"refusal has to come from the defect itself. A crash is a fault in the\n"+
				"checker, never a refusal of the file.", c.name, j.Verdict, j.Reason, c.verdict, c.reason)
		}
	}
}

// strictPlant is run by the guard below. It loads the checker, puts checks that
// misbehave in place of real ones and asks through the same path --batch
// takes, printing one answer per line.
const strictPlant = `import importlib.util, json, sys
sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location("tfg_strict", sys.argv[1])
strict = importlib.util.module_from_spec(spec)
spec.loader.exec_module(strict)

def raises(data):
    raise ValueError("planted")

def says_nothing(data):
    return None

def refuses_inside_a_handler(data):
    try:
        strict.fail("planted refusal")
    except Exception:
        pass
    strict.ok("the refusal was swallowed")

strict.CHECKS["png"] = raises
strict.CHECKS["gif"] = says_nothing
strict.CHECKS["bmp"] = refuses_inside_a_handler
for kind in ("png", "gif", "bmp"):
    print(json.dumps(strict.batch_answer(json.dumps({"format": kind, "path": sys.argv[1]}))))
`

// TestACheckThatStopsOnAnExceptionIsACrashNotARefusal holds the line the
// witness of a damage stands on. The checker answers CRASH when a check stops
// on an exception or ends without a verdict, and never FAIL. Counted as a
// refusal, a checker that falls over on every damaged file would witness every
// damage (docs/DAMAGE-ETAP-2-2026-11-09.md section 3.3).
//
// The third check refuses inside its own `except Exception` and carries on. A
// verdict has to stop the check anyway, which is why it is raised as a
// SystemExit rather than an ordinary exception.
//
// Planted checks, because the real ones are meant to have no crash left in
// them, so no real file can reach these states.
func TestACheckThatStopsOnAnExceptionIsACrashNotARefusal(t *testing.T) {
	python, script := strictProgram(t)
	plant := filepath.Join(t.TempDir(), "plant.py")
	if err := os.WriteFile(plant, []byte(strictPlant), 0o600); err != nil {
		t.Fatal(err)
	}
	// The interpreter is the one found on PATH, the script is the plant this
	// guard just wrote, and its one argument is the checker of this repository.
	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	out, err := exec.Command(python, plant, script).CombinedOutput()
	if err != nil {
		t.Fatalf("the planted checks did not run: %v\n%s", err, out)
	}

	want := []struct{ verdict, reason string }{
		{oracle.StrictCrash, "ValueError: planted"},
		{oracle.StrictCrash, "the check returned without a verdict"},
		{oracle.StrictFail, "planted refusal"},
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != len(want) {
		t.Fatalf("%d answer(s) for %d planted checks:\n%s", len(lines), len(want), out)
	}
	for i, w := range want {
		var j oracle.StrictJudgement
		if err := json.Unmarshal([]byte(strings.TrimSpace(lines[i])), &j); err != nil {
			t.Fatalf("answer %d is not an answer: %v: %s", i+1, err, lines[i])
		}
		if j.Verdict != w.verdict || !strings.Contains(j.Reason, w.reason) {
			t.Errorf("planted check %d was answered %s %q, and %s saying %q was wanted.\n"+
				"Reason: a check that falls over is a fault in the checker. It must never\n"+
				"read as a refusal, and a refusal must never be swallowed by a check's own\n"+
				"error handling.", i+1, j.Verdict, j.Reason, w.verdict, w.reason)
		}
	}
}

// TestTheStructuralCheckerAnswersARequestItCannotActOnWithAnError holds the
// other side of the batch: a request the checker cannot act on is answered
// ERROR, in its place, and every request after it is still answered.
//
// Through StrictBatch for what a Go caller can send (a format with no check,
// settings to a check that takes none, a file that is not there, a path with
// a NUL in it), and through the program itself for lines no Go caller writes
// (a line that is not JSON, JSON nested past what the decoder goes). Before
// 2026-11-10 the last two of the first group and the nested line stopped the
// whole batch. The healthy file between them is the control: a batch that
// answered ERROR to everything would pass the rest.
func TestTheStructuralCheckerAnswersARequestItCannotActOnWithAnError(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.zip")
	if err := os.WriteFile(good, writeEncoded(t, "zip", 16384, nil), 0o600); err != nil {
		t.Fatal(err)
	}
	asked := []struct {
		req    oracle.StrictRequest
		answer string
		reason string
	}{
		{oracle.StrictRequest{Format: "nothing-of-that-name", Path: good}, oracle.StrictError, "there is no check"},
		{oracle.StrictRequest{Format: "zip", Path: good, Settings: map[string]string{"a": "b"}}, oracle.StrictError, "takes no settings"},
		{oracle.StrictRequest{Format: "zip", Path: good}, oracle.StrictOK, ""},
		{oracle.StrictRequest{Format: "zip", Path: filepath.Join(dir, "never-written.zip")}, oracle.StrictError, "cannot be read"},
		{oracle.StrictRequest{Format: "zip", Path: good + "\x00.zip"}, oracle.StrictError, "cannot be read"},
		{oracle.StrictRequest{Format: "zip", Path: good}, oracle.StrictOK, ""},
	}
	reqs := make([]oracle.StrictRequest, len(asked))
	for i, a := range asked {
		reqs[i] = a.req
	}
	got, available, err := oracle.StrictBatch(t.Context(), reqs)
	if !available {
		t.Skip("the structural checker needs python, and there is none on this machine")
	}
	if err != nil {
		t.Fatalf("one request the checker cannot act on stopped the whole batch: %v", err)
	}
	for i, a := range asked {
		if got[i].Verdict != a.answer || !strings.Contains(got[i].Reason, a.reason) {
			t.Errorf("request %d (%s %q) was answered %s %q, and %s saying %q was wanted",
				i+1, a.req.Format, a.req.Path, got[i].Verdict, got[i].Reason, a.answer, a.reason)
		}
	}

	python, script := strictProgram(t)
	lines := []string{
		"this is not a request",
		strings.Repeat("[", 200000) + strings.Repeat("]", 200000),
		`{"format": "zip", "path": ` + strconv.Quote(good) + `}`,
	}
	// The interpreter is the one found on PATH, the script is the checker of
	// this repository, and the input is the lines above.
	// nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd := exec.CommandContext(t.Context(), python, script, "--batch")
	cmd.Stdin = strings.NewReader(strings.Join(lines, "\n") + "\n")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("a line the checker cannot read stopped the whole batch: %v", err)
	}
	answers := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(answers) != len(lines) {
		t.Fatalf("%d line(s) asked and %d answered:\n%s", len(lines), len(answers), out)
	}
	for i, want := range []string{oracle.StrictError, oracle.StrictError, oracle.StrictOK} {
		var j oracle.StrictJudgement
		if err := json.Unmarshal([]byte(strings.TrimSpace(answers[i])), &j); err != nil {
			t.Fatalf("answer %d is not an answer: %v", i+1, err)
		}
		if j.Verdict != want {
			t.Errorf("line %d was answered %s %q, and %s was wanted", i+1, j.Verdict, j.Reason, want)
		}
	}
}

// TestAStoppedCallerStopsTheStructuralChecker holds that the checker runs
// under its caller's context. A guard stopped by its time limit has to stop
// the Python process with it, rather than leave it running under a context
// of its own.
func TestAStoppedCallerStopsTheStructuralChecker(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "good.zip")
	if err := os.WriteFile(good, writeEncoded(t, "zip", 16384, nil), 0o600); err != nil {
		t.Fatal(err)
	}
	reqs := []oracle.StrictRequest{{Format: "zip", Path: good}}

	// The control: the same request under a live context is answered.
	if _, available, err := oracle.StrictBatch(t.Context(), reqs); !available {
		t.Skip("the structural checker needs python, and there is none on this machine")
	} else if err != nil {
		t.Fatalf("the request was not answered under a live context: %v", err)
	}

	stopped, cancel := context.WithCancel(t.Context())
	cancel()
	if got, _, err := oracle.StrictBatch(stopped, reqs); err == nil {
		t.Errorf("a stopped caller got %d answer(s) back. The checker ran under a context of its own, "+
			"so a guard that is stopped leaves it running", len(got))
	}
}

// strictProgram is the interpreter on PATH and the checker of this
// repository, for a guard that runs the program itself.
func strictProgram(t *testing.T) (python, script string) {
	t.Helper()
	python, err := exec.LookPath("python")
	if err != nil {
		t.Skip("the structural checker needs python, and there is none on this machine")
	}
	here, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	script, ok := oracle.StrictScriptUnder(here)
	if !ok {
		t.Fatalf("the structural checker is not above %s", here)
	}
	return python, script
}

// zeroed is the file with its first n bytes set to zero.
func zeroed(b []byte, n int) []byte {
	out := append([]byte{}, b...)
	clear(out[:n])
	return out
}

// appended is the file with n bytes of a fixed pattern after its end.
func appended(b []byte, n int) []byte {
	out := append([]byte{}, b...)
	for i := range n {
		out = append(out, byte(i*37+11))
	}
	return out
}

// firstEntry finds the first local file header of an archive, and where its
// name and its data start.
func firstEntry(t *testing.T, b []byte) (name, data int) {
	t.Helper()
	at := bytes.Index(b, []byte("PK\x03\x04"))
	if at < 0 || at+30 > len(b) {
		t.Fatal("the archive has no local file header to change, so this case would test nothing")
	}
	nameLen := int(binary.LittleEndian.Uint16(b[at+26:]))
	extraLen := int(binary.LittleEndian.Uint16(b[at+28:]))
	if nameLen == 0 || at+30+nameLen+extraLen >= len(b) {
		t.Fatal("the first entry has no name or no data, so this case would test nothing")
	}
	return at + 30, at + 30 + nameLen + extraLen
}

// renamed is the archive with one letter of the name in its first local
// header changed, so the header and the central directory disagree.
func renamed(t *testing.T, b []byte) []byte {
	out := append([]byte{}, b...)
	name, _ := firstEntry(t, out)
	out[name] ^= 0x01
	return out
}

// unknownMethod is the package with the compression method of its first
// central directory entry set to one no reader knows.
func unknownMethod(t *testing.T, b []byte) []byte {
	out := append([]byte{}, b...)
	at := bytes.Index(out, []byte("PK\x01\x02"))
	if at < 0 || at+12 > len(out) {
		t.Fatal("the package has no central directory entry to change, so this case would test nothing")
	}
	binary.LittleEndian.PutUint16(out[at+10:], 77)
	return out
}

// garbledOffset is the PDF with the first digit of object 1's offset in its
// cross reference table replaced by a letter.
func garbledOffset(t *testing.T, b []byte) []byte {
	out := append([]byte{}, b...)
	// On a line of its own, because "startxref" near the end holds the word too.
	at := bytes.LastIndex(out, []byte("\nxref\n"))
	if at >= 0 {
		at++
	}
	// Past the keyword, the count line and the free entry of object 0.
	for range 3 {
		next := bytes.IndexByte(out[max(at, 0):], '\n')
		if at < 0 || next < 0 {
			t.Fatal("the PDF has no cross reference entry for object 1, so this case would test nothing")
		}
		at += next + 1
	}
	if at >= len(out) || out[at] < '0' || out[at] > '9' {
		t.Fatal("the entry for object 1 does not start with a digit, so this case would test nothing")
	}
	out[at] = 'x'
	return out
}

// unclosedLabel is the page with the end tag of its label heading taken off.
func unclosedLabel(t *testing.T, b []byte) []byte {
	at := bytes.Index(b, []byte("</h1>"))
	if at < 0 {
		t.Fatal("the page has no label heading, so this case would test nothing")
	}
	return append(append([]byte{}, b[:at]...), b[at+len("</h1>"):]...)
}

// flipped is the package with one byte of the first part's data changed, so
// the part no longer matches the checksum its headers carry.
func flipped(t *testing.T, b []byte) []byte {
	out := append([]byte{}, b...)
	_, data := firstEntry(t, out)
	out[data] ^= 0xff
	return out
}
