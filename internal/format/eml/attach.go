package eml

import (
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

const (
	// base64Line is the longest line of base64 RFC 2045 allows.
	base64Line = 76

	// maxMessage is the largest message this format plans. Far above anything
	// a disk holds, and far enough below the int64 every size here is counted
	// in that adding the parts up cannot wrap round.
	maxMessage = int64(1) << 60

	// namePrefixWide is what the name of an attached file starts with when
	// headers asks for names outside ASCII.
	namePrefixWide = "Załącznik_日本語_"

	// percentRun is how much of a name RFC 2231 percent encoding puts on one
	// line before it continues on the next, short enough that the line stays
	// under 78 characters and cut never inside a %XX.
	percentRun = 48
)

// child is one attached file, planned before a byte is written.
type child struct {
	desc format.Descriptor
	plan format.Plan
	// name is the file's name as a person reads it.
	name string
	// contentType and disposition are the two header values, parameters and
	// folding included, worked out once while planning.
	contentType, disposition string
	// encoded is how many bytes the file takes in base64 with its line
	// breaks - not the header lines above it, and not the line break the next
	// boundary brings.
	encoded int64
}

// planChildren plans every file the message carries.
//
// The files are real files of another format, each valid on its own, with
// seeds derived from the message's seed and their place in it (RC4), numbered
// per format across the whole message - the archives' rules, for their
// reasons (zip/children.go).
func planChildren(r format.Request, groups []format.Content, s settings) ([]child, error) {
	total := 0
	for _, g := range groups {
		total += g.Count
	}
	out := make([]child, 0, total)
	numbered := map[string]int{}
	var sum int64
	index := 0
	for _, g := range groups {
		desc, err := format.Get(g.Format)
		if err != nil {
			return nil, err
		}
		for i := 0; i < g.Count; i++ {
			cp, err := desc.Generator.Plan(format.Request{Bytes: g.Bytes, Seed: core.FileSeed(r.Seed, index), Label: r.Label})
			if err != nil {
				return nil, core.Refuse(core.Says("eml.TheAttachedFileCannotBe", "eml: the attached %s file cannot be made: %w", core.A("Format", g.Format), core.A("Err", err)))
			}
			numbered[g.Format]++
			c := child{desc: desc, plan: cp, name: fileName(s, g.Format, numbered[g.Format], desc.Extension)}
			if cp.Bytes > maxMessage {
				return nil, tooLarge(cp.Bytes)
			}
			c.encoded = encodedLength(cp.Bytes, len(s.eol))
			if sum += c.encoded; sum > maxMessage {
				return nil, tooLarge(sum)
			}
			c.contentType, c.disposition = nameHeaders(s, desc.MediaType, c.name)
			out = append(out, c)
			index++
		}
	}
	return out, nil
}

// tooLarge refuses attachments that add up past maxMessage.
func tooLarge(asked int64) error {
	return &format.AboveMaximumError{
		Format:    "EML",
		Requested: asked,
		Maximum:   maxMessage,
		Reason:    core.Says("eml.AMessageThisLargeCannotBe", "a message this large cannot be counted to the byte in this build"),
		Hint:      core.Says("eml.AskForSmallerAttachments", "Ask for fewer or smaller attachments."),
	}
}

// encodedLength is how many bytes n bytes take in base64 lines of 76
// characters, the breaks between them included and none after the last.
func encodedLength(n int64, eol int) int64 {
	chars := (n + 2) / 3 * 4
	if chars == 0 {
		return 0
	}
	return chars + (chars-1)/base64Line*int64(eol)
}

// fileName is the name of an attached file: the format, its number among the
// files of that format, and the extension - with a prefix in Polish and
// Japanese when headers asks for names outside ASCII.
func fileName(s settings, id string, n int, ext string) string {
	name := fmt.Sprintf("%s_%04d%s", id, n, ext)
	if s.headers != ASCII {
		name = namePrefixWide + name
	}
	return name
}

// nameHeaders are the Content-Type and Content-Disposition values of one
// attached file, with its name where filename_style puts it and written the
// way headers says. Each parameter goes on a line of its own, so no name pushes
// a line past 78 characters. docs/EML-2026-10-07.md section 10.3 has the table.
func nameHeaders(s settings, mediaType, name string) (contentType, disposition string) {
	var inType, inDisposition []string
	switch s.names {
	case RFC2231:
		inDisposition = rfc2231Params(s, name)
	case RFC2047:
		inType = []string{quotedParam(s, "name", name)}
		inDisposition = []string{quotedParam(s, "filename", name)}
	case Both:
		inType = []string{quotedParam(s, "name", name)}
		inDisposition = rfc2231Params(s, name)
	default:
		inType = []string{quotedParam(s, "name", name)}
	}
	return withParams(s, mediaType, inType), withParams(s, "attachment", inDisposition)
}

// withParams is a header value with its parameters, each folded onto a line
// of its own.
func withParams(s settings, value string, params []string) string {
	for _, p := range params {
		value += ";" + s.eol + " " + p
	}
	return value
}

// quotedParam is a parameter in quotes: the name as it is when it is ASCII or
// when utf8 asks for it raw, and RFC 2047 encoded words, folded, when encoded
// asks for them. Words inside quotes are what RFC 2047 forbids and what
// filename_style rfc2047 exists to hand a tester.
func quotedParam(s settings, key, name string) string {
	return key + `="` + phrase(s, name) + `"`
}

// rfc2231Params is the name as RFC 2231 writes it: in quotes when it is ASCII
// or raw, and percent encoded in UTF-8 otherwise, continued over numbered
// parameters when it is long.
func rfc2231Params(s settings, name string) []string {
	if s.headers != Encoded || isASCII(name) {
		return []string{`filename="` + name + `"`}
	}
	encoded := percentEncoded(name)
	if len(encoded) <= percentRun {
		return []string{"filename*=UTF-8''" + encoded}
	}
	var out []string
	for n := 0; encoded != ""; n++ {
		cut := min(percentRun, len(encoded))
		// Never inside a %XX - step back to before the percent sign.
		if i := strings.LastIndexByte(encoded[:cut], '%'); i > cut-3 && cut < len(encoded) {
			cut = i
		}
		prefix := ""
		if n == 0 {
			prefix = "UTF-8''"
		}
		out = append(out, fmt.Sprintf("filename*%d*=%s%s", n, prefix, encoded[:cut]))
		encoded = encoded[cut:]
	}
	return out
}

// percentEncoded is RFC 2231 section 7's encoding: the characters a parameter
// may carry as they are, every other byte as %XX.
func percentEncoded(text string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(text); i++ {
		c := text[i]
		if c > ' ' && c < 0x7f && !strings.ContainsRune(`*'%()<>@,;:\"/[]?=`, rune(c)) {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&15])
	}
	return b.String()
}

