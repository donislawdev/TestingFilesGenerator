package eml

import (
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

// international is the line in Polish and Japanese that quoted-printable and
// 8bit add to the text - two byte letters and three byte ones, which is what
// those encodings are for.
const international = "Zażółć gęślą jaźń. 日本語のテキストです。"

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

// charset is what the text parts declare.
func (s settings) charset() string {
	if s.international() {
		return "utf-8"
	}
	return "us-ascii"
}

// internationalLine is the line in Polish and Japanese the way the text is
// encoded, or nothing for 7bit.
func (s settings) internationalLine(open, close string) string {
	if !s.international() {
		return ""
	}
	line := open + international + close
	if s.encoding == QuotedPrintable {
		line = quotedPrintable(line, s.eol)
	}
	return line + s.eol
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
