package targz

import (
	"archive/tar"
	"compress/flate"
	"context"
	"encoding/binary"
	"fmt"
	"hash"
	"hash/crc32"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// An archive laid out for a cut (format.Request.CutAt,
// docs/TRUNCATE-2026-11-11.md section 11).
//
// An archive nothing cuts keeps its padding in the gzip header, in front of
// everything, and what the header cannot hold in a filler entry behind the
// files. Cut at any share, a large one lost padding and no file, or every file
// and no padding. So an archive a damage cuts has its padding on both sides of
// its files, split so the cut keeps the share of them it keeps of the whole:
//
//	gzip header - the extra field, padding in front to the byte, and the
//	              comment, which carries the label
//	directories - stored
//	front entry - fillerName, stored, when the front is more than the extra
//	              field holds
//	files       - stored, or squeezed by a compressor of their own
//	back entry  - stored: fillerName, or fillerEndName when one stands in
//	              front
//	marker      - the end of archive marker, stored, its last block final
//	trailer     - the CRC-32 and the length of the tar
//
// One gzip member and one deflate stream, every part in blocks of its own, so
// each part ends on a byte and the map says where. gzip.Writer compresses the
// whole tar as one stream and cannot write this, so the frame is written here,
// to RFC 1951 and RFC 1952.
//
// The files of a squeezed archive go through a compressor that starts with an
// empty window and ends on a byte with a sync flush, so how long they come to
// does not depend on what stands in front of them. One pass into a counter
// measures it, where an archive nothing cuts settles its padding over several
// (compress.go).
//
// The padding behind lands on the byte in the deflate stream, because the tar
// cannot take it: POSIX makes a tar whole blocks of 512 with nothing after the
// marker, and strict.py holds this format to that. The back entry takes whole
// blocks, the marker as many stored blocks as take the rest in steps of five
// bytes, and one to three empty blocks of fixed codes - ten bits each, the block
// compress/flate itself closes a stored stream with - the last few bytes. A
// rest of 1 to 4 or 8 bytes behind the marker's floor has no such frame, and
// goes in front.
//
// Rejected, after the readers were measured (section 11.3). A second gzip
// member behind the first, which 7-Zip reports as a tar of 0 B and which would
// make the squeezed files depend on the padding in front of them. One member
// for each part, which a reader that reads only the first member takes for a
// whole archive however it is cut. And zeros after the marker, which every
// reader takes and which leave the tar a part of a block long.

const (
	// fillerEndName is the entry of padding behind the files when one stands
	// in front of them with fillerName. Two names, because two entries of one
	// name are one file overwritten when the archive is unpacked.
	fillerEndName = "tfg-padding-end.bin"

	// storedHeader is what a stored deflate block that starts on a byte costs
	// in front of its bytes: the byte holding BFINAL and BTYPE, then LEN and
	// NLEN (RFC 1951 section 3.2.4).
	storedHeader = 5

	// markerLength is the end of archive marker, two blocks of zeros, and
	// tailFloor the shortest tail: the marker in one stored block.
	markerLength = 2 * tarBlock
	tailFloor    = markerLength + storedHeader
)

// frame is how an archive laid out for a cut is framed (layOut), and its zero
// value is the archive nothing cuts, framed by gzip.Writer.
type frame struct {
	on bool
	// comment is the label, with one space after it when the front is one
	// byte, which the extra field cannot cost.
	comment   string
	withExtra bool
	extraLen  int64
	// front and back are how many bytes of the tar the entries of padding in
	// front of the files and behind them take, header and data, 0 for none.
	front, back int64
	// content is how long the files' part comes to in the file.
	content int64
	// markerBlocks is how many stored blocks the end of archive marker takes,
	// and empties how many empty blocks of fixed codes stand in front of its
	// second block and its third.
	markerBlocks int64
	empties      [2]int
}

// cuts says the archive is laid out for a cut: one falls inside it, and there
// are files for it to keep a share of.
func (m memo) cuts(target int64) bool {
	return m.cutAt > 0 && m.cutAt < target && len(m.children) > 0
}

// settleForACut lays out a stored archive a damage will cut, while planning.
// A squeezed one is laid out as it is written (squeezeForACut), because how
// long its files come to is known only once they are squeezed. An archive too
// small for the frame keeps the layout of one nothing cuts, with the few bytes
// of padding pad put in its header, in front of the files.
func (m *memo) settleForACut(p *format.Plan, target int64, label string) error {
	if m.squeeze.On() || !m.cuts(target) {
		return nil
	}
	f, fits, err := layOut(*m, label, fewestBlocks(filesLength(*m)).length(), target)
	if err != nil || !fits {
		return err
	}
	m.frame = f
	delete(p.Properties, "padding_entry")
	if f.front > 0 || f.back > 0 {
		p.Properties["padding_entry"] = fillerName
	}
	return nil
}

// squeezeForACut lays out a squeezed archive a damage will cut, as it is
// written: its files are squeezed into a counter to learn how long they come
// to, and the pass that writes them has to come to the same (writeForACut).
// That generates every file inside twice, and only for a squeezed archive a
// damage cuts.
func (m *memo) squeezeForACut(ctx context.Context) error {
	if !m.squeeze.On() || !m.cuts(m.target) {
		return nil
	}
	c := &counter{}
	if err := writeFiles(ctx, c, io.Discard, *m); err != nil {
		return err
	}
	f, _, err := layOut(*m, m.comment, c.n, m.target)
	m.frame = f
	return err
}

// layOut frames an archive of target bytes whose files' part comes to content
// bytes, with the padding in front of the files that keeps the share
// (format.PaddingInFront) and the rest behind them. It says false when the
// frame does not fit: it costs a few bytes more than the layout of an archive
// nothing cuts, because each part closes its own blocks.
//
// The front is the extra field alone up to what it holds, and above that an
// entry of padding in front of the files with the extra field taking the last
// bytes the entry's 512 byte steps cannot. One byte, which no field costs,
// lengthens the comment, and without a label the front is two instead. That,
// and a rest behind that no frame of the marker takes, put a few bytes more in
// front, and the cut then takes as many more of the files - the "at least"
// missing promises. Where the padding is too short for the files the cut
// should take, all of it goes in front.
func layOut(m memo, label string, content, target int64) (frame, bool, error) {
	head := gzipHead + commentCost(label)
	dirs := fewestBlocks(dirsLength(m)).length()
	spare := target - (head + dirs + content + tailFloor + gzipTrailer)
	if spare < 0 || spare == 1 && label == "" {
		// One byte over the frame's floor can go only to the comment, as a
		// space behind the label, so without a label it does not fit either.
		return frame{}, false, nil
	}
	want := min(max(format.PaddingInFront(head+dirs, content, m.cutAt, target), 0), spare)
	// Each round puts in front what the tail of the one before left over, so
	// the second lands, or the third when the second met the one byte only a
	// label can cost.
	for range 3 {
		f := frame{on: true, comment: label, content: content}
		front := f.placeFront(want, spare, label)
		left := f.placeTail(spare - front)
		if left == 0 {
			return f, true, f.agrees(m, target)
		}
		want = front + left
	}
	return frame{}, false, core.Defect(fmt.Errorf("targz: no frame lays out %d B of padding around the files for a cut at %d", spare, m.cutAt))
}

// placeFront puts front bytes of padding in front of the files, and returns
// how many it put there.
func (f *frame) placeFront(front, spare int64, label string) int64 {
	switch {
	case front == 0:
	case front == 1 && label != "":
		f.comment = label + " "
	case front == 1 && spare < 2:
		return 0
	case front == 1:
		f.withExtra = true
		return 2
	case front <= extraPaddingLimit+2:
		f.withExtra, f.extraLen = true, front-2
	default:
		f.front = entryIn(front - 2)
		f.withExtra, f.extraLen = true, front-fewestBlocks(f.front).length()-2
	}
	return front
}

// placeTail lays out rest bytes behind the files beyond tailFloor: the back
// entry in whole tar blocks, as many as leave the marker a rest it can be
// framed to, and the marker's frame. It returns what no frame took, which is
// at most four bytes.
func (f *frame) placeTail(rest int64) int64 {
	f.back = entryIn(rest)
	over := rest - fewestBlocks(f.back).length()
	if !markerFrames(over) && f.back > 0 {
		f.back -= tarBlock
		over = rest - fewestBlocks(f.back).length()
	}
	left := int64(0)
	for !markerFrames(over) {
		over--
		left++
	}
	f.frameMarker(over)
	return left
}

// markerFrames says whether over bytes beyond one stored block can frame the
// end of archive marker: five for each block more, and one, two or four for
// one to three empty blocks of fixed codes in front of a block after the
// first (emptiesCost). Three takes two such places, and so two blocks more.
func markerFrames(over int64) bool {
	return over == 0 || over >= 5 && over != 8
}

// frameMarker frames the end of archive marker in over bytes beyond one
// stored block, which markerFrames allows.
func (f *frame) frameMarker(over int64) {
	f.markerBlocks = 1 + over/5
	switch over % 5 {
	case 1:
		f.empties = [2]int{1, 0}
	case 2:
		f.empties = [2]int{2, 0}
	case 3:
		f.empties = [2]int{1, 2}
	case 4:
		f.empties = [2]int{3, 0}
	}
}

// emptiesCost is what n empty blocks of fixed codes cost in front of a stored
// block that would otherwise start on a byte: ten bits each, and the stored
// block's own three bits then end in a later byte.
func emptiesCost(n int) int64 {
	return int64((10*n+3+7)/8 - 1)
}

// entryIn is the longest entry of padding, header and data in whole tar
// blocks, whose stored blocks come to no more than budget bytes - 0 when not
// even a header fits.
func entryIn(budget int64) int64 {
	if budget < tarBlock+storedHeader {
		return 0
	}
	blocks := (budget + storeBlock - 1) / storeBlock
	n := max((budget-storedHeader*blocks)/tarBlock*tarBlock, tarBlock)
	for fewestBlocks(n+tarBlock).length() <= budget {
		n += tarBlock
	}
	return n
}

// marker is the end of archive marker in its stored blocks.
func (f frame) marker() blocks {
	return blocks{n: markerLength, k: f.markerBlocks}
}

// agrees says whether the archive f frames comes to target bytes, added up
// part by part as markForACut and writeForACut lay them down - a defect
// rather than an archive the engine refuses for its length.
func (f frame) agrees(m memo, target int64) error {
	got := gzipHead + extraCost(f.withExtra, f.extraLen) + commentCost(f.comment) +
		fewestBlocks(dirsLength(m)).length() + fewestBlocks(f.front).length() + f.content +
		fewestBlocks(f.back).length() + f.marker().length() +
		emptiesCost(f.empties[0]) + emptiesCost(f.empties[1]) + gzipTrailer
	if got != target {
		return core.Defect(fmt.Errorf("targz: laid out for a cut, the archive comes to %d B where %d B was asked for", got, target))
	}
	return nil
}

// behindName is the name of the entry of padding behind the files.
func (f frame) behindName() string {
	if f.front > 0 {
		return fillerEndName
	}
	return fillerName
}

// markForACut marks the map of an archive laid out for a cut, before its first
// byte. Every part ends on a byte, so each has a length, the files' included -
// a cut inside them says how long they were.
func markForACut(w io.Writer, m memo) {
	f := m.frame
	extra, comment := extraCost(f.withExtra, f.extraLen), commentCost(f.comment)
	head := gzipHead + extra + comment
	format.Mark(w, format.RoleStructure, 0, gzipHead)
	format.Mark(w, format.RolePadding, gzipHead, extra)
	format.Mark(w, format.RoleStructure, gzipHead+extra, comment)
	dirs := fewestBlocks(dirsLength(m)).length()
	format.Mark(w, format.RoleStructure, head, dirs)
	front := fewestBlocks(f.front).length()
	format.Mark(w, format.RolePadding, head+dirs, front)
	at := head + dirs + front
	format.Mark(w, format.RoleContent, at, f.content)
	at += f.content
	back := fewestBlocks(f.back).length()
	format.Mark(w, format.RolePadding, at, back)
	format.Mark(w, format.RoleEnd, at+back, -1)
}

// writeForACut writes an archive laid out for a cut.
func writeForACut(ctx context.Context, w io.Writer, m memo) error {
	f := m.frame
	markForACut(w, m)
	if _, err := w.Write(gzipHeader(m.squeeze.Level, f)); err != nil {
		return err
	}
	sum := &tarSum{crc: crc32.NewIEEE()}
	if err := storedPart(w, sum, fewestBlocks(dirsLength(m)), func(dst io.Writer) error {
		return flushed(tar.NewWriter(dst), func(tw *tar.Writer) error { return writeDirectories(ctx, tw, m) })
	}); err != nil {
		return err
	}
	if err := writeFront(ctx, w, sum, m); err != nil {
		return err
	}
	files := &counter{}
	if err := writeFiles(ctx, io.MultiWriter(w, files), sum, m); err != nil {
		return err
	}
	if files.n != f.content {
		return core.Defect(fmt.Errorf("targz: the files inside came to %d B where the pass that measured them made %d B", files.n, f.content))
	}
	if err := writeTail(ctx, w, sum, m); err != nil {
		return err
	}
	var trailer [gzipTrailer]byte
	binary.LittleEndian.PutUint32(trailer[:4], sum.crc.Sum32())
	// ISIZE is the length modulo 2^32 (RFC 1952 section 2.3.1), so the
	// conversion drops what it should.
	binary.LittleEndian.PutUint32(trailer[4:], uint32(sum.n))
	_, err := w.Write(trailer[:])
	return err
}

// writeFront writes the entry of padding in front of the files, if there is
// one. Its bytes come from the seed of an index no file inside has, so the two
// entries of padding do not open with the same bytes.
func writeFront(ctx context.Context, w, sum io.Writer, m memo) error {
	f := m.frame
	if f.front == 0 {
		return nil
	}
	return storedPart(w, sum, fewestBlocks(f.front), func(dst io.Writer) error {
		return flushed(tar.NewWriter(dst), func(tw *tar.Writer) error {
			return writeFillerEntry(ctx, tw, m, fillerName, f.front-tarBlock, core.FileSeed(m.seed, len(m.children)))
		})
	})
}

// writeFiles writes the files' part into w, stored or squeezed as the archive
// is, and the tar's bytes into sum. Squeezed, the compressor ends with a sync
// flush rather than closing: that ends the files on a byte with a block that
// is not the last, and the tail carries the stream on.
func writeFiles(ctx context.Context, w, sum io.Writer, m memo) error {
	entries := func(tw *tar.Writer) error { return writeFileEntries(ctx, tw, m) }
	if !m.squeeze.On() {
		return storedPart(w, sum, fewestBlocks(filesLength(m)), func(dst io.Writer) error {
			return flushed(tar.NewWriter(dst), entries)
		})
	}
	zw, err := flate.NewWriter(w, m.squeeze.Level)
	if err != nil {
		return core.Defect(fmt.Errorf("targz: the files inside could not be squeezed: %w", err))
	}
	if err := flushed(tar.NewWriter(io.MultiWriter(zw, sum)), entries); err != nil {
		return err
	}
	return zw.Flush()
}

// writeTail writes the back entry and the end of archive marker, each in its
// own stored blocks. One tar writer writes both, so the marker is the one
// archive/tar closes an archive with, and only where its bytes go changes.
func writeTail(ctx context.Context, w, sum io.Writer, m memo) error {
	f := m.frame
	to := &switchWriter{}
	tw := tar.NewWriter(to)
	if err := storedPart(w, sum, fewestBlocks(f.back), func(dst io.Writer) error {
		to.w = dst
		if f.back == 0 {
			return nil
		}
		return flushed(tw, func(tw *tar.Writer) error {
			return writeFillerEntry(ctx, tw, m, f.behindName(), f.back-tarBlock, m.seed)
		})
	}); err != nil {
		return err
	}
	s := &storedSegment{w: w, b: f.marker(), last: true, empties: f.empties}
	to.w = io.MultiWriter(s, sum)
	if err := tw.Close(); err != nil {
		return err
	}
	return s.done()
}

// switchWriter passes what it is given to w, which can change between writes.
type switchWriter struct{ w io.Writer }

func (s *switchWriter) Write(p []byte) (int, error) { return s.w.Write(p) }

// flushed runs body on tw and then pads its last entry to a whole block,
// without the end of archive marker Close would add.
func flushed(tw *tar.Writer, body func(*tar.Writer) error) error {
	if err := body(tw); err != nil {
		return err
	}
	return tw.Flush()
}

// gzipHeader is the gzip header of an archive laid out for a cut, as
// gzip.Writer writes that of one nothing cuts (RFC 1952 section 2.3): no name
// and no time, the extra field and the comment when there are any, XFL as the
// level says, and the system unknown.
func gzipHeader(level int, f frame) []byte {
	h := []byte{0x1f, 0x8b, 8, 0, 0, 0, 0, 0, 0, 255}
	switch level {
	case flate.BestCompression:
		h[8] = 2
	case flate.BestSpeed:
		h[8] = 4
	}
	if f.withExtra {
		h[3] |= 4
		h = binary.LittleEndian.AppendUint16(h, uint16(f.extraLen))
		h = append(h, make([]byte, f.extraLen)...)
	}
	if f.comment != "" {
		h[3] |= 16
		h = append(h, f.comment...)
		h = append(h, 0)
	}
	return h
}

// tarSum is the CRC-32 and the length of the tar, which the gzip trailer
// carries.
type tarSum struct {
	crc hash.Hash32
	n   int64
}

func (s *tarSum) Write(p []byte) (int, error) {
	s.n += int64(len(p))
	return s.crc.Write(p)
}

// blocks is n bytes cut into k stored blocks, as evenly as they go: the first
// n%k blocks a byte longer than the rest. k is no more than n, so no block is
// empty, and at least n/storeBlock rounded up, so none is longer than a stored
// block can be.
type blocks struct{ n, k int64 }

// fewestBlocks is n bytes in as few stored blocks as hold them.
func fewestBlocks(n int64) blocks {
	return blocks{n: n, k: (n + storeBlock - 1) / storeBlock}
}

// length is how long the blocks come to in the file, headers and all.
func (b blocks) length() int64 { return b.n + storedHeader*b.k }

// size is how many bytes block i carries.
func (b blocks) size(i int64) int64 {
	if i < b.n%b.k {
		return b.n/b.k + 1
	}
	return b.n / b.k
}

// storedPart writes the bytes body gives it into w as the stored blocks b
// lays out, and into sum as well. A body that gives more or fewer than b.n
// bytes is a defect.
func storedPart(w, sum io.Writer, b blocks, body func(io.Writer) error) error {
	s := &storedSegment{w: w, b: b}
	if err := body(io.MultiWriter(s, sum)); err != nil {
		return err
	}
	return s.done()
}

// storedSegment writes what it is given as the stored blocks b lays out,
// opening each with its header as the first of its bytes arrives. The last
// block is final when last is set, and empties are the empty blocks of fixed
// codes in front of the second block and the third.
type storedSegment struct {
	w       io.Writer
	b       blocks
	last    bool
	empties [2]int
	// next is the block to open next, and left how many bytes the open one
	// still takes.
	next, left int64
}

func (s *storedSegment) Write(p []byte) (int, error) {
	n := 0
	for len(p) > 0 {
		if err := s.openWhenFull(); err != nil {
			return n, err
		}
		k := min(s.left, int64(len(p)))
		written, err := s.w.Write(p[:k])
		n += written
		s.left -= int64(written)
		if err != nil {
			return n, err
		}
		p = p[k:]
	}
	return n, nil
}

// openWhenFull opens the next block once the open one has all its bytes.
func (s *storedSegment) openWhenFull() error {
	if s.left > 0 {
		return nil
	}
	if s.next == s.b.k {
		return core.Defect(fmt.Errorf("targz: a part of the archive came to more than the %d B it was laid out for", s.b.n))
	}
	empties := 0
	if s.next == 1 || s.next == 2 {
		empties = s.empties[s.next-1]
	}
	size := s.b.size(s.next)
	s.next++
	s.left = size
	_, err := s.w.Write(blockHeader(empties, s.last && s.next == s.b.k, size))
	return err
}

// done says whether every block got all its bytes.
func (s *storedSegment) done() error {
	if s.next != s.b.k || s.left != 0 {
		return core.Defect(fmt.Errorf("targz: a part of the archive came to less than the %d B it was laid out for", s.b.n))
	}
	return nil
}

// blockHeader is what stands in front of a stored block's bytes: empties
// empty blocks of fixed codes - BFINAL 0, BTYPE 01 and the end of block code,
// seven zero bits - then the block's own BFINAL and BTYPE 00, the bits to the
// next byte, LEN and NLEN (RFC 1951 sections 3.2.3, 3.2.4 and 3.2.6). Bits go
// in from the least significant end of each byte.
func blockHeader(empties int, final bool, size int64) []byte {
	var bits uint64
	n := 0
	for range empties {
		bits |= 1 << (n + 1)
		n += 10
	}
	if final {
		bits |= 1 << n
	}
	n += 3
	h := make([]byte, 0, (n+7)/8+4)
	for i := 0; i < n; i += 8 {
		h = append(h, byte(bits>>i))
	}
	h = binary.LittleEndian.AppendUint16(h, uint16(size))
	return binary.LittleEndian.AppendUint16(h, ^uint16(size))
}
