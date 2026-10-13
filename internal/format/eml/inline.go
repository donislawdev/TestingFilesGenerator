package eml

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/archive"
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

// Setting names, public under rule 10.
const (
	Inline       = "inline"
	InlineFormat = "inline_format"
	InlineSize   = "inline_size"
)

// picturesGroup is the block the picture settings sit in, on both surfaces.
const picturesGroup = "Pictures in the text"

// defaultPictureFormat is what a picture is when nothing says - the format a
// mail program puts a pasted picture in.
const defaultPictureFormat = "png"

// pictures is what the inline settings ask for.
type pictures struct {
	count  int
	format string
	size   int64
}

// pictureProperties are the settings of the pictures the text shows.
func pictureProperties() []format.Property {
	return []format.Property{
		{
			Name: Inline, Kind: format.PropertyInt,
			Min: 0, Max: archive.MaxMembers,
			Default: "0",
			Detail: "How many pictures the text shows. With html and alternative each one sits beside the HTML and the HTML points at it through cid:. " +
				"With plain there is no HTML to point from, so they travel after the text as pictures to show inline. " +
				"mailparser lists them among the attachments, and so does Python's email package when the HTML is alone. MimeKit never does.",
			Group: picturesGroup,
		},
		{
			Name: InlineFormat, Kind: format.PropertyText,
			Shape:   "the id of a picture format, as tfg formats lists them",
			Default: defaultPictureFormat,
			Detail:  "The format of the pictures. Any format whose files are pictures, such as png, jpg or gif.",
			Group:   picturesGroup,
		},
		{
			Name: InlineSize, Kind: format.PropertySize,
			Default: archive.DefaultSizeText,
			Detail:  "How big each picture is. Like an attached file it travels in base64, about a third larger.",
			Group:   picturesGroup,
		},
	}
}

// readPictures is the three settings as numbers. Whether the format is a
// picture is asked in Plan, where the registry is complete - this also runs
// while the formats are still registering, to work out the minimum.
func readPictures(values map[string]string) (pictures, error) {
	n, err := strconv.Atoi(values[Inline])
	if err != nil {
		return pictures{}, core.Defect(err)
	}
	size, err := core.ParseSize(values[InlineSize])
	if err != nil {
		return pictures{}, core.Defect(err)
	}
	return pictures{count: n, format: values[InlineFormat], size: size}, nil
}

// pictureFormat is the descriptor of the format the pictures are, or the
// refusal when it is not a picture.
func pictureFormat(id string) (format.Descriptor, error) {
	desc, err := format.Get(id)
	if err != nil {
		return format.Descriptor{}, err
	}
	if strings.HasPrefix(desc.MediaType, "image/") {
		return desc, nil
	}
	return format.Descriptor{}, &format.PropertyValueError{
		Format: "eml", Key: InlineFormat, Value: id,
		Reason: core.Says("eml.APictureInTheTextIs", "a picture in the text has to be a file a mail program can show as a picture"),
		Remedy: core.Says("eml.UseOneOfThePictureFormats", "Use one of: %s.", core.A("Formats", strings.Join(pictureFormats(), ", "))),
	}
}

// pictureFormats are the formats of this build whose files are pictures, from
// the media type each declares rather than from a list kept here.
func pictureFormats() []string {
	var out []string
	for _, d := range format.All() {
		if strings.HasPrefix(d.MediaType, "image/") {
			out = append(out, d.ID)
		}
	}
	return out
}

// addPictures plans the pictures, after the attached files, with seeds that
// follow theirs (RC4) - so asking for pictures changes no attached file.
//
// The format is asked whether it is a picture even when none is asked for: a
// value the setting cannot take is a mistake either way, and accepting it in
// silence would hand back a message the recipe did not describe.
func (p *planner) addPictures(s settings) ([]child, error) {
	desc, err := pictureFormat(s.pictures.format)
	if err != nil || s.pictures.count == 0 {
		return nil, err
	}
	out := make([]child, 0, s.pictures.count)
	first := len(p.out)
	for i := 0; i < s.pictures.count; i++ {
		c, err := p.plan(desc, s.pictures.size, first+i)
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
