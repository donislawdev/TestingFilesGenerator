package video

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// The one place in this package where anything runs beside anything else, and
// it is listed in internal/guard/concurrency_test.go with that reason.
//
// Why it exists. A film is its pictures: every change is a whole picture
// through gav1d, which codes on one goroutine and has no setting that searches
// less than the one already used. A 1920x1080 picture is about 0.35 s, so a
// thirty minute film at a change a second was ten minutes on one core while
// fifteen others idled - measured 2026-10-06 at 611.7 s for the owner's own
// order, 2 GB, 1920x1080, thirty minutes (docs/WEBM-WYDAJNOSC-2026-10-06.md).
// The pictures do not depend on one another, so they can be coded at once and
// written in order, and the bytes are the ones one goroutine makes. Measured
// with tools/probes/videoparallel, 48 pictures at 1920x1080, three rounds:
//
//	W1 1.00x   W2 1.81x   W4 3.24x   W8 5.12x   W12 6.95x   W16 7.60x
//
// with every picture the same size at every width.
//
// The limit is the process's, not the film's. The engine already writes
// GOMAXPROCS files at once, so helpers taken per film would be sixteen films
// times sixteen pictures in memory. Here the goroutine writing a film always
// codes, which is what guarantees it moves on, and the helpers beside it come
// from one count for the whole process, GOMAXPROCS less one. A film that
// finds none free codes alone, as every film did before, and asks again at its
// next picture.
//
// Since a picture is cut into tiles (grid.go), what a helper codes is a tile:
// a new tile of the clock at every change, the square's ten places once each,
// and the rest of the picture once. The pictures still come out in order and
// the bytes are still those of one goroutine.

// helping is how many helpers are coding beside the writers right now.
var helping atomic.Int64

// codedBeside counts the tiles a helper coded rather than the goroutine that
// asked for them, since the process started.
var codedBeside atomic.Int64

// codedAll counts every tile coded since the process started, by a helper or
// by the goroutine that asked for it.
var codedAll atomic.Int64

// Helpers is how many helpers are coding right now and how many tiles helpers
// have coded since the process started. Guards read it: a film held to
// the bytes of one goroutine says nothing unless several really coded it, and
// a film abandoned half way has to leave none of them running.
func Helpers() (running, coded int64) { return helping.Load(), codedBeside.Load() }

// Coded is how many tiles have been coded since the process started, by
// anybody. A guard reads it: an MP4 codes its film twice, and the second pass
// is to code nothing the first one kept (Pictures.Again).
func Coded() int64 { return codedAll.Load() }

// takeHelper claims a place for one more helper, if the process has one.
func takeHelper() bool {
	limit := int64(runtime.GOMAXPROCS(0) - 1)
	for {
		n := helping.Load()
		if n >= limit {
			return false
		}
		if helping.CompareAndSwap(n, n+1) {
			return true
		}
	}
}

// job is one tile to code, of the picture with this look. Whoever takes it
// first - a helper, or the writer that needs it - codes it, and done is
// released when it is coded, so the writer waits only for a tile somebody else
// is coding. A WaitGroup inside the job rather than a channel beside it,
// because a channel is one more allocation for every tile of every film.
type job struct {
	look   look
	change int64
	tile   int
	taken  atomic.Bool
	done   sync.WaitGroup
	coded  tileCoded
	err    error
	// panicked is what coding it panicked with. A panic on a helper would end
	// the process, where the same panic on the writer becomes the run's error
	// (internal/engine, writeWithoutCrashing), so a helper keeps it here and
	// the writer panics with it when it reaches the tile.
	panicked any
	// counted says the tile's bytes are to be added to what the film keeps of
	// the clock's tiles once it is coded. Read and written by the owning
	// goroutine only.
	counted bool
}

func newJob(l look, change int64, tile int) *job {
	j := &job{look: l, change: change, tile: tile}
	j.done.Add(1)
	return j
}

// codedJob is a tile already coded, which nobody codes again.
func codedJob(t tileCoded) *job {
	j := &job{coded: t}
	j.taken.Store(true)
	return j
}

// crew codes the tiles of one film: the goroutine that owns it, and the
// helpers it has been able to take. Jobs are offered in the order the film
// shows them and waited for in the same order, so the first error a film
// meets is the first in its order, whoever coded what.
type crew struct {
	film    *film
	keys    keyer
	qindex  int
	own     *painter
	queue   chan *job
	most    int
	helpers int
	wg      sync.WaitGroup
	stopped atomic.Bool
	closed  bool
	// encode stands in for painting a tile and coding it through gav1d,
	// told whether a helper is the one asking. Nil in every film - only
	// CodeBeside sets it.
	encode func(change int64, beside bool) (tileCoded, error)
}

// newCrew is the crew for a film of at most this many tiles to code. A film of
// one tile takes no helper, and neither does a process of one thread.
func newCrew(f *film, ks keyer, qindex int, jobs int64) *crew {
	c := &crew{film: f, keys: ks, qindex: qindex, own: f.painter(ks)}
	if most := min(int64(runtime.GOMAXPROCS(0)-1), jobs-1); most > 0 {
		c.most = int(most)
		c.queue = make(chan *job, c.ahead()*len(ks.tiles))
	}
	return c
}

