package guard

// What a text file claims about itself, and whether it is that.
//
// TXT and MD gained an encoding on 2026-09-07, and it is the first setting in
// this project that changes which SIZES exist rather than what fills them. A
// UTF-16 file is a whole number of sixteen bit units, so half of all sizes stop
// being reachable - and the exact size promise turns those into refusals rather
// than into files that are one byte out.
//
// The measurement that shaped all of this, on three readers in three languages:
// a UTF-16 file cut to an odd length is rejected by Python, by V8 and by .NET
// when each is asked strictly, and repaired in silence by all three when it is
// not. Get-Content shows the cut file and the whole one identically. So the
// file this tool must never write is exactly the one a person could not tell
// apart by looking.

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/textenc"
	"github.com/donislawdev/TestingFilesGenerator/internal/oracle"
)

// encodedFormats is the formats that take an encoding, named rather than
// derived - so a third one arriving without being added here is a gap somebody
// has to notice rather than a loop that quietly gets shorter.
var encodedFormats = []string{"txt", "md"}

type encodingCase struct {
	encoding string
	bom      bool
	mark     []byte
	width    int64
}

func encodingCases() []encodingCase {
	return []encodingCase{
		{textenc.UTF8, false, nil, 1},
		{textenc.UTF8, true, []byte{0xEF, 0xBB, 0xBF}, 1},
		{textenc.UTF16LE, false, nil, 2},
		{textenc.UTF16LE, true, []byte{0xFF, 0xFE}, 2},
		{textenc.UTF16BE, false, nil, 2},
		{textenc.UTF16BE, true, []byte{0xFE, 0xFF}, 2},
	}
}

func (c encodingCase) props() map[string]string {
	return map[string]string{
		textenc.Setting:    c.encoding,
		textenc.SettingBOM: fmt.Sprintf("%t", c.bom),
	}
}

func (c encodingCase) settings() []string {
	return []string{
		textenc.Setting + "=" + c.encoding,
		textenc.SettingBOM + "=" + fmt.Sprintf("%t", c.bom),
	}
}

func (c encodingCase) String() string {
	return fmt.Sprintf("%s/bom=%t", c.encoding, c.bom)
}

// writeEncoded produces one file and hands back its bytes, insisting on the
// ordered size before anything else looks at it.
func writeEncoded(t *testing.T, id string, size int64, props map[string]string) []byte {
	t.Helper()
	d, err := format.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	p, err := d.Generator.Plan(format.Request{Bytes: size, Seed: 7741, Label: true, Properties: props})
	if err != nil {
		t.Fatalf("%s: planning %d B with %v: %v", id, size, props, err)
	}
	var buf bytes.Buffer
	if err := d.Generator.Write(context.Background(), &buf, p); err != nil {
		t.Fatalf("%s: writing %d B with %v: %v", id, size, props, err)
	}
	if int64(buf.Len()) != size {
		t.Fatalf("%s %v: ordered %d B and produced %d - the size is exact or it is an error",
			id, props, size, buf.Len())
	}
	return buf.Bytes()
}

// TestATextFileIsTheEncodingItDeclares is the whole claim in one place: the
// size is exact, the mark is there when it was ordered and absent when it was
// not, and somebody else's decoder agrees the bytes are what they say.
//
// The decoder is TOLD which encoding to expect rather than left to work it out.
// A checker that sniffed would decode a UTF-16 file as UTF-16 whatever was
// ordered and call a file written in the wrong encoding correct, which is the
// one question this has to answer.
func TestATextFileIsTheEncodingItDeclares(t *testing.T) {
	dir := t.TempDir()
	checked, skipped := 0, 0

	for _, id := range encodedFormats {
		d, err := format.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range encodingCases() {
			smallest := d.SmallestAccepted(format.Request{Seed: 7741, Label: true, Properties: c.props()})
			if c.width == 2 && smallest%2 != 0 {
				t.Errorf("%s %v: the smallest size it accepts is %d, which a two byte encoding cannot write",
					id, c, smallest)
			}

			for _, size := range []int64{smallest, smallest + c.width, 4096, 40960} {
				name := fmt.Sprintf("%s/%s/%d", id, c, size)
				t.Run(name, func(t *testing.T) {
					body := writeEncoded(t, id, size, c.props())

					if !bytes.HasPrefix(body, c.mark) {
						t.Fatalf("a %s mark was ordered and the file opens with % x", c.encoding, first(body, 4))
					}
					if !c.bom && startsWithAnyMark(body) {
						t.Fatalf("no mark was ordered and the file opens with % x", first(body, 4))
					}

					path := filepath.Join(dir, fmt.Sprintf("s%d.%s", size, id))
					if err := os.WriteFile(path, body, 0o600); err != nil {
						t.Fatal(err)
					}
					res := oracle.Strict(id, path, c.settings()...)
					if !res.Available {
						skipped++
						t.Skip("the structural check needs python")
					}
					if res.Err != nil {
						t.Fatalf("%s is not %s: %v", id, c.encoding, res.Err)
					}
					checked++
				})
			}
		}
	}

	if checked == 0 {
		t.Errorf("nothing was decoded by anything outside this package - %d case(s) skipped", skipped)
	}
	t.Logf("%d file(s) decoded strictly by Python, %d skipped", checked, skipped)
}

