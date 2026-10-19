package archivescan

import (
	"bufio"
	"bytes"
	"compress/flate"
	"errors"
	"hash/crc32"
	"io"
	"sort"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// Reading a ZIP archive: the record at its end, which says where the list of
// contents is, then the list, one record at a time, then each entry where the
// list says it starts, and what each entry holds.
//
// Read here rather than through archive/zip, which holds the whole list in
// memory before a caller can ask how long it is - 216 bytes an entry, measured
// (docs/SKANER-ARCHIWUM-2026-10-19.md §2.1) - and which never shows the name
// written at the entry itself, the one a program unpacking from the front
// reads instead of the list's.

// zipScan is the reading of one ZIP: the scan it reports to, the archive and
// where its list is.
type zipScan struct {
	*scan
	r   io.ReaderAt
	end zipEnd
	// data is where the contents of each entry start, by its place in the
	// list - zero for an entry whose header is not where the list says.
	data []uint64
	// shared are the entries whose bytes lie in another's.
	shared map[int]bool
	// open is an archive whose contents are read: not one already found
	// built to explode, which is what reading them would do.
	open bool
	// inflate unpacks every deflated entry, one for the archive: a new one
	// holds tens of kilobytes, and an archive can hold a million entries.
	inflate io.ReadCloser
}

// zip reads a ZIP archive.
func (s *scan) zip(r io.ReaderAt) error {
	end, err := findEnd(r, s.size)
	switch {
	case errors.Is(err, errNoEnd):
		s.noteSaid(noteDamaged, "", core.Says("archivescan.NoEnd", "the record at its end, which says where its list of contents is, is missing"))
		return nil
	case err != nil:
		return err
	case end.split:
		s.noteSaid(noteDamaged, "", core.Says("archivescan.Split", "it says it is one part of an archive split across several files"))
		return nil
	case end.listAt > end.at || end.listSize > end.at-end.listAt:
		s.noteSaid(noteDamaged, "", core.Says("archivescan.ListElsewhere", "its list of contents is not where the record at its end says"))
		return nil
	}
	z := &zipScan{scan: s, r: r, end: end}
	spans, err := z.list()
	if err != nil {
		return err
	}
	if !s.stopped && uint64(len(z.data)) != end.entries {
		s.noteSaid(noteDamaged, "", core.Says("archivescan.Counted", "the record at its end gives the number of entries as %d and the list holds %d",
			core.A("Counted", end.entries), core.A("Listed", len(z.data))))
	}
	z.shared = overlapping(spans)
	s.weigh()
	z.open = !s.stopped && len(z.shared) == 0 && !s.exploded
	return z.contents()
}

// span is where one entry lies in the archive, from its header to the end of
// its contents.
type span struct {
	from, to uint64
	entry    int
}

// list is the first look at every entry: its name, its place among the
// others, and its header where the list says it starts. It hands back where
// each entry lies.
func (z *zipScan) list() ([]span, error) {
	var spans []span
	err := z.each(true, func(e zipEntry) bool {
		if !z.entry(e.name, isFolder(e)) {
			return false
		}
		z.grow(e.unpacked)
		at := z.header(e)
		z.data = append(z.data, at)
		if at != 0 {
			spans = append(spans, span{from: e.at, to: at + e.packed, entry: len(z.data) - 1})
		}
		return true
	})
	return spans, err
}

// header reads the header an entry begins with, and says where its contents
// start - or nothing, when the header is not where the list says or the
// contents run into the list.
func (z *zipScan) header(e zipEntry) uint64 {
	end := z.end
	if e.at > end.listAt || end.listAt-e.at < lenLocal {
		return z.broken(e, core.Says("archivescan.NotWhereListed", "it does not start where the list of contents says"))
	}
	// One read for the header and a name as long as the listed one, which is
	// what it nearly always is - a second only when it is not.
	head := make([]byte, min(uint64(lenLocal+len(e.name)), end.listAt-e.at))
	if _, err := z.r.ReadAt(head, int64(e.at)); err != nil || le32(head) != sigLocal {
		return z.broken(e, core.Says("archivescan.NotWhereListed", "it does not start where the list of contents says"))
	}
	nameLen, extraLen := uint64(le16(head[26:])), uint64(le16(head[28:]))
	data := e.at + lenLocal + nameLen + extraLen
	if data > end.listAt || e.packed > end.listAt-data {
		return z.broken(e, core.Says("archivescan.RunsIntoList", "its contents run into the list of contents"))
	}
	local := head[lenLocal:]
	if uint64(len(local)) != nameLen {
		local = make([]byte, nameLen)
		if _, err := z.r.ReadAt(local, int64(e.at+lenLocal)); err != nil {
			return z.broken(e, core.Says("archivescan.NotWhereListed", "it does not start where the list of contents says"))
		}
	}
	if !bytes.Equal(local, []byte(e.name)) {
		z.noteSaid(noteTwoNames, e.name, core.Says("archivescan.TwoNames", "%s in the list, %s at the entry",
			core.A("Listed", core.Shown(e.name)), core.A("Local", core.Shown(string(local)))))
	}
	return data
}

// broken notes an entry that is not what the list says, and hands back the
// nothing its header's reader answers with then.
func (z *zipScan) broken(e zipEntry, why core.Said) uint64 {
	z.noteSaid(noteDamaged, e.name, entryWhy(e.name, why))
	return 0
}

// overlapping is every entry whose bytes lie in another's - the way the
// archives that unpack to a million times their size are built, since one
// stretch of contents read as many files is the only way past what deflate
// can do once (docs/SKANER-ARCHIWUM-2026-10-19.md §3).
func overlapping(spans []span) map[int]bool {
	order := make([]span, len(spans))
	copy(order, spans)
	sort.Slice(order, func(i, j int) bool { return order[i].from < order[j].from })
	shared := map[int]bool{}
	reach, owner := uint64(0), -1
	for i, sp := range order {
		if i > 0 && sp.from < reach {
			shared[sp.entry] = true
			shared[owner] = true
		}
		if sp.to > reach {
			reach, owner = sp.to, sp.entry
		}
	}
	return shared
}

// contents is the second look at every entry the first one counted: the kind
// of thing it is, whether its bytes are another's and what it holds.
func (z *zipScan) contents() error {
	index := 0
	var failed error
	err := z.each(false, func(e zipEntry) bool {
		if index >= len(z.data) || !z.going() {
			return false
		}
		failed = z.inside(e, index)
		index++
		return failed == nil
	})
	if failed != nil {
		return failed
	}
	return err
}

// inside is the second look at one entry.
func (z *zipScan) inside(e zipEntry, index int) error {
	if z.shared[index] {
		z.note(noteOverlapping, e.name)
	}
	special, link := unixKind(e)
	if !special.IsZero() {
		z.special(e.name, special)
		return nil
	}
	at := z.data[index]
	readable := z.open && at != 0 && !isFolder(e)
	if link {
		target := ""
		var err error
		if readable {
			target, _, err = z.read(e, at, false)
		}
		z.noteSaid(noteLinks, e.name, linked(e.name, target))
		return err
	}
	if !readable {
		return nil
	}
	_, whole, err := z.read(e, at, true)
	if err != nil || !whole {
		z.hold.drop()
		return err
	}
	return z.within(e.name, &z.hold)
}

// read reads what an entry holds, through its checksum and no further than
// one byte past what the list says it holds, and hands back its first bytes
// and whether it read them whole and as the list says - and holds them, when
// asked to look and they are an archive. What is wrong with the contents is
// noted, and only a stop or a disk that fails to read is an error.
func (z *zipScan) read(e zipEntry, at uint64, look bool) (string, bool, error) {
	switch {
	case e.flags&1 != 0 || e.method == methodAES:
		z.noteSaid(noteUnchecked, e.name, entryWhy(e.name, core.Says("archivescan.Encrypted", "it is encrypted")))
		return "", false, nil
	case e.method != methodStored && e.method != methodDeflated:
		z.noteSaid(noteUnchecked, e.name, entryWhy(e.name, core.Says("archivescan.Method",
			"it is packed with method %d, which this tool does not read", core.A("Method", int(e.method)))))
		return "", false, nil
	}
	var src io.Reader = &counting{r: io.NewSectionReader(z.r, int64(at), int64(e.packed)), s: z.scan}
	if e.method == methodDeflated {
		src = z.inflated(src)
	}
	sum := crc32.NewIEEE()
	z.hold.start(z.tree, e.unpacked, look && z.depth < MostDepth)
	got, head, err := drain(z.tree.ctx, z.tree.buf, src, e.unpacked, io.MultiWriter(sum, &z.hold), nil)
	var corrupt flate.CorruptInputError
	switch {
	case z.tree.ctx.Err() != nil:
		return "", false, z.tree.ctx.Err()
	case got > e.unpacked:
		z.note(noteLonger, e.name)
	case errors.As(err, &corrupt), errors.Is(err, io.ErrUnexpectedEOF), got < e.unpacked:
		z.broken(e, core.Says("archivescan.EndsEarly", "its contents end before the size the list gives them"))
	case err != nil:
		return "", false, err
	case sum.Sum32() != e.crc:
		z.broken(e, core.Says("archivescan.Checksum", "its contents do not match their checksum"))
	default:
		return head, true, nil
	}
	return head, false, nil
}

// inflated is what deflated contents unpack to, through the archive's one
// reader of deflate. Its Reset starts it over on new contents and keeps its
// buffers, and never fails (compress/flate).
func (z *zipScan) inflated(src io.Reader) io.Reader {
	if z.inflate == nil {
		z.inflate = flate.NewReader(src)
		return z.inflate
	}
	_ = z.inflate.(flate.Resetter).Reset(src, nil)
	return z.inflate
}

// each reads the list of contents one record at a time and hands each to see,
// until the list ends or see says to stop. A list that ends in the middle of a
// record, or holds something that is not one, is noted as damage on the first
// look and passed over on the second, which reads the same bytes again.
func (z *zipScan) each(first bool, see func(zipEntry) bool) error {
	in := bufio.NewReaderSize(io.NewSectionReader(z.r, int64(z.end.listAt), int64(z.end.listSize)), 64<<10)
	for {
		if err := z.tree.ctx.Err(); err != nil {
			return err
		}
		e, size, problem, err := readListed(in)
		switch {
		case errors.Is(err, io.EOF):
			return nil
		case err != nil:
			return err
		case !problem.IsZero():
			return z.listBroken(first, problem)
		}
		if first {
			z.counted(int64(size))
		}
		if !see(e) {
			return nil
		}
	}
}

// listBroken notes a list of contents that is not one, on the first look.
func (z *zipScan) listBroken(first bool, problem core.Said) error {
	if first {
		z.noteSaid(noteDamaged, "", problem)
	}
	return nil
}

// counting tells the scan how much of the archive is read through it.
type counting struct {
	r io.Reader
	s *scan
}

func (c *counting) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.s.counted(int64(n))
	return n, err
}

// addCapped adds two sizes, holding at the most a size can be. An archive
// whose entries claim more than that in total is reported as built to explode
// either way.
func addCapped(a, b uint64) uint64 {
	if a+b < a {
		return ^uint64(0)
	}
	return a + b
}
