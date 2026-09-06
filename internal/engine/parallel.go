// Part of package engine. See engine.go.
package engine

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// This file is the only place in internal/engine that runs anything beside
// anything else, and it is listed in internal/guard/concurrency_test.go with
// that reason. Keeping it to one file is the point, the same way
// internal/audit/parallel.go does: everything else in this package stays a
// plain loop, and a reader looking for the goroutines finds them here.
//
// Why it exists. Writing the files is what a run is: measured on 2026-09-06,
// after P7 stopped planning from encoding the picture twice, planning a run of
// 300 PNGs is 51 ms and writing it is 2741 ms - 2% against 98%. So the write
// loop is the whole of what is left to parallelise, and the plan is not.
//
// Measured with tools/probes/writeparallel, which writes a real planned run
// with N goroutines in ONE process - the shape this file has, rather than the
// N separate processes an earlier probe used:
//
//	png 200 kB x240   W1 1.00x   W2 2.05x   W4 3.03x   W8 4.19x   W16 5.51x
//	zip 2 MB x80      W1 1.00x   W2 1.78x   W4 2.77x   W8 3.87x
//
// The question that had to be answered before any of this was written is what
// a shared heap does to it, because separate processes have one each. It does
// nothing: the same run allocates 563.0 MB at one goroutine and 562.8 MB at
// sixteen, and collects a quarter as often. The numbers, the instrument and
// the two mistakes made getting them are in
// docs/PERFORMANCE-REVIEW-2026-09-05.md section 14.
//
// What this costs, said out loud rather than left to be found. N generators
// are live at once, so the memory a run holds while writing is N times one
// generator's working set. Every generator streams - there is a guard for that
// - so that set is bounded by the format rather than by the file, and the
// largest of them hold one picture. It is still N times what it was. And the
// answers are collected before any of them reaches the manifest, which is
// forty bytes a file that the sequential loop did not need: forty megabytes at
// the millionth file, beside a plan that is already far larger.

// widthFor is how many goroutines a run of n files gets.
//
// GOMAXPROCS rather than a number, for the reason written beside the same
// function in internal/audit: a constant would describe this machine rather
// than the one the tool is run on. Never more than there are files, so a run
// of three does not start sixteen goroutines to have thirteen of them find
// nothing to do.
//
// Measured on an SSD, and that is a limit of the measurement rather than a
// property of the answer. A run held up by a slow disk rather than by the
// processor could be worse at sixteen writers than at one, and that case is
// NOT measured - both formats measured above kept improving to the widest
// setting tried. If it turns up, it turns up as a run that is slower than it
// was, and the number to change is here.
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

// fileResult is what one file came to. The zero value is a file that was never
// started, which a cancelled run leaves behind and which is neither a success
// nor a failure - it gets no manifest entry, exactly as it gets none today.
type fileResult struct {
	sha string
	ok  bool
	err error
}

// writeAll writes every planned file, over several goroutines, and answers for
// each one at its own index.
//
// Answers by index rather than by appending, and that is not tidiness. The
// manifest is built from this slice in order afterwards, and the order of the
// manifest is what cleanup PRINTS to a person before deleting from it - there
// is a guard for that. A list assembled out of completion order holds the same
// files and offers them in an order nobody was shown.
//
// A cancelled run keeps EVERY FILE THAT FINISHED, which may leave a hole where
// a writer was stopped half way. That is the one behaviour that changes here,
// it is a decision of the owner's from 2026-09-06, and the alternative is
// worse in a way untouchable rule 7 names: a finished file with no manifest
// entry is a file no command of this tool can ever remove. internal/audit
// wants the opposite - a contiguous prefix - because a sentence about an
// interrupted verify is a sentence about a prefix, and that difference is why
// there are two pools rather than one shared one.
func writeAll(ctx context.Context, files []PlannedFile, outDir string, gate *progressGate) []fileResult {
	out := make([]fileResult, len(files))

	// next is the only thing every goroutine touches, and it is an atomic
	// counter. out is written at indices no other goroutine is given, because
	// the counter hands each index out exactly once.
	var next atomic.Int64
	var wg sync.WaitGroup

	work := func() {
		defer wg.Done()
		drain(ctx, &next, files, outDir, gate, out)
	}
	for w := widthFor(len(files)); w > 0; w-- {
		wg.Add(1)
		go work()
	}
	wg.Wait()

	return out
}

