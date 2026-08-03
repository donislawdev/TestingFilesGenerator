package core

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"unicode/utf8"
)

// Several formats are read one record at a time - a log entry, a CSV row, a
// JSON object, an XML element. For all of them the same rule holds: filling to
// an exact byte count must never leave a half record at the end.
//
// A truncated last record is the worst kind of defect this tool can ship. The
// file is the right size, it is repeatable, and it looks like a real file
// caught mid write - so the failure reads as realism rather than as a bug. Only
// a structural check notices.
//
// The remedy is the same everywhere, so it lives here once: whole records while
// another one still leaves room, then a closing record built to the byte, with
// the difference absorbed by a value the format has room to stretch.

// recordChunk is how much is built before a write. It also sets how often
// cancellation is noticed, so it stays small enough that interrupting a multi
// gigabyte file feels immediate.
const recordChunk = 32 * 1024

// Record is one whole unit of a record based format.
//
// Two methods rather than one with a "natural length" sentinel. Zero is a legal
// length to ask for elsewhere in this codebase, and a sentinel that collides
// with a legal value is how a guard ends up testing the wrong thing.
type Record interface {
	// Append adds one record of whatever length it comes out.
	Append(dst []byte, rng *rand.Rand) []byte
	// AppendExact adds one record of exactly n bytes, including whatever
	// terminates it. It is only ever called with n at or above Shortest.
	AppendExact(dst []byte, rng *rand.Rand, n int64) []byte
	// Shortest is the fewest bytes AppendExact can produce, for any draw. It
	// has to hold for every draw rather than for the lucky one, otherwise the
	// closing record cannot reach the length it was asked for.
	Shortest() int64
	// Discard undoes whatever the last Append changed about the builder. That
	// record was built only to measure it and then thrown away, which happens
	// exactly once per file - on the one that no longer leaves room for a whole
	// closing record.
	//
	// A builder that keeps nothing between records has nothing to undo. One
	// that counts, though, has already counted the record that was thrown away,
	// and without this the file ends with a hole in the numbering. Nothing else
	// in this package can see that: the size is exact, the bytes repeat, and
	// every reader still parses the file.
	Discard()
}

// FillRecords writes exactly remaining bytes as whole records.
//
// It never emits a partial record. The caller has already written any prologue
// and subtracted it from remaining, and the closing record carries whatever
// epilogue the format needs.
func FillRecords(ctx context.Context, w io.Writer, rng *rand.Rand, remaining int64, rec Record) error {
	shortest := rec.Shortest()
	if remaining < shortest {
		return fmt.Errorf("core: %d B are owed and the shortest whole record needs %d B", remaining, shortest)
	}

	// One buffer, reused. A record built into its own allocation costs a
	// multiple of the file size in garbage over a large file, and the resource
	// guard measures exactly that.
	buf := make([]byte, 0, recordChunk+1024)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		mark := len(buf)
		buf = rec.Append(buf, rng)
		// A record that adds nothing would leave remaining untouched and the
		// buffer never reaching the chunk size, so this loop would spin without
		// end and without growing - the one failure shape a size guard cannot
		// see, because no file is ever produced to measure. No builder does it
		// today and the interface does not forbid it, so it is named here
		// rather than left to a hang somebody has to interrupt.
		if len(buf) == mark {
			return fmt.Errorf("core: a record of this format added no bytes, so the file could never be filled")
		}
		// Stop while what is left still fits a whole closing record. That is
		// what keeps the last one from being a stub.
		if remaining-int64(len(buf)-mark) < shortest {
			buf = buf[:mark]
			// The bytes are gone, so anything the builder counted for them has
			// to go too. The draw itself is not put back, and does not need to
			// be - the run stays repeatable either way.
			rec.Discard()
			break
		}
		remaining -= int64(len(buf) - mark)

		if len(buf) >= recordChunk {
			if err := WriteAll(w, buf); err != nil {
				return err
			}
			buf = buf[:0]
		}
	}

	mark := len(buf)
	buf = rec.AppendExact(buf, rng, remaining)
	if got := int64(len(buf) - mark); got != remaining {
		return fmt.Errorf("core: the closing record came to %d B and %d B were owed", got, remaining)
	}
	return WriteAll(w, buf)
}

// AppendFiller appends whole words until exactly n bytes have been added, and
// returns the result.
//
// This is the padding that absorbs the remainder of a record built to an exact
// length. It was written out six times before it moved here, and the six copies
// had already drifted into five different shapes for the same job - which is
// the argument for a primitive rather than a convention.
//
// separator says what goes in front of word i, for i above zero. Nil means one
// space, which is what most formats want.
//
// 🔴 The cut lands on a character boundary, not on a byte. Every vocabulary in
// this project is ASCII today, so the two are the same thing and this costs
// nothing - measured, the pinned values did not move. They stop being the same
// thing the day a locale pack arrives, and `locale` is already a key in the
// recipe schema. Measured before the fix, with a vocabulary of Polish words
// across 304 sizes: every file the right size, and 86 of them carrying a
// character cut in half. The size guard, the determinism guard and the pinned
// values were all green through it.
func AppendFiller(dst []byte, words []string, n int64, separator func(i int) string) []byte {
	if n <= 0 {
		// An empty value is legal, and it is what the smallest files get.
		// Anything below zero would mean the minimum was ignored, and
		// FillRecords turns that into an error rather than a wrong size.
		return dst
	}
	if len(words) == 0 {
		// The loop below indexes words modulo its length, so an empty
		// vocabulary divides by zero. No caller passes one today and none
		// should: a format with nothing to say cannot pad to a length. Saying
		// which mistake it is beats a runtime panic with no name on it.
		panic("core: AppendFiller was given no words to pad with")
	}

	start := len(dst)
	for i := 0; int64(len(dst)-start) < n; i++ {
		if len(dst) > start {
			if separator == nil {
				dst = append(dst, ' ')
			} else {
				dst = append(dst, separator(i)...)
			}
		}
		dst = append(dst, words[i%len(words)]...)
	}

	cut := start + int(n)
	out := dst[:cut]

	// Back off over a character the cut split. One cut can only split one
	// character and a character is at most four bytes, so three steps is always
	// enough. With ASCII the first check passes and nothing moves at all.
	for k := 0; k < 3 && len(out) > start; k++ {
		r, size := utf8.DecodeLastRune(out[start:])
		if r != utf8.RuneError || size != 1 {
			break
		}
		out = out[:len(out)-1]
	}

	// Put the length back with spaces. The byte count is the promise, so it is
	// the one thing that cannot give way.
	for len(out) < cut {
		out = append(out, ' ')
	}
	return out
}

// WriteAll writes every byte or returns the error that stopped it. A short
// write that went unnoticed would leave a file of the wrong size, which is the
// one thing this tool promises never to do.
func WriteAll(w io.Writer, b []byte) error {
	for len(b) > 0 {
		n, err := w.Write(b)
		if err != nil {
			return err
		}
		b = b[n:]
	}
	return nil
}
