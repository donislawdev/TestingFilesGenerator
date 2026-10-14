package htmlfile

import (
	"context"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// content is what a page writes after its label - the levels of nesting, the
// paragraphs in other scripts, the catalogue, the links, the blocks and what
// closes them - each one written from the settings and counted without being
// built. Split from settings on 2026-10-14, when indentation and links took
// that type past the methods one type may carry (typeshape_test.go): settings
// says what was asked for, and this writes it.
type content struct{ s settings }

// content is the writing of a page of these settings.
func (s settings) content() content { return content{s} }

// chunks builds what a page repeats and writes it each time it passes
// writeChunk, asking between two writes whether the run was stopped. After the
// first error it builds nothing more, and close says what the error was.
type chunks struct {
	ctx context.Context
	w   io.Writer
	buf []byte
	err error
}

func newChunks(ctx context.Context, w io.Writer) *chunks {
	return &chunks{ctx: ctx, w: w, buf: make([]byte, 0, writeChunk+1024)}
}

// add appends two pieces, one after the other - most often an indentation and
// what stands at it.
func (c *chunks) add(first, second string) {
	if c.err != nil {
		return
	}
	c.buf = append(append(c.buf, first...), second...)
	if len(c.buf) >= writeChunk {
		c.err = flush(c.ctx, c.w, c.buf)
		c.buf = c.buf[:0]
	}
}

// close writes what is left, or says why nothing more was written.
func (c *chunks) close() error {
	if c.err != nil {
		return c.err
	}
	return core.WriteAll(c.w, c.buf)
}

// flush writes what was built so far, after asking whether the run was
// interrupted.
func flush(ctx context.Context, w io.Writer, buf []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return core.WriteAll(w, buf)
}

// levelIndent is the indentation of the level k levels in, cut from the
// deepest one rather than built for every level, nothing unless the page is
// indented.
func (c content) levelIndent(in indents, k int) string {
	return in.lead[:min(len(in.lead), int(indentationBytes(c.s.bodyDepth()+k)))]
}

// levelsBytes is what the levels take opened, or closed - each one at the depth
// it opens at, which an indented page writes in front of it. Indented, a level
// k deep has 2(b+k) spaces, so the sum is worked out rather than added up.
func (c content) levelsBytes(piece string) int64 {
	s := c.s
	n := int64(s.nesting)
	total := n * int64(len(piece))
	if s.indented() {
		b := int64(s.bodyDepth())
		total += int64(len(indentUnit)) * (n*b + n*(n-1)/2)
	}
	return total
}

// writeOpening writes what follows the label: the levels of nesting, then the
// paragraphs in other scripts and the catalogue. The levels are written a
// chunk at a time - a hundred thousand of them indented are 10 GB.
func (c content) writeOpening(ctx context.Context, w io.Writer, in indents) error {
	s, m := c.s, c.s.spelling()
	out := newChunks(ctx, w)
	for k := 0; k < s.nesting && out.err == nil; k++ {
		out.add(c.levelIndent(in, k), m[divOpen])
	}
	if out.err == nil {
		out.buf = c.appendInternational(out.buf, in.lead)
		out.buf = c.appendCatalogue(out.buf, in.lead)
	}
	return out.close()
}

// openingBytes is what writeOpening writes, worked out without building it.
func (c content) openingBytes() int64 {
	return c.levelsBytes(c.s.spelling()[divOpen]) + c.internationalBytes() + c.catalogueBytes()
}

// writeClosing writes what closes the levels from the innermost out, then the
// body and the document. It follows the last block rather than ending it, so a
// page closing a hundred thousand indented levels never builds them as one
// record.
func (c content) writeClosing(ctx context.Context, w io.Writer, in indents) error {
	s, m := c.s, c.s.spelling()
	out := newChunks(ctx, w)
	for k := s.nesting - 1; k >= 0 && out.err == nil; k-- {
		out.add(c.levelIndent(in, k), m[divClose])
	}
	out.add("", c.documentClose())
	return out.close()
}

// closingBytes is what writeClosing writes, worked out without building it.
func (c content) closingBytes() int64 {
	return c.levelsBytes(c.s.spelling()[divClose]) + int64(len(c.documentClose()))
}

// documentClose closes the body and the document of a page, and nothing in a
// fragment, which has neither.
func (c content) documentClose() string {
	s := c.s
	if s.shape != structureDocument {
		return ""
	}
	closing := s.spelling()[bodyClose]
	if s.indented() {
		return joined(layOut(closing, s.bodyDepth()))
	}
	return closing
}

// blocks is the body builder for these settings.
//
// What closes the levels, the body and the document follows the last record
// (writeClosing) and is counted out of the bytes the records fill, so a change
// to it has to reach both. Until 2026-10-14 it sat in the last record, and the
// golden hashes - a fragment and a page nested on one line among them - hold
// the bytes to what they were then.
func (c content) blocks(in indents) blocks {
	return blocks{m: c.s.spelling(), lead: in.lead, inner: in.inner}
}

// shortestRecord is the smallest closing record, worked out without building
// the indentation: an empty paragraph at the depth of the content.
func (c content) shortestRecord() int64 {
	m := c.s.spelling()
	return c.s.leadBytes() + int64(len(m[paraOpen])+len(m[paraClose]))
}
