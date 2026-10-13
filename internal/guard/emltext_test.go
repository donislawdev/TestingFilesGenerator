package guard

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/eml"
)

// A text in base64 grows four characters at a time and a line break at a
// time, so three sizes in four are out of reach of the words alone. Every one
// of them has to be reached all the same (D6): empty lines after the text
// take what is left, and with crlf the odd byte becomes one more digit in the
// Message-ID (docs/EML-2-2026-10-13.md section 8).
//
// Three hundred sizes from the smallest upwards, in each shape the text can
// take. writeEML refuses a message of any other size than the one asked for.
// The guard asserts that it reached every way of making up the difference -
// the long Message-ID under crlf and never under lf, and the empty lines -
// rather than assuming the sizes it walked happened to need them.
func TestEveryBase64TextSizeIsExact(t *testing.T) {
	d, err := format.Get("eml")
	if err != nil {
		t.Fatal(err)
	}
	reached := map[string]bool{}
	for _, props := range []map[string]string{
		{eml.TextEncoding: eml.Base64},
		{eml.TextEncoding: eml.Base64, eml.LineEndings: eml.LF},
		{eml.TextEncoding: eml.Base64, eml.Body: eml.Alternative, eml.Charset: eml.CharsetWindows1250},
		{eml.TextEncoding: eml.Base64, eml.Body: eml.HTML, eml.Inline: "1", eml.InlineSize: "1kb"},
	} {
		eol := "\r\n"
		if props[eml.LineEndings] == eml.LF {
			eol = "\n"
		}
		smallest := d.SmallestAccepted(format.Request{Properties: props, Label: true})
		for size := smallest; size < smallest+300; size++ {
			b, plan := writeEML(t, d, size, 11, props)
			long := len(plan.Properties["message_id"].(string)) == len("<0123456789abcdef0@example.com>")
			if long && props[eml.LineEndings] == eml.LF {
				t.Fatalf("%v at %d B: the Message-ID is long under lf, where line breaks reach every size", props, size)
			}
			reached["long Message-ID"] = reached["long Message-ID"] || long
			reached["short Message-ID"] = reached["short Message-ID"] || !long
			reached["empty lines"] = reached["empty lines"] || bytes.Contains(b, []byte(eol+eol+"--")) || bytes.HasSuffix(b, []byte(eol+eol))
		}
	}
	for _, way := range []string{"long Message-ID", "short Message-ID", "empty lines"} {
		if !reached[way] {
			t.Errorf("no size reached the %s, so this guard did not see that way of making up a size", way)
		}
	}
}

// The single byte character sets write the Polish line in the bytes Python's
// codecs give for them (measured on 2026-10-13, docs/EML-2-2026-10-13.md
// section 7) - ą, ś and ź as different bytes in each, which is what makes a
// reader that takes one for the other show the wrong letters - and they carry
// no Japanese, which neither has. UTF-8 carries both halves. The charset line
// and the manifest say which. With 7bit the text is plain ASCII and says
// us-ascii, whatever the setting is.
func TestTheCharsetsWriteTheLettersTheyDifferIn(t *testing.T) {
	d, err := format.Get("eml")
	if err != nil {
		t.Fatal(err)
	}
	for charset, line := range map[string]string{
		eml.CharsetLatin2:      "5a61bff3b3e62067eab66cb1206a61bcf12e",
		eml.CharsetWindows1250: "5a61bff3b3e62067ea9c6cb9206a619ff12e",
		eml.CharsetUTF8:        hex.EncodeToString([]byte("Zażółć gęślą jaźń. 日本語のテキストです。")),
	} {
		want, _ := hex.DecodeString(line)
		b, plan := writeEML(t, d, 4000, 59, map[string]string{eml.TextEncoding: eml.EightBit, eml.Charset: charset})
		japanese := bytes.Contains(b, []byte("日本語"))
		if !bytes.Contains(b, want) || !bytes.Contains(b, []byte("charset="+charset)) || japanese != (charset == eml.CharsetUTF8) {
			t.Errorf("charset=%s: the text does not carry the line in the bytes measured, or says another charset", charset)
		}
		if plan.Properties[eml.Charset] != charset {
			t.Errorf("charset=%s: the manifest says %v", charset, plan.Properties[eml.Charset])
		}
	}
	b, plan := writeEML(t, d, 4000, 59, map[string]string{eml.Charset: eml.CharsetLatin2})
	if !bytes.Contains(b, []byte("charset=us-ascii")) || plan.Properties[eml.Charset] != "us-ascii" || bytes.ContainsFunc(b, func(r rune) bool { return r > 0x7f }) {
		t.Errorf("7bit with charset=iso-8859-2 is not plain ASCII that says us-ascii, in the file and in the manifest")
	}
}
