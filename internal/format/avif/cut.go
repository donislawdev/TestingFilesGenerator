package avif

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/isobmff"
)

// writeForACut writes a file a damage will cut after its first m.cutAt bytes,
// with the padding split around the picture so that the cut keeps the share of
// the picture it keeps of the whole file (format.Request.CutAt): the coder's
// boxes up to the media data, free boxes, the media data, and free boxes
// behind it.
//
// With all of the padding behind the picture, as an uncut file has it, a large
// file cut at any share kept the whole picture and lost only padding. A free
// box may stand anywhere among the top level boxes, and readers skip it
// (docs/TRUNCATE-2026-11-11.md section 13). The one thing a box in front of the
// media data changes is where the picture starts, and the item locations in
// the meta box say that in bytes from the start of the file, so they move by
// as much. Their offsets are four bytes long as the coder writes them. One
// that the move would take past 4 GiB is widened to eight bytes, which the
// item location box allows, and the meta box grows by four bytes for each
// offset - only then, so every file that does not need it keeps the coder's
// meta box but for the numbers.
//
// A file whose share wants nothing in front, or too little for a free box, is
// written as one nothing cuts.
func writeForACut(ctx context.Context, w io.Writer, m memo, coded []byte) (bool, error) {
	head, mdat, err := splitAtTheMediaData(coded)
	if err != nil {
		return false, err
	}
	pad := m.total - int64(len(coded))
	front := format.SplitFront(pad, int64(len(head)), int64(len(mdat)), m.cutAt, m.total, boxHeader)
	if front == 0 {
		return false, nil
	}
	locs, err := readItemLocations(head)
	if err != nil {
		return false, err
	}
	wide := locs.furthest+front > math.MaxUint32
	if wide {
		grown := locs.growth()
		front = format.SplitFront(pad-grown, int64(len(head))+grown, int64(len(mdat)), m.cutAt, m.total, boxHeader)
		if front == 0 {
			return false, nil
		}
	}
	moved, err := locs.moved(head, front, wide)
	if err != nil {
		return false, err
	}
	back := m.total - int64(len(moved)) - front - int64(len(mdat))
	if back < 0 || back > 0 && back < boxHeader {
		return false, core.Defect(fmt.Errorf("avif: a split of %d B in front of the picture leaves %d B behind it", front, back))
	}

	markBoxes(w, moved)
	at := int64(len(moved))
	format.Mark(w, format.RolePadding, at, front)
	format.Mark(w, format.RoleContent, at+front, int64(len(mdat)))
	if back > 0 {
		format.Mark(w, format.RolePadding, at+front+int64(len(mdat)), back)
	}

	// Both sides draw their filler from the file's seed. Nobody reads padding,
	// and the side behind the picture lies past any cut.
	if _, err := w.Write(moved); err != nil {
		return true, err
	}
	if err := isobmff.WritePadding(ctx, w, m.seed, front); err != nil {
		return true, err
	}
	if _, err := w.Write(mdat); err != nil {
		return true, err
	}
	return true, isobmff.WritePadding(ctx, w, m.seed, back)
}

// splitAtTheMediaData parts the coder's file into the boxes in front of its
// media data and the media data box, which has to be the last one: that is
// where a box in front of the picture goes, and nothing behind it would move.
func splitAtTheMediaData(coded []byte) (head, mdat []byte, err error) {
	for pos := 0; pos+boxHeader <= len(coded); {
		size := int(binary.BigEndian.Uint32(coded[pos:]))
		if size < boxHeader || pos+size > len(coded) {
			break
		}
		mdat := string(coded[pos+4:pos+boxHeader]) == "mdat"
		if mdat && pos+size == len(coded) {
			return coded[:pos], coded[pos:], nil
		}
		if mdat {
			break
		}
		pos += size
	}
	return nil, nil, core.Defect(fmt.Errorf("avif: the coder's file does not end with one media data box behind its other boxes"))
}

