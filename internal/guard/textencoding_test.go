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
var encodedFormats = []string{"txt", "md", "xml"}

// markRequired is the formats where a wide encoding has to carry a byte order
// mark, so the two cases without one are refused rather than written.
//
// XML is here for a measured reason rather than a tidy one. Its specification
// requires a mark on a UTF-16 entity, and - the half that decided it - our
// oracle cannot go red on a document that lacks one: expat ACCEPTS such a file
// and reads it correctly, measured 2026-09-08 in both directions. A file
// nothing here could check is a file this tool does not write. The refusal
// itself is proven by TestXMLRefusesAWideEncodingWithoutAMark, so this map is
// a declared behaviour rather than a way of skipping cases.
var markRequired = map[string]bool{"xml": true}

// refusedBy says whether this format turns this combination down by design.
func (c encodingCase) refusedBy(id string) bool {
	return markRequired[id] && c.width == 2 && !c.bom
}

// labelLine is how one format writes a label, which the note's arithmetic
// depends on and no format exposes. A format missing from here stops the check
// rather than silently measuring an empty wrapper.
func labelLine(t *testing.T, id string, size int64, seed uint64) string {
	t.Helper()
	body := core.Label(id, size, seed)
	switch id {
	case "txt":
		return body + "\n"
	case "md":
		return body + "\n\n"
	case "xml":
		return "<!-- " + body + " -->\n"
	}
	t.Fatalf("%s takes an encoding and this check does not know how it wraps a label", id)
	return ""
}

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
	checked, skipped, refused := 0, 0, 0

	for _, id := range encodedFormats {
		d, err := format.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range encodingCases() {
			if c.refusedBy(id) {
				refused++
				continue
			}
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
	// The count is here so a format quietly declaring every combination
	// refused would show up as nothing being checked rather than as a pass.
	t.Logf("%d file(s) decoded strictly by Python, %d skipped, %d combination(s) refused by design",
		checked, skipped, refused)
}

// TestAWideEncodingRefusesAnOddSizeAndNamesOneItCanWrite is the refusal, and
// the control beside it is what makes it mean anything.
//
// The same odd size is accepted under UTF-8, so the refusal is about the
// ENCODING rather than about the number. Without that half, a generator that
// refused every odd size in every encoding would pass this.
func TestAWideEncodingRefusesAnOddSizeAndNamesOneItCanWrite(t *testing.T) {
	for _, id := range encodedFormats {
		d, err := format.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range encodingCases() {
			if c.refusedBy(id) {
				continue
			}
			// The odd sizes are taken from the floor rather than written down.
			// A format with a minimum of its own - XML holds a declaration, a
			// root and one whole record - would refuse a small fixed number for
			// being too SMALL, and the refusal under test would never be the
			// one that fired. A wide floor is even, so each of these is odd.
			base := d.SmallestAccepted(format.Request{Seed: 7741, Label: true, Properties: c.props()})
			for _, size := range []int64{base + 1, base + 235, base + 4001} {
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
		d, err := format.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		floor := d.SmallestAccepted(format.Request{Seed: 7741, Label: true})
		for _, size := range []int64{floor, floor + 1, floor + 4096} {
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
	for _, id := range encodedFormats {
		d, err := format.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		props := map[string]string{textenc.Setting: textenc.UTF16LE, textenc.SettingBOM: "true"}
		floor := d.SmallestAccepted(format.Request{Seed: 7741, Label: true, Properties: props})

		// The size has to sit in a window or half of this check discriminates
		// nothing: at least as long as the label READS, and below twice that.
		// Inside it the label fits the file and does not fit what the file
		// HOLDS, which is the whole difference between the two comparisons a
		// generator could make - and a mutation swapping them is in the set.
		//
		// Taking the floor alone was wrong and the mutation run said so rather
		// than the reading: TXT and MD sit on a floor of almost nothing, so the
		// floor lands BELOW the window and both comparisons agree there.
		//
		// XML is the other way round. Its smallest document is a declaration, a
		// root and a whole record - about six times its label - so the window is
		// under its floor and unreachable. There the floor is the size and only
		// the cost half of this check applies, which is honest: XML asks a
		// different question of its own fit, and its own mutation covers it.
		reads := int64(len(labelLine(t, id, floor, 7741)))
		size := floor
		if size < reads {
			size = reads + 2
		}
		if size%2 != 0 {
			size++
		}
		p, err := d.Generator.Plan(format.Request{
			Bytes: size, Seed: 7741, Label: true, Properties: props})
		if err != nil {
			t.Fatalf("%s: planning %d B in utf-16le: %v", id, size, err)
		}

		line := labelLine(t, id, size, 7741)
		wide, narrow := int64(len(line))*2, int64(len(line))

		// The control, and it replaces a precondition that stopped meaning
		// anything. Comparing the label with the file size only works while a
		// format has no floor of its own - XML's floor is six times its label,
		// so that comparison would have called this case broken. What has to be
		// true is that a threshold EXISTS: given room, the note goes away.
		roomy := size + wide*2
		if roomy%2 != 0 {
			roomy++
		}
		if q, err := d.Generator.Plan(format.Request{
			Bytes: roomy, Seed: 7741, Label: true, Properties: props}); err != nil {
			t.Fatalf("%s: planning %d B in utf-16le: %v", id, roomy, err)
		} else if noteWithCode(q.Notes, "label_omitted") != nil {
			t.Fatalf("%s: %d B has room for a %d B label and the note fired anyway, so this is not measuring a threshold",
				id, roomy, wide)
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
