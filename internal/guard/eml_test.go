package guard

import (
	"bytes"
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/eml"
	"github.com/donislawdev/TestingFilesGenerator/internal/oracle"
)

// emlCases is every value of every setting, one at a time, then the
// combinations where the settings meet: names outside ASCII in every style,
// both line endings under quoted-printable, and alternative with all of it -
// and since 2026-10-13 the second set: base64 text in both line endings and in
// alternative, both single byte character sets in every encoding that carries
// them, no names, several recipients with encoded names, pictures beside HTML,
// alternative and plain text, and messages forwarded inside messages - and
// since 2026-10-15 newsletters.
var emlCases = []map[string]string{
	{},
	{eml.Body: eml.HTML},
	{eml.Body: eml.Alternative},
	{eml.TextEncoding: eml.QuotedPrintable},
	{eml.TextEncoding: eml.EightBit},
	{eml.Headers: eml.Encoded},
	{eml.Headers: eml.UTF8},
	{eml.LineEndings: eml.LF},
	{eml.FilenameStyle: eml.RFC2047},
	{eml.FilenameStyle: eml.Both},
	{eml.FilenameStyle: eml.ContentType},
	{eml.Headers: eml.Encoded, eml.FilenameStyle: eml.RFC2047},
	{eml.Headers: eml.Encoded, eml.FilenameStyle: eml.Both},
	{eml.Headers: eml.Encoded, eml.FilenameStyle: eml.ContentType},
	{eml.Headers: eml.UTF8, eml.FilenameStyle: eml.Both, eml.LineEndings: eml.LF},
	{eml.TextEncoding: eml.QuotedPrintable, eml.LineEndings: eml.LF, eml.Body: eml.HTML},
	{eml.Body: eml.Alternative, eml.TextEncoding: eml.QuotedPrintable, eml.Headers: eml.Encoded,
		eml.FilenameStyle: eml.Both, eml.LineEndings: eml.LF},
	{eml.Body: eml.Alternative, eml.TextEncoding: eml.EightBit, eml.Headers: eml.UTF8, eml.FilenameStyle: eml.RFC2047},
	{eml.TextEncoding: eml.Base64},
	{eml.TextEncoding: eml.Base64, eml.LineEndings: eml.LF, eml.Body: eml.Alternative},
	{eml.TextEncoding: eml.EightBit, eml.Charset: eml.CharsetLatin2},
	{eml.TextEncoding: eml.QuotedPrintable, eml.Charset: eml.CharsetWindows1250, eml.Body: eml.HTML},
	{eml.TextEncoding: eml.Base64, eml.Charset: eml.CharsetLatin2, eml.Body: eml.Alternative},
	{eml.FilenameStyle: eml.NoName},
	{eml.Recipients: "4", eml.Headers: eml.Encoded},
	{eml.Inline: "2", eml.InlineSize: "1kb", eml.Body: eml.HTML},
	{eml.Inline: "1", eml.InlineSize: "1kb", eml.Body: eml.Alternative, eml.TextEncoding: eml.QuotedPrintable, eml.FilenameStyle: eml.NoName},
	{eml.Inline: "2", eml.InlineSize: "1kb", eml.LineEndings: eml.LF},
	{eml.MessageDepth: "3"},
	{eml.MessageDepth: "2", eml.LineEndings: eml.LF, eml.Body: eml.Alternative, eml.TextEncoding: eml.Base64},
	// A newsletter in each text, each encoding of it, with pictures in its
	// frame and a message forwarded inside it (docs/EML-3-2026-10-15.md).
	{eml.Layout: eml.Newsletter},
	{eml.Layout: eml.Newsletter, eml.Body: eml.Alternative, eml.TextEncoding: eml.QuotedPrintable},
	{eml.Layout: eml.Newsletter, eml.Body: eml.HTML, eml.TextEncoding: eml.EightBit, eml.LineEndings: eml.LF,
		eml.Inline: "1", eml.InlineSize: "1kb"},
	{eml.Layout: eml.Newsletter, eml.Body: eml.Alternative, eml.TextEncoding: eml.Base64, eml.Charset: eml.CharsetLatin2},
	{eml.Layout: eml.Newsletter, eml.Body: eml.Alternative, eml.Headers: eml.Encoded, eml.MessageDepth: "2"},
}