// itemLocations is the item location box in a meta box, read far enough to
// move the places it gives.
type itemLocations struct {
	// metaAt and ilocAt are where the meta box and the item location box start
	// in the head, ilocLen how long the item location box is.
	metaAt, ilocAt, ilocLen int
	version                 byte
	// offsetSize, lengthSize, baseSize and indexSize are the four field sizes
	// in bytes, as the box gives them.
	offsetSize, lengthSize, baseSize, indexSize int
	// moving is how many fields hold a place in the file - the base offset of
	// each item when the box has one, each extent's offset otherwise.
	moving int
	// furthest is the largest place any of them gives.
	furthest int64
}

// countSize is how long the item count and each item's number are: two bytes,
// and four from version 2 on.
func (l itemLocations) countSize() int {
	if l.version == 2 {
		return 4
	}
	return 2
}

// growth is how much the item location box, and so the meta box, grows when
// the fields that hold a place are widened from four bytes to eight.
func (l itemLocations) growth() int64 { return int64(4 * l.moving) }

// readItemLocations finds the item location box in the meta box of head and
// reads the places it gives. Only places in this file (construction method 0)
// are what the coder writes, and only those are read.
func readItemLocations(head []byte) (itemLocations, error) {
	l, why := locateItemLocations(head)
	if why == "" {
		why = l.readPlaces(head[l.ilocAt+boxHeader : l.ilocAt+l.ilocLen])
	}
	if why != "" {
		return itemLocations{}, core.Defect(fmt.Errorf("avif: the coder's item locations %s", why))
	}
	return l, nil
}

// locateItemLocations finds the meta box among the top level boxes of head and
// the item location box in it, or says why it cannot.
func locateItemLocations(head []byte) (itemLocations, string) {
	l := itemLocations{metaAt: -1}
	for pos := 0; pos+boxHeader <= len(head); {
		size := int(binary.BigEndian.Uint32(head[pos:]))
		if size < boxHeader || pos+size > len(head) {
			return l, "stand in a box this cannot walk past"
		}
		why := ""
		if string(head[pos+4:pos+boxHeader]) == "meta" {
			l.metaAt = pos
			why = l.findIn(head, pos, size)
		}
		if why != "" {
			return l, why
		}
		pos += size
	}
	if l.metaAt < 0 || l.ilocLen == 0 {
		return l, "are missing"
	}
	return l, ""
}

// findIn finds the item location box among the boxes of the meta box that
// starts at pos. meta is a full box: its version and flags come before its
// boxes.
func (l *itemLocations) findIn(head []byte, pos, size int) string {
	for at := pos + boxHeader + 4; at+boxHeader <= pos+size; {
		n := int(binary.BigEndian.Uint32(head[at:]))
		if n < boxHeader || at+n > pos+size {
			return "stand in a meta box this cannot walk"
		}
		if string(head[at+4:at+boxHeader]) == "iloc" {
			l.ilocAt, l.ilocLen = at, n
		}
		at += n
	}
	return ""
}

// readPlaces reads the field sizes of the item location box's content b and
// every place in this file it gives, or says why it cannot.
func (l *itemLocations) readPlaces(b []byte) string {
	if len(b) < 8 {
		return "are cut short"
	}
	l.version = b[0]
	if l.version > 2 {
		return fmt.Sprintf("are in version %d", l.version)
	}
	l.offsetSize, l.lengthSize, l.baseSize = int(b[4]>>4), int(b[4]&15), int(b[5]>>4)
	if l.version >= 1 {
		l.indexSize = int(b[5] & 15)
	}
	moveBase := l.baseSize > 0
	if !moveBase && l.offsetSize == 0 {
		return "give no offset to move"
	}
	r := reader{b: b, at: 6}
	for range r.uint(l.countSize()) {
		r.uint(l.countSize())
		if l.version >= 1 && r.uint(2)&15 != 0 {
			return "point outside the file"
		}
		r.uint(2)
		l.place(moveBase, r.uint(l.baseSize))
		for range r.uint(2) {
			r.uint(l.indexSize)
			off := r.uint(l.offsetSize)
			r.uint(l.lengthSize)
			l.place(!moveBase, off)
		}
	}
	if r.short || r.at != len(b) {
		return "do not read to their end"
	}
	return ""
}

