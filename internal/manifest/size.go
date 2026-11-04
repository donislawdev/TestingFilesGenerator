package manifest

import "encoding/json"

// How long a manifest is, to the byte.
//
// Counted rather than estimated since 2026-10-26. Until then a constant stood
// here - 786 B an entry, measured on txt files on 2026-08-26 - and the warning
// that a manifest would be too large to read came from it. Measured again on
// 2026-10-26 (docs/STABILITY-REVIEW-2026-10-26.md, section 6.0): an entry
// costs 838 B for txt, 893 for csv and 1577 for eml, so the constant was low
// for every format, and a run of 20 500 text files wrote a manifest of
// 17 170 219 B with no warning and no too_large_to_read_back in it - which
// verify and cleanup then refused. No single number is right for every format,
// names and notes, so the document is measured instead.
//
// Measured an entry at a time rather than by encoding the whole document, so
// knowing the size costs one entry's worth of memory. A dry run of a million
// files already peaks near three gigabytes (measured the same day), and a
// second copy of the document to count it would be a large share of that.
//
// The arithmetic below is the layout Encode writes (encode.go): the document
// with an empty "files" list, and then the entries in place of the brackets.
// Since 2026-11-02 Encode writes it an entry at a time rather than asking
// encoding/json for the whole document, and the two are still held apart on
// purpose: a guard holds this count to the length of Encode's output for
// documents of none, one and many entries
// (TestTheManifestSizeIsTheLengthOfTheManifest), and another holds Encode's
// bytes to encoding/json's own, so a layout both got wrong the same way is
// still caught.
//
// Until 2026-11-04 the count also settled whether this build would read the
// document back, into a field of the summary. The ceiling it was held against
// was on the whole document, and it went when the reader learned to take a
// document a piece at a time (read.go) - the field went with it, and what is
// left is the length: what the free space is held against.

// entryIndent is what Encode puts in front of every line of an entry: the
// entry is two levels into the document.
const entryIndent = "    "

// entryBytes is how long one entry is inside the document, its indentation
// included.
func entryBytes(f File) (int64, error) {
	b, err := json.MarshalIndent(f, entryIndent, "  ")
	if err != nil {
		return 0, err
	}
	// MarshalIndent puts the prefix in front of every line but the first, and
	// in the document the first line is indented like the rest.
	return int64(len(entryIndent) + len(b)), nil
}

// documentBytes joins an empty document and its entries. With none, the
// document is what it is - "files": [] - and with some, the two brackets
// open onto a line each, the entries are separated by a comma and a line
// break, and the list closes on a line of its own one level in.
func documentBytes(empty, entries int64, n int) int64 {
	if n == 0 {
		return empty
	}
	const open, between, closing int64 = int64(len("[\n") - len("[]")), int64(len(",\n")), int64(len("\n  ]"))
	return empty + open + entries + between*int64(n-1) + closing
}

// emptyBytes is how long this document is with no entries in it.
func (m *Manifest) emptyBytes() (int64, error) {
	shell := *m
	if len(shell.Files) > 0 {
		shell.Files = []File{}
	}
	var n counted
	if err := shell.Encode(&n); err != nil {
		return 0, err
	}
	return int64(n), nil
}

// counted is a writer that keeps the count and nothing else.
type counted int64

func (c *counted) Write(p []byte) (int, error) {
	*c += counted(len(p))
	return len(p), nil
}

// entriesBytes is how long the entries of this document are, together.
func (m *Manifest) entriesBytes() (int64, error) {
	var entries int64
	for _, f := range m.Files {
		b, err := entryBytes(f)
		if err != nil {
			return 0, err
		}
		entries += b
	}
	return entries, nil
}

// EncodedBytes is exactly how long Encode makes this document as it stands,
// and changes nothing in it.
func (m *Manifest) EncodedBytes() (int64, error) {
	entries, err := m.entriesBytes()
	if err != nil {
		return 0, err
	}
	empty, err := m.emptyBytes()
	if err != nil {
		return 0, err
	}
	return documentBytes(empty, entries, len(m.Files)), nil
}

// Projection is the length of a document a run has not written yet, counted
// as its entries are offered and none of them kept.
//
// A run holds the free space against its manifest before its first byte, and
// a preview says what the run would write. Both have the plan and no document,
// and building the document a million entries long to measure it would hold
// it all at once. This counts each entry the moment it is offered and keeps
// only the summary, the same summary Add keeps.
type Projection struct {
	shell   *Manifest
	entries int64
	n       int
	err     error
}

// Projection measures the document m would become with entries added to it.
// m is taken over as the header - its run, its tool - and must hold no files.
func (m *Manifest) Projection() *Projection {
	return &Projection{shell: m}
}

// Add counts one entry the document would hold.
func (p *Projection) Add(f File) {
	if p.err != nil {
		return
	}
	b, err := entryBytes(f)
	if err != nil {
		p.err = err
		return
	}
	p.entries += b
	p.n++
	p.shell.count(f)
}

// Bytes is how long the document would be.
func (p *Projection) Bytes() (int64, error) {
	if p.err != nil {
		return 0, p.err
	}
	empty, err := p.shell.emptyBytes()
	if err != nil {
		return 0, err
	}
	return documentBytes(empty, p.entries, p.n), nil
}
