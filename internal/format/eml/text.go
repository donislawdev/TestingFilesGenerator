package eml

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// The text of a message is where the padding goes, decided on 2026-10-07 after
// both candidates were measured (docs/EML-2026-10-07.md section 5.1): every
// reader keeps the text, a system that saves the message again keeps it too,
// and a message of one part has nowhere else to put it. The epilogue after the
// last boundary held up as well in every reader, and RFC 2046 lets anything in
// between throw it away.

const (
	// lineWidth is one under the 76 quoted-printable allows a line, so the
	// last byte of the text can always become a letter - see fill.
	lineWidth = 75
	// chunkSize is how much text is built before it is written.
	chunkSize = 32 * 1024
)

// international is the line in Polish and Japanese that quoted-printable, 8bit
// and base64 add to the text in UTF-8 - two byte letters and three byte ones,
// which is what those encodings are for. A character set without Japanese
// carries the Polish half, which holds the three letters ISO-8859-2 and
// Windows-1250 write as different bytes.
const (
	polish        = "Zażółć gęślą jaźń."
	international = polish + " 日本語のテキストです。"
)

// singleByte is how ISO-8859-2 and Windows-1250 write the letters of the
// Polish line, checked against Python's codecs on 2026-10-13
// (docs/EML-2-2026-10-13.md section 7) rather than written from memory. Every
// other character of the line is ASCII.
var singleByte = map[string]map[rune]byte{
	CharsetLatin2: {
		'ą': 0xb1, 'ć': 0xe6, 'ę': 0xea, 'ł': 0xb3, 'ń': 0xf1, 'ó': 0xf3, 'ś': 0xb6, 'ź': 0xbc, 'ż': 0xbf,
	},
	CharsetWindows1250: {
		'ą': 0xb9, 'ć': 0xe6, 'ę': 0xea, 'ł': 0xb3, 'ń': 0xf1, 'ó': 0xf3, 'ś': 0x9c, 'ź': 0x9f, 'ż': 0xbf,
	},
}

// encodedLine is the international line in the character set the text is
// written in.
func (s settings) encodedLine() string {
	table, ok := singleByte[s.charset]
	if !ok {
		return international
	}
	out := make([]byte, 0, len(polish))
	for _, r := range polish {
		b, known := table[r]
		if r < 0x80 {
			b, known = byte(r), true
		}
		if !known {
			panic(fmt.Sprintf("eml: %q has no byte in %s", r, s.charset))
		}
		out = append(out, b)
	}
	return string(out)
}

// words are lower case and plain, so no line of the text can begin with
// "From " (which mbox files rewrite), with "--" (which reads as a boundary) or
// with a lone full stop (which ends a message in SMTP), and none holds "=".
var words = []string{
	"account", "address", "after", "again", "answer", "before", "below", "between",
	"change", "check", "close", "color", "copy", "date", "detail", "draft", "early",
	"every", "field", "file", "first", "follow", "group", "help", "issue", "keep",
	"later", "letter", "line", "list", "local", "message", "month", "needed", "note",
	"number", "office", "order", "other", "page", "paper", "point", "print", "question",
	"ready", "record", "reply", "report", "request", "result", "review", "second",
	"sender", "short", "small", "status", "subject", "summary", "table", "thank",
	"ticket", "today", "update", "value", "week", "while", "words", "write", "year",
}

// declared is the character set the text parts declare: the one asked for,
// or us-ascii for text that is plain ASCII.
func (s settings) declared() string {
	if s.international() {
		return s.charset
	}
	return "us-ascii"
}

// internationalLine is the line in Polish and Japanese the way the text is
// encoded, or nothing for 7bit. In base64 it is the text before encoding,
// which the part encodes with the rest.
func (s settings) internationalLine(open, close string) string {
	if !s.international() {
		return ""
	}
	line := open + s.encodedLine() + close
	if s.encoding == QuotedPrintable {
		line = quotedPrintable(line, s.eol)
	}
	return line + s.textEOL()
}

