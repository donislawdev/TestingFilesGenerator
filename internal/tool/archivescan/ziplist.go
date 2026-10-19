package archivescan

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/core"
)

// The records a ZIP is made of, read from their bytes: the one at the end
// that says where the list of contents is, and the records of the list. What
// is done with them is in zip.go.

// The records of a ZIP, by the four bytes each begins with, and their lengths
// before the parts of variable length (APPNOTE 6.3.10, sections 4.3.7-4.3.16).
const (
	sigEnd       = 0x06054b50
	sigEnd64     = 0x06064b50
	sigLocator64 = 0x07064b50
	sigListed    = 0x02014b50
	sigLocal     = 0x04034b50

	lenEnd       = 22
	lenLocator64 = 20
	lenEnd64     = 56
	lenListed    = 46
	lenLocal     = 30
	mostComment  = 0xFFFF
)

// The methods whose contents are read.
const (
	methodStored   = 0
	methodDeflated = 8
	// methodAES is WinZip's encryption, which a ZIP marks with a method of
	// its own rather than with the encryption flag.
	methodAES = 99
)

// zipEnd is the record at the end of a ZIP that says where its list is.
type zipEnd struct {
	// at is where the record starts. The list has to end before it.
	at       uint64
	entries  uint64
	listAt   uint64
	listSize uint64
	// split is an archive that says it is one part of several.
	split bool
}

// zipEntry is one record of the list of contents.
type zipEntry struct {
	name     string
	flags    uint16
	method   uint16
	system   byte
	crc      uint32
	packed   uint64
	unpacked uint64
	at       uint64
	external uint32
}

// errNoEnd is a file that begins as a ZIP and has no record at its end saying
// where its list of contents is.
var errNoEnd = core.Defect(errors.New("a ZIP without an end record was not noted as damaged"))

func le16(b []byte) uint16 { return binary.LittleEndian.Uint16(b) }
func le32(b []byte) uint32 { return binary.LittleEndian.Uint32(b) }
func le64(b []byte) uint64 { return binary.LittleEndian.Uint64(b) }

// findEnd reads the record at the end of a ZIP. The comment after it is up to
// 64 KiB long, so the record is searched for from the end, and taken only
// where its comment ends exactly where the file does - a record found inside
// somebody's comment would point at a list of their choosing.
func findEnd(r io.ReaderAt, size int64) (zipEnd, error) {
	tail := min(size, lenEnd+mostComment)
	buf := make([]byte, tail)
	if _, err := r.ReadAt(buf, size-tail); err != nil && !errors.Is(err, io.EOF) {
		return zipEnd{}, err
	}
	for i := len(buf) - lenEnd; i >= 0; i-- {
		rec := buf[i:]
		if le32(rec) != sigEnd || i+lenEnd+int(le16(rec[20:])) != len(buf) {
			continue
		}
		e := zipEnd{
			at:       uint64(size - tail + int64(i)),
			entries:  uint64(le16(rec[10:])),
			listSize: uint64(le32(rec[12:])),
			listAt:   uint64(le32(rec[16:])),
			split:    le16(rec[4:]) != 0 || le16(rec[6:]) != 0,
		}
		if e.entries == 0xFFFF || e.listSize == 0xFFFFFFFF || e.listAt == 0xFFFFFFFF {
			return findEnd64(r, e)
		}
		return e, nil
	}
	return zipEnd{}, errNoEnd
}

// findEnd64 reads the larger record a ZIP of more than 65 535 entries or 4 GiB
// keeps before the ordinary one, through the locator just before that.
func findEnd64(r io.ReaderAt, e zipEnd) (zipEnd, error) {
	if e.at < lenLocator64+lenEnd64 {
		return zipEnd{}, errNoEnd
	}
	var loc [lenLocator64]byte
	if _, err := r.ReadAt(loc[:], int64(e.at-lenLocator64)); err != nil {
		return zipEnd{}, errNoEnd
	}
	at := le64(loc[8:])
	if le32(loc[:]) != sigLocator64 || at > e.at-lenLocator64-lenEnd64 {
		return zipEnd{}, errNoEnd
	}
	var rec [lenEnd64]byte
	if _, err := r.ReadAt(rec[:], int64(at)); err != nil || le32(rec[:]) != sigEnd64 {
		return zipEnd{}, errNoEnd
	}
	return zipEnd{
		at:       at,
		entries:  le64(rec[32:]),
		listSize: le64(rec[40:]),
		listAt:   le64(rec[48:]),
		split:    e.split || le32(loc[16:]) > 1 || le32(rec[16:]) != 0 || le32(rec[20:]) != 0,
	}, nil
}

