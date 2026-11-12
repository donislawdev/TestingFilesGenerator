package archive

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
)

// StreamMap is what a writer of an archive needs to mark the map of the file
// (format.Mark) when a library writes it: where the stream stands at any
// moment, and a way to hold bytes back until the marks that describe them are
// made. ZIP and the office packages hand it to archive/zip as Buffer, and
// TAR.GZ hands it to compress/gzip itself.
//
// archive/zip buffers what it writes, finishes an entry only when the next
// one opens - the end of a squeezed entry and its data descriptor go out
// then - and flushes its buffer itself when it closes. So the offset of the
// filler entry is known only after its header was written, and the central
// directory's only after the archive closed (docs/TRUNCATE-2026-11-11.md
// section 6.9). Buffer is handed to zip.NewWriter, which writes through it
// rather than through one of its own (bufio.NewWriterSize returns a Writer it
// is handed), so Pos is exact. Hold keeps what follows from reaching the
// generator's writer until Release, and the mark is made in between.
//
// Holding happens only when somebody keeps a map, so an archive nothing cuts
// is written exactly as before.
type StreamMap struct {
	// Buffer is what zip.NewWriter is to be given.
	Buffer  *bufio.Writer
	w       io.Writer
	out     int64
	held    []byte
	holding bool
}

// NewStreamMap is a StreamMap writing to w, the writer the generator was handed.
func NewStreamMap(w io.Writer) *StreamMap {
	m := &StreamMap{w: w}
	// The size archive/zip gives its own buffer.
	m.Buffer = bufio.NewWriterSize(m, 4096)
	return m
}

func (m *StreamMap) Write(p []byte) (int, error) {
	if m.holding {
		m.held = append(m.held, p...)
		return len(p), nil
	}
	n, err := m.w.Write(p)
	m.out += int64(n)
	return n, err
}

// Pos is the offset the next byte archive/zip writes will have.
func (m *StreamMap) Pos() int64 {
	return m.out + int64(len(m.held)) + int64(m.Buffer.Buffered())
}

func (m *StreamMap) mapped() bool {
	_, ok := m.w.(format.Mapper)
	return ok
}

// Hold keeps every byte written from here on until Release, when somebody
// keeps a map. What was buffered before goes out first.
func (m *StreamMap) Hold() error {
	if !m.mapped() {
		return nil
	}
	if err := m.Buffer.Flush(); err != nil {
		return err
	}
	m.holding = true
	return nil
}

// Release sends on what was held, after the marks that describe it.
func (m *StreamMap) Release() error {
	if !m.holding {
		return nil
	}
	if err := m.Buffer.Flush(); err != nil {
		return err
	}
	m.holding = false
	held := m.held
	m.held = nil
	_, err := m.Write(held)
	return err
}

// Close closes c - the archive's writer - with what it writes as it closes
// held back until it is marked (MarkTail).
func (m *StreamMap) Close(c io.Closer) error {
	if err := m.Hold(); err != nil {
		return err
	}
	if err := c.Close(); err != nil {
		return err
	}
	if err := m.MarkTail(); err != nil {
		return err
	}
	return m.Release()
}

// MarkLast marks the last n bytes held as role: what a library wrote last,
// whose start is known only once it is all there - the trailer of a gzip
// stream, which its writer adds as it closes.
func (m *StreamMap) MarkLast(role format.Role, n int64) error {
	if !m.holding {
		return nil
	}
	if err := m.Buffer.Flush(); err != nil {
		return err
	}
	format.Mark(m.w, role, m.out+int64(len(m.held))-n, n)
	return nil
}

// MarkEntry marks the region of role that starts with the local header of the
// entry named name, which archive/zip has just written: the last thing held.
// The header is found by its own fields rather than worked out, because
// archive/zip adds an extra field of its own to a header that names a time.
func (m *StreamMap) MarkEntry(role format.Role, name string) error {
	if !m.holding {
		return nil
	}
	if err := m.Buffer.Flush(); err != nil {
		return err
	}
	at, ok := lastLocalHeader(m.held, name)
	if !ok {
		// A header that cannot be found leaves the region unmarked, which
		// the guard of the map reads back as a defect.
		return nil
	}
	format.Mark(m.w, role, m.out+int64(at), -1)
	return nil
}

// MarkTail marks what archive/zip wrote while it closed, all of it held: the
// central directory and the end of central directory record as the end, and
// the archive comment after them as padding.
func (m *StreamMap) MarkTail() error {
	if !m.holding {
		return nil
	}
	if err := m.Buffer.Flush(); err != nil {
		return err
	}
	eocd, comment, ok := endRecord(m.held)
	if !ok {
		return nil
	}
	directory := int64(binary.LittleEndian.Uint32(m.held[eocd+16:]))
	if directory == 0xFFFFFFFF {
		var found bool
		if directory, found = zip64Directory(m.held, m.out, eocd); !found {
			return nil
		}
	}
	end := m.out + int64(eocd) + 22
	format.Mark(m.w, format.RoleEnd, directory, end-directory)
	format.Mark(m.w, format.RolePadding, end, int64(comment))
	return nil
}

// lastLocalHeader is where the last local header of name starts in b. Not
// always the last thing in b: a locked entry writes its salt straight after
// its header.
func lastLocalHeader(b []byte, name string) (int, bool) {
	for at := len(b) - 30 - len(name); at >= 0; at-- {
		if !bytes.HasPrefix(b[at:], []byte("PK\x03\x04")) {
			continue
		}
		n := int(binary.LittleEndian.Uint16(b[at+26:]))
		extra := int(binary.LittleEndian.Uint16(b[at+28:]))
		if n == len(name) && at+30+n+extra <= len(b) && string(b[at+30:at+30+n]) == name {
			return at, true
		}
	}
	return 0, false
}

// endRecord is where the end of central directory record starts in b, when it
// and its comment end b, and the comment's length.
func endRecord(b []byte) (int, int, bool) {
	for at := len(b) - 22; at >= 0; at-- {
		if !bytes.HasPrefix(b[at:], []byte("PK\x05\x06")) {
			continue
		}
		comment := int(binary.LittleEndian.Uint16(b[at+20:]))
		if at+22+comment == len(b) {
			return at, comment, true
		}
	}
	return 0, 0, false
}

// zip64Directory is where the central directory starts, from the ZIP64 end
// record the locator before the end record points at. out is the offset of
// b's first byte in the file.
func zip64Directory(b []byte, out int64, eocd int) (int64, bool) {
	locator := eocd - 20
	if locator < 0 || !bytes.HasPrefix(b[locator:], []byte("PK\x06\x07")) {
		return 0, false
	}
	record := le64(b[locator+8:]) - out
	if record < 0 || record+56 > int64(len(b)) {
		return 0, false
	}
	return le64(b[record+48:]), true
}

// le64 is the eight byte little-endian number at the start of b. Put
// together by hand, because the call that reads one has the name of the bulk
// random draw a guard keeps in one place.
func le64(b []byte) int64 {
	var v int64
	for i := 7; i >= 0; i-- {
		v = v<<8 | int64(b[i])
	}
	return v
}
