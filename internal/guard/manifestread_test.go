package guard

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/cli"
	"github.com/donislawdev/TestingFilesGenerator/internal/manifest"
)

// How a manifest is read, as apart from what it says.
//
// verify and cleanup read the whole document into memory and handed it to
// encoding/json, and some things about reading were true only because that is
// what encoding/json does: the keys may come in any order and any case,
// nothing may follow the document, a document with two faults is refused for
// the one that comes first in a fixed order, and a document with nothing in it
// gets the answer it always got. Nothing asked about any of them, because
// nothing could break them. A reader that takes the document a piece at a time
// (docs/MANIFEST-STREAMING-2026-11-04.md) can break every one, so each is said
// here against the reader it replaced, before the new one was written.

// keyed is one key of a document's outermost object and its value as written.
type keyed struct {
	key   string
	value json.RawMessage
}

// outermost is the outermost keys of a document in the order they are
// written, which a map cannot say.
func outermost(t *testing.T, raw []byte) []keyed {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(raw))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		t.Fatalf("the manifest does not open with an object: %v %v", tok, err)
	}
	var out []keyed
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			t.Fatalf("reading a key: %v", err)
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			t.Fatalf("reading the value of %v: %v", tok, err)
		}
		key, _ := tok.(string)
		out = append(out, keyed{key: key, value: value})
	}
	return out
}

// rewritten writes the manifest at path again from these keys, in this order.
func rewritten(t *testing.T, path string, keys []keyed) {
	t.Helper()
	var b bytes.Buffer
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		name, err := json.Marshal(k.key)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(name)
		b.WriteByte(':')
		b.Write(k.value)
	}
	b.WriteByte('}')
	if err := os.WriteFile(path, b.Bytes(), 0o644); err != nil {
		t.Fatalf("writing the manifest again: %v", err)
	}
}

// keysOf is the outermost keys of the manifest at path, in order.
func keysOf(t *testing.T, path string) []keyed {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the manifest: %v", err)
	}
	return outermost(t, raw)
}

// textFilesIn counts the .txt files in dir, which is what generated writes.
func textFilesIn(t *testing.T, dir string) int {
	t.Helper()
	n := 0
	for _, name := range entryNames(t, dir) {
		if strings.HasSuffix(name, ".txt") {
			n++
		}
	}
	return n
}

// The keys of a manifest may come in any order.
//
// This tool writes files after the version, and a reader that took that for
// granted would act on entries before it knew which schema they were written
// to - the mistake the check of the major in Load exists to rule out. A
// manifest that went through somebody's tool on the way, with its keys sorted,
// has files first.
func TestAManifestIsReadTheSameWhateverOrderItsKeysComeIn(t *testing.T) {
	out, mf := generated(t)
	var files, version keyed
	var between []keyed
	for _, k := range keysOf(t, mf) {
		switch k.key {
		case "files":
			files = k
		case "manifest_version":
			version = k
		default:
			between = append([]keyed{k}, between...)
		}
	}
	if files.key == "" || version.key == "" || len(between) == 0 {
		t.Fatalf("the manifest a run wrote lacks files, manifest_version or anything between them")
	}
	rewritten(t, mf, append(append([]keyed{files}, between...), version))

	// The state this is about, asked of the document rather than assumed.
	if got := keysOf(t, mf); got[0].key != "files" || got[len(got)-1].key != "manifest_version" {
		t.Fatalf("the manifest was not turned round: its keys are %v", got)
	}
	if code, stdout, errOut := run(t, "verify", mf); code != cli.ExitOK || !strings.Contains(stdout, "3 files checked") {
		t.Errorf("verify on a manifest whose files come first ended %d:\n%s%s", code, stdout, errOut)
	}
	if code, _, errOut := run(t, "cleanup", mf, "--yes"); code != cli.ExitOK {
		t.Errorf("cleanup on a manifest whose files come first ended %d:\n%s", code, errOut)
	}
	if n := textFilesIn(t, out); n != 0 {
		t.Errorf("cleanup left %d of the three files a manifest whose files come first lists", n)
	}
}

