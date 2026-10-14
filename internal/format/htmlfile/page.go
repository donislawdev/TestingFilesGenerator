package htmlfile

import "strings"

// How a page is laid out from its settings, split out of html.go on 2026-10-14.
// The order is the one measured and decided in docs/HTML-2-2026-10-14.md
// section 12: the mark, the skeleton and the head, the label, the levels of
// nesting, the paragraphs in other scripts, the catalogue, the blocks, and the
// closing paragraph with everything it closes after it.

// The markup itself - the skeleton, the label, the levels and the blocks - is
// spelled in markup.go, in every layout and every way of carrying styles.

// emptyTitle is the title a page carries until a label fills it in.
const emptyTitle = `<title></title>`

// byteOrderMark is the UTF-8 mark, the only one a page may carry: the standard
// requires UTF-8 (docs/HTML-STRUCTURE-2026-09-08.md section 1). Written as its
// bytes so no escape in this file can be turned into something else on the way.
var byteOrderMark = string([]byte{0xEF, 0xBB, 0xBF})

// prologue is everything in front of the label: the mark, and the skeleton down
// to the body with whatever the catalogue and the style sheet put in the head -
// or for a fragment the mark and the sheet. A fragment has no head, and a sheet
// at its very start is one a parser puts into the head it makes up, where the
// Nu HTML Checker finds nothing wrong with it. Further down it would be a
// sheet in the body, which the checker calls an error rule by rule
// (docs/HTML-4-2026-10-14.md section 1).
func (s settings) prologue() string {
	m := s.spelling()
	p := m[sheet]
	if s.shape == structureDocument {
		p = m[headOpen] + s.catalogueHeadFor() + m[sheet] + m[headClose]
	}
	if s.bom {
		p = byteOrderMark + p
	}
	return p
}

// opening is what follows the label: the levels of nesting, then the paragraphs
// in other scripts and the catalogue. Built when the file is written and never
// kept in the plan - a hundred thousand levels are 600 KB, and the engine holds
// the plan of every file at once.
func (s settings) opening() string {
	return strings.Repeat(s.spelling()[divOpen], s.nesting) + s.internationalBlocks() + s.catalogueBlocks()
}

// openingBytes is the length of opening, worked out without building it.
func (s settings) openingBytes() int64 {
	return int64(s.nesting*len(s.spelling()[divOpen]) + len(s.internationalBlocks()) + len(s.catalogueBlocks()))
}

// blocks is the body builder for these settings.
//
// The settings have to reach the BUILDER and not only the prologue, and that is
// the part of this easy to miss: the bytes that close the levels, the body and
// the document sit in the last RECORD rather than in a footer. A change that
// swapped only the prologue would end a fragment with </body></html>, or a page
// nested a thousand deep with not one </div> - the right size, deterministic,
// and nonsense.
func (s settings) blocks() blocks {
	m := s.spelling()
	tail := m[paraClose] + strings.Repeat(m[divClose], s.nesting)
	if s.shape == structureDocument {
		tail += m[bodyClose]
	}
	return blocks{tail: tail, m: m}
}

// tailBytes is the length of what the last record ends with, worked out
// without building it.
func (s settings) tailBytes() int64 {
	m := s.spelling()
	n := len(m[paraClose]) + s.nesting*len(m[divClose])
	if s.shape == structureDocument {
		n += len(m[bodyClose])
	}
	return int64(n)
}

// adds says the settings put something into the page beyond the skeleton and
// the blocks, which is what the refusal of a size too small has to name. A page
// on one line is smaller than the same page on many, so formatting adds nothing.
func (s settings) adds() bool {
	return s.nesting > 0 || s.international || s.every || s.bom || s.styleName() != stylesNone
}
