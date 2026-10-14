package htmlfile

import "strings"

// How a page is laid out from its settings, split out of html.go on 2026-10-14.
// The order is the one measured and decided in docs/HTML-2-2026-10-14.md
// section 12: the mark, the skeleton and the head, the label, the levels of
// nesting, the paragraphs in other scripts, the catalogue, the blocks, and the
// closing paragraph with everything it closes after it.

const (
	// The skeleton of a page, in two halves so the catalogue can put what it
	// needs into the head between them. Joined they are the bytes every page
	// opened with before there were settings.
	headOpen = `<!DOCTYPE html>` + "\n" +
		`<html lang="en">` + "\n" +
		`<head>` + "\n" +
		`<meta charset="utf-8">` + "\n" +
		`<title></title>` + "\n"
	headClose = `</head>` + "\n" +
		`<body>` + "\n"

	// The title is filled in per file when a label is wanted, so the empty pair
	// above is what the smallest document carries.
	emptyTitle = `<title></title>`

	paraOpen  = "<p>"
	paraClose = "</p>\n"
	bodyClose = "</body>\n</html>\n"

	// One level of nesting, each on a line of its own. A page whose levels
	// shared a line would be a page of one very long line, which is a different
	// question with a setting of its own to come.
	divOpen  = "<div>\n"
	divClose = "</div>\n"
)

// byteOrderMark is the UTF-8 mark, the only one a page may carry: the standard
// requires UTF-8 (docs/HTML-STRUCTURE-2026-09-08.md section 1). Written as its
// bytes so no escape in this file can be turned into something else on the way.
var byteOrderMark = string([]byte{0xEF, 0xBB, 0xBF})

// prologue is everything in front of the label: the mark, and the skeleton down
// to the body with whatever the catalogue puts in the head - or for a fragment
// nothing but the mark.
func (s settings) prologue() string {
	p := ""
	if s.shape == structureDocument {
		p = headOpen + s.catalogueHeadFor() + headClose
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
	return strings.Repeat(divOpen, s.nesting) + s.internationalBlocks() + s.catalogueBlocks()
}

// openingBytes is the length of opening, worked out without building it.
func (s settings) openingBytes() int64 {
	return int64(s.nesting*len(divOpen) + len(s.internationalBlocks()) + len(s.catalogueBlocks()))
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
	tail := paraClose + strings.Repeat(divClose, s.nesting)
	if s.shape == structureDocument {
		tail += bodyClose
	}
	return blocks{tail: tail}
}

// tailBytes is the length of what the last record ends with, worked out
// without building it.
func (s settings) tailBytes() int64 {
	n := len(paraClose) + s.nesting*len(divClose)
	if s.shape == structureDocument {
		n += len(bodyClose)
	}
	return int64(n)
}

// adds says the settings put something into the page beyond the skeleton and
// the blocks, which is what the refusal of a size too small has to name.
func (s settings) adds() bool { return s.nesting > 0 || s.international || s.every || s.bom }
