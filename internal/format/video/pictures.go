package video

import (
	"fmt"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// keptClockBytes is how many bytes of tiles that show the clock one film keeps
// to use again. The rest of its tiles it keeps whatever they come to - the
// tiles nothing changes, once each, and the square's, ten places each - but a
// tile of the clock is new at nearly every change, and a day of changes on a
// narrow picture, the whole clock in one tile, would be a hundred thousand of
// them. Eight megabytes holds every place of the clock's seconds tile at
// 1920x1080 several times over (a tile is a kilobyte or two). A tile not kept
// is coded again when its key comes back, which costs time and no bytes.
const keptClockBytes = 8 << 20

// keysAhead is how many keys a tile is given room for before its film's maps
// grow - the ten places of the square and its picture without it, so a short
// film, the one whose allocations are counted, never grows them.
const keysAhead = 12

// Pictures codes the pictures of one film in the order the film shows them, a
// tile at a time (grid.go), on the goroutine that asks for them and on the
// helpers it can take (ahead.go). A tile is coded the first time its key comes
// up and taken from what was coded every time after, and the pictures coded
// ahead of the one being written are a few - an hour at a change a second is
// 3600 pictures, and holding them all would hold the film.
//
// Close has to be called when the film is written or abandoned, because the
// helpers are goroutines.
type Pictures struct {
	choice Choice
	stream Stream
	// crew holds the film and its keyer as well as the helpers.
	crew *crew
	// known is every tile coded or offered in this film, by its key, and kept
	// counts the bytes of the ones that show the clock.
	known map[tileKey]*job
	kept  int
	// seen is every key that has come up, kept or not, for the work a change
	// reports - the same walk Stream.Work makes.
	seen map[tileKey]struct{}
	// pending are the pictures offered and not yet reached, in order, each
	// with the change it opens. Changes that look the same as the one before
	// open none, so they are not in it.
	pending []opened
	next    int64 // the first change not looked at yet
	last    look  // the look of change next-1
	change  int64
	work    int64
	tiles   []tileCoded
	key     []byte
	copied  []byte
	// spare are the job lists of pictures already made, cleared, for the
	// pictures opened after them.
	spare [][]*job
}

// opened is a picture offered: the change it opens, the job of each of its
// tiles in frame order, and the work its new keys are worth.
type opened struct {
	change int64
	jobs   []*job
	work   int64
}

// Pictures is the coder of this choice's film.
func (c Choice) Pictures(st Stream) *Pictures {
	f := newFilm(c.Width, c.Height, c.Seed, c.Label, st.Timeline)
	ks := newKeyer(f.geometry, st.shape.grid)
	p := &Pictures{
		choice: c, stream: st, change: -1,
		crew:  newCrew(f, ks, c.QIndex, st.Changes()*int64(len(ks.tiles))),
		known: make(map[tileKey]*job, len(c.tiles)+keysAhead*len(ks.tiles)),
		seen:  make(map[tileKey]struct{}, keysAhead*len(ks.tiles)),
		tiles: make([]tileCoded, len(ks.tiles)),
	}
	p.pending = make([]opened, 0, p.crew.ahead()+1)
	for k, t := range c.tiles {
		p.known[k] = codedJob(t)
	}
	return p
}

// At makes picture c the current one, coding the tiles of it nobody has coded
// yet, unless it looks the same as the one before. Pictures are asked for in
// the order the film shows them - the same change again is fine, an earlier
// one is a defect of the caller.
//
// Every picture is held to the stream's reserve, because the level and the
// file around it were planned on it. A picture over it is a ceiling that did
// not hold - the same class of failure as a ladder rung coding past its
// ceiling - and it is refused as a defect rather than written into a file that
// would come out the wrong size.
func (p *Pictures) At(c int64) error {
	if c == p.change {
		return nil
	}
	if c < p.change {
		return core.Defect(fmt.Errorf("video: picture %d was asked for after picture %d, and a film's pictures are coded in the order it shows them", c, p.change))
	}
	p.offerTo(c)
	var reached []opened
	n := 0
	for n < len(p.pending) && p.pending[n].change <= c {
		n++
	}
	reached = p.pending[:n]
	p.change, p.work = c, 0
	for _, o := range reached {
		p.work += o.work
	}
	if n == 0 {
		// The same look as the picture before it: shown again, not coded.
		return nil
	}
	err := p.make(reached[n-1].jobs)
	for _, o := range reached {
		clear(o.jobs)
		p.spare = append(p.spare, o.jobs)
	}
	// Moved down rather than sliced off, so the one array made for the
	// window lasts the whole film.
	p.pending = p.pending[:copy(p.pending, p.pending[n:])]
	return err
}

// make waits for the tiles of the picture and builds the two samples it is
// carried in.
func (p *Pictures) make(jobs []*job) error {
	size := 0
	for i, j := range jobs {
		t, err := p.crew.wait(j)
		if err != nil {
			return err
		}
		p.tiles[i] = t
		size += len(t.data)
		if j.counted {
			p.kept += len(t.data)
			j.counted = false
		}
	}
	if size > p.stream.Reserve {
		return core.Defect(fmt.Errorf("video: picture %d of a %dx%d film coded to %d B of tiles and the film was planned on %d B a picture, so the file cannot be kept",
			p.change, p.choice.Width, p.choice.Height, size, p.stream.Reserve))
	}
	var err error
	if p.key, err = p.stream.shape.sample(p.key, p.stream.seq, keyKind, p.tiles, nil); err != nil {
		return err
	}
	p.copied, err = p.stream.shape.sample(p.copied, nil, copyKind, p.tiles, p.stream.show)
	return err
}

// offerTo looks at every change up to c and as far past it as the crew codes
// ahead, and offers the tiles of each one that looks different from the one
// before it.
func (p *Pictures) offerTo(c int64) {
	n := p.stream.Changes()
	for p.next < n && (p.next <= c || len(p.pending) < p.crew.ahead()) {
		l := p.crew.film.lookOf(p.next)
		if p.next == 0 || l != p.last {
			p.pending = append(p.pending, p.open(l))
		}
		p.last = l
		p.next++
	}
	if p.next == n {
		p.crew.finish()
	}
}

// open is the picture with this look at change p.next: each tile a job
// already known by its key, or a new one offered to the helpers.
func (p *Pictures) open(l look) opened {
	o := opened{change: p.next}
	if n := len(p.spare); n > 0 {
		o.jobs, p.spare = p.spare[n-1], p.spare[:n-1]
	} else {
		o.jobs = make([]*job, len(p.crew.keys.tiles))
	}
	o.work = p.crew.keys.fresh(l, p.seen) * workPerPixel
	for i := range p.crew.keys.tiles {
		o.jobs[i] = p.jobFor(i, l)
	}
	return o
}

// jobFor is the job of tile i of the picture with this look: the one already
// known by its key, or a new one offered to the helpers and kept by its key -
// a tile of the clock only while the film has room for more of them.
func (p *Pictures) jobFor(i int, l look) *job {
	k := p.crew.keys.key(i, l)
	if j, ok := p.known[k]; ok {
		return j
	}
	j := newJob(l, p.next, i)
	p.crew.offer(j)
	if !k.showsClock(p.crew.keys) {
		p.known[k] = j
	} else if p.kept < keptClockBytes {
		p.known[k], j.counted = j, true
	}
	return j
}

// Close stops the helpers and waits for them, so nothing this film started is
// running once it returns. A helper in the middle of a tile finishes it first.
func (p *Pictures) Close() { p.crew.stop() }

// KeySample is the current picture as a key frame, with the sequence header
// before it. It is written over by the next picture At makes.
func (p *Pictures) KeySample() []byte { return p.key }

// CopySample is the current picture as a hidden intra only copy, and the frame
// that shows it. It is written over by the next picture At makes.
func (p *Pictures) CopySample() []byte { return p.copied }

// CodedSize is the bytes of the tiles of picture change of this choice's
// film, each coded afresh under the film's layout - what a picture takes of
// its reserve. For the probe that measures the ladder's ceilings, which asks
// about one picture at a time.
func (c Choice) CodedSize(t Timeline, change int64) (int, error) {
	f := newFilm(c.Width, c.Height, c.Seed, c.Label, t)
	ks := newKeyer(f.geometry, gridFor(f.geometry, t.FPS))
	p, l := f.painter(ks), f.lookOf(change)
	size := 0
	for _, tile := range ks.tiles {
		src, at := p.source(l, tile.rect)
		coded, err := encodeTile(src, at, c.QIndex)
		if err != nil {
			return 0, err
		}
		size += len(coded.data)
	}
	return size, nil
}

// Work is what making the current picture was worth, for the bar a run draws
// - the pixels of its tiles whose keys came up for the first time, nought for
// a picture shown again. Added up over a film it is Stream.Work.
func (p *Pictures) Work() int64 { return p.work }