// readListed reads one record of the list of contents, and how long it was.
// io.EOF is a list that ended between records, and a problem one that did not.
func readListed(in *bufio.Reader) (zipEntry, int, core.Said, error) {
	endsEarly := core.Says("archivescan.ListEndsEarly", "its list of contents ends in the middle of an entry")
	var head [lenListed]byte
	if _, err := io.ReadFull(in, head[:]); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return zipEntry{}, 0, endsEarly, nil
		}
		return zipEntry{}, 0, core.Said{}, err
	}
	if le32(head[:]) != sigListed {
		return zipEntry{}, 0, core.Says("archivescan.ListHoldsOther", "its list of contents holds something that is not an entry"), nil
	}
	nameLen, comment := int(le16(head[28:])), int(le16(head[32:]))
	rest := make([]byte, nameLen+int(le16(head[30:])))
	if _, err := io.ReadFull(in, rest); err != nil {
		return zipEntry{}, 0, endsEarly, nil
	}
	if _, err := in.Discard(comment); err != nil {
		return zipEntry{}, 0, endsEarly, nil
	}
	e := zipEntry{
		name: string(rest[:nameLen]), flags: le16(head[8:]), method: le16(head[10:]),
		system: head[5], crc: le32(head[16:]),
		packed: uint64(le32(head[20:])), unpacked: uint64(le32(head[24:])),
		at: uint64(le32(head[42:])), external: le32(head[38:]),
	}
	larger(&e, rest[nameLen:])
	return e, lenListed + len(rest) + comment, core.Said{}, nil
}

// larger takes the sizes and the place of an entry from its ZIP64 field,
// where the list writes the most a 32 bit number holds instead.
func larger(e *zipEntry, extra []byte) {
	for len(extra) >= 4 {
		id, size := le16(extra), int(le16(extra[2:]))
		extra = extra[4:]
		if size > len(extra) {
			return
		}
		if id == 0x0001 {
			zip64(e, extra[:size])
		}
		extra = extra[size:]
	}
}

// zip64 reads the ZIP64 field (APPNOTE 4.5.3): each value is there only when
// its field in the record holds the most 32 bits do, and in this order.
func zip64(e *zipEntry, field []byte) {
	for _, v := range []*uint64{&e.unpacked, &e.packed, &e.at} {
		if *v != 0xFFFFFFFF {
			continue
		}
		if len(field) < 8 {
			return
		}
		*v = le64(field)
		field = field[8:]
	}
}

// isFolder says whether a listed entry is a folder: a name that ends in a
// separator, or the folder bit of the system that wrote it.
func isFolder(e zipEntry) bool {
	if n := len(e.name); n > 0 && (e.name[n-1] == '/' || e.name[n-1] == '\\') {
		return true
	}
	mode, unix := unixMode(e)
	return (unix && mode&0xF000 == 0x4000) || e.external&0x10 != 0
}

// unixMode is the Unix file mode of an entry written on a Unix system, which
// keeps it in the upper half of its external attributes.
func unixMode(e zipEntry) (uint32, bool) {
	switch e.system {
	case 3, 19: // Unix, and OS X - which writes the same
		return e.external >> 16, true
	}
	return 0, false
}

// unixKind says what an entry is besides a file or a folder, from the mode a
// Unix system wrote: a link, or something that is not a plain file, with the
// words for it.
func unixKind(e zipEntry) (core.Said, bool) {
	mode, unix := unixMode(e)
	if !unix {
		return core.Said{}, false
	}
	return kindOfMode(mode)
}
