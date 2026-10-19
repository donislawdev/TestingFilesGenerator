package archivescan

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// Reading a TAR archive, bare or inside gzip, and a gzip file that holds one
// file. A TAR has no list of contents: each entry is a header and its bytes,
// one after another, so the only way to the next name is through the bytes
// before it. That is why an archive built to explode is caught while it is
// read rather than from a list, and reading stops there.

// stream is the reading of one TAR or gzip archive, front to back.
type stream struct {
	*scan
}

// tar reads a TAR archive from its first byte.
func (s *scan) tar(r io.Reader) error {
	if err := (stream{s}).walk(&counting{r: r, s: s}); err != nil {
		return err
	}
	s.weigh()
	return nil
}

// gzip reads a gzip file, and a TAR in it when there is one.
//
// What the gzip stream unpacks to is counted on its own, below any TAR: that
// is what gunzip writes before anything looks inside, so an archive built to
// explode at that layer is one whatever the TAR in it says - and a stream of
// zeros, which a TAR reader takes for an empty TAR, is caught there.
func (s *scan) gzip(r io.Reader) error {
	s.kind = kindGzip
	z, err := gzip.NewReader(&counting{r: r, s: s})
	if err != nil {
		if failedToRead(err) {
			return err
		}
		s.noteSaid(noteDamaged, "", core.Says("archivescan.GzipHeader", "its gzip header is broken"))
		return nil
	}
	in := bufio.NewReaderSize(&inflating{r: z, s: s}, 64<<10)
	head, err := in.Peek(headLength)
	if err != nil && failedToRead(err) {
		return err
	}
	if err := (stream{s}).gzipped(in, head); err != nil {
		return err
	}
	s.weigh()
	return nil
}

// gzipped reads what a gzip stream holds: a TAR, told by its magic and not by
// a block of zeros - inside gzip that is the start of any stream of zeros -
// or one file.
func (st stream) gzipped(in io.Reader, head []byte) error {
	if len(head) < 257+len(tarMagic) || !bytes.Equal(head[257:257+len(tarMagic)], tarMagic) {
		return st.one(in)
	}
	st.kind = kindTarGz
	if err := st.walk(in); err != nil || st.stopped {
		return err
	}
	// What follows the end of the TAR, which a TAR program never reads and
	// gunzip writes all the same.
	_, _, err := drain(st.ctx, st.buf, in, ^uint64(0), nil, nil)
	return st.ended(err, "")
}

// one reads a gzip file that holds one file. Its name is not judged: gunzip
// names what it unpacks after the archive, not after the name inside.
func (st stream) one(in io.Reader) error {
	st.scan.entries = 1
	_, head, err := drain(st.ctx, st.buf, in, ^uint64(0), nil, st.unpacking)
	if err := st.ended(err, ""); err != nil || st.stopped {
		return err
	}
	if archiveIn([]byte(head)) != "" {
		st.note(noteNested, "")
	}
	return nil
}

// walk reads the entries of a TAR one after another.
func (st stream) walk(r io.Reader) error {
	tr := tar.NewReader(r)
	for !st.stopped {
		h, err := st.next(tr)
		if err != nil || h == nil {
			return err
		}
		if h.Typeflag == tar.TypeXGlobalHeader {
			st.global(h)
			continue
		}
		if !st.entry(h.Name, h.Typeflag == tar.TypeDir) {
			return nil
		}
		st.kindOf(h)
		if h.Size <= 0 {
			continue
		}
		_, head, err := drain(st.ctx, st.buf, tr, uint64(h.Size), nil, st.unpacking)
		if err := st.ended(err, h.Name); err != nil {
			return err
		}
		if archiveIn([]byte(head)) != "" {
			st.note(noteNested, h.Name)
		}
	}
	return nil
}

