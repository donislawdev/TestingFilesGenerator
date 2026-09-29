package pdf

// The document itself: its objects, the cross reference table and the trailer.

import (
	"bytes"
	"fmt"
	"strings"
)

// document builds everything up to the trailer, and the closing lines.
//
// The two are returned apart because the padding goes between them. The
// closing lines carry the offset of the cross reference table, which sits in
// the first part and therefore does not move.
func document(m memo) (prefix, suffix []byte) {
	var body bytes.Buffer
	body.WriteString("%PDF-1.7\n")
	// A comment of high bytes tells any tool handling the file that it is
	// binary, which stops a transfer from mangling the line endings.
	body.Write([]byte{'%', 0xe2, 0xe3, 0xcf, 0xd3, '\n'})

	var objects []string

	kids := make([]string, 0, m.pages)
	for i := 0; i < m.pages; i++ {
		// Objects: 1 catalog, 2 pages, 3 font, 4 info, then per page a page
		// object and a content stream.
		kids = append(kids, fmt.Sprintf("%d 0 R", 5+i*2))
	}

	objects = append(objects, "<</Type/Catalog/Pages 2 0 R>>")
	objects = append(objects, fmt.Sprintf("<</Type/Pages/Kids[%s]/Count %d>>",
		strings.Join(kids, " "), m.pages))
	objects = append(objects, "<</Type/Font/Subtype/Type1/BaseFont/Helvetica/Encoding/WinAnsiEncoding>>")
	objects = append(objects, infoObject(m))

	for i := 0; i < m.pages; i++ {
		content := pageContent(m, i)
		objects = append(objects, fmt.Sprintf(
			"<</Type/Page/Parent 2 0 R/MediaBox[0 0 %d %d]/Contents %d 0 R/Resources<</Font<</F1 3 0 R>>>>>>",
			m.pageSize.width, m.pageSize.height, 6+i*2))
		objects = append(objects, fmt.Sprintf("<</Length %d>>\nstream\n%sendstream", len(content), content))
	}

	offsets := make([]int, 0, len(objects))
	for i, obj := range objects {
		offsets = append(offsets, body.Len())
		fmt.Fprintf(&body, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}

	xrefPos := body.Len()
	fmt.Fprintf(&body, "xref\n0 %d\n", len(objects)+1)
	body.WriteString("0000000000 65535 f \n")
	for _, off := range offsets {
		fmt.Fprintf(&body, "%010d 00000 n \n", off)
	}
	fmt.Fprintf(&body, "trailer\n<</Size %d/Root 1 0 R/Info 4 0 R>>\n", len(objects)+1)

	suffix = []byte(fmt.Sprintf("startxref\n%d\n%%%%EOF\n", xrefPos))
	return body.Bytes(), suffix
}
