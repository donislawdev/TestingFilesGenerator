package pdf

// The padding: a comment block that brings the document to the size asked for.

import (
	"context"
	"fmt"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

const (
	// padChunkSize is how much padding is built before each write, and how
	// often cancellation is noticed.
	padChunkSize = 32 * 1024

	// A comment is at least a per cent sign and a newline, so one byte of
	// padding is the single amount that cannot be produced.
	minComment = 2
)

// The padding channel is a comment block placed after the trailer and before
// startxref.
//
// The obvious place, and the one this project's own notes assumed, is between
// startxref and %%EOF. Measured, that is wrong twice over.
//
// A reader finds the cross reference table by scanning backwards from the end
// of the file for the startxref keyword, and how far back it looks is up to
// the reader. Xpdf 4.06 reads a comment of 1004 bytes there and fails at
// 1005. The Windows renderer reads any size. So that placement produces files
// that open for one tester and not for another, which is worse than a
// placement that fails for everybody.
//
// After the trailer nothing downstream refers to a position: the objects and
// the cross reference table both sit earlier, and startxref still points at
// the same offset. Measured at 64 bytes, 4 KB, 100 KB, 1 MB, 2 MB and 10 MB
// against Xpdf, exiftool and the Windows renderer - all read the document and
// report one page.

// writeComment emits the padding as a comment block without holding it in
// memory. Every line starts with a per cent sign, so a reader skips the lot.
func writeComment(ctx context.Context, w io.Writer, seed uint64, n int64) error {
	if n <= 0 {
		return nil
	}
	if n < minComment {
		return fmt.Errorf("pdf: %d B of padding cannot be written as a comment", n)
	}

	// Line length is fixed so the block stays readable in an editor. The last
	// line takes whatever is left.
	const lineLen = 64
	rng := core.NewRand(seed)
	buf := make([]byte, 0, padChunkSize+lineLen)

	remaining := n
	for remaining > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		buf = buf[:0]
		for len(buf) < padChunkSize && remaining > int64(len(buf)) {
			left := remaining - int64(len(buf))
			size := int64(lineLen)
			switch {
			case left <= lineLen:
				// Everything left goes on one line. The caller guaranteed
				// this is never exactly one byte.
				size = left
			case left-lineLen < minComment:
				// A full line here would leave one byte behind, and one byte
				// cannot be a comment. Shorten this line so the last one has
				// room. Found by the property test, not by reasoning.
				size = lineLen - minComment
			}
			if size < minComment {
				return fmt.Errorf("pdf: %d B left over, which cannot form a comment line", size)
			}
			buf = append(buf, '%')
			for i := int64(0); i < size-2; i++ {
				buf = append(buf, padAlphabet[rng.IntN(len(padAlphabet))])
			}
			buf = append(buf, '\n')
		}

		if _, err := w.Write(buf); err != nil {
			return err
		}
		remaining -= int64(len(buf))
	}
	return nil
}

// padAlphabet keeps the padding printable, so opening the file in an editor
// shows comment lines rather than binary noise.
var padAlphabet = []byte("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