// next is the header of the next entry, or nothing at the end of the archive
// or where a broken one ends the reading.
func (st stream) next(tr *tar.Reader) (*tar.Header, error) {
	if err := st.ctx.Err(); err != nil {
		return nil, err
	}
	h, err := tr.Next()
	switch {
	case err == nil:
		return h, nil
	case errors.Is(err, io.EOF):
		return nil, nil
	case st.exploded || failedToRead(err):
		return nil, st.ended(err, "")
	}
	st.noteSaid(noteDamaged, "", core.Says("archivescan.TarHeader", "a header is broken, or it ends in the middle of one"))
	return nil, nil
}

// kindOf notes an entry that is not a plain file or folder.
func (st stream) kindOf(h *tar.Header) {
	switch h.Typeflag {
	case tar.TypeSymlink, tar.TypeLink:
		st.noteSaid(noteLinks, h.Name, linked(h.Name, h.Linkname))
		return
	case tar.TypeChar, tar.TypeBlock:
		st.special(h.Name, core.Says("archivescan.Device", "it is a device"))
		return
	case tar.TypeFifo:
		st.special(h.Name, core.Says("archivescan.Pipe", "it is a pipe"))
		return
	case tar.TypeReg, tar.TypeDir, tar.TypeGNUSparse, tar.TypeCont, '\x00':
	default:
		st.special(h.Name, core.Says("archivescan.Type", "it is an entry of type %q, which most programs do not unpack", core.A("Type", string(h.Typeflag))))
		return
	}
	if h.Typeflag != tar.TypeDir && h.Mode&(modeSetUser|modeSetGroup) != 0 {
		st.special(h.Name, core.Says("archivescan.Rights", "it runs with the rights of its owner"))
	}
}

// global looks at a PAX global header, which is not an entry: it gives values
// to every entry after it, and git archive writes one at the front of every
// TAR to hold the commit, as a comment. One setting anything that changes what
// an entry is - its name, its link, its size - changes entries in a way this
// tool does not follow, and is noted. Not under its name: archive/tar hands
// the header back named by the path it sets.
func (st stream) global(h *tar.Header) {
	first := ""
	for key := range h.PAXRecords {
		if !globalHarmless[key] && (first == "" || key < first) {
			first = key
		}
	}
	if first != "" {
		st.noteSaid(noteSpecial, "", core.Says("archivescan.Global", "a header sets %s for every entry after it, which this tool does not follow",
			core.A("Key", core.Shown(first))))
	}
}

// globalHarmless are the values a PAX global header may give the entries
// after it and leave what they are alone: words, times and owners.
var globalHarmless = map[string]bool{
	"comment": true, "charset": true, "hdrcharset": true,
	"atime": true, "mtime": true, "ctime": true,
	"uid": true, "gid": true, "uname": true, "gname": true,
}

// unpacking is told every piece of contents read from a stream, and says
// whether to go on: not once what it unpacks to is an archive built to explode.
// Contents and not only a gzip layer, because a sparse entry in a bare TAR
// unpacks to its holes too.
func (st stream) unpacking(n int) bool {
	st.unpacked = addCapped(st.unpacked, uint64(n))
	return !st.explodes(st.unpacked)
}

// ended sorts the end of reading one entry of a stream: a stop and a disk that
// would not read are errors, and contents that end early or do not unpack are
// damage, after which nothing more is read.
func (st stream) ended(err error, name string) error {
	switch {
	case err == nil, st.exploded:
		return nil
	case st.ctx.Err() != nil:
		return st.ctx.Err()
	case failedToRead(err):
		return err
	}
	words := core.Says("archivescan.StreamBroken", "it ends early, or its contents do not unpack")
	if name != "" {
		words = entryWhy(name, words)
	}
	st.noteSaid(noteDamaged, name, words)
	st.stopped = true
	return nil
}

// explodes says whether having unpacked this much from what was read of the
// archive to get there is an archive built to explode - a gigabyte, and more
// than a hundred times what was read - and notes it the first time. Reading
// stops there: going on is what such an archive is built to make a program do.
func (s *scan) explodes(unpacked uint64) bool {
	if s.exploded {
		return true
	}
	if unpacked < BombBytes || unpacked <= uint64(s.read)*BombRatio {
		return false
	}
	s.exploded = true
	s.noteSaid(noteBomb, "", core.Says("archivescan.Bomb", "%s from %s", core.A("Unpacked", bytesOf(unpacked)), core.A("Size", core.Bytes(s.read))))
	s.stop(core.Says("archivescan.StoppedBomb", "it unpacks to more than %d times what was read of it, and no more is read", core.A("Ratio", BombRatio)))
	return true
}

