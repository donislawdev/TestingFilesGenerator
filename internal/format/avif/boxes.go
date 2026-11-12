package avif

import (
	"encoding/binary"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/isobmff"
)

// markBoxes marks the map of the top level boxes the coder handed back in b,
// written from the first byte of the file w writes: the media data as the
// content, every other box as structure.
//
// The boxes are the coder's, so they are read rather than assumed: by the four
// byte length, the eight byte one a length of one announces, and the rest of b
// a length of nought means.
func markBoxes(w io.Writer, b []byte) {
	for pos := int64(0); pos+isobmff.BoxHeader <= int64(len(b)); {
		size := int64(binary.BigEndian.Uint32(b[pos:]))
		switch {
		case size == 0:
			size = int64(len(b)) - pos
		case size == 1 && pos+2*isobmff.BoxHeader <= int64(len(b)):
			size = int64(binary.BigEndian.Uint32(b[pos+8:]))<<32 | int64(binary.BigEndian.Uint32(b[pos+12:]))
		}
		if size < isobmff.BoxHeader {
			// Not a box this could walk past. What is left stays unmarked,
			// and the guard of the map says so.
			return
		}
		role := format.RoleStructure
		if string(b[pos+4:pos+isobmff.BoxHeader]) == "mdat" {
			role = format.RoleContent
		}
		format.Mark(w, role, pos, size)
		pos += size
	}
}