// The keys of a manifest may be written in any case, which is what
// encoding/json has always accepted, so a manifest somebody's tool wrote that
// way was read and has to go on being read.
func TestAManifestIsReadTheSameWhateverCaseItsKeysAreIn(t *testing.T) {
	_, mf := generated(t)
	keys := keysOf(t, mf)
	shouted := 0
	for i, k := range keys {
		if k.key == "files" || k.key == "manifest_version" {
			keys[i].key = strings.ToUpper(k.key)
			shouted++
		}
	}
	if shouted != 2 {
		t.Fatalf("expected to find files and manifest_version, found %d of them", shouted)
	}
	rewritten(t, mf, keys)
	if code, stdout, errOut := run(t, "verify", mf); code != cli.ExitOK || !strings.Contains(stdout, "3 files checked") {
		t.Errorf("verify on a manifest with FILES and MANIFEST_VERSION ended %d:\n%s%s", code, stdout, errOut)
	}
}

// Nothing may follow a manifest but white space.
//
// Two documents in one file are not a manifest, and a reader that stopped
// where the first one closed would act on half of something. encoding/json
// refuses this on its own. A reader that goes a piece at a time stops where
// the object closes unless it is made to ask what comes after.
func TestAManifestWithAnythingAfterItIsRefused(t *testing.T) {
	for _, tail := range []struct{ name, text string }{
		{"a second object", "{}"},
		{"a second copy of itself", ""},
		{"a stray letter", "x"},
		{"a closing brace too many", "}"},
	} {
		t.Run(tail.name, func(t *testing.T) {
			out, mf := generated(t)
			raw, err := os.ReadFile(mf)
			if err != nil {
				t.Fatal(err)
			}
			text := tail.text
			if text == "" {
				text = string(raw)
			}
			if err := os.WriteFile(mf, append(raw, []byte("\n"+text)...), 0o644); err != nil {
				t.Fatal(err)
			}
			code, stdout, errOut := run(t, "cleanup", mf, "--yes")
			if code != cli.ExitIO || stdout != "" || !strings.Contains(errOut, "cannot be read as a manifest") {
				t.Errorf("cleanup on a manifest followed by %s ended %d, expected %d and a refusal:\n%s%s", tail.name, code, cli.ExitIO, stdout, errOut)
			}
			if n := textFilesIn(t, out); n != 3 {
				t.Errorf("cleanup acted on a manifest followed by %s: %d of the three files are left", tail.name, n)
			}
			if code, _, errOut := run(t, "verify", mf); code != cli.ExitIO {
				t.Errorf("verify on a manifest followed by %s ended %d, expected %d:\n%s", tail.name, code, cli.ExitIO, errOut)
			}
		})
	}
}

// White space after a manifest is nothing. An editor that saves one ends it
// with a newline, and some with more than one.
func TestWhiteSpaceAfterAManifestIsNothing(t *testing.T) {
	_, mf := generated(t)
	raw, err := os.ReadFile(mf)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(mf, append(raw, []byte("\r\n\n\t  \n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if code, _, errOut := run(t, "verify", mf); code != cli.ExitOK {
		t.Errorf("verify on a manifest followed by white space ended %d:\n%s", code, errOut)
	}
}

// claimOf writes a file into dir and gives back a manifest entry that claims
// it, its size and hash right - an entry cleanup removes the file for.
func claimOf(t *testing.T, dir, name, content string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(content))
	return fmt.Sprintf(`{"path":%q,"name":%q,"materialized":true,"bytes":%d,"hashes":{"sha256":%q}}`,
		name, name, len(content), hex.EncodeToString(sum[:]))
}

// detailOf is what a refusal to read a manifest says is wrong with it - the
// part after the path, because the path of a guard's directory carries the
// name of the guard and the name may hold any word.
func detailOf(errOut string) string {
	_, detail, _ := strings.Cut(errOut, "cannot be read as a manifest: ")
	return detail
}

// A manifest that names one of its keys twice is refused whole.
//
// encoding/json keeps the last of the two, so a document with two lists of
// files was checked and acted on by its second list, and a reader going a
// piece at a time would have to read the first, check it and throw it away to
// do the same. Neither is an answer about a record of one run: this tool never
// writes a key twice, so a document that does was written by something else,
// and which of its two halves it meant is a guess. The second spelling counts
// as the same key, because encoding/json reads Files as files.
func TestAManifestThatNamesAKeyTwiceIsRefused(t *testing.T) {
	for _, c := range []struct{ name, doc, key string }{
		{"files twice", `{"manifest_version":"1.0","files":[],"files":[%s]}`, `"files"`},
		{"files under a second spelling", `{"manifest_version":"1.0","files":[],"Files":[%s]}`, `"Files"`},
		{"the version twice", `{"manifest_version":"9.0","manifest_version":"1.0","files":[%s]}`, `"manifest_version"`},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			mf := filepath.Join(dir, "manifest.json")
			doc := fmt.Sprintf(c.doc, claimOf(t, dir, "ours.txt", "written by the run\n"))
			if err := os.WriteFile(mf, []byte(doc), 0o644); err != nil {
				t.Fatal(err)
			}
			code, stdout, errOut := run(t, "cleanup", mf, "--yes")
			if code != cli.ExitIO || stdout != "" {
				t.Errorf("cleanup on a manifest with %s ended %d, expected %d:\n%s%s", c.name, code, cli.ExitIO, stdout, errOut)
			}
			if !strings.Contains(detailOf(errOut), c.key) {
				t.Errorf("the refusal does not name the key %s:\n%s", c.key, errOut)
			}
			if _, err := os.Stat(filepath.Join(dir, "ours.txt")); err != nil {
				t.Errorf("cleanup acted on a manifest with %s: %v", c.name, err)
			}
		})
	}
}

