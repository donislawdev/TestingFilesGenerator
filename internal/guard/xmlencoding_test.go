package guard

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/textenc"
	"github.com/donislawdev/TestingFilesGenerator/internal/oracle"
)

// XML gained an encoding on 2026-09-08, and it is the first format here whose
// file SAYS which encoding it is in. TXT and MD carry that fact only in their
// bytes, so nothing inside them can disagree with anything. An XML declaration
// can, and a document announcing UTF-8 while holding UTF-16 is the classic
// parser trap.
//
// The measurement that made this worth building: expat refuses the mismatch in
// both directions - "encoding specified in XML declaration is incorrect" - so
// there is a witness outside this project. These two guards are the half that
// runs without one. See docs/XML-ENCODING-2026-09-08.md.

// asDeclared widens an ASCII string the way the encoding under test would, so
// the expected declaration is built rather than written out three times.
func asDeclared(s string, c encodingCase) []byte {
	if c.width == 1 {
		return []byte(s)
	}
	out := make([]byte, 0, len(s)*2)
	for _, b := range []byte(s) {
		if c.encoding == textenc.UTF16BE {
			out = append(out, 0x00, b)
		} else {
			out = append(out, b, 0x00)
		}
	}
	return out
}

// TestTheXMLDeclarationNamesTheEncodingTheBytesAreIn is the whole point of the
// setting in one sentence.
//
// Deliberately not asking Python. The structural check does ask, and it is the
// stronger reader - but it is skipped wherever Python is missing, and the one
// thing this change can break should not be provable only on a machine that
// happens to have an interpreter.
//
// Built from the bytes rather than decoded, because a decoder would have to be
// told which encoding to expect and would then be agreeing with itself.
func TestTheXMLDeclarationNamesTheEncodingTheBytesAreIn(t *testing.T) {
	d, err := format.Get("xml")
	if err != nil {
		t.Fatal(err)
	}

	checked := 0
	for _, c := range encodingCases() {
		if c.refusedBy("xml") {
			continue
		}

		want := `<?xml version="1.0" encoding="UTF-8"?>`
		other := `<?xml version="1.0" encoding="UTF-16"?>`
		if c.width == 2 {
			want, other = other, want
		}

		floor := d.SmallestAccepted(format.Request{Seed: 7741, Label: true, Properties: c.props()})
		for _, size := range []int64{floor, floor + int64(c.width)*512} {
			body := writeEncoded(t, "xml", size, c.props())

			head := append(append([]byte{}, c.mark...), asDeclared(want, c)...)
			if !bytes.HasPrefix(body, head) {
				t.Errorf("xml %v at %d B: the file does not open with %q in its own encoding, it opens with % x",
					c, size, want, first(body, len(head)))
			}
			// The control. Without it a generator writing BOTH declarations, or
			// one writing the right bytes for the wrong reason, would pass the
			// line above.
			wrong := append(append([]byte{}, c.mark...), asDeclared(other, c)...)
			if bytes.HasPrefix(body, wrong) {
				t.Errorf("xml %v at %d B: the declaration says %q and the bytes are %s",
					c, size, other, c.encoding)
			}
			checked++
		}
	}

	if checked == 0 {
		t.Fatal("no combination was checked, so this guard proves nothing")
	}
	t.Logf("%d file(s) opened with a declaration matching their bytes", checked)
}

// TestXMLRefusesAWideEncodingWithoutAMark proves the entry in markRequired is a
// behaviour rather than an excuse for skipping two cases.
//
// The specification requires a mark on a UTF-16 entity. The reason it is
// REFUSED rather than merely discouraged is that nothing here could catch it:
// expat accepts a UTF-16 document with no mark and reads it correctly, measured
// 2026-09-08, so the oracle beside this cannot go red on one. A file this tool
// cannot check is a file it does not write, and a fixture that breaks a
// specification on purpose belongs to the chaos lab, which does not exist yet.
func TestXMLRefusesAWideEncodingWithoutAMark(t *testing.T) {
	d, err := format.Get("xml")
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{textenc.UTF16LE, textenc.UTF16BE} {
		props := map[string]string{textenc.Setting: name, textenc.SettingBOM: "false"}
		_, err := d.Generator.Plan(format.Request{Bytes: 4096, Seed: 7741, Label: true, Properties: props})
		if err == nil {
			t.Errorf("%s without a mark was accepted, and the specification requires one", name)
			continue
		}

		// The refusal has to name both halves. One saying only "bom cannot be
		// false" sends somebody to turn a setting on without saying why, and
		// one naming only the encoding hides which knob to reach for.
		msg := err.Error()
		for _, part := range []string{textenc.SettingBOM, name, "byte order mark"} {
			if !bytes.Contains([]byte(msg), []byte(part)) {
				t.Errorf("%s: the refusal does not mention %q: %s", name, part, msg)
			}
		}

		// The control: the same encoding WITH a mark is accepted, so the
		// refusal is about the missing mark rather than about UTF-16.
		props[textenc.SettingBOM] = "true"
		if _, err := d.Generator.Plan(format.Request{
			Bytes: 4096, Seed: 7741, Label: true, Properties: props}); err != nil {
			t.Errorf("%s with a mark was refused too, so the refusal is not about the mark: %v", name, err)
		}
	}
}

// TestTheStructuralCheckRefusesADeclarationThatDisagreesWithTheBytes is a
// canary, and it exists because of what the check beside it cannot prove.
//
// Every other guard here hands the structural check a CORRECT file, so all of
// them would stay green if that check stopped looking at the declaration
// entirely. That is the worst shape a mutation takes in this project: the
// pattern is found, the code compiles, and the broken text never reaches an
// assertion. The only way to know the check works is to hand it something
// wrong and watch it refuse.
//
// A rewritten declaration rather than a differently generated file, because
// this tool cannot be asked to produce one - which is the point.
func TestTheStructuralCheckRefusesADeclarationThatDisagreesWithTheBytes(t *testing.T) {
	dir := t.TempDir()
	told := []string{textenc.Setting + "=" + textenc.UTF8, textenc.SettingBOM + "=false"}

	body := writeEncoded(t, "xml", 4096, map[string]string{
		textenc.Setting: textenc.UTF8, textenc.SettingBOM: "false"})

	truth := []byte(`<?xml version="1.0" encoding="UTF-8"?>`)
	lie := []byte(`<?xml version="1.0" encoding="UTF-16"?>`)
	if !bytes.HasPrefix(body, truth) {
		t.Fatalf("this canary rewrites the declaration and the file does not open with the one it expects: % x",
			first(body, len(truth)))
	}
	lying := append(append([]byte{}, lie...), body[len(truth):]...)

	honest := filepath.Join(dir, "honest.xml")
	broken := filepath.Join(dir, "lying.xml")
	if err := os.WriteFile(honest, body, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(broken, lying, 0o600); err != nil {
		t.Fatal(err)
	}

	// The control first. A check that refused everything would pass the half
	// below without seeing anything at all.
	res := oracle.Strict("xml", honest, told...)
	if !res.Available {
		t.Skip("the structural check needs python")
	}
	if res.Err != nil {
		t.Fatalf("the untouched file was refused, so this canary is measuring something else: %v", res.Err)
	}

	if res := oracle.Strict("xml", broken, told...); res.Err == nil {
		t.Error("a document holding UTF-8 while announcing UTF-16 was called correct, so nothing here can see the one defect this setting is able to cause")
	}
}