// quotedPrintable is text in quoted-printable (RFC 2045 section 6.7): every
// byte outside printable ASCII, and the equals sign, as =XX, no line longer
// than 76 characters - soft line breaks where a line would be - and no space
// at the end of a line.
//
// Written here for the reason encodedWords is: the bytes it gives have to be
// these bytes on every Go version.
func quotedPrintable(text, eol string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	col := 0
	for i := 0; i < len(text); i++ {
		c := text[i]
		token := string(c)
		if c == '=' || c < ' ' || c > '~' || (c == ' ' && i == len(text)-1) {
			token = "=" + string(hex[c>>4]) + string(hex[c&15])
		}
		if col+len(token) > 75 {
			b.WriteString("=" + eol)
			col = 0
		}
		b.WriteString(token)
		col += len(token)
	}
	return b.String()
}

// fill writes exactly n bytes of words in lines of at most lineWidth, each
// line but the last ending in eol.
//
// The last byte is a letter, whatever falls there. A space at the end of a
// line is what quoted-printable forbids, and a carriage return cut from its
// line feed is a lone CR that no reader handles the same way twice. Both end
// a line of at most lineWidth, so the letter keeps it within 76.
func fill(o *out, n int64, seed uint64) {
	if n <= 0 || o.err != nil {
		return
	}
	if o.w == nil {
		o.n += n
		return
	}
	w := wordWriter{rng: core.NewRand(seed), eol: o.eol}
	buf := o.scratch[:0]
	for remaining := n; remaining > 0 && o.err == nil; {
		if o.cancelled() {
			return
		}
		buf = w.chunk(buf[:0], remaining)
		if int64(len(buf)) >= remaining {
			buf = endInALetter(buf[:remaining])
		}
		o.bytes(buf)
		remaining -= int64(len(buf))
	}
	o.scratch = buf
}

// wordWriter is the words of one text part as they are laid into lines.
type wordWriter struct {
	rng interface{ IntN(int) int }
	eol string
	col int
}

// chunk appends words to buf until it holds chunkSize bytes or the bytes still
// owed, whichever comes first. The last word may run past either, and the
// caller cuts it.
func (w *wordWriter) chunk(buf []byte, owed int64) []byte {
	for len(buf) < chunkSize && int64(len(buf)) < owed {
		word := words[w.rng.IntN(len(words))]
		switch {
		case w.col > 0 && w.col+1+len(word) > lineWidth:
			buf = append(buf, w.eol...)
			w.col = 0
		case w.col > 0:
			buf = append(buf, ' ')
			w.col++
		}
		buf = append(buf, word...)
		w.col += len(word)
	}
	return buf
}

// endInALetter makes the last byte of the text a letter - see fill.
func endInALetter(buf []byte) []byte {
	if last := buf[len(buf)-1]; last == ' ' || last == '\r' {
		buf[len(buf)-1] = 'x'
	}
	return buf
}

// encodeText writes one text part's body: as it is, or through base64 when
// text_encoding asks for it - in lines of 76, with the text inside in its
// canonical form, and blanks empty lines after it, which every reader measured
// on 2026-10-13 skips (docs/EML-2-2026-10-13.md section 7). While the message
// is only being sized, the text is counted once as it is and the base64 is
// worked out from that.
func (m *message) encodeText(o *out, write func(*out), blanks int) {
	if m.s.encoding != Base64 {
		write(o)
		return
	}
	plain := out{eol: m.s.textEOL()}
	write(&plain)
	want := encodedLength(plain.n, len(m.s.eol))
	if o.w == nil || o.err != nil {
		o.n += want
	} else {
		base64Text(o, m.s, write, want)
	}
	for i := 0; i < blanks; i++ {
		o.put(m.s.eol)
	}
}

// base64Text writes the text through base64 into the file.
func base64Text(o *out, s settings, write func(*out), want int64) {
	l := lines{w: o.w, eol: s.eol}
	enc := base64.NewEncoder(base64.StdEncoding, &l)
	inner := out{w: enc, ctx: o.ctx, eol: s.textEOL(), scratch: o.scratch}
	write(&inner)
	o.scratch = inner.scratch
	if o.err = inner.err; o.err == nil {
		o.err = enc.Close()
	}
	o.n += l.n
	if o.err == nil && l.n != want {
		o.err = core.Defect(fmt.Errorf("eml: the text came out as %d B of base64 where its plan gives %d B", l.n, want))
	}
}