// Three readers and a scanner, each asked what the others cannot answer
// (docs/EML-2026-10-07.md section 5.1). Python's email package has to read
// every message with no defect but the one headers=utf8 is measured to bring.
// mailparser, where it is installed, has to read the same attached files under
// the same names. The structural checker is told the settings and has to find
// the message made the way they say - the header lines RFC 5322 requires, the
// lines, the encodings, where each name is written. Go's standard library has
// its own guard beside this one.
//
// Every size from the smallest the settings allow, where the label does not fit
// and the text is empty, through the next one, to a message with room for
// words. Named to match ReferenceTool, because that is the one job in CI with
// mailparser installed. Anywhere without python it skips, and says so.
func TestEveryEMLSettingSurvivesItsReferenceTool(t *testing.T) {
	d, err := format.Get("eml")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	var parsed, byNode atomic.Int64
	// Each case in a subtest of its own, run side by side: every message is a
	// file of its own and every reader a process of its own, and one after
	// another the 144 messages kept the job waiting on interpreters starting
	// (review of #173). The group returns when every case has.
	t.Run("cases", func(t *testing.T) {
		for i, base := range emlCases {
			for _, count := range []string{"0", "2"} {
				t.Run(strconv.Itoa(i)+"_"+count, func(t *testing.T) {
					t.Parallel()
					props := withAttachments(base, count, "txt", "1kb")
					// Asked with the label the messages are written with: the
					// messages depth forwards carry labels of their own, so
					// their size, and the smallest message, depends on it.
					smallest := d.SmallestAccepted(format.Request{Properties: props, Label: true})
					for _, size := range []int64{smallest, smallest + 1, smallest + 2, 20000} {
						path := filepath.Join(dir, strconv.Itoa(i)+"_"+count+"_"+strconv.FormatInt(size, 10)+".eml")
						b, _ := writeEML(t, d, size, uint64(31+i), props)
						if err := os.WriteFile(path, b, 0o600); err != nil {
							t.Fatal(err)
						}
						py, node := emlSurvivesReaders(t, path, props, size)
						parsed.Add(int64(py))
						byNode.Add(int64(node))
					}
				})
			}
		}
	})
	// Where mailparser is named it has to have read something, or the step
	// that installed it and this guard disagree about where it is.
	if os.Getenv("TFG_MAILPARSER") != "" && byNode.Load() == 0 {
		t.Errorf("TFG_MAILPARSER names %q and mailparser read no message", os.Getenv("TFG_MAILPARSER"))
	}
	t.Logf("%d messages read by python, %d by mailparser", parsed.Load(), byNode.Load())
	if parsed.Load() == 0 {
		t.Skip("python is not installed, so no message was read - a skip, not a pass")
	}
}

// emlSurvivesReaders asks the readers about one message and says which of
// python and mailparser read it.
func emlSurvivesReaders(t *testing.T, path string, props map[string]string, size int64) (python, node int) {
	t.Helper()
	var allowed []string
	if props[eml.Headers] == eml.UTF8 {
		allowed = append(allowed, "from:UndecodableBytesDefect", "to:UndecodableBytesDefect")
	}
	py := oracle.PythonEmail(path, allowed...)
	if !py.Available {
		return 0, 0
	}
	if py.Err != nil {
		t.Fatalf("%v at %d B: %v", props, size, py.Err)
	}
	mp := oracle.MailParser(path)
	if mp.Available && mp.Err != nil {
		t.Fatalf("%v at %d B: %v", props, size, mp.Err)
	} else if mp.Available && attachmentsOf(mp.Output) != attachmentsOf(py.Output) {
		t.Fatalf("%v at %d B: mailparser and python read different files\n  mailparser %s\n  python     %s",
			props, size, attachmentsOf(mp.Output), attachmentsOf(py.Output))
	}
	if s := oracle.Strict("eml", path, emlSettings(props)...); s.Err != nil {
		t.Fatalf("%v at %d B: %v", props, size, s.Err)
	}
	return 1, boolInt(mp.Available)
}