// TestAWideEncodingRefusesAnOddSizeAndNamesOneItCanWrite is the refusal, and
// the control beside it is what makes it mean anything.
//
// The same odd size is accepted under UTF-8, so the refusal is about the
// ENCODING rather than about the number. Without that half, a generator that
// refused every odd size in every encoding would pass this.
func TestAWideEncodingRefusesAnOddSizeAndNamesOneItCanWrite(t *testing.T) {
	odd := []int64{4001, 65, 1235}

	for _, id := range encodedFormats {
		d, err := format.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range encodingCases() {
			for _, size := range odd {
				_, err := d.Generator.Plan(format.Request{
					Bytes: size, Seed: 7741, Label: true, Properties: c.props()})

				if c.width == 1 {
					if err != nil {
						t.Errorf("%s %v: %d B is a size a one byte encoding can write, and it was refused: %v",
							id, c, size, err)
					}
					continue
				}

				var below *format.BelowMinimumError
				if !errors.As(err, &below) {
					t.Errorf("%s %v: %d B cannot be written and the answer was %v, not a BelowMinimumError",
						id, c, size, err)
					continue
				}
				if below.Minimum != size+1 {
					t.Errorf("%s %v: refusing %d B named %d as the next size it can write",
						id, c, size, below.Minimum)
				}
				// A refusal naming a size it also cannot write would be worse
				// than no refusal, so the number it gives is taken up.
				if _, err := d.Generator.Plan(format.Request{
					Bytes: below.Minimum, Seed: 7741, Label: true, Properties: c.props()}); err != nil {
					t.Errorf("%s %v: refusing %d B pointed at %d B, and that is refused too: %v",
						id, c, size, below.Minimum, err)
				}
				// The four parts every refusal in this tool carries.
				if !strings.Contains(below.Reason, "two bytes") {
					t.Errorf("%s %v: the reason does not say why an odd size cannot exist: %q", id, c, below.Reason)
				}
				if !strings.Contains(below.Hint, fmt.Sprintf("%d B", size-1)) {
					t.Errorf("%s %v: the hint does not offer the size below: %q", id, c, below.Hint)
				}
			}
		}
	}
}

// TestTheDefaultEncodingIsTheBytesTheseFormatsAlwaysWrote is the way back.
//
// A setting whose default changes the file is a breaking change wearing the
// clothes of a feature. This is the same pin the animated formats got when
// frames arrived: saying nothing and saying the default out loud have to be
// the same bytes, and the golden file beside it holds them to what they were
// before the setting existed.
func TestTheDefaultEncodingIsTheBytesTheseFormatsAlwaysWrote(t *testing.T) {
	for _, id := range encodedFormats {
		for _, size := range []int64{0, 33, 4096} {
			silent := writeEncoded(t, id, size, nil)
			spoken := writeEncoded(t, id, size, map[string]string{
				textenc.Setting: textenc.UTF8, textenc.SettingBOM: "false"})
			if !bytes.Equal(silent, spoken) {
				t.Errorf("%s at %d B: saying nothing and saying utf-8 produce different bytes", id, size)
			}
			if size > 0 && startsWithAnyMark(silent) {
				t.Errorf("%s at %d B: the default file opens with a byte order mark", id, size)
			}
		}
	}
}

