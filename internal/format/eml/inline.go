package eml

import (
	"fmt"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/picture"
)

// Pictures in the text: real files of a picture format, carried beside the
// HTML that shows them through cid: (RFC 2392), in multipart/related (RFC
// 2387).
//
// Measured on 2026-10-13 (docs/EML-2-2026-10-13.md section 7), and this is
// what makes them worth a setting: every reader took every shape, and each
// counted the pictures its own way. mailparser lists every picture among the
// attachments and turns cid: in the HTML into data:, Python's email package
// lists them when the HTML is alone, MimeKit never does. A system that limits
// or counts attachments meets the same file three ways.

// Setting names, public under rule 10. The declaration and its reading are
// shared with html, which carries pictures inside the page (package picture).
const (
	Inline       = picture.Count
	InlineFormat = picture.Format
	InlineSize   = picture.Size
)

// picturesGroup is the block the picture settings sit in, on both surfaces.
const picturesGroup = "Pictures in the text"

// pictureProperties are the settings of the pictures the text shows.
func pictureProperties() []format.Property {
	return picture.Properties(picturesGroup, picture.Details{
		Count: "How many pictures the text shows. With html and alternative each one sits beside the HTML and the HTML points at it through cid:. " +
			"With plain there is no HTML to point from, so they travel after the text as pictures to show inline. " +
			"mailparser lists them among the attachments, and so does Python's email package when the HTML is alone. MimeKit never does.",
		Format: "The format of the pictures. Any format whose files are pictures, such as png, jpg or gif.",
		Size:   "How big each picture is. Like an attached file it travels in base64, about a third larger.",
	})
}

// pictureFormat is the descriptor of the format the pictures are, or the
// refusal when it is not a picture.
func pictureFormat(id string) (format.Descriptor, error) {
	desc, ok, err := picture.Of(id)
	if err != nil || ok {
		return desc, err
	}
	return format.Descriptor{}, &format.PropertyValueError{
		Format: "eml", Key: InlineFormat, Value: id,
		Reason: core.Says("eml.APictureInTheTextIs", "a picture in the text has to be a file a mail program can show as a picture"),
		Remedy: core.Says("eml.UseOneOfThePictureFormats", "Use one of: %s.", core.A("Formats", strings.Join(picture.Formats(), ", "))),
	}
}

// addPictures plans the pictures, after the attached files, with seeds that
// follow theirs (RC4) - so asking for pictures changes no attached file.
//
// The format is asked whether it is a picture even when none is asked for: a
// value the setting cannot take is a mistake either way, and accepting it in
// silence would hand back a message the recipe did not describe.
func (p *planner) addPictures(s settings) ([]child, error) {
	desc, err := pictureFormat(s.pictures.Format)
	if err != nil || s.pictures.Count == 0 {
		return nil, err
	}
	out := make([]child, 0, s.pictures.Count)
	first := len(p.out)
	for i := 0; i < s.pictures.Count; i++ {
		c, err := p.plan(desc, s.pictures.Size, first+i)
		if err != nil {
			return nil, err
		}
		c.name = fileName(s, "inline", i+1, desc.Extension)
		// Only the HTML points at a picture, so beside a plain text it has
		// no Content-ID - the shape measured on 2026-10-13.
		if s.body != Plain {
			c.cid = contentID(i+1, p.r.Seed)
		}
		c.contentType, c.disposition = nameHeaders(s, desc.MediaType, c.name, "inline")
		out = append(out, c)
	}
	return out, nil
}

// contentID is a picture's Content-ID without its angle brackets: its number
// and the message's seed, so it is unique across messages as RFC 2392 asks,
// and the same length whatever the seed.
func contentID(n int, seed uint64) string {
	return fmt.Sprintf("inline_%04d.%016x@example.com", n, seed)
}

// imageLines are the lines of the HTML that show the pictures, one each, in
// the encoding the text is written in.
func imageLines(s settings, pictures []child) string {
	var b strings.Builder
	for _, c := range pictures {
		line := `<img src="cid:` + c.cid + `">`
		if s.encoding == QuotedPrintable {
			line = quotedPrintable(line, s.textEOL())
		}
		b.WriteString(line + s.textEOL())
	}
	return b.String()
}

// related writes the HTML with its pictures as multipart/related, the HTML
// first because it is the root (RFC 2387 section 3.2). top is a message with
// nothing else in it, where a closing boundary ends the file with a line
// break.
func (m *message) related(o *out, top bool) {
	eol, b := m.s.eol, m.id.related
	o.put(`Content-Type: multipart/related; type="text/html";`, eol, ` boundary="`, b, `"`, eol, eol)
	o.put("--", b, eol)
	m.textPart(o, "text/html", m.html, true)
	m.parts(o, b, m.pictures)
	o.put(eol, "--", b, "--")
	if top {
		o.put(eol)
	}
}
