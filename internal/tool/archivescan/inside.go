package archivescan

import (
	"context"
	"errors"
	"io"
	"math"
	"slices"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/tool"
)

// The archives inside an archive: each is read the way the one given is, from
// memory, and what is wrong in it is wrong in the one given - a program that
// unpacks everything inside unpacks those too (docs/SKANER-W-GLAB-2026-10-19.md).
//
// Nothing here grows with what an archive holds. The limits are the run's,
// shared by the archive given and every archive inside it, so an archive of a
// thousand archives is allowed no more than one archive alone. How deep the
// tool looks is counted before it looks, and an archive inside is read only
// once the entry holding it has been read whole, so one is read at a time.

// tree is what one run of the tool shares across every archive it reads.
type tree struct {
	ctx  context.Context
	root *scan
	// limit and progress are the archive given's: the limit a person set,
	// or -1, and where the reading of it is told.
	limit    int64
	progress tool.Progress
	// entries, placeRoom and nameRoom are counted once for the whole run.
	entries   int
	placeRoom int
	nameRoom  int
	// held is the bytes of archives inside held in memory right now.
	held int
	// unpacked is what everything read unpacks to, on every level.
	unpacked uint64
	// listed is the bytes of names the report keeps, and reports how many
	// reports of archives inside it keeps.
	listed  int
	reports int
	// exploded is a run whose archives, together, were found built to
	// explode.
	exploded bool
	// buf is the one buffer every archive's contents are read into.
	buf []byte
}

// explodes says whether what everything read so far unpacks to, the archive
// given and every one inside it, makes the archive given one built to explode,
// and says so on it the first time. Only once something inside has unpacked:
// before that the archive given asks it of itself.
func (t *tree) explodes() bool {
	if t.exploded {
		return true
	}
	r := t.root
	if t.unpacked == r.unpacked || t.unpacked < BombBytes || t.unpacked <= uint64(r.size)*BombRatio {
		return false
	}
	t.exploded = true
	r.exploded = true
	r.noteSaid(noteBomb, "", core.Says("archivescan.BombInside", "with the archives inside it, %s from %s",
		core.A("Unpacked", bytesOf(t.unpacked)), core.A("Size", core.Bytes(r.size))))
	r.stop(core.Says("archivescan.StoppedBombInside", "with the archives inside it, it unpacks to more than %d times its size, and no more is read",
		core.A("Ratio", BombRatio)))
	return true
}

// check asks the run's rule once an archive has asked its own, and ends the
// reading of that archive when the run is found built to explode - saying so
// in an archive inside that had not found it of itself, whose own reading
// ends unfinished.
func (t *tree) check(s *scan) {
	if !t.explodes() || s.exploded {
		return
	}
	s.exploded = true
	if s != t.root {
		s.stop(core.Says("archivescan.StoppedAround", "the archive given was found built to explode with the archives inside it, and no more is read"))
	}
}

// holding is what an entry holds, kept while it is read when it begins as an
// archive this tool reads, to be read in turn once it has been read whole. It
// is kept in pieces that are never copied to grow - growing by copying holds
// the old copy and the new one at once - and read back through ReadAt.
type holding struct {
	t *tree
	// said is what the entry says it holds, or -1 when nothing says.
	said int64
	// open is whether an archive found here may be read: not one already as
	// deep as the tool looks.
	open    bool
	decided bool
	head    []byte
	// kind is the archive the contents begin as, or nothing.
	kind string
	// parts are what is held, each of piece bytes but the last, size how
	// much that is, and held whether the contents are being held.
	parts [][]byte
	size  int64
	held  bool
	// over is an archive inside larger than is left to hold, not held.
	over bool
}

// piece is how much of an archive inside one allocation holds.
const piece = 1 << 20

// start readies the holding for the contents of one entry.
func (h *holding) start(t *tree, said uint64, open bool) {
	h.drop()
	h.t, h.open, h.decided, h.head, h.kind, h.over = t, open, false, h.head[:0], "", false
	h.said = int64(min(said, math.MaxInt64))
}

// unsaid readies the holding for contents nothing gives the size of.
func (h *holding) unsaid(t *tree, open bool) {
	h.start(t, 0, open)
	h.said = -1
}

// Write is a piece of the contents, in order. It never fails: what does not
// fit is not held, and the reading goes on.
func (h *holding) Write(p []byte) (int, error) {
	n := len(p)
	if !h.decided {
		take := min(len(p), headLength-len(h.head))
		h.head = append(h.head, p[:take]...)
		if len(h.head) < headLength {
			return n, nil
		}
		h.decide()
		p = p[take:]
	}
	h.keep(p)
	return n, nil
}