// errExploded unwinds a reader once the archive has been found built to
// explode, from inside the gzip layer the TAR reader reads through.
var errExploded = core.Defect(errors.New("reading an archive built to explode did not stop where it was found"))

// inflating counts what a gzip stream unpacks to.
type inflating struct {
	r   io.Reader
	s   *scan
	out uint64
}

func (i *inflating) Read(p []byte) (int, error) {
	n, err := i.r.Read(p)
	i.out = addCapped(i.out, uint64(n))
	if i.s.explodes(i.out) {
		return n, errExploded
	}
	return n, err
}

// failedToRead says whether an error is the disk's rather than the archive's.
func failedToRead(err error) bool {
	var path *os.PathError
	return errors.As(err, &path)
}

// The mode bits that make a program run with the rights of its owner or its
// group, in the octal a Unix mode is written in. On a folder they say whose
// the files made in it are, and run nothing.
const (
	modeSetUser  = 0o4000
	modeSetGroup = 0o2000
)

// modeFolder is the type a Unix mode gives a folder.
const modeFolder = 0x4000

// kindOfMode is what a Unix mode says an entry is besides a file or a folder:
// a link, or the words for something else. Nothing for a plain file or folder.
func kindOfMode(mode uint32) (core.Said, bool) {
	switch mode & 0xF000 {
	case 0xA000:
		return core.Said{}, true
	case 0x1000:
		return core.Says("archivescan.Pipe", "it is a pipe"), false
	case 0x2000, 0x6000:
		return core.Says("archivescan.Device", "it is a device"), false
	case 0xC000:
		return core.Says("archivescan.Socket", "it is a socket"), false
	}
	if mode&0xF000 != modeFolder && mode&(modeSetUser|modeSetGroup) != 0 {
		return core.Says("archivescan.Rights", "it runs with the rights of its owner"), false
	}
	return core.Said{}, false
}

// linked is a link in words: its name and where it points, when known.
func linked(name, target string) core.Said {
	if target == "" {
		return core.Says("archivescan.Name", "%s", core.A("Name", core.Shown(name)))
	}
	return core.Says("archivescan.Link", "%s -> %s", core.A("Name", core.Shown(name)), core.A("Target", core.Shown(target)))
}

// keepHead is how much of an entry's contents is kept: enough to tell an
// archive inside it, and the whole target of a link, which a system keeps to
// 4096 bytes.
const keepHead = 4096

// drain reads r through sum, when there is one, until it ends or one byte past
// most - an entry holding more than it was said to is found without reading
// the rest of it - and hands back how much it read and the first bytes. After
// every piece it asks more, when there is one, whether to go on. It reads
// into buf, one for the whole scan: an archive can hold a million entries.
func drain(ctx context.Context, buf []byte, r io.Reader, most uint64, sum io.Writer, more func(int) bool) (uint64, string, error) {
	var got uint64
	var head []byte
	for {
		if err := ctx.Err(); err != nil {
			return got, string(head), err
		}
		n, err := r.Read(buf[:wanted(len(buf), most, got)])
		got += uint64(n)
		head = append(head, buf[:min(n, keepHead-len(head))]...)
		if sum != nil {
			_, _ = sum.Write(buf[:n])
		}
		if n > 0 && more != nil && !more(n) {
			return got, string(head), nil
		}
		if got > most || errors.Is(err, io.EOF) {
			return got, string(head), nil
		}
		if err != nil {
			return got, string(head), err
		}
	}
}

// wanted is how much to read next: a buffer, or what is left to one byte past
// most when that is less. Most of all bits is no bound.
func wanted(buffer int, most, got uint64) int {
	if most == ^uint64(0) || most+1-got >= uint64(buffer) {
		return buffer
	}
	return int(most + 1 - got)
}