// escaping is a manifest entry whose path leaves the directory.
const escaping = `{"path":"../escaped.txt","materialized":true}`

// twoFaultDocuments is documents with two things wrong with them, what the
// refusal of each says and what it does not, because it is about the other.
var twoFaultDocuments = []struct {
	name, doc string
	says      []string
	not       []string
}{
	{"cut short after an entry that escapes", `{"manifest_version":"1.0","files":[` + escaping + `,`,
		nil, []string{"escaped", "manifest_version"}},
	{"a version of the wrong type after an entry that escapes", `{"files":[` + escaping + `],"manifest_version":1}`,
		[]string{"cannot unmarshal"}, []string{"escaped"}},
	{"no version and an entry that escapes", `{"files":[` + escaping + `]}`,
		[]string{"no manifest_version"}, []string{"escaped"}},
	{"a future version after an entry that escapes", `{"files":[` + escaping + `],"manifest_version":"9.0"}`,
		[]string{"9.0"}, []string{"escaped"}},
	{"two entries that escape, the first in a failed entry", `{"manifest_version":"1.0","files":[{"path":"fine.txt"},{"path":"../first.txt","failed":true},{"path":"/second.txt","materialized":true}]}`,
		[]string{"entry 2", "../first.txt"}, []string{"second"}},
	{"instructions that escape before an entry that escapes", `{"manifest_version":"1.0","run":{"instructions":"../x.instructions.md"},"files":[` + escaping + `]}`,
		[]string{"escaped"}, []string{"run.instructions"}},
}

// A document with two faults is refused for the one that comes first, in an
// order that does not depend on where in the document each one sits.
//
// What is wrong with the document as a document first, then the version, then
// the first entry that leaves the directory, counted over every entry, then
// the instructions. A reader that answered with whichever fault it met first
// would refuse one manifest in different words depending on the order of its
// keys - and say "an entry lands outside" about a document of a future schema,
// whose entries this build does not know how to read.
func TestAManifestWithTwoFaultsIsRefusedForTheOneThatComesFirst(t *testing.T) {
	for _, c := range twoFaultDocuments {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			mf := filepath.Join(dir, "manifest.json")
			if err := os.WriteFile(mf, []byte(c.doc), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, command := range []string{"verify", "cleanup"} {
				code, stdout, errOut := run(t, command, mf)
				detail := detailOf(errOut)
				if code != cli.ExitIO || stdout != "" || detail == "" {
					t.Errorf("%s on a document %s ended %d, expected %d and a refusal to read it:\n%s%s", command, c.name, code, cli.ExitIO, stdout, errOut)
					continue
				}
				for _, s := range c.says {
					if !strings.Contains(detail, s) {
						t.Errorf("%s refused a document %s without saying %q:\n%s", command, c.name, s, detail)
					}
				}
				for _, s := range c.not {
					if strings.Contains(detail, s) {
						t.Errorf("%s refused a document %s for a fault that comes later, saying %q:\n%s", command, c.name, s, detail)
					}
				}
			}
		})
	}
}