// again is a crew for a second pass over the same film, painting with this
// one's painter (Pictures.Again). This one has to be stopped first, because
// the painter is used by one goroutine at a time.
func (c *crew) again() *crew {
	n := &crew{film: c.film, keys: c.keys, qindex: c.qindex, own: c.own, most: c.most, encode: c.encode}
	if n.most > 0 {
		n.queue = make(chan *job, n.ahead()*len(n.keys.tiles))
	}
	return n
}

// ahead is how many pictures past the one being written may be offered: two
// for each goroutine that codes, so none of them waits for the next picture
// while the writer is still busy with the last. Nought without helpers - a
// film coded alone looks at no picture before it needs it.
func (c *crew) ahead() int {
	if c.most == 0 {
		return 0
	}
	return 2 * (c.most + 1)
}

// offer hands a tile to the helpers, taking one more first when the film may
// have more and the process has a place for it. With no helper the
// writer codes it when it gets there. Called by the owning goroutine only.
func (c *crew) offer(j *job) {
	if c.queue == nil || c.closed {
		return
	}
	if c.helpers < c.most && takeHelper() {
		c.helpers++
		c.wg.Add(1)
		go c.help()
	}
	if c.helpers == 0 {
		return
	}
	select {
	case c.queue <- j:
	default:
		// The queue holds every tile of the window, so this is a tile the
		// writer will reach before any helper would have.
	}
}

func (c *crew) help() {
	defer c.wg.Done()
	defer helping.Add(-1)
	p := c.film.painter(c.keys)
	for j := range c.queue {
		if c.stopped.Load() {
			continue
		}
		if j.taken.CompareAndSwap(false, true) {
			c.code(p, j, true)
			codedBeside.Add(1)
		}
	}
}

func (c *crew) code(p *painter, j *job, beside bool) {
	defer j.done.Done()
	codedAll.Add(1)
	defer func() {
		if v := recover(); v != nil {
			j.panicked = v
		}
	}()
	if c.encode != nil {
		j.coded, j.err = c.encode(j.change, beside)
		return
	}
	src, at := p.source(j.look, c.keys.tiles[j.tile].rect)
	j.coded, j.err = encodeTile(src, at, c.qindex)
}

// wait is the tile of j, coded here when no helper has taken it.
func (c *crew) wait(j *job) (tileCoded, error) {
	if j.taken.CompareAndSwap(false, true) {
		c.code(c.own, j, false)
	}
	j.done.Wait()
	if j.panicked != nil {
		panic(j.panicked)
	}
	return j.coded, j.err
}

// finish says no tile is coming after the ones offered, so the helpers end
// once the queue is empty and give their places back to the process.
func (c *crew) finish() {
	if c.queue != nil && !c.closed {
		c.closed = true
		close(c.queue)
	}
}

// stop ends the crew and waits for its helpers. A helper in the middle of a
// tile finishes that tile - gav1d cannot be interrupted, and that was so
// before there were helpers - and codes nothing after it. Nothing a crew
// started is running once this returns.
func (c *crew) stop() {
	c.stopped.Store(true)
	c.finish()
	c.wg.Wait()
}

// all offers every job at once and waits for each in order - a set known in
// advance, like the sample planning codes, rather than a film coded as it is
// written. Each job's tile is in the job once all returns.
func (c *crew) all(jobs []*job) error {
	for _, j := range jobs {
		c.offer(j)
	}
	c.finish()
	for _, j := range jobs {
		if _, err := c.wait(j); err != nil {
			return err
		}
	}
	return nil
}

// CodeBeside codes tiles 0 to n-1 the way a film's are coded - this goroutine
// and the helpers it can take, offered ahead and taken in order - with code
// standing in for painting and gav1d, told whether a helper is the one
// calling, and gives back the size code returned for each.
//
// It is for the guards, and only because what it lets them hold cannot be
// reached another way: a panic on a helper has to come out on the goroutine
// that asked for the tile, where the engine turns it into the run's error,
// rather than end the process - and nothing outside makes gav1d panic.
// Measured on 2026-10-06: a quantizer out of range is refused with an error,
// and only planes shorter than the picture panic, which no painter makes.
func CodeBeside(n int64, code func(change int64, beside bool) (int, error)) ([]int, error) {
	t := Timeline{FPS: 1, Frames: n, DurationMs: n * 1000, KeyEvery: n, ChangeEvery: 1}
	f := newFilm(1, 1, 0, "", t)
	c := newCrew(f, newKeyer(f.geometry, oneTile(1, 1)), 0, n)
	defer c.stop()
	c.encode = func(change int64, beside bool) (tileCoded, error) {
		size, err := code(change, beside)
		return tileCoded{data: make([]byte, size)}, err
	}
	jobs := make([]*job, n)
	for i := range jobs {
		jobs[i] = newJob(look{}, int64(i), 0)
	}
	if err := c.all(jobs); err != nil {
		return nil, err
	}
	sizes := make([]int, n)
	for i, j := range jobs {
		sizes[i] = len(j.coded.data)
	}
	return sizes, nil
}
