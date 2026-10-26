package audit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"syscall"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// Reading a file the manifest lists. One way in for verify and cleanup, so
// the two cannot come to read the same file differently - and the way in
// carries three things the plain os.Open it replaced did not, each found by
// review on 2026-10-21 (docs/STABILITY-REVIEW-2026-10-26.md, R01 and R04):
//
//   - The file is opened without waiting and only a file is read
//     (core.OpenRegular). A name the manifest lists for an empty file and
//     somebody swapped for a pipe had the right size, 0, and os.Open then
//     waited on it for good.
//   - Ctrl+C is asked between pieces, not only between files. A run of this
//     tool writes files measured in gigabytes, and the pool asked only at the
//     next file, so stopping verify meant waiting out the largest one.
//   - No more is read than the manifest records, plus one byte to tell that
//     there is more. A file somebody keeps appending to was read until they
//     stopped.

// listed is what reading one listed file found.
type listed struct {
	// seen is the file as its handle described it before the first byte was
	// read. Taken from the handle rather than the name, because on Windows
	// os.Stat reads the identity of a file lazily, by name, at the moment it
	// is compared - so two looks by name both describe whatever is there at
	// the end and agree about a file that was replaced in between (the review
	// of #157, point 4, read from stat_windows.go). Measured on 2026-10-26 on
	// NTFS: this side against os.Lstat tells a replaced file of the same size
	// and time apart, and a rewrite in place moves the time at once, even
	// while the writer still holds the file open.
	seen os.FileInfo
	// last is the file as its handle described it when the reading ended, and
	// the same as seen when nothing was read.
	last os.FileInfo
	// sum is the SHA-256 of what was read, empty when nothing was.
	sum string
	// read is how many bytes were read.
	read int64
}

// absent says an open failed because nothing is at the name.
//
// One answer for every system, and that is what it is for. A listed file
// under a directory that has since become a file is ENOTDIR on Linux and
// macOS and "the path is not found" on Windows, which Go already counts as
// not existing - so without this, verify called the same directory "missing"
// on one machine and "unreadable" on the other (untouchable rule 24).
func absent(err error) bool {
	return errors.Is(err, fs.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}

// grew says more was read than the manifest records.
func (l listed) grew(want int64) bool { return l.read > want }

// moved says the file was not the same when the reading ended as when it
// began - its size or its time changed under the read.
func (l listed) moved() bool {
	return l.last.Size() != l.seen.Size() || !l.last.ModTime().Equal(l.seen.ModTime())
}

// readListed opens a listed file and hashes it, when its size is the size the
// manifest records. A file of another size is not read at all, so a file of
// obviously the wrong length does not cost a read of however many gigabytes it
// is.
//
// scratch belongs to the caller and is reused across every file that caller
// answers for, for the reason written at drain. It may be nil, and then this
// asks for one of its own.
//
// A cancelled read comes back as the context's error and nothing else. It is
// not an answer about the file, and the pool drops it rather than letting it
// stand as one (drain).
func readListed(ctx context.Context, path string, want int64, scratch []byte) (listed, error) {
	f, seen, err := core.OpenRegular(path)
	if err != nil {
		return listed{}, err
	}
	defer func() { _ = f.Close() }()

	out := listed{seen: seen, last: seen}
	if seen.Size() != want {
		return out, nil
	}
	if scratch == nil {
		scratch = make([]byte, hashScratch)
	}
	// One past what is recorded, so a file that is still being written to
	// shows as longer rather than as the right length with the wrong content.
	h := sha256.New()
	if out.read, err = HashAtMost(ctx, f, want+1, h, scratch); err != nil {
		return listed{}, err
	}
	out.sum = hex.EncodeToString(h.Sum(nil))

	if out.last, err = f.Stat(); err != nil {
		return listed{}, err
	}
	return out, nil
}

// HashAtMost feeds up to limit bytes of r to h, a buffer at a time, and asks
// about Ctrl+C before every one.
//
// Exported for the guard that asks both of those of it directly. Reaching them
// through verify needs a read that is still going when the cancellation lands,
// and the only way to arrange that from outside is a clock - which is a guard
// that is slow on a slow machine and green for the wrong reason on a fast one.
//
// A loop of its own rather than io.CopyBuffer. The buffer size is measured -
// hashing a 256 MB file, median of five on 2026-09-05: io.Copy 199 ms, 64 KiB
// 182 ms, 256 KiB 161 ms, 1 MiB 162 ms - and io.CopyBuffer threw it away for an
// os.File, which implements io.WriterTo and takes the copy over with 32 KiB of
// its own. So the loop is what makes the buffer count, as well as what lets a
// read be stopped part way.
func HashAtMost(ctx context.Context, r io.Reader, limit int64, h io.Writer, scratch []byte) (int64, error) {
	var read int64
	for read < limit {
		if err := ctx.Err(); err != nil {
			return read, err
		}
		piece := scratch
		if left := limit - read; left < int64(len(piece)) {
			piece = piece[:left]
		}
		n, err := r.Read(piece)
		if n > 0 {
			_, _ = h.Write(piece[:n]) // a hash never refuses a write
			read += int64(n)
		}
		if errors.Is(err, io.EOF) {
			return read, nil
		}
		if err != nil {
			return read, err
		}
	}
	return read, nil
}
