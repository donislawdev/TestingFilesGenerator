package webp

import (
	"context"
	"encoding/binary"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

const (
	// vp8xBlock is the extended format's VP8X chunk whole: its header, and ten
	// bytes of flags, reserved bits and the canvas size.
	vp8xBlock = chunkHeader + 4 + 10
	// leastChunk is the smallest chunk there is: a header and nothing in it.
	leastChunk = chunkHeader + 4
)

// splitForACut lays a WebP out for a cut after its first cutAt bytes
// (format.Request.CutAt), so that the cut keeps the share of the picture it
// keeps of the whole file, given the picture's chunk (picture bytes, its
// header included) and the padding settlePadding left behind it.
//
// Padding in front of the picture needs the extended format. RFC 9649 lets
// the simple one hold the picture's chunk and nothing else, and lets unknown
// chunks of the extended one appear out of order (section 2.7) - so a VP8X
// chunk first and the private chunk in front of the picture. Ten decoders
// give the pixels of a WebP nothing cuts from that, and eight of them refuse
// the private chunk in front without the VP8X (docs/TRUNCATE-2026-11-11.md
// section 15.3).
//
// The front is a chunk, so it is even and at least eight bytes, and the VP8X
// before it costs eighteen. It rounds up, so a cut takes at least the share,
// as format.SplitFront has it. Where the share wants nothing in front, or the
// padding is too short for the VP8X and a chunk, the file keeps the simple
// format with all of its padding behind the picture - then the cut keeps up
// to those 26 B more of the picture, and in a file of a few kilobytes cut by
// 1% it may fall in that padding.
//
// A WebP nothing cuts is byte for byte what it was (D11).
func splitForACut(m *memo, picture, total, cutAt int64) {
	pad := total - riffHeader - picture
	if cutAt <= 0 || cutAt >= total || pad < vp8xBlock+leastChunk ||
		format.PaddingInFront(riffHeader, picture, cutAt, total) <= 0 {
		return
	}
	room := pad - vp8xBlock
	front := max(leastChunk, format.SplitFront(room, riffHeader+vp8xBlock, picture, cutAt, total, leastChunk))
	front += front % 2
	if front > room {
		front -= 2
	}
	m.front = front
	settlePadding(m, room-front)
}

// inFront is what a WebP laid out for a cut puts between its RIFF header and
// its picture: the VP8X chunk, and the private chunk in front. A WebP nothing
// cuts puts neither.
func inFront(m memo) (structure, padding int64) {
	if m.front == 0 {
		return 0, 0
	}
	return vp8xBlock, m.front
}

// writeFront writes the VP8X chunk and the private chunk in front of the
// picture, when the plan has them. The flags stay clear - no colour profile,
// no transparency, no metadata, no animation - and the canvas is the
// picture's size.
func writeFront(ctx context.Context, w io.Writer, m memo) error {
	if m.front == 0 {
		return nil
	}
	var x [vp8xBlock]byte
	copy(x[:4], "VP8X")
	binary.LittleEndian.PutUint32(x[4:8], vp8xBlock-leastChunk)
	putUint24(x[12:15], m.width-1)
	putUint24(x[15:18], m.height-1)
	if err := writeAll(w, x[:]); err != nil {
		return err
	}
	return writeFiller(ctx, w, paddingName, m.seed+2, m.front-leastChunk)
}

func putUint24(b []byte, v int) {
	b[0], b[1], b[2] = byte(v), byte(v>>8), byte(v>>16)
}