// attach writes one attached file: its header lines, a blank line and the
// file itself in base64, made by its own generator as it is written.
func (m *message) attach(o *out, c *child) {
	eol := m.s.eol
	o.put("Content-Type: ", c.contentType, eol)
	o.put("Content-Disposition: ", c.disposition, eol)
	o.put("Content-Transfer-Encoding: base64", eol, eol)
	if o.w == nil || o.err != nil {
		o.n += c.encoded
		return
	}
	o.lines = lines{w: o.w, eol: eol}
	enc := base64.NewEncoder(base64.StdEncoding, &o.lines)
	if err := c.desc.Generator.Write(o.ctx, enc, c.plan); err != nil {
		o.err = err
		return
	}
	if err := enc.Close(); err != nil {
		o.err = err
		return
	}
	o.n += o.lines.n
	if o.lines.n != c.encoded {
		o.err = core.Defect(fmt.Errorf("eml: the attached %s file came out as %d B of base64 where its plan gives %d B",
			c.desc.ID, o.lines.n, c.encoded))
	}
}

// lines cuts base64 into lines of 76 characters. The break goes in front of
// the next character rather than after the 76th, so the last line never ends
// in one - the boundary that follows brings its own.
type lines struct {
	w   io.Writer
	eol string
	col int
	n   int64
}

func (l *lines) Write(p []byte) (int, error) {
	written := 0
	for len(p) > 0 {
		if l.col == base64Line {
			if _, err := io.WriteString(l.w, l.eol); err != nil {
				return written, err
			}
			l.n += int64(len(l.eol))
			l.col = 0
		}
		k := min(base64Line-l.col, len(p))
		if err := core.WriteAll(l.w, p[:k]); err != nil {
			return written, err
		}
		l.col += k
		l.n += int64(k)
		written += k
		p = p[k:]
	}
	return written, nil
}