// nearlyEmptyDocuments is documents with nothing in them, or nothing that is a
// manifest, and the code verify ends with on each.
var nearlyEmptyDocuments = []struct {
	name, doc string
	code      int
}{
	{"no bytes at all", ``, cli.ExitIO},
	{"an array", `[]`, cli.ExitIO},
	{"a string", `"manifest"`, cli.ExitIO},
	{"null", `null`, cli.ExitIO},
	{"an empty object", `{}`, cli.ExitIO},
	{"a byte order mark in front", "\uFEFF" + `{"manifest_version":"1.0","files":[]}`, cli.ExitIO},
	{"files that are an object", `{"manifest_version":"1.0","files":{}}`, cli.ExitIO},
	{"an entry that is a number", `{"manifest_version":"1.0","files":[1]}`, cli.ExitIO},
	{"an entry that is null", `{"manifest_version":"1.0","files":[null]}`, cli.ExitIO},
	{"an entry with no path", `{"manifest_version":"1.0","files":[{"materialized":true}]}`, cli.ExitIO},
	{"no files key", `{"manifest_version":"1.0"}`, cli.ExitOK},
	{"files that are null", `{"manifest_version":"1.0","files":null}`, cli.ExitOK},
	{"only entries the run never wrote", `{"manifest_version":"1.0","files":[{"path":"a.txt","failed":true},{"path":"b.txt"}]}`, cli.ExitOK},
}

// A document with nothing in it, or nothing that is a manifest, gets the
// answer it always got: refused as a document when it is not an object with a
// version, and nothing to check when it is one that lists nothing on the disk.
func TestAManifestWithNothingInItIsAnsweredAsItAlwaysWas(t *testing.T) {
	for _, c := range nearlyEmptyDocuments {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			mf := filepath.Join(dir, "manifest.json")
			if err := os.WriteFile(mf, []byte(c.doc), 0o644); err != nil {
				t.Fatal(err)
			}
			code, stdout, errOut := run(t, "verify", mf)
			if code != c.code {
				t.Fatalf("verify on %s ended %d, expected %d:\n%s%s", c.name, code, c.code, stdout, errOut)
			}
			if c.code == cli.ExitIO && (stdout != "" || detailOf(errOut) == "") {
				t.Errorf("verify on %s did not refuse it as a manifest it cannot read:\n%s%s", c.name, stdout, errOut)
			}
			if c.code == cli.ExitOK && !strings.Contains(errOut, "nothing to check") {
				t.Errorf("verify on %s did not say there was nothing to check:\n%s", c.name, errOut)
			}
		})
	}
}

// Another run's record is another run's record however long it is.
//
// The records of the other runs in a directory were read through the door that
// refused a manifest past sixteen megabytes, and a record that is refused is
// not a record - so the files of a run past that size were called extra in the
// directory they share, the word for files nobody asked for. Nothing said so,
// because no guard had a neighbour that large.
func TestAnotherRunsRecordIsAnotherRunsRecordHoweverLongItIs(t *testing.T) {
	out, mf := generated(t)
	theirs := claimOf(t, out, "theirs.bin", "the other run wrote this\n")

	// One file of its own and then enough it never wrote to pass the old
	// ceiling.
	record := filepath.Join(out, "other.json")
	if err := os.WriteFile(record, []byte(longManifest(theirs, manifest.MaxBytes)), 0o644); err != nil {
		t.Fatal(err)
	}
	// The state this is about: the record is longer than the old reader took.
	if info, err := os.Stat(record); err != nil || info.Size() <= manifest.MaxBytes {
		t.Fatalf("the neighbour's record is not past %d B: %v %v", manifest.MaxBytes, info, err)
	}

	code, stdout, errOut := run(t, "verify", mf, "--json")
	report := stdout
	if code != cli.ExitOK {
		report = errOut
		t.Errorf("verify called the directory a mismatch because of another run's files, exit %d", code)
	}
	var got struct {
		Differences []struct{ Kind, Path, Expected string } `json:"differences"`
	}
	if err := json.Unmarshal([]byte(report), &got); err != nil {
		t.Fatalf("reading the report: %v\n%s", err, report)
	}
	found := false
	for _, d := range got.Differences {
		if d.Path == "theirs.bin" {
			found = true
			if d.Kind != "another-run" || d.Expected != "other.json" {
				t.Errorf("the other run's file was reported as %q (%q), expected another-run claimed by other.json", d.Kind, d.Expected)
			}
		}
	}
	if !found {
		t.Errorf("the other run's file is not in the report at all:\n%s", report)
	}
}

// longManifest is a manifest whose list opens with first and goes on, in
// entries of a kilobyte that claim nothing - as long as the entries of a real
// run are - until the document is longer than past.
func longManifest(first string, past int) string {
	var b strings.Builder
	b.WriteString(`{"manifest_version":"1.0","files":[` + first)
	padding := strings.Repeat("p", 1000)
	for i := 0; b.Len() <= past; i++ {
		fmt.Fprintf(&b, `,{"path":"never_%07d.txt","materialized":false,"padding":%q}`, i, padding)
	}
	b.WriteString("]}")
	return b.String()
}
