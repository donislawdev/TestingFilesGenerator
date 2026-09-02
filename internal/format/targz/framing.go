// What gzip costs on top of the bytes it carries, measured rather than
// written down.
//
// This file exists because a number that was written down turned out to be a
// fact about one Go release. The size of a TAR.GZ is arithmetic - it has to
// be, because the bytes pass through deflate and building the archive to
// measure it would make a preview cost what the run costs. The arithmetic
// needs to know what the gzip stream adds, and until 2026-09-01 that was three
// constants in targz.go.
//
// Go 1.27.0 changed one of them. The block that closes a level zero stream
// went from a five byte empty STORED block to a two byte one, so every archive
// came out three bytes short of its plan and the format refused to write
// anything at all - measured on every size tried, from 64 kB to 10 MB. The
// engine was right to refuse. The arithmetic was describing Go 1.26.
//
// Bumping the constant would have worked until the next release. Measuring
// asks the library that is actually linked, so it holds for the one after that
// too. The model is the same under both releases and only the constants move:
//
//	overhead(n) = base + perBlock * ceil(n / storeBlock)
//
// Measured 2026-09-01, level zero: perBlock is 5 under both, base is 23 under
// go1.26.7 and 20 under go1.27.0.
//
// One honest limit, because it would otherwise look like this file is proven
// and it is only half proven. Replacing the measurement with today's two
// constants written down would pass every test in this repository, today, on
// this compiler - the mutation runner was pointed at exactly that and it
// cannot go red. What the measurement buys is the NEXT release, and no test
// that runs today can demonstrate that. The mutations here cover the
// arithmetic being wrong; they cannot cover it being right for the wrong
// reason. That is why this comment is long: it is the only thing standing
// between a later reader and a tidy simplification back to the bug.
package targz

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"sync"
)

// framing is what a level zero gzip stream costs beyond its content.
type framing struct {
	// base is the header, the trailer, and the block that closes the stream.
	base int64
	// perBlock is what each stored block of content costs.
	perBlock int64
}

// gzipFraming measures the framing once and hands back the same answer after.
//
// Lazy rather than at init because the answer is only needed when a size is
// being worked out, and a package that measures something on every program
// start makes every command pay for the one that needs it.
var measuredFraming = sync.OnceValues(measureFraming)

// measureFraming works the two constants out from three compressions, and then
// checks the model against a fourth.
//
// Two points settle the line and the third says whether it is a line at all.
// Without that check a change to the BLOCK SIZE - rather than to the cost of a
// block - would be read as a change to the constants, and the arithmetic would
// be quietly wrong instead of loudly refused. That is the failure this whole
// file exists to stop happening a second time, so it is worth one more
// compression of a buffer that is already in memory.
func measureFraming() (framing, error) {
	one, err := storedOverhead(storeBlock)
	if err != nil {
		return framing{}, err
	}
	two, err := storedOverhead(2 * storeBlock)
	if err != nil {
		return framing{}, err
	}

	f := framing{perBlock: two - one}
	f.base = one - f.perBlock

	// Two independent checks the two points above cannot make on their own: a
	// third multiple of the block, and the empty stream, which is base alone.
	three, err := storedOverhead(3 * storeBlock)
	if err != nil {
		return framing{}, err
	}
	empty, err := storedOverhead(0)
	if err != nil {
		return framing{}, err
	}
	if want := f.base + 3*f.perBlock; three != want {
		return framing{}, fmt.Errorf(
			"targz: this build of Go frames a gzip stream in a shape this tool does not understand. "+
				"Three blocks of content cost %d B where the two measured before them predict %d B, "+
				"so the size of an archive cannot be worked out without building it",
			three, want)
	}
	if empty != f.base {
		return framing{}, fmt.Errorf(
			"targz: this build of Go frames an empty gzip stream at %d B where the measurement says %d B, "+
				"so the size of an archive cannot be worked out without building it",
			empty, f.base)
	}
	return f, nil
}

// storedOverhead is what gzip adds to n bytes at compression level zero.
//
// The content is zeros, and that is safe precisely because the level is zero:
// stored blocks carry their input unchanged, so the framing does not depend on
// what is in them. At any other level it would.
func storedOverhead(n int64) (int64, error) {
	var out bytes.Buffer
	w, err := gzip.NewWriterLevel(&out, gzip.NoCompression)
	if err != nil {
		return 0, err
	}
	if n > 0 {
		if _, err := io.CopyN(w, zeros{}, n); err != nil {
			return 0, err
		}
	}
	if err := w.Close(); err != nil {
		return 0, err
	}
	return int64(out.Len()) - n, nil
}

// zeros is an endless run of zero bytes, so the measurement allocates one
// small buffer rather than the megabyte it reads.
type zeros struct{}

func (zeros) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}
