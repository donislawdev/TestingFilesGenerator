package rtf

import (
	"context"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// The links a document carries when links is above 0: HYPERLINK fields, one to
// a paragraph, after what the settings open the document with and before the
// pictures and the blocks - the order a page has them in.
//
// Seventeen kinds, in turn, each saying in its text what kind it is, so what a
// program pulls out of the document reads without this file beside it. Read on
// 2026-10-15 by six readers, and each made something different of them
// (docs/RTF-2-2026-10-15.md section 1.2), which is what the detail below says.
// A switch of the field is written with its backslash escaped, the way RTF
// writes a backslash. A page has a download link and a nofollow link, and a
// field has nothing that says either, so they are not here. A path on the disk
// or a share and a field that fetches something belong to the security set
// (decision D3).
//
// Every address is on a domain RFC 2606 keeps for examples, and the number is
// in the range NANPA keeps for fiction, so a link followed by mistake reaches
// nobody. All of them are ASCII, which keeps links and characters independent.
var linkKinds = []struct{ instruction, text string }{
	{`HYPERLINK \\l "tfg_links"`, "link to a bookmark in this document"},
	{`HYPERLINK "page-2.rtf"`, "relative link"},
	{`HYPERLINK "../folder/page-3.rtf"`, "link to the folder above"},
	{`HYPERLINK "/root/page-4.rtf"`, "link from the root"},
	{`HYPERLINK "https://example.com/path/page.html?a=1&b=2#part"`, "absolute link with a query and a fragment"},
	{`HYPERLINK "https://example.com/path/page.html" \\l "part"`, "absolute link with a location"},
	{`HYPERLINK "http://example.org/"`, "http link"},
	{`HYPERLINK "//example.net/scheme-relative"`, "link without a scheme"},
	{`HYPERLINK "https://example.com/caf%C3%A9?q=two%20words"`, "percent encoded link"},
	{`HYPERLINK "https://xn--bcher-kva.example/"`, "international domain link"},
	{`HYPERLINK "https://example.com:8443/port"`, "link with a port"},
	{`HYPERLINK "mailto:someone@example.com?subject=Hello%20there"`, "mail link"},
	{`HYPERLINK "tel:+1-555-0100"`, "phone link"},
	{`HYPERLINK "ftp://example.com/file.txt"`, "ftp link"},
	{`HYPERLINK "https://example.org/new" \\t "_blank"`, "link to a new window"},
	{`HYPERLINK "https://example.net/tip" \\o "This sentence is the screen tip of a link."`, "link with a screen tip"},
	{`HYPERLINK "data:text/plain,hello"`, "data link"},
}

// The pieces of a link's paragraph around its kind, and the bookmark the first
// kind points at, which spans the whole list the way the id of the list does
// in a page.
const (
	fieldOpen     = `{\field{\*\fldinst `
	fieldResult   = `}{\fldrslt `
	fieldClose    = `}}`
	bookmarkStart = `{\*\bkmkstart tfg_links}`
	bookmarkEnd   = `{\*\bkmkend tfg_links}`
)

// linksDetail is what both surfaces say about the setting.
const linksDetail = "How many links the document carries, each a HYPERLINK field in a paragraph of its own, above the pictures and the blocks. " +
	"They go through 17 kinds in turn and then repeat - a link to a bookmark in the document, relative, to the folder above, from the root, " +
	"absolute with a query and a fragment, absolute with a location, http, without a scheme, percent encoded, an international domain, a port, " +
	"mailto:, tel:, ftp:, a new window, a screen tip and data: - and the text of each says which it is. " +
	"Every address is on a domain kept for examples. " +
	"LibreOffice turns the link without a scheme into a file:// address and rewrites the relative ones from where it saves the document. " +
	"pandoc 3.12 leaves the bookmark link without an address, pandoc 3.1 and 3.7 drop every link with its text, " +
	"and RichEdit returns the address of each link in front of its text. " +
	"LibreOffice took over 11 minutes over a million links. " +
	"The catalogue of every part has one link of its own, which this does not count."

// linksBytes is what the list takes, worked out without building it.
func linksBytes(n int) int64 {
	if n == 0 {
		return 0
	}
	var cycle, part int64
	for i, k := range linkKinds {
		size := int64(len(k.instruction) + len(k.text))
		cycle += size
		if i < n%len(linkKinds) {
			part += size
		}
	}
	each := int64(len(fieldOpen) + len(fieldResult) + len(fieldClose) + len(endBlock))
	count := int64(n)
	return int64(len(bookmarkStart)+len(bookmarkEnd)) + count*each + count/int64(len(linkKinds))*cycle + part
}

// writeLinks writes the list a chunk at a time, so a million links never sit in
// memory and an interrupted run stops between two chunks.
func writeLinks(ctx context.Context, w io.Writer, n int) error {
	out := chunks{ctx: ctx, w: w}
	for i := 0; i < n && out.err == nil; i++ {
		k := linkKinds[i%len(linkKinds)]
		if i == 0 {
			out.add(bookmarkStart)
		}
		out.add(fieldOpen, k.instruction, fieldResult, k.text, fieldClose)
		if i == n-1 {
			out.add(bookmarkEnd)
		}
		out.add(endBlock)
	}
	return out.close()
}

// writeChunk is how much is gathered before it is written, and so how often a
// stop is noticed.
const writeChunk = 32 * 1024

// chunks gathers pieces and writes them a chunk at a time, after asking
// whether the run was stopped.
type chunks struct {
	ctx context.Context
	w   io.Writer
	buf []byte
	err error
}

func (c *chunks) add(pieces ...string) {
	if c.err != nil {
		return
	}
	for _, p := range pieces {
		c.buf = append(c.buf, p...)
	}
	if len(c.buf) < writeChunk {
		return
	}
	if c.err = c.ctx.Err(); c.err == nil {
		c.err = core.WriteAll(c.w, c.buf)
	}
	c.buf = c.buf[:0]
}

// close writes what is left, or says why nothing more was written.
func (c *chunks) close() error {
	if c.err != nil {
		return c.err
	}
	return core.WriteAll(c.w, c.buf)
}
