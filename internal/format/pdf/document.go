package pdf

// The document itself: its objects, the cross reference table and the trailer.

import (
	"bytes"
	"fmt"
	"strings"
)

// places is where the parts of a document start, for its map: the first
// object, and the cross reference table the trailer follows.
type places struct {
	objects, xref int64
}

// document builds everything up to the trailer, and the closing lines, of a
// document with front bytes of padding between its header lines and its first
// object (splitForACut).
//
// The two are returned apart because the padding goes between them. The
// closing lines carry the offset of the cross reference table, which sits in
// the first part and therefore does not move. The front padding is not in the
// first part - the writer puts it in at at.objects - but every offset counts
// it. The cross reference table writes each offset in ten digits, so the first
// part is the same length whatever front is.
func document(m memo, front int64) (prefix, suffix []byte, at places) {
	var body bytes.Buffer
	// The version is the one asked for and nothing else follows from it: the
	// file uses nothing newer than 1.4, so every version offered here is true
	// of it. 2.0 is not offered, because it requires things this file does not
	// have yet - a file identifier and the widths of the font's characters.
	body.WriteString("%PDF-" + m.opts.version + "\n")
	// A comment of high bytes tells any tool handling the file that it is
	// binary, which stops a transfer from mangling the line endings.
	body.Write([]byte{'%', 0xe2, 0xe3, 0xcf, 0xd3, '\n'})

	var objects []string

	pages := m.opts.pages
	kids := make([]string, 0, pages)
	for i := 0; i < pages; i++ {
		// Objects: 1 catalog, 2 pages, 3 font, 4 info, then per page a page
		// object and a content stream.
		kids = append(kids, fmt.Sprintf("%d 0 R", 5+i*2))
	}

	objects = append(objects, "<</Type/Catalog/Pages 2 0 R>>")
	objects = append(objects, fmt.Sprintf("<</Type/Pages/Kids[%s]/Count %d>>",
		strings.Join(kids, " "), pages))
	objects = append(objects, "<</Type/Font/Subtype/Type1/BaseFont/Helvetica/Encoding/WinAnsiEncoding>>")
	objects = append(objects, infoObject(m))

	// A page that asks to be turned says so after its paper, and a page that
	// does not says nothing - an explicit /Rotate 0 would be bytes the file
	// never had before it could be turned.
	rotate := ""
	if m.opts.rotate != 0 {
		rotate = fmt.Sprintf("/Rotate %d", m.opts.rotate)
	}
	for i := 0; i < pages; i++ {
		content := pageContent(m, i)
		paper := m.opts.geometry(i)
		objects = append(objects, fmt.Sprintf(
			"<</Type/Page/Parent 2 0 R/MediaBox[0 0 %d %d]%s/Contents %d 0 R/Resources<</Font<</F1 3 0 R>>>>>>",
			paper.width, paper.height, rotate, 6+i*2))
		objects = append(objects, fmt.Sprintf("<</Length %d>>\nstream\n%sendstream", len(content), content))
	}

	offsets := make([]int, 0, len(objects))
	for i, obj := range objects {
		offsets = append(offsets, body.Len())
		fmt.Fprintf(&body, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}

	xrefPos := body.Len()
	at = places{objects: int64(offsets[0]), xref: int64(xrefPos)}
	fmt.Fprintf(&body, "xref\n0 %d\n", len(objects)+1)
	body.WriteString("0000000000 65535 f \n")
	for _, off := range offsets {
		fmt.Fprintf(&body, "%010d 00000 n \n", int64(off)+front)
	}
	fmt.Fprintf(&body, "trailer\n<</Size %d/Root 1 0 R/Info 4 0 R>>\n", len(objects)+1)

	return body.Bytes(), closing(at.xref + front), at
}

// closing is startxref, the offset of the cross reference table it points at,
// and the end of file marker.
func closing(xref int64) []byte {
	return []byte(fmt.Sprintf("startxref\n%d\n%%%%EOF\n", xref))
}
