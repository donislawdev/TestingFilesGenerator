package htmlfile

import (
	"context"
	"io"
)

// The links a page carries when links is above 0: a list above the pictures
// and the blocks, before the pictures because lxml loses the rest of a page
// after a picture of 15 MB (docs/HTML-3-2026-10-14.md section 9).
//
// Eighteen kinds, in turn, each saying in its text what kind it is, so what a
// program pulls out of the page reads without this file beside it. Every one of
// them was read on 2026-10-14 by the Nu HTML Checker, which finds nothing wrong
// with any, by jsdom, lxml, BeautifulSoup and Python's urljoin, which resolve
// them alike, and by DOMPurify, which with its defaults takes out the data:
// address and target (docs/HTML-5-2026-10-14.md section 1.1). The two the
// checker calls errors - a user name in the address and a raw space - are left
// out: they belong to damaged files and to the security set (decision D3).
//
// Every address is on a domain RFC 2606 keeps for examples, and the number is
// in the range NANPA keeps for fiction, so a link followed by mistake reaches
// nobody. All of them are ASCII, which keeps links and characters independent.
var linkKinds = []string{
	`<a href="#tfg-links">link to this list</a>`,
	`<a href="page-2.html">relative link</a>`,
	`<a href="../folder/page-3.html">link to the folder above</a>`,
	`<a href="/root/page-4.html">link from the root</a>`,
	`<a href="?page=5">link with only a query</a>`,
	`<a href="https://example.com/path/page.html?a=1&amp;b=2#part">absolute link with a query and a fragment</a>`,
	`<a href="http://example.org/">http link</a>`,
	`<a href="//example.net/scheme-relative">link without a scheme</a>`,
	`<a href="https://example.com/caf%C3%A9?q=two%20words">percent encoded link</a>`,
	`<a href="https://xn--bcher-kva.example/">international domain link</a>`,
	`<a href="https://example.com:8443/port">link with a port</a>`,
	`<a href="mailto:someone@example.com?subject=Hello%20there">mail link</a>`,
	`<a href="tel:+1-555-0100">phone link</a>`,
	`<a href="ftp://example.com/file.txt">ftp link</a>`,
	`<a href="https://example.com/files/report.pdf" download="report.pdf">download link</a>`,
	`<a href="https://example.org/new" target="_blank" rel="noopener noreferrer">link to a new window</a>`,
	`<a href="https://example.net/sponsored" rel="nofollow sponsored">nofollow link</a>`,
	`<a href="data:text/plain,hello">data link</a>`,
}

// linksDetail is what both surfaces say about the setting.
const linksDetail = "How many links the page carries, in a list above the pictures and the blocks. " +
	"They go through 18 kinds in turn and then repeat - a link to the list itself, relative, to the folder above, from the root, a query alone, " +
	"absolute with a query and a fragment, http, without a scheme, percent encoded, an international domain, a port, " +
	"mailto:, tel:, ftp:, a download, a new window, nofollow and data: - and the text of each says which it is. " +
	"Every address is on a domain kept for examples. DOMPurify with its defaults takes out the data: address and the target that opens a new window, " +
	"and it ran out of 8 GB of memory over a million links, which lxml read in 3 seconds. " +
	"The catalogue of every element has six links of its own, which this does not count."

// linksBytes is what the list takes, worked out without building it: the list
// around its items, and the items, the kinds in turn.
func (c content) linksBytes() int64 {
	s := c.s
	if s.links == 0 {
		return 0
	}
	m, lead := s.spelling(), s.leadBytes()
	n := int64(s.links)
	var cycle, part int64
	for i, k := range linkKinds {
		cycle += int64(len(k))
		if i < s.links%len(linkKinds) {
			part += int64(len(k))
		}
	}
	item := s.innerBytes() + int64(len(m[liOpen])+len(m[liClose]))
	return 2*lead + int64(len(m[linksOpen])+len(m[linksClose])) + n*item + n/int64(len(linkKinds))*cycle + part
}

// writeLinks writes the list a chunk at a time, so a million links never sit in
// memory and an interrupted run stops between two chunks.
func (c content) writeLinks(ctx context.Context, w io.Writer, in indents) error {
	s, m := c.s, c.s.spelling()
	if s.links == 0 {
		return nil
	}
	out := newChunks(ctx, w)
	out.add(in.lead, m[linksOpen])
	for i := 0; i < s.links && out.err == nil; i++ {
		out.add(in.inner, m[liOpen])
		out.add(linkKinds[i%len(linkKinds)], m[liClose])
	}
	out.add(in.lead, m[linksClose])
	return out.close()
}
