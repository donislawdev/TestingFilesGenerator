package otlp

import (
	// D11 promises the same bytes from the same seed, so a deliberate,
	// reproducible generator is the product rather than a weakness. Nothing
	// here ever makes a secret.
	// nosemgrep: go.lang.security.audit.crypto.math_random.math-random-used
	"math/rand/v2"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// records builds the spans or the log records of a file, one at a time, for
// core.FillRecords.
//
// A record here is more than its own braces. The first of a batch carries what
// opens the batch in front of it, and the last carries what closes it, so the
// filler sees whole pieces and a file never ends inside a batch.
type records struct {
	s settings
	e emitter
	// open starts a batch and shut ends one. Both are the same for every
	// batch of a file, so they are built once.
	open, shut []byte
	tail       []byte
	st, before state
	shortest   int64
}

// state is what the builder remembers from one record to the next. Kept in one
// value so the record built only to learn that it does not fit can be undone
// by putting the value back.
type state struct {
	// count is how many records the file holds so far.
	count int64
	// inBatch is how many of them the open batch holds. Nought means the next
	// record opens a batch.
	inBatch int
	trace   trace
}

func newRecords(s settings) *records {
	r := &records{
		s: s, e: emitter{indent: s.layout.indent},
		open: opening(envelopes[s.signal], s.layout.indent), shut: closing(s.layout),
	}
	r.shortest = r.measureShortest()
	return r
}

// Shortest is the smallest closing record this builder can end the file with:
// the longest record the settings can draw, with what opens a batch in front
// of it, because the closing record may be the first of its batch.
func (r *records) Shortest() int64 { return r.shortest }

// Discard takes back everything the thrown away record changed: its place in
// the batch, its tick of the clock and its place in its trace. The closing
// record then carries all three instead.
func (r *records) Discard() { r.st = r.before }

func (r *records) Append(dst []byte, rng *rand.Rand) []byte {
	r.before = r.st
	dst, _ = r.record(dst, rng, false)
	if r.s.layout.lines && r.st.inBatch == r.s.batch {
		dst = append(dst, r.shut...)
		r.st.inBatch = 0
	}
	return dst
}

// AppendExact writes the closing record at exactly n bytes, with what ends
// its batch. The padding attribute takes the remainder, which is shorter than
// a record: the filler stops as soon as another whole one would not fit.
func (r *records) AppendExact(dst []byte, rng *rand.Rand, n int64) []byte {
	mark := len(dst)
	dst, at := r.record(dst, rng, true)
	r.tail = append(r.tail[:0], dst[at:]...)
	fill := n - int64(len(dst)-mark) - int64(len(r.shut))
	dst = core.AppendFiller(dst[:at], words, fill, nil)
	dst = append(dst, r.tail...)
	return append(dst, r.shut...)
}

// record draws the next record and writes it after whatever has to stand in
// front of it: what opens a batch, or the comma after the record before.
func (r *records) record(dst []byte, rng *rand.Rand, padded bool) (out []byte, at int) {
	if r.st.inBatch == 0 {
		dst = append(dst, r.open...)
	}
	r.e.b, r.e.depth, r.e.fresh = dst, recordDepth, r.st.inBatch == 0
	if r.s.signal == Traces {
		at = r.writeSpan(r.drawSpan(rng), padded)
	} else {
		at = r.writeEntry(r.drawEntry(rng, r.st.count), padded)
	}
	r.st.count++
	r.st.inBatch++
	return r.e.b, at
}

// measureShortest is the closing record at its longest with nothing in its
// padding, opening a batch and ending it. Measured on the bytes rather than
// worked out, like everything here that has to agree with bytes beside it.
func (r *records) measureShortest() int64 {
	widest := 0
	for _, sp := range r.widestSpans() {
		widest = max(widest, r.measured(func() { r.writeSpan(sp, true) }))
	}
	for _, en := range r.widestEntries() {
		widest = max(widest, r.measured(func() { r.writeEntry(en, true) }))
	}
	return int64(len(r.open) + widest + len(r.shut))
}

// measured is how many bytes write adds as the first record of a batch.
func (r *records) measured(write func()) int {
	r.e.b, r.e.depth, r.e.fresh = r.e.b[:0], recordDepth, true
	write()
	return len(r.e.b)
}

// LongestLine is the longest line a file of these settings can hold, without
// its line ending, for the guard that keeps every line under LineCeiling.
//
// A full batch of the longest record, and what the closing record can add to
// it. The filler stops only when another whole record would not fit, so the
// closing one is handed up to the smallest closing record and a record more,
// and the smallest closing record counts what opens a batch even where the
// batch is open already.
func LongestLine(props map[string]string) (int64, error) {
	s, err := parse(props)
	if err != nil {
		return 0, err
	}
	r := newRecords(s)
	record := r.shortest - int64(len(r.open)+len(r.shut)) + 1
	return int64(2*len(r.open)) + int64(s.batch+1)*record + int64(len(r.shut)-len(s.layout.end)), nil
}