// attachmentsOf is the list of attached files a reader printed, for comparing
// two readers - the subject is left out, because each prints it its own way.
func attachmentsOf(output string) string {
	i := strings.Index(output, `"attachments"`)
	if i < 0 {
		return output
	}
	return strings.ReplaceAll(output[i:], " ", "")
}

// A message too small for its label goes without one and says so - the note
// and label_embedded, both - rather than being refused or rounded. The size is
// taken from between the two minimums, so the guard is in the state it is
// about: it asserts that the bare message fits and the labelled one does not.
func TestAnEMLTooSmallForItsLabelSaysItCarriesNone(t *testing.T) {
	d, err := format.Get("eml")
	if err != nil {
		t.Fatal(err)
	}
	bare := d.SmallestAccepted(format.Request{})
	_, labelled := writeEML(t, d, bare+200, 5, nil)
	if labelled.Properties[format.PropertyLabelEmbedded] != true {
		t.Fatalf("a message of %d B carries no label, so this guard is not between the two minimums", bare+200)
	}
	p, err := d.Generator.Plan(format.Request{Bytes: bare, Seed: 5, Label: true})
	if err != nil {
		t.Fatalf("the smallest message with a label asked for was refused: %v", err)
	}
	if p.Properties[format.PropertyLabelEmbedded] != false || len(p.Notes) != 1 || p.Notes[0].Code != "label_omitted" {
		t.Errorf("a message of %d B with no room for its label says label_embedded=%v and notes %v",
			bare, p.Properties[format.PropertyLabelEmbedded], p.Notes)
	}
}

// Below the smallest message the settings allow is refused with the four
// parts every refusal of size has (D6), and with the reason a tester needs:
// the files travel in base64, a third larger. A message inside a message is
// no longer refused - mail is forwarded that way - and has a guard of its own,
// TestAMessageCarriesAMessageAsItIs.
func TestEMLRefusesASizeBelowItsAttachmentsInBase64(t *testing.T) {
	d, err := format.Get("eml")
	if err != nil {
		t.Fatal(err)
	}
	props := withAttachments(nil, "3", "txt", "64kb")
	_, err = d.Generator.Plan(format.Request{Bytes: 3 * 64 << 10, Seed: 1, Properties: props})
	var below *format.BelowMinimumError
	if !errors.As(err, &below) {
		t.Fatalf("three files of 64 KiB in a message of 192 KiB were not refused as too small: %v", err)
	}
	if below.Minimum <= 3*64<<10*4/3 || !strings.Contains(below.Reason.String(), "base64") {
		t.Errorf("the refusal gives the minimum %d B and the reason %q", below.Minimum, below.Reason)
	}
}

