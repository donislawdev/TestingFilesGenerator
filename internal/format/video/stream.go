package video

import (
	"fmt"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// maxSuffixBits is the longest frame header splitFrame accepts from tile_info
// on: tile_info at most 3 bits for one tile, the quantizer 14, the loop filter
// 29 with the tx mode, and reduced_tx_set 1. Planning counts every picture
// with it, so no picture the reader accepts can carry a longer header than
// planning counted.
const maxSuffixBits = 47

// The frame headers this package writes before a picture's suffix, in bits:
// a shown key frame says eight things, a hidden intra only copy eighteen.
const (
	keyHeaderBits    = 8
	hiddenHeaderBits = 18
)

// Stream is what a film's AV1 samples have in common: the sequence header and
// its level, the frame that shows a picture again, and the most bytes any one
// picture may take. The samples that carry a picture are made per picture, by
// Pictures - no temporal delimiters, every OBU with its size, as both
// containers carry them.
type Stream struct {
	Timeline
	Width, Height int
	// Level is the seq_level_idx the sequence header declares.
	Level int
	// Reserve is the most tile bytes any picture of the film may take.
	Reserve int

	config []byte
	seq    []byte
	show   []byte
}

// NewStream is the stream of a film whose pictures each take at most reserve
// bytes of tile.
//
// The level is chosen from the reserve rather than from the pictures, because
// the sequence header is written before any picture after the first is coded.
// The reserve is an upper bound on every one of them, so a level that admits
// it admits the film.
//
// The sequence header travels in every key sample as well as in the
// container's codec configuration, so a player that starts at a key frame has
// it there - which is what the AV1 bindings of both containers ask of a sample
// a player can start from.
func NewStream(t Timeline, reserve, width, height int) Stream {
	key, _ := frameLens(maxSuffixBits, reserve)
	level := chooseLevel(width, height, t.FPS, key, int(t.CodedPerSecond()))
	seq := obu(obuSequenceHeader, sequenceHeader(width, height, level))
	return Stream{
		Timeline: t, Width: width, Height: height, Level: level, Reserve: reserve,
		config: codecConfig(level, seq), seq: seq, show: showCopy(),
	}
}

// frameLens is how long the key frame OBU and the hidden copy OBU around a
// picture are, from the length of its header suffix and of its tile - counted,
// not built, so planning a film never allocates its pictures.
func frameLens(suffixBits, tile int) (key, hidden int) {
	obuLen := func(headerBits int) int {
		payload := (headerBits+7)/8 + tile
		return 1 + len(leb128(payload)) + payload
	}
	return obuLen(keyHeaderBits + suffixBits), obuLen(hiddenHeaderBits + suffixBits)
}

// Config is the AV1 codec configuration record with the sequence header OBU,
// what MP4 puts in av1C and Matroska in CodecPrivate.
func (s Stream) Config() []byte { return s.config }

// ShowSample is a frame that shows the current picture again. Shared between
// frames, and must not be written to.
func (s Stream) ShowSample() []byte { return s.show }

// BoundBytes is the longest each of the three kinds of sample can be - a key
// frame, a hidden copy with the frame that shows it, and a frame that shows a
// picture again - for a container's arithmetic to plan with before any picture
// is coded.
func (s Stream) BoundBytes() (key, copied, shown int) {
	k, h := frameLens(maxSuffixBits, s.Reserve)
	return len(s.seq) + k, h + len(s.show), len(s.show)
}

// joined is a followed by b in one allocation of the right size.
func joined(a, b []byte) []byte {
	return append(append(make([]byte, 0, len(a)+len(b)), a...), b...)
}

// Pictures codes the pictures of one film in the order the film shows them,
// on the goroutine that asks for them and on the helpers it can take
// (ahead.go), and keeps only the last one it was asked for and the few coded
// ahead of it - an hour at a change a second is 3600 pictures, and holding
// them all would hold the film.
//
// Close has to be called when the film is written or abandoned, because the
// helpers are goroutines.
type Pictures struct {
	choice Choice
	stream Stream
	film   *film
	crew   *crew
	// pending are the pictures offered and not yet reached, in order, each
	// with the change it opens. Changes that look the same as the one before
	// open none, so they are not in it.
	pending []openedBy
	next    int64 // the first change not looked at yet
	last    look  // the look of change next-1
	change  int64
	key     []byte
	copied  []byte
}

type openedBy struct {
	change int64
	job    *job
}

// Pictures is the coder of this choice's film.
func (c Choice) Pictures(st Stream) *Pictures {
	f := newFilm(c.Width, c.Height, c.Seed, c.Label, st.Timeline)
	cr := newCrew(f, c.QIndex, st.Changes())
	return &Pictures{choice: c, stream: st, film: f, crew: cr, change: -1,
		pending: make([]openedBy, 0, cr.ahead()+1)}
}

// At makes picture c the current one, coding it unless it looks the same as
// the one before. Pictures are asked for in the order the film shows them -
// the same change again is fine, an earlier one is a defect of the caller.
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
	var opened *job
	reached := 0
	for reached < len(p.pending) && p.pending[reached].change <= c {
		opened = p.pending[reached].job
		reached++
	}
	// Moved down rather than sliced off, so the one array made for the
	// window lasts the whole film.
	p.pending = p.pending[:copy(p.pending, p.pending[reached:])]
	if opened == nil {
		// The same look as the picture before it: shown again, not coded.
		p.change = c
		return nil
	}
	coded, err := p.crew.wait(opened)
	if err != nil {
		return err
	}
	if coded.Size() > p.stream.Reserve {
		return core.Defect(fmt.Errorf("video: picture %d of a %dx%d film coded to a %d B tile and the film was planned on %d B a picture, so the file cannot be kept",
			c, p.choice.Width, p.choice.Height, coded.Size(), p.stream.Reserve))
	}
	p.change = c
	p.key = joined(p.stream.seq, keyFrame(coded))
	p.copied = joined(hiddenCopy(coded), p.stream.show)
	return nil
}

// offerTo looks at every change up to c and as far past it as the crew codes
// ahead, and offers each one that looks different from the one before it.
// Change 0 is the picture planning already coded, when it coded one.
func (p *Pictures) offerTo(c int64) {
	n := p.stream.Changes()
	for p.next < n && (p.next <= c || len(p.pending) < p.crew.ahead()) {
		l := p.film.lookOf(p.next)
		if p.next == 0 || l != p.last {
			j := (*job)(nil)
			if p.next == 0 && p.choice.First != nil {
				j = codedJob(*p.choice.First)
			} else {
				j = newJob(l)
				p.crew.offer(j)
			}
			p.pending = append(p.pending, openedBy{change: p.next, job: j})
		}
		p.last = l
		p.next++
	}
	if p.next == n {
		p.crew.finish()
	}
}

// Close stops the helpers and waits for them, so nothing this film started is
// running once it returns. A helper in the middle of a picture finishes it
// first, which is at most one picture's time.
func (p *Pictures) Close() { p.crew.stop() }

// KeySample is the current picture as a key frame, with the sequence header
// before it.
func (p *Pictures) KeySample() []byte { return p.key }

// CopySample is the current picture as a hidden intra only copy, and the frame
// that shows it.
func (p *Pictures) CopySample() []byte { return p.copied }
