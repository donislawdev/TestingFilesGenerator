package targz

import (
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// The map of a tar.gz (format.Mark, docs/TRUNCATE-2026-11-11.md section 6.9).
//
// The gzip header first: its fixed part, the extra field that carries padding,
// and the comment that carries the label. Then the stream. A stored stream -
// the default - carries the tar unchanged in blocks of storeBlock bytes, each
// behind a header of its own, so where each entry lands in the file is
// arithmetic: the directories, the files as the content, the filler entry as
// padding, and the end of archive marker with the stream's closing block and
// the gzip trailer as the end.
//
// A compressed stream is content from its first byte to the trailer, filler
// entry included. Where the filler's bytes land inside deflate's blocks is
// not knowable without decoding them, so the map cannot part it from the
// files, and a cut there reads as a cut in the content (O336).

const (
	// gzipHead is the fixed part of a gzip header. This writer gives no name
	// and asks for no header checksum, so nothing else is in it.
	gzipHead = 10
	// gzipTrailer is the checksum and the length that close a gzip stream.
	gzipTrailer = 8
)

// markArchive marks the map of the archive m describes, before its first byte.
func markArchive(w io.Writer, m memo) {
	extra, comment := extraCost(m.withExtra, m.extraLen), commentCost(m.comment)
	head := gzipHead + extra + comment
	format.Mark(w, format.RoleStructure, 0, gzipHead)
	format.Mark(w, format.RolePadding, gzipHead, extra)
	format.Mark(w, format.RoleStructure, gzipHead+extra, comment)
	if m.squeeze.On() {
		format.Mark(w, format.RoleContent, head, -1)
		return
	}

	f, _ := measuredFraming()
	// at is where the tar's byte t lands in the file. A region that starts
	// at the start of a block takes the block's header with it.
	at := func(t int64) int64 {
		k, r := t/storeBlock, t%storeBlock
		out := head + k*(storeBlock+f.perBlock)
		if r != 0 {
			out += f.perBlock + r
		}
		return out
	}
	dirs := int64(len(m.layout.Directories())) * tarBlock
	files := dirs
	for _, c := range m.children {
		files += tarBlock + roundUpBlock(c.plan.Bytes)
	}
	end := tarLength(m) - 2*tarBlock
	format.Mark(w, format.RoleStructure, head, at(dirs)-head)
	format.Mark(w, format.RoleContent, at(dirs), at(files)-at(dirs))
	format.Mark(w, format.RolePadding, at(files), at(end)-at(files))
	format.Mark(w, format.RoleEnd, at(end), -1)
}
