package eml

import (
	"encoding/base64"
	"fmt"
	"io"
	"strconv"
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

	// messageType is what a message inside a message is (RFC 2046 section
	// 5.2.1), and it is the one part that goes as it is rather than in base64.
	messageType = "message/rfc822"
)

// child is one attached file or picture, planned before a byte is written.
type child struct {
	desc format.Descriptor
	plan format.Plan
	// name is the file's name as a person reads it.
	name string
	// contentType and disposition are the two header values, parameters and
	// folding included, worked out once while planning.
	contentType, disposition string
	// cid is a picture's Content-ID, without its angle brackets, and nothing
	// for an attached file.
	cid string
	// encoded is how many bytes the file takes in the message - base64 with
	// its line breaks, or the message itself for a message - and not the
	// header lines above it or the line break the next boundary brings.
	encoded int64
}

// raw says the file goes into the message as it is: a message, which RFC 2046
// section 5.2.1 allows in 7bit, 8bit or binary and never in base64.
func (c *child) raw() bool { return c.desc.MediaType == messageType }

// planChildren plans every file the message carries: the attached files, then
// the forwarded message depth asks for, then the pictures.
//
// The files are real files of another format, each valid on its own, with
// seeds derived from the message's seed and their place in it (RC4), numbered
// per format across the whole message - the archives' rules, for their
// reasons (zip/children.go). The order is the order the seeds were given in
// before pictures and depth existed, so neither moves an attached file.
func planChildren(r format.Request, groups []format.Content, s settings) (children, pics []child, err error) {
	total := 0
	for _, g := range groups {
		total += g.Count
	}
	p := planner{r: r, s: s, out: make([]child, 0, total+1), numbered: map[string]int{}}
	for _, g := range groups {
		desc, err := format.Get(g.Format)
		if err != nil {
			return nil, nil, err
		}
		if err := p.add(desc, g); err != nil {
			return nil, nil, err
		}
	}
	if err := p.forward(); err != nil {
		return nil, nil, err
	}
	pics, err = p.addPictures(s)
	return p.out, pics, err
}

// planner is planChildren's place in the message as it goes: how many files
// came before, how many of each format, and how many bytes of them so far.
type planner struct {
	r        format.Request
	s        settings
	out      []child
	numbered map[string]int
	sum      int64
}

// add plans the files of one group.
func (p *planner) add(desc format.Descriptor, g format.Content) error {
	for i := 0; i < g.Count; i++ {
		c, err := p.plan(desc, g.Bytes, len(p.out))
		if err != nil {
			return err
		}
		p.attachment(c)
	}
	return nil
}

// attachment names a planned file and puts it among the attached ones.
func (p *planner) attachment(c child) {
	p.numbered[c.desc.ID]++
	c.name = fileName(p.s, c.desc.ID, p.numbered[c.desc.ID], c.desc.Extension)
	c.contentType, c.disposition = nameHeaders(p.s, c.desc.MediaType, c.name, "attachment")
	p.out = append(p.out, c)
}

// plan plans one file of a size with the seed of its place, and counts what it
// adds to the message.
func (p *planner) plan(desc format.Descriptor, size int64, place int) (child, error) {
	cp, err := desc.Generator.Plan(p.request(desc, size, place))
	if err != nil {
		return child{}, core.Refuse(core.Says("eml.TheAttachedFileCannotBe", "eml: the attached %s file cannot be made: %w", core.A("Format", desc.ID), core.A("Err", err)))
	}
	if cp.Bytes > maxMessage {
		return child{}, tooLarge(cp.Bytes)
	}
	c := child{desc: desc, plan: cp, encoded: encodedLength(cp.Bytes, len(p.s.eol))}
	if c.raw() {
		c.encoded = cp.Bytes
	}
	if p.sum += c.encoded; p.sum > maxMessage {
		return child{}, tooLarge(p.sum)
	}
	return c, nil
}

// request is what a file is asked for with. A message inside a message ends
// its lines the way the one around it does - a file with two kinds of line
// ending is a broken file, not a test of either.
func (p *planner) request(desc format.Descriptor, size int64, place int) format.Request {
	r := format.Request{Bytes: size, Seed: core.FileSeed(p.r.Seed, place), Label: p.r.Label}
	if desc.MediaType == messageType {
		r.Properties = map[string]string{LineEndings: p.s.endings}
	}
	return r
}

// forward plans the message depth asks for inside this one: the smallest
// message one level less deep, its size worked out from what it carries -
// which is nothing but the next one.
func (p *planner) forward() error {
	if p.s.depth < 2 {
		return nil
	}
	desc, err := format.Get("eml")
	if err != nil {
		return err
	}
	r := p.request(desc, 0, len(p.out))
	r.Properties[MessageDepth] = strconv.Itoa(p.s.depth - 1)
	r.SizeFromContents, r.Contains = true, []format.Content{}
	cp, err := desc.Generator.Plan(r)
	if err != nil {
		return err
	}
	if p.sum += cp.Bytes; p.sum > maxMessage {
		return tooLarge(p.sum)
	}
	p.attachment(child{desc: desc, plan: cp, encoded: cp.Bytes})
	return nil
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
// attached file or picture, with its name where filename_style puts it and
// written the way headers says. Each parameter goes on a line of its own, so no
// name pushes a line past 78 characters. docs/EML-2026-10-07.md section 10.3
// has the table.
func nameHeaders(s settings, mediaType, name, disposition string) (contentType, dispositionValue string) {
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
	case ContentType:
		inType = []string{quotedParam(s, "name", name)}
	}
	return withParams(s, mediaType, inType), withParams(s, disposition, inDisposition)
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

// attach writes one attached file or picture: its header lines, a blank line
// and the file itself, made by its own generator as it is written - in base64,
// or as it is for a message.
func (m *message) attach(o *out, c *child) {
	eol := m.s.eol
	o.put("Content-Type: ", c.contentType, eol)
	o.put("Content-Disposition: ", c.disposition, eol)
	if c.cid != "" {
		o.put("Content-ID: <", c.cid, ">", eol)
	}
	if !c.raw() {
		o.put("Content-Transfer-Encoding: base64", eol)
	}
	o.put(eol)
	if o.w == nil || o.err != nil {
		o.n += c.encoded
		return
	}
	if c.raw() {
		m.attachRaw(o, c)
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
	o.expect(c, o.lines.n)
}

// attachRaw writes a message inside the message as it is.
func (m *message) attachRaw(o *out, c *child) {
	w := counted{w: o.w}
	if err := c.desc.Generator.Write(o.ctx, &w, c.plan); err != nil {
		o.err = err
		return
	}
	o.n += w.n
	o.expect(c, w.n)
}

// expect turns a file that came out other than its plan into a defect - the
// size of the whole message rests on it.
func (o *out) expect(c *child, got int64) {
	if got != c.encoded && o.err == nil {
		o.err = core.Defect(fmt.Errorf("eml: the attached %s file came out as %d B where its plan gives %d B",
			c.desc.ID, got, c.encoded))
	}
}

// counted is a writer that keeps count.
type counted struct {
	w io.Writer
	n int64
}

func (c *counted) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
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
		if err := l.breakFull(); err != nil {
			return written, err
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

// breakFull ends a line that holds 76 characters, before the next one starts.
func (l *lines) breakFull() error {
	if l.col != base64Line {
		return nil
	}
	if _, err := io.WriteString(l.w, l.eol); err != nil {
		return err
	}
	l.n += int64(len(l.eol))
	l.col = 0
	return nil
}
