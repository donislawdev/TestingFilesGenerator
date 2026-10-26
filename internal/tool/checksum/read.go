package checksum

import (
	"context"
	"encoding/hex"
	"errors"
	"hash"
	"io"
	"os"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// Reading files, for all three tools of this package. One way in, so a file
// in a folder is opened with the same care as a file named on its own - the
// review of #157 found a pipe swapped in between two looks at a name, and the
// answer to that has to hold for the thousandth file of a folder as well
// (docs/NARZEDZIA-SUMY-2026-09-29.md §13.4).

// counted is told each piece of a file as it is read, with the size the file
// had when it was opened.
type counted func(read, size int64)

// digest reads a file once and works out every chosen checksum of it.
//
// scratch is the buffer to read through, owned by the caller - one per worker
// when a folder is read, so a hundred thousand files are not a hundred
// thousand buffers. nil asks for one of its own.
//
// What was opened is handed back as well: its size is the size of what was
// read, and its identity is what tells the checksum file itself from a file
// it lists. Asked of the open file, which on Windows is the only way to have
// the identity at all (docs/REVIEW-157-2026-09-30.md, point 4).
func digest(ctx context.Context, path string, chosen map[string]bool, scratch []byte, count counted) (map[string]string, os.FileInfo, error) {
	f, before, err := openRegular(path)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = f.Close() }()

	hashes := map[string]hash.Hash{}
	writers := make([]io.Writer, 0, len(chosen))
	for _, a := range algorithms {
		if chosen[a.name] {
			h := a.make()
			hashes[a.name] = h
			writers = append(writers, h)
		}
	}
	size := before.Size()
	read, err := copyWatching(ctx, io.MultiWriter(writers...), f, scratch, func(n int64) { count(n, size) })
	if err != nil {
		return nil, nil, err
	}
	after, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	if changed(before, after, read) {
		return nil, nil, &ChangedError{Path: path}
	}

	out := make(map[string]string, len(hashes))
	for name, h := range hashes {
		out[name] = hex.EncodeToString(h.Sum(nil))
	}
	return out, before, nil
}

// openRegular is core.OpenRegular with its refusal in the words of these
// tools - a checksum of a directory, not just a directory.
func openRegular(path string) (*os.File, os.FileInfo, error) {
	f, info, err := core.OpenRegular(path)
	var not *core.NotRegularError
	if errors.As(err, &not) {
		return nil, nil, &NotAFileError{Path: path, Directory: not.Directory}
	}
	return f, info, err
}

// changed says whether the file moved under the read. A checksum of a file
// that was being written is a checksum of no version of it.
func changed(before, after os.FileInfo, read int64) bool {
	return after.Size() != before.Size() || read != before.Size() || !after.ModTime().Equal(before.ModTime())
}

// chunk is how much is read between two looks at the context, when the caller
// has no buffer of its own. A megabyte is a millisecond of sha256 and a few of
// the slowest algorithm, so a stop lands at once and the look costs nothing.
const chunk = 1 << 20

// copyWatching is io.Copy that stops when asked and says how far it got.
func copyWatching(ctx context.Context, dst io.Writer, src io.Reader, buf []byte, count func(n int64)) (int64, error) {
	if buf == nil {
		buf = make([]byte, chunk)
	}
	var done int64
	for {
		if err := ctx.Err(); err != nil {
			return done, err
		}
		// What was read is written before the error is looked at, because a
		// reader may hand back its last bytes and io.EOF in one call. Nothing
		// read is nothing written, so there is no need to ask first.
		n, err := src.Read(buf)
		if _, werr := dst.Write(buf[:n]); werr != nil {
			return done, werr
		}
		done += int64(n)
		count(int64(n))
		if err == io.EOF {
			return done, nil
		}
		if err != nil {
			return done, err
		}
	}
}
