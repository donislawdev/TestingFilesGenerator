package guard

import (
	"context"
	"errors"
	"io"
	"sync/atomic"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// An Office package stops while it checksums its padding, not after it -
// docs/STABILITY-REVIEW-2026-10-26.md, R09.
//
// The padding part is stored, and its checksum has to be in the header before
// the part is written, so the padding is generated twice: once into a hash and
// once into the file. Until 2026-10-27 the first pass ran under a context of
// its own, so a stop that arrived during it waited for the whole padding to be
// hashed. Measured that day with a real Ctrl+Break on a 2 GB docx: 0.44 to 0.50
// s after the signal, where a txt of the same size ended in 0.02 s.
//
// Counted, not timed. The first version of this guard timed a stopped write of
// four gigabytes against 250 ms, and CodeRabbit asked on #207 for a signal a
// slow runner cannot fake (docs/REVIEW-207-2026-10-27.md). The padding asks
// the context once for every chunk it generates, so two whole writes whose
// padding differs by one mebibyte differ by the chunks of that mebibyte - once
// if only the write asks, twice if the checksum pass asks as well. And a stop
// placed inside the checksum pass has to end the write at the very next
// question, or the pass carried on past it. Both go through the generator's
// Write, so the context is followed down the path a run takes. docx stands for
// the three, as they reach the padding through one function in
// internal/format/opc.
func TestAnOfficePackageStopsWhileItChecksumsItsPadding(t *testing.T) {
	desc, err := format.Get("docx")
	if err != nil {
		t.Fatal(err)
	}
	// Without the label, so the parts of the two packages are the same bytes
	// and their padding differs by exactly the difference in size.
	const smaller, larger = 4 << 20, 5 << 20
	questions := func(size int64, ctx *stopAfter) (int64, error) {
		plan, err := format.PlanWith(context.Background(), desc.Generator, format.Request{Bytes: size, Seed: 1})
		if err != nil {
			t.Fatalf("planning a %d B docx: %v", size, err)
		}
		err = desc.Generator.Write(ctx, io.Discard, plan)
		return ctx.asked.Load(), err
	}

	whole := func(size int64) int64 {
		n, err := questions(size, &stopAfter{Context: context.Background(), after: 1 << 62})
		if err != nil {
			t.Fatalf("writing a %d B docx: %v", size, err)
		}
		return n
	}
	small, large := whole(smaller), whole(larger)
	const chunks = (larger - smaller) / paddingChunk
	if got := large - small; got != 2*chunks {
		t.Fatalf("a mebibyte more padding asked the context %d more times, expected %d - twice for each of its %d chunks, once in the checksum pass and once in the write. %d means one of the two passes does not ask",
			got, 2*chunks, chunks, chunks)
	}

	// Half way through a whole write's questions is inside the checksum pass:
	// the parts ask once each and come first, and the padding of 4 MiB asks
	// far more often than there are parts.
	at := small / 2
	asked, err := questions(smaller, &stopAfter{Context: context.Background(), after: at})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("with the stop after %d questions the write ended with %v rather than the stop", at, err)
	}
	if asked != at+1 {
		t.Errorf("with the stop after %d questions the write asked %d times - the checksum pass went on after it was told to stop", at, asked)
	}
}

// paddingChunk is how much padding is generated between two questions of the
// context - writeChunk in internal/format/opc/opc.go. If that changes, the
// arithmetic above says so in its own words rather than passing.
const paddingChunk = 32 << 10

// stopAfter is a context that is live for its first after questions and
// stopped for every one after, and counts them.
type stopAfter struct {
	context.Context
	after int64
	asked atomic.Int64
}

func (c *stopAfter) Done() <-chan struct{} {
	if c.asked.Add(1) > c.after {
		return closedDone
	}
	return nil
}

func (c *stopAfter) Err() error {
	if c.asked.Load() > c.after {
		return context.Canceled
	}
	return nil
}

var closedDone = func() chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}()