// decide tells from the first bytes whether the contents are an archive, and
// holds them when they are one this tool reads and there is room for them.
func (h *holding) decide() {
	h.decided = true
	h.kind = archiveIn(h.head)
	if !h.open || !readable(h.kind) || (h.said >= 0 && h.said < int64(len(h.head))) {
		return
	}
	if h.said > int64(MostInside-h.t.held) {
		h.over = true
		return
	}
	h.held = true
	h.keep(h.head)
}

// keep holds more of the contents, while they are being held.
func (h *holding) keep(p []byte) {
	for len(p) > 0 && h.held {
		if h.full() && !h.grow() {
			return
		}
		last := len(h.parts) - 1
		part := h.parts[last]
		n := copy(part[len(part):cap(part)], p)
		h.parts[last] = part[:len(part)+n]
		h.size += int64(n)
		p = p[n:]
	}
}

// full says whether there is no room left in the pieces held so far.
func (h *holding) full() bool {
	last := len(h.parts) - 1
	return last < 0 || len(h.parts[last]) == cap(h.parts[last])
}

// grow adds a piece. Past what the entry said it holds, nothing more is held
// and it is let go - the entry is reported for holding more than it says, and
// never read. Past what is left to hold, the same, and the archive is too
// large to look into.
func (h *holding) grow() bool {
	want := int64(piece)
	if h.said >= 0 {
		want = min(want, h.said-h.size)
	}
	if want <= 0 || want > int64(MostInside-h.t.held) {
		h.over = want > 0
		h.drop()
		return false
	}
	h.parts = append(h.parts, make([]byte, 0, want))
	h.t.held += int(want)
	return true
}

// drop lets go of what is held.
func (h *holding) drop() {
	for _, part := range h.parts {
		h.t.held -= cap(part)
	}
	h.parts, h.size, h.held = nil, 0, false
}

// errBefore is a read before the first byte held, which nothing in this
// package asks for.
var errBefore = core.Defect(errors.New("an archive held in memory was read before its first byte"))

// ReadAt reads what is held, the way a file of it would be read.
func (h *holding) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, errBefore
	}
	n := 0
	for n < len(p) && off < h.size {
		c := copy(p[n:], h.parts[off/piece][off%piece:])
		n += c
		off += int64(c)
	}
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// within reads the archive an entry held, the last step of reading an entry
// whose contents were whole - or says why it is not read.
func (s *scan) within(name string, h *holding) error {
	if !h.decided {
		// Contents shorter than a TAR header, read whole: their size is
		// known now.
		if h.said < 0 {
			h.said = int64(len(h.head))
		}
		h.decide()
	}
	defer h.drop()
	var why core.Said
	switch {
	case h.kind == "":
		return nil
	case !readable(h.kind):
		why = core.Says("archivescan.InsideUnread", "it is a %s archive, which this tool does not read", core.A("Kind", h.kind))
	case !h.open:
		why = core.Says("archivescan.TooDeep", "it is an archive more than %d archives deep, deeper than this tool looks", core.A("Depth", MostDepth))
	case h.over:
		why = core.Says("archivescan.TooLarge", "it is an archive larger than the %s this tool holds to look into", core.A("Most", core.Bytes(MostInside)))
	case !h.held:
		return nil
	}
	if !why.IsZero() {
		s.noteSaid(noteUnchecked, name, entryWhy(name, why))
		return nil
	}
	c := blank(name, h.size, s.tree)
	c.depth = s.depth + 1
	err := c.look(h)
	c.hold.drop()
	if err != nil {
		return err
	}
	s.took(name, c)
	return nil
}

// took is an archive read inside this one: what is wrong in it is wrong here,
// each said with the way to it.
func (s *scan) took(name string, c *scan) {
	c.names = nil
	s.note(noteNested, name)
	s.nest.unpacked = addCapped(s.nest.unpacked, addCapped(c.unpacked, c.nest.unpacked))
	for _, d := range declared {
		for _, f := range slices.Concat(c.found.own[d.ID], c.found.deep[d.ID]) {
			s.tree.add(s.found.deep, s.found.deepMore, d.ID, deeper(name, f))
		}
		s.found.deepMore[d.ID] += c.found.more[d.ID] + c.found.deepMore[d.ID]
	}
	if c.safe() {
		return
	}
	s.nest.unsafe = true
	if len(s.nest.kept) >= MostListed || s.tree.reports >= MostReports {
		s.nest.more++
		return
	}
	s.nest.kept = append(s.nest.kept, c)
	s.tree.reports++
}

// deeper is a finding of an archive inside, said with the name of the entry
// that holds it. Its own name is in the words, not the data: --json names what
// is in one archive, and the archives inside have reports of their own.
func deeper(name string, f found) found {
	words := f.words
	if words.IsZero() {
		words = core.Says("archivescan.Name", "%s", core.A("Name", shownName(f.name)))
	}
	return found{words: core.Says("archivescan.Inside", "%s > %s", core.A("Archive", shownName(name)), core.A("Item", words))}
}