// place counts a field that holds a place in the file, when moves says it is
// one of the fields a move changes.
func (l *itemLocations) place(moves bool, at int64) {
	if moves {
		l.moving++
		l.furthest = max(l.furthest, at)
	}
}

// moved is head with every place the item locations give moved by front, and
// by what the widening adds to the meta box when wide. Widening turns the four
// byte fields that hold a place into eight byte ones.
func (l itemLocations) moved(head []byte, front int64, wide bool) ([]byte, error) {
	by := front
	from, to := l.offsetSize, l.offsetSize
	if l.baseSize > 0 {
		from, to = l.baseSize, l.baseSize
	}
	if wide {
		if from != 4 {
			return nil, core.Defect(fmt.Errorf("avif: the coder's item locations hold places in %d bytes, which this does not widen", from))
		}
		to = 8
		by += l.growth()
	}

	b := head[l.ilocAt+boxHeader : l.ilocAt+l.ilocLen]
	out := append([]byte(nil), b[:6]...)
	if l.baseSize > 0 {
		out[5] = byte(to)<<4 | b[5]&15
	} else {
		out[4] = byte(to)<<4 | b[4]&15
	}
	r := reader{b: b, at: 6}
	count := r.copy(&out, l.countSize())
	for range count {
		r.copy(&out, l.countSize())
		if l.version == 1 || l.version == 2 {
			r.copy(&out, 2)
		}
		r.copy(&out, 2)
		r.move(&out, l.baseSize, l.baseSize > 0, by, to)
		for range r.copy(&out, 2) {
			r.copy(&out, l.indexSize)
			r.move(&out, l.offsetSize, l.baseSize == 0, by, to)
			r.copy(&out, l.lengthSize)
		}
	}
	if to < 8 && by > 0 && l.furthest+by > math.MaxUint32 {
		return nil, core.Defect(fmt.Errorf("avif: a place moved by %d B no longer fits four bytes", by))
	}

	grown := len(out) + boxHeader - l.ilocLen
	moved := make([]byte, 0, len(head)+grown)
	moved = append(moved, head[:l.ilocAt]...)
	moved = binary.BigEndian.AppendUint32(moved, uint32(boxHeader+len(out)))
	moved = append(moved, "iloc"...)
	moved = append(moved, out...)
	moved = append(moved, head[l.ilocAt+l.ilocLen:]...)
	meta := binary.BigEndian.Uint32(head[l.metaAt:])
	binary.BigEndian.PutUint32(moved[l.metaAt:], meta+uint32(grown))
	return moved, nil
}

// reader reads big endian fields of a box's content and remembers whether it
// ran out.
type reader struct {
	b     []byte
	at    int
	short bool
}

func (r *reader) uint(n int) int64 {
	if r.at+n > len(r.b) {
		r.short, r.at = true, len(r.b)
		return 0
	}
	var v int64
	for _, c := range r.b[r.at : r.at+n] {
		v = v<<8 | int64(c)
	}
	r.at += n
	return v
}

// copy reads a field of n bytes, appends it to out as it was and returns it.
func (r *reader) copy(out *[]byte, n int) int64 {
	start := r.at
	v := r.uint(n)
	*out = append(*out, r.b[start:r.at]...)
	return v
}

// move reads a field of n bytes and appends it to out - as it was, or, when
// moves, moved by by and written in to bytes.
func (r *reader) move(out *[]byte, n int, moves bool, by int64, to int) {
	if !moves {
		r.copy(out, n)
		return
	}
	*out = appendUint(*out, r.uint(n)+by, to)
}

func appendUint(out []byte, v int64, n int) []byte {
	for i := n - 1; i >= 0; i-- {
		out = append(out, byte(v>>(8*i)))
	}
	return out
}