// A message has a largest size, the one the format counts in, and above it is
// refused at once with the four parts (D6) - in every encoding of the text.
//
// Until 2026-10-27 only the attachments were held to it. A text in base64 of
// 9223372036854775807B or one byte less never came back: the length of the
// encoded words wraps round near the top of int64, the search for how many
// words fit was handed a size that does not grow with them, and what was left
// went to empty lines, billions of them, one at a time
// (docs/STABILITY-REVIEW-2026-10-26.md, R15). The other encodings planned a
// message of 9 EiB that the format says it does not plan.
func TestAnEMLAboveTheLargestMessageIsRefusedAtOnce(t *testing.T) {
	d, err := format.Get("eml")
	if err != nil {
		t.Fatal(err)
	}
	const largest = int64(1) << 60
	// The last carries files, three of 2^59 B that add up past the ceiling
	// on their own: the refusal is still about the size asked, so it is the
	// size asked that comes first (found by review on 2026-10-27).
	for _, props := range []map[string]string{
		{"text_encoding": "base64"},
		{"text_encoding": "quoted-printable"},
		{"text_encoding": "7bit"},
		{"text_encoding": "base64", "attachments": "3", "attachment_format": "txt", "attachment_size": "576460752303423488b"},
	} {
		encoding := props["text_encoding"] + " with " + props["attachments"] + " attachment(s)"
		for _, size := range []int64{largest + 1, math.MaxInt64 - 1, math.MaxInt64} {
			done := make(chan error, 1)
			go func() {
				_, err := d.Generator.Plan(format.Request{Bytes: size, Seed: 1, Label: true, Properties: props})
				done <- err
			}()
			select {
			case err := <-done:
				var above *format.AboveMaximumError
				if !errors.As(err, &above) || above.Maximum != largest || above.Requested != size || above.Hint.IsZero() {
					t.Errorf("%s at %d B gave %v, expected a refusal of that size naming %d B as the largest", encoding, size, err, largest)
				}
			case <-time.After(5 * time.Second):
				t.Errorf("%s at %d B had no answer after five seconds", encoding, size)
			}
		}
		// The largest itself is a message the format plans.
		if props["attachments"] != "" {
			continue
		}
		if _, err := d.Generator.Plan(format.Request{Bytes: largest, Seed: 1, Label: true, Properties: props}); err != nil {
			t.Errorf("%s at the largest message, %d B, was refused: %v", encoding, largest, err)
		}
	}
}

// alternative carries the same words in both parts, so its text grows two
// bytes at a time, and an odd byte left over becomes one space in the HTML,
// where it changes nothing a person sees. Two neighbouring sizes have to land
// in both states - asserted rather than assumed, because a golden case named
// for the odd byte was first pinned at a size that left none (2026-10-07).
func TestAlternativePutsTheOddByteInTheHTML(t *testing.T) {
	d, err := format.Get("eml")
	if err != nil {
		t.Fatal(err)
	}
	reached := map[bool]bool{}
	for size := int64(4000); size < 4002; size++ {
		b, _ := writeEML(t, d, size, 9, map[string]string{eml.Body: eml.Alternative})
		reached[bytes.Contains(b, []byte("<html> <body>"))] = true
	}
	if !reached[true] || !reached[false] {
		t.Errorf("two neighbouring sizes did not reach both an even and an odd byte: %v", reached)
	}
}

// withAttachments is a case with the attachment settings added.
func withAttachments(base map[string]string, count, id, size string) map[string]string {
	out := map[string]string{eml.Attachments: count, eml.AttachmentFormat: id, eml.AttachmentSize: size}
	for k, v := range base {
		out[k] = v
	}
	return out
}

// writeEML plans and writes one message, and checks it is the size ordered.
func writeEML(t *testing.T, d format.Descriptor, size int64, seed uint64, props map[string]string) ([]byte, format.Plan) {
	t.Helper()
	p, err := d.Generator.Plan(format.Request{Bytes: size, Seed: seed, Label: true, Properties: props})
	if err != nil {
		t.Fatalf("planning %d B with %v: %v", size, props, err)
	}
	var buf bytes.Buffer
	if err := d.Generator.Write(context.Background(), &buf, p); err != nil {
		t.Fatalf("writing %d B with %v: %v", size, props, err)
	}
	if int64(buf.Len()) != size {
		t.Fatalf("%v: asked for %d B and got %d", props, size, buf.Len())
	}
	return buf.Bytes(), p
}

// emlSettings is a case as the words the structural checker is told.
func emlSettings(props map[string]string) []string {
	out := make([]string, 0, len(props))
	for k, v := range props {
		if k == eml.AttachmentFormat || k == eml.AttachmentSize {
			continue
		}
		out = append(out, k+"="+v)
	}
	sort.Strings(out)
	return out
}
