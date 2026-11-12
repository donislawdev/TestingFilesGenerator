package wav

import (
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// mark marks the map of the file (format.Mark), all of it known from the
// plan: the headers and the label, the data chunk with its samples, and the
// padding chunk after it.
func (m memo) mark(w io.Writer) {
	data := int64(baseSize) + int64(len(m.info)) - chunkHeader
	format.Mark(w, format.RoleStructure, 0, data)
	format.Mark(w, format.RoleContent, data, chunkHeader+m.dataLen)
	if m.withPad {
		format.Mark(w, format.RolePadding, data+chunkHeader+m.dataLen, chunkHeader+m.junkLen)
	}
}
