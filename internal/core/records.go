package core

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
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
		// Stop while what is left still fits a whole closing record. That is
		// what keeps the last one from being a stub.
		if remaining-int64(len(buf)-mark) < shortest {
			buf = buf[:mark]
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
