package eml

import (
	"context"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// message is one message as it will be written, worked out while planning.
//
// The same code counts it and writes it: emit runs against an out with no
// writer to find the size, and against the file to produce it. A size worked
// out by a second copy of the layout would agree with the bytes only for as
// long as somebody kept the two alike by hand.
type message struct {
	s        settings
	id       identity
	children []child
	seed     uint64
	// label is the label line's text, or nothing for a message without one.
	label string
	// text is how many bytes of words each text part carries, and parity is
	// the one byte alternative cannot split between its two parts - a space
	// in the HTML, where it changes nothing a person sees.
	text   int64
	parity bool
	// plainLine and htmlLine are the line in Polish and Japanese as each part
	// writes it, encoded once rather than at every count.
	plainLine, htmlLine string
}

func newMessage(s settings, seed uint64, children []child) *message {
	return &message{
		s: s, id: identify(seed, s), children: children, seed: seed,
		plainLine: s.internationalLine("", ""),
		htmlLine:  s.internationalLine("<p>", "</p>"),
	}
}

// size is how many bytes the message comes to as it stands.
func (m *message) size() int64 {
	o := out{eol: m.s.eol}
	m.emit(&o)
	return o.n
}

// entities is every MIME entity in the message, the message itself included -
// the number mailparser counts and refuses past a thousand (measured
// 2026-10-07, docs/EML-2026-10-07.md section 5.1).
func (m *message) entities() int {
	text := 1
	if m.s.body == Alternative {
		text = 3
	}
	if len(m.children) == 0 {
		return text
	}
	return 1 + text + len(m.children)
}

// emit writes the message: the header lines, then the text alone, or the text
// and every attached file as the parts of multipart/mixed.
func (m *message) emit(o *out) {
	m.head(o)
	if len(m.children) == 0 {
		m.content(o, true)
		return
	}
	eol, b := m.s.eol, m.id.boundary
	o.put(`Content-Type: multipart/mixed; boundary="`, b, `"`, eol, eol)
	o.put("--", b, eol)
	m.content(o, false)
	for i := range m.children {
		if o.cancelled() {
			return
		}
		o.put(eol, "--", b, eol)
		m.attach(o, &m.children[i])
	}
	o.put(eol, "--", b, "--", eol)
}

// content writes the text as one entity: its header lines, a blank line and
// what it says. top is a message with nothing attached, where the text is the
// whole message and a closing boundary ends the file with a line break.
func (m *message) content(o *out, top bool) {
	switch m.s.body {
	case Plain:
		m.textHead(o, "text/plain")
		m.plain(o)
	case HTML:
		m.textHead(o, "text/html")
		m.html(o)
	default:
		eol, in := m.s.eol, m.id.inner
		o.put(`Content-Type: multipart/alternative; boundary="`, in, `"`, eol, eol)
		o.put("--", in, eol)
		m.textHead(o, "text/plain")
		m.plain(o)
		o.put(eol, "--", in, eol)
		m.textHead(o, "text/html")
		m.html(o)
		o.put(eol, "--", in, "--")
		if top {
			o.put(eol)
		}
	}
}

func (m *message) textHead(o *out, contentType string) {
	eol := m.s.eol
	o.put("Content-Type: ", contentType, "; charset=", m.s.charset(), eol)
	o.put("Content-Transfer-Encoding: ", m.s.encoding, eol, eol)
}

// plain is the text part: the label, the line in Polish and Japanese, and the
// words that bring the message to its size.
func (m *message) plain(o *out) {
	if m.label != "" {
		o.put(m.label, m.s.eol)
	}
	o.put(m.plainLine)
	fill(o, m.text, m.seed)
}

// html is the same text as HTML, the words in one paragraph - line breaks
// inside a paragraph are white space, so the lines keep the text's width.
func (m *message) html(o *out) {
	eol := m.s.eol
	o.put("<html>")
	if m.parity {
		o.put(" ")
	}
	o.put("<body>", eol)
	if m.label != "" {
		o.put("<p>", m.label, "</p>", eol)
	}
	o.put(m.htmlLine, "<p>", eol)
	fill(o, m.text, m.seed)
	o.put(eol, "</p>", eol, "</body></html>")
}

// out is where a message goes - a file, or nowhere while it is being counted.
//
// The first error sticks and everything after it is counted and not written,
// so the layout reads as a list of lines rather than a list of checks.
type out struct {
	w   io.Writer // nil while counting
	ctx context.Context
	eol string
	n   int64
	err error
	// scratch is the buffer the words are built in, and lines the writer that
	// cuts base64, both kept for the whole message rather than made per part.
	scratch []byte
	lines   lines
}

func (o *out) put(parts ...string) {
	for _, p := range parts {
		o.n += int64(len(p))
		if o.w != nil && o.err == nil {
			_, o.err = io.WriteString(o.w, p)
		}
	}
}

func (o *out) bytes(b []byte) {
	o.n += int64(len(b))
	if o.w != nil && o.err == nil {
		o.err = core.WriteAll(o.w, b)
	}
}

// cancelled says whether the run was interrupted, and makes that the error.
func (o *out) cancelled() bool {
	if o.ctx == nil || o.err != nil {
		return o.err != nil
	}
	if err := o.ctx.Err(); err != nil {
		o.err = err
		return true
	}
	return false
}
