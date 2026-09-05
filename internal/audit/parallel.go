package audit

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
)

// This file is the only place in internal/audit that runs anything beside
// anything else, and it is listed in internal/guard/concurrency_test.go with
// that reason. Keeping it to one file is the point: everything else in this
// package stays a plain loop, and a reader looking for the goroutines finds
// them here rather than spread through two passes.
//
// Why it exists. Verify and Inspect both walk the files a manifest claims and
// hash each one, and hashing is the work that dominates once O117 took the
// path resolution out of the loop. Measured 2026-09-05 with
// tools/probes/hashparallel on 6.1 GB in 96 files of 64 MB, three interleaved
// repetitions, drift canary held:
//
//	workers   1      2      4      8     16     32
//	median 5074ms 2645ms 1424ms  766ms  544ms  571ms
//	speedup  1.00x  1.92x  3.56x  6.63x  9.33x  8.88x
//
// O116 turned this down on 2026-08-20 and the number it turned down was real:
// on 3000 files of 1 kB the whole of verify is about a second and hashing is
// half of it, so the most that could be won was half a second. That reading
// describes small files. The measurement above describes the corpora this tool
// exists to produce, and the owner reopened the decision on 2026-09-05 with it
// in hand.
//
// The plateau at sixteen is this machine's hardware thread count, not a
// constant worth writing down. O117 recorded a plateau at eight from the
// 1 kB corpus and said the number would not need measuring again - it did,
// because a performance number is a property of the SHAPE OF THE WORK as well
// as of the machine.

// widthFor is how many goroutines a pass of n files gets.
//
// GOMAXPROCS rather than a number, because the plateau measured above sat
// exactly at this machine's thread count and a constant would describe this
// machine rather than the one the tool is run on. Never more than there are
// files, so a manifest of three entries does not start sixteen goroutines to
// have thirteen of them find nothing to do.
func widthFor(n int) int {
	w := runtime.GOMAXPROCS(0)
	if w > n {
		w = n
	}
	if w < 1 {
		w = 1
	}
	return w
}

// inOrder answers one question for each of n items, over several goroutines,
// and hands the answers back in the order the items were given.
//
// Three properties, and each one is depended on by something else in this
// package rather than being tidiness:
//
//   - Answers come back IN ORDER. Verify sorts its differences afterwards so
//     it would not notice, but Inspect's list is what cleanup deletes from and
//     what it printed to a person beforehand, so a reordered list would remove
//     things in an order nobody was shown.
//   - A cancelled pass returns the CONTIGUOUS PREFIX that finished, not
//     whatever happened to be done. The sequential loop this replaces returned
//     a prefix, and "verify was interrupted after N differences" is a sentence
//     about a prefix. A set with holes in it would make that sentence describe
//     a directory nobody checked in that shape.
//   - one is never given a reason to fail. Everything that can refuse a whole
//     pass - a path that leaves the directory - is settled before this is
//     called, on one goroutine, in order. That is not a simplification for its
//     own sake: if a refusal could arrive here, stopping the other goroutines
//     would mean a LOWER index never got asked, and the same manifest would
//     name a different file on different days.
func inOrder[T any](ctx context.Context, n int, one func(i int) T) ([]T, error) {
	out := make([]T, n)
	done := make([]bool, n)

	// next is the only thing every goroutine touches, and it is an atomic
	// counter. Everything else they write - out and done - is written at
	// indices no other one is given. See drain.
	var next atomic.Int64
	var wg sync.WaitGroup

	work := func() {
		defer wg.Done()
		drain(ctx, &next, out, done, one)
	}
	for w := widthFor(n); w > 0; w-- {
		wg.Add(1)
		go work()
	}
	wg.Wait()

	return out[:finishedPrefix(done)], ctx.Err()
}

// drain takes items off the counter until there are none left, answering each
// one and marking it done.
//
// A function of its own rather than the body of the goroutine above, and the
// reason is the same one written beside depthChange in internal/recipe: a
// literal inside a loop counts one level deeper than it reads, so loop plus
// literal plus loop plus branch is four, and the shape guard counts how many
// functions sit three deep as well as how deep the deepest one is. Splitting
// is what that guard asks for and it costs nothing here.
//
// out and done are written at indices no other goroutine will take, because
// the counter hands each index out exactly once. They are the only things
// written at all.
func drain[T any](ctx context.Context, next *atomic.Int64, out []T, done []bool, one func(i int) T) {
	for {
		i := int(next.Add(1)) - 1
		// Cancellation is asked per item rather than per pass, so a Ctrl+C
		// during a long hash is noticed at the next file rather than at the
		// end of the run.
		if i >= len(out) || ctx.Err() != nil {
			return
		}
		out[i] = one(i)
		done[i] = true
	}
}

// finishedPrefix is how many items were answered before the first one that was
// not - which on a cancelled pass is everything the caller may speak about.
func finishedPrefix(done []bool) int {
	for i, ok := range done {
		if !ok {
			return i
		}
	}
	return len(done)
}
