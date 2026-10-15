package htmlfile

// How a page is laid out from its settings, split out of html.go on 2026-10-14.
// The order is the one measured and decided in docs/HTML-2-2026-10-14.md
// section 12: the mark, the skeleton and the head, the label, the levels of
// nesting, the paragraphs in other scripts, the catalogue, the links, the
// pictures, the blocks, and what closes the levels, the body and the document.

// The markup itself - the skeleton, the label, the levels and the blocks - is
// spelled in markup.go, in every layout and every way of carrying styles.

// placeholderTitle is the title a page carries when no label fills it in: at
// the smallest sizes, and in every page under --clean. Not empty, because the
// standard asks a title for text and the Nu HTML Checker calls an empty one an
// error (O277). A word that says nothing about this tool, because --clean is
// there to keep the tool's description out of the file
// (docs/HTML-6-2026-10-15.md).
const placeholderTitle = `<title>Page</title>`

// byteOrderMark is the UTF-8 mark, the only one a page may carry: the standard
// requires UTF-8 (docs/HTML-STRUCTURE-2026-09-08.md section 1). Written as its
// bytes so no escape in this file can be turned into something else on the way.
var byteOrderMark = string([]byte{0xEF, 0xBB, 0xBF})

// writeChunk is how much of what a page repeats is built before it is written,
// and so how often an interrupted run is noticed - the size the blocks are
// written in (core.FillRecords).
const writeChunk = 32 * 1024

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
	if s.indented() {
		p = joined(layOut(p, 0))
	}
	if s.bom {
		p = byteOrderMark + p
	}
	return p
}

// depth is how many elements the content sits in: the levels of nesting, and
// for a page the html and body elements around them. The label sits at the
// depth of the body, outside the levels.
func (s settings) depth() int {
	return s.bodyDepth() + s.nesting
}

// bodyDepth is the depth of what the body holds - none for a fragment, which
// has no body.
func (s settings) bodyDepth() int {
	if s.shape == structureDocument {
		return 2
	}
	return 0
}

// indents is the indentation the content is written with, and the one of what
// sits a level inside it - a list's items and a table's rows. Both are empty
// unless the page is indented, so the same writing serves every layout.
type indents struct{ lead, inner string }

// indents builds them, when the file is written. A hundred thousand levels
// deep the lead is 200 KB, and the plan of every file is held at once, so it
// is never kept in one.
func (s settings) indents() indents {
	if !s.indented() {
		return indents{}
	}
	return indents{lead: indentation(s.depth()), inner: indentation(s.depth() + 1)}
}

// leadBytes and innerBytes are their lengths, worked out without building them.
func (s settings) leadBytes() int64 {
	if !s.indented() {
		return 0
	}
	return indentationBytes(s.depth())
}

func (s settings) innerBytes() int64 {
	if !s.indented() {
		return 0
	}
	return indentationBytes(s.depth() + 1)
}

// adds says the settings put something into the page beyond the skeleton and
// the blocks, which is what the refusal of a size too small has to name. A page
// on one line is smaller than the same page on many, so only indented of the
// layouts adds anything.
func (s settings) adds() bool {
	return s.nesting > 0 || s.international || s.every || s.bom || s.styleName() != stylesNone ||
		s.links > 0 || s.indented()
}