// drain takes files off the counter until there are none left.
//
// A function of its own rather than the body of the goroutine above, for the
// reason written beside the same split in internal/audit: a literal inside a
// loop counts one level deeper than it reads, and the shape guard asks for the
// split rather than for a larger ceiling.
func drain(ctx context.Context, next *atomic.Int64, files []PlannedFile, outDir string, gate *progressGate, out []fileResult) {
	for {
		i := int(next.Add(1)) - 1
		// Cancellation is asked before taking a file rather than only at the
		// top of a pass, so a Ctrl+C is noticed at the next file rather than
		// after every remaining one has been begun.
		//
		// NO MUTATION PROVES THIS LINE and that is not an oversight. Every
		// generator takes the context and refuses on it too, so taking this
		// question out changes nothing anybody can observe - it only has a
		// stopped run create and delete a temporary file for every index it
		// had left, which on a million file run is a million of them. What it
		// buys is work not done, and the guard that would catch its absence
		// would be a guard on a clock.
		if i >= len(out) || ctx.Err() != nil {
			return
		}
		// One of these per file, on this goroutine's own stack. How far this
		// writer has already reported is the one number that cannot live in
		// the gate: with several files in flight there is no such thing as
		// "the file being written".
		p := fileProgress{gate: gate}
		sum, err := writeOne(ctx, files[i], outDir, &p)
		out[i] = fileResult{sha: sum, ok: err == nil, err: err}
		p.finished(files[i].Plan.Bytes, err == nil)
	}
}

// progressGate is how progress leaves a run that has several writers in it.
//
// Options.OnProgress used to promise "called from the same goroutine doing the
// work". Both callers were built on that: the command line bar moves last and
// printed with no lock, and the window's throttle reads and writes a timestamp
// with none either. The promise cannot survive this file, so what replaces it
// is a weaker one that costs those callers nothing - NEVER TWO AT ONCE. The
// lock gives happens-before, so both remain correct without a line of change.
//
// The callback runs INSIDE the lock rather than beside it. Outside, two
// callbacks could run at once and the whole point would be gone.
//
// That lock is taken once per Write INSIDE a file, not once per file, which is
// often enough to be worth pricing rather than assuming. Measured 2026-09-06
// on gif, the most talkative generator in the tree at one call per 260 bytes:
// 319 840 callbacks in a run, over three million a second through this one
// mutex, and the timings with and without it overlap. No cost is claimed
// because none was found.
//
// A nil gate is a run nobody is watching. Every method takes a nil receiver,
// so a run without progress does no locking and allocates nothing for it.
type progressGate struct {
	mu         sync.Mutex
	filesDone  int
	bytesDone  int64
	filesTotal int
	bytesTotal int64
	report     func(Progress)
}

func newProgressGate(files []PlannedFile, report func(Progress)) *progressGate {
	if report == nil {
		return nil
	}
	return &progressGate{
		filesTotal: len(files),
		bytesTotal: TotalBytes(files),
		report:     report,
	}
}

// advance moves the run on by what one writer has added since it last spoke.
//
// Deltas rather than a running total, because with several files in flight
// "how far the run had got before this file started, plus what is in this
// file" describes no run at all. At one writer the numbers this produces are
// the same ones the sequential loop produced, which is what lets the guards
// that watch the bar stay as they were.
func (g *progressGate) advance(delta int64) {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.bytesDone += delta
	g.say()
}

// finished is the end of one file: the delta that squares this writer's
// reporting with what the plan promised, and one more file done.
func (g *progressGate) finished(delta int64) {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.bytesDone += delta
	g.filesDone++
	g.say()
}

// say hands the current position out. Called with the lock held, always.
func (g *progressGate) say() {
	g.report(Progress{
		FilesDone: g.filesDone, FilesTotal: g.filesTotal,
		BytesDone: g.bytesDone, BytesTotal: g.bytesTotal,
	})
}

// fileProgress is one writer's share of the bar: the gate, and how much of
// this file it has already accounted for.
//
// It exists on the writing goroutine's stack and nowhere else, which is what
// makes the arithmetic below unshared. Everything it hands the gate is a
// delta, so the gate never has to know which file it came from.
type fileProgress struct {
	gate     *progressGate
	reported int64
}

// advance is the counting writer's callback: n is the running total for this
// file, and the gate is told the difference.
func (p *fileProgress) advance(n int64) {
	p.gate.advance(n - p.reported)
	p.reported = n
}

