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
	s  settings
	id identity
	// children are the attached files, the forwarded message last, and
	// pictures the pictures the text shows.
	children []child
	pictures []child
	seed     uint64
	// label is the label line's text, or nothing for a message without one.
	label string
	// text is how many bytes of words each text part carries, and parity is
	// the one byte alternative cannot split between its two parts - a space
	// in the HTML, where it changes nothing a person sees.
	text   int64
	parity bool
	// blanks and longID are how a text in base64 reaches the bytes its words
	// cannot: empty lines after it, and one more digit in the Message-ID for
	// the odd byte crlf leaves (docs/EML-2-2026-10-13.md section 8).
	blanks int
	longID bool
	// plainLine and htmlLine are the line in Polish and Japanese as each part
	// writes it, and images the lines that show the pictures, encoded once
	// rather than at every count.
	plainLine, htmlLine, images string
}

func newMessage(s settings, seed uint64, children, pics []child) *message {
	return &message{
		s: s, id: identify(seed, s), children: children, pictures: pics, seed: seed,
		plainLine: s.internationalLine("", ""),
		htmlLine:  s.internationalLine("<p>", "</p>"),
		images:    imageLines(s, pics),
	}
}

// size is how many bytes the message comes to as it stands.
func (m *message) size() int64 {
	o := out{eol: m.s.eol}
	m.emit(&o)
	return o.n
}

// hasRelated says the HTML travels with its pictures in multipart/related.
func (m *message) hasRelated() bool { return len(m.pictures) > 0 && m.s.body != Plain }

// beside is the pictures that travel after a plain text, which has no HTML to
// show them from.
func (m *message) beside() []child {
	if m.s.body == Plain {
		return m.pictures
	}
	return nil
}

// mixed says the message is multipart/mixed: it carries files, or pictures
// beside a plain text.
func (m *message) mixed() bool { return len(m.children)+len(m.beside()) > 0 }

// entities is every MIME entity in the message, the message itself included -
// the number mailparser counts and refuses past a thousand (measured
// 2026-10-07, docs/EML-2026-10-07.md section 5.1). A message inside counts as
// one: mailparser does not go into it (measured 2026-10-13).
func (m *message) entities() int {
	text := 1
	if m.s.body == Alternative {
		text = 3
	}
	if m.hasRelated() {
		text += 1 + len(m.pictures)
	}
	if !m.mixed() {
		return text
	}
	return 1 + text + len(m.children) + len(m.beside())
}

// emit writes the message: the header lines, then the text alone, or the text
// and every file as the parts of multipart/mixed.
func (m *message) emit(o *out) {
	m.head(o)
	if !m.mixed() {
		m.content(o, true)
		return
	}
	eol, b := m.s.eol, m.id.boundary
	o.put(`Content-Type: multipart/mixed; boundary="`, b, `"`, eol, eol)
	o.put("--", b, eol)
	m.content(o, false)
	m.parts(o, b, m.beside())
	m.parts(o, b, m.children)
	o.put(eol, "--", b, "--", eol)
}

// parts writes files as the parts of a multipart entity, each after its
// boundary.
func (m *message) parts(o *out, boundary string, list []child) {
	for i := range list {
		if o.cancelled() {
			return
		}
		o.put(m.s.eol, "--", boundary, m.s.eol)
		m.attach(o, &list[i])
	}
}

// content writes the text as one entity: its header lines, a blank line and
// what it says. top is a message with nothing else in it, where a closing
// boundary ends the file with a line break.
func (m *message) content(o *out, top bool) {
	switch {
	case m.s.body == Plain:
		m.textPart(o, "text/plain", m.plain, true)
	case m.s.body == HTML && m.hasRelated():
		m.related(o, top)
	case m.s.body == HTML:
		m.textPart(o, "text/html", m.html, true)
	default:
		m.alternative(o, top)
	}
}

// alternative writes the plain text and the HTML as multipart/alternative, the
// HTML with its pictures when there are any.
func (m *message) alternative(o *out, top bool) {
	eol, in := m.s.eol, m.id.inner
	o.put(`Content-Type: multipart/alternative; boundary="`, in, `"`, eol, eol)
	o.put("--", in, eol)
	m.textPart(o, "text/plain", m.plain, false)
	o.put(eol, "--", in, eol)
	if m.hasRelated() {
		m.related(o, false)
	} else {
		m.textPart(o, "text/html", m.html, true)
	}
	o.put(eol, "--", in, "--")
	if top {
		o.put(eol)
	}
}

// textPart writes one text entity. last is the part the empty lines of a text
// in base64 follow.
func (m *message) textPart(o *out, contentType string, write func(*out), last bool) {
	eol := m.s.eol
	o.put("Content-Type: ", contentType, "; charset=", m.s.declared(), eol)
	o.put("Content-Transfer-Encoding: ", m.s.encoding, eol, eol)
	blanks := 0
	if last {
		blanks = m.blanks
	}
	m.encodeText(o, write, blanks)
}

// plain is the text part: the label, the line in Polish and Japanese, and the
// words that bring the message to its size. Its lines end the way the text's
// do, which inside base64 is not the way the file's do.
func (m *message) plain(o *out) {
	if m.label != "" {
		o.put(m.label, o.eol)
	}
	o.put(m.plainLine)
	fill(o, m.text, m.seed)
}

// html is the same text as HTML, the words in one paragraph - line breaks
// inside a paragraph are white space, so the lines keep the text's width - and
// the pictures before them.
func (m *message) html(o *out) {
	eol := o.eol
	o.put("<html>")
	if m.parity {
		o.put(" ")
	}
	o.put("<body>", eol)
	if m.label != "" {
		o.put("<p>", m.label, "</p>", eol)
	}
	o.put(m.htmlLine, m.images, "<p>", eol)
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