// TestALabelThatWillNotFitSaysWhatItWouldCost holds the note to the encoding.
//
// The note used to be built from the length of the label as a string, which is
// what it costs in UTF-8 and half of what it costs in UTF-16. A note that is
// out by a factor of two is worse than no note: it tells somebody to ask for
// 66 B when the file needs 132.
func TestALabelThatWillNotFitSaysWhatItWouldCost(t *testing.T) {
	const size = int64(64) // below the label's cost in a wide encoding, even
	tails := map[string]string{"txt": "\n", "md": "\n\n"}

	for _, id := range encodedFormats {
		d, err := format.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		props := map[string]string{textenc.Setting: textenc.UTF16LE}
		p, err := d.Generator.Plan(format.Request{
			Bytes: size, Seed: 7741, Label: true, Properties: props})
		if err != nil {
			t.Fatalf("%s: planning %d B in utf-16le: %v", id, size, err)
		}

		line := core.Label(id, size, 7741) + tails[id]
		wide, narrow := int64(len(line))*2, int64(len(line))
		if wide <= size {
			t.Fatalf("%s: the label costs %d B at %d B, so this case no longer sits below the threshold",
				id, wide, size)
		}

		note := noteWithCode(p.Notes, "label_omitted")
		if note == nil {
			t.Fatalf("%s: the label does not fit and nothing said so - silence is banned", id)
		}
		if !strings.Contains(note.Detail, fmt.Sprintf("needs %d B", wide)) {
			t.Errorf("%s: the note should say the label needs %d B in this encoding: %q", id, wide, note.Detail)
		}
		if strings.Contains(note.Detail, fmt.Sprintf("needs %d B", narrow)) {
			t.Errorf("%s: the note reports what the label costs in UTF-8, not in the encoding asked for: %q",
				id, note.Detail)
		}
	}
}

// TestACharacterSplitAcrossTwoWritesSurvives is the one defence here that our
// own generators cannot redden, so it is reddened on purpose.
//
// They write whole words, so a character never straddles two writes. The
// encoder holds the tail anyway, because a writer that only works when its
// caller is careful is a trap for the next caller - and a defence nothing can
// redden is not a defence, which is why this exists rather than a comment
// saying it was thought about.
func TestACharacterSplitAcrossTwoWritesSurvives(t *testing.T) {
	const text = "zażółć" // two byte characters, so a split lands mid character

	for _, name := range []string{textenc.UTF16LE, textenc.UTF16BE} {
		codec, err := textenc.Parse("txt", map[string]string{textenc.Setting: name})
		if err != nil {
			t.Fatal(err)
		}

		var whole bytes.Buffer
		if _, err := codec.Writer(&whole).Write([]byte(text)); err != nil {
			t.Fatal(err)
		}

		for cut := 1; cut < len(text); cut++ {
			var split bytes.Buffer
			w := codec.Writer(&split)
			if _, err := w.Write([]byte(text)[:cut]); err != nil {
				t.Fatal(err)
			}
			if _, err := w.Write([]byte(text)[cut:]); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(whole.Bytes(), split.Bytes()) {
				t.Fatalf("%s: writing in two goes at byte %d gave % x, and in one go it is % x",
					name, cut, split.Bytes(), whole.Bytes())
			}
		}
	}
}

// TestTheTextGeneratorsWriteOnlyASCII names the assumption the arithmetic
// stands on.
//
// The budget is worked out as "file bytes divided by the width of a
// character", which is only the same thing as "how much prose fits" while
// every character costs one byte in UTF-8. The day the filler stops being
// English that stops being true, and it should stop here rather than in a file
// that is one byte short.
func TestTheTextGeneratorsWriteOnlyASCII(t *testing.T) {
	for _, id := range encodedFormats {
		body := writeEncoded(t, id, 40960, nil)
		for i, b := range body {
			if b > 0x7f {
				t.Fatalf("%s: byte %d is %#x, and the size arithmetic assumes one byte per character", id, i, b)
			}
		}
	}
}

func noteWithCode(notes []format.Note, code string) *format.Note {
	for i := range notes {
		if notes[i].Code == code {
			return &notes[i]
		}
	}
	return nil
}

func startsWithAnyMark(body []byte) bool {
	for _, m := range [][]byte{{0xEF, 0xBB, 0xBF}, {0xFF, 0xFE}, {0xFE, 0xFF}} {
		if bytes.HasPrefix(body, m) {
			return true
		}
	}
	return false
}

func first(body []byte, n int) []byte {
	if len(body) < n {
		return body
	}
	return body[:n]
}