// finished squares this file up. A file that succeeded is topped up to exactly
// what the plan promised, so the bar lands on the total rather than near it. A
// file that failed gives back everything it reported, because a file that
// failed counts for nothing - the bar would otherwise claim bytes nobody can
// find. That the total can go backwards on a failed file is how it already
// behaves. What is new is only that another writer's bytes may sit in the same
// total while it happens.
func (p *fileProgress) finished(planned int64, ok bool) {
	if ok {
		p.gate.finished(planned - p.reported)
		return
	}
	p.gate.finished(-p.reported)
}

// writeOne writes one file under a temporary name and renames it only once it
// is whole, so the output directory never holds an incomplete file. That
// invariant covers the process ending - Ctrl+C, kill, a CI timeout. It does
// not cover power loss, because that would need a flush per file and ten
// thousand of those is a real cost.
//
// It lives here rather than in engine.go because it is what a worker does, and
// this file is meant to be the whole answer to "what runs beside what".
func writeOne(ctx context.Context, f PlannedFile, outDir string, p *fileProgress) (string, error) {
	final := filepath.Join(outDir, f.Name)
	// The process id is in the name because two runs writing into one directory
	// used to meet on it. Measured on 2026-08-03: two runs of the same target
	// collided on the temporary file, one of them reported two files it could
	// not produce, and the bytes of the other had already gone through the same
	// handle. The name never survives the run, so nothing about it has to be
	// repeatable - and the file it becomes is settled by the plan, not by this.
	//
	// Two WRITERS of one run cannot meet on it, because planning refuses two
	// files that want the same name and this name is built from that one.
	tmp := tempPathFor(outDir, f.Name)

	// Claimed rather than created, and core.CreateNew carries the measurement
	// that settles how.
	//
	// O_EXCL on its own was tried here and taken back out on 2026-08-25, for a
	// reason that has not changed. Measured with a probe, a file created in a
	// directory reached through a symbolic link:
	//
	//   os.Create                 works
	//   O_CREATE|O_EXCL|O_WRONLY  fails with "The file exists"
	//
	// about a file that does not exist. Go asks for the reparse point rather
	// than what it points at when O_EXCL is set, so every file of a run whose
	// output directory is a link failed - and this tool supports exactly that
	// on purpose, because people keep fixtures on a mounted workspace or a
	// scratch disk. Two guards said so within a minute of the change.
	//
	// What came back on 2026-09-06 is not that flag on its own. It is the
	// pair: create exclusively, and believe the refusal only when os.Lstat
	// says something is really there. The supported setup keeps working, and
	// the window this file used to leave open closes with it.
	//
	// That window is small and it was the last one of its kind: preflight
	// refuses every name that is taken before the run starts, so what was left
	// is somebody creating our temporary name - with our process id in it -
	// during the run, and a create that follows links putting the bytes
	// wherever it pointed. Owner's call on 2026-09-06, after the same class
	// was found unguarded in two other places. See core.CreateNew.
	fh, err := core.CreateNew(tmp, 0o666)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			// In our own words. The fault is the one preflight names, arriving
			// later than preflight can look.
			return "", &CollisionError{Path: tmp}
		}
		return "", err
	}

	h := sha256.New()
	buffered := bufio.NewWriterSize(fh, 64<<10)
	counter := &countingWriter{w: io.MultiWriter(buffered, h)}
	// Left nil when nobody is listening, so a run without progress does no
	// locking at all and allocates nothing for it.
	if p.gate != nil {
		counter.report = p.advance
	}

	writeErr := writeWithoutCrashing(ctx, f, counter)
	if writeErr == nil {
		writeErr = buffered.Flush()
	}
	closeErr := fh.Close()

	if writeErr != nil {
		_ = os.Remove(tmp)
		return "", writeErr
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return "", closeErr
	}

	// The size is the promise. A generator that missed it by a byte is a bug
	// worth catching here rather than in someone's test suite, so the file
	// never reaches its final name.
	if counter.n != f.Plan.Bytes {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("generator for %s produced %d B where the plan said %d B",
			f.Desc.ID, counter.n, f.Plan.Bytes)
	}

	if err := os.Rename(tmp, final); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type countingWriter struct {
	w io.Writer
	n int64
	// report, when set, is called with the running total for this file. It is
	// what gives progress inside a single large file rather than only between
	// files - the case where silence is worst, because one 5 GB file is one
	// callback if you only count finished files.
	report func(int64)
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	if c.report != nil {
		c.report(c.n)
	}
	return n, err
}
