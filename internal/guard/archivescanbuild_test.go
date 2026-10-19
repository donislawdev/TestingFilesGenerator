package guard

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/flate"
	"compress/gzip"
	"encoding/binary"
	"hash/crc32"
	"io"
	"os"
	"testing"
)

// The archives the guards of the archive scanner look into, built here rather
// than kept as files: each is the smallest that holds one trap, and a builder
// says what it holds where a file of bytes would not.

// zipPart is one entry of a ZIP built with archive/zip.
type zipPart struct {
	name string
	body string
	// mode is the Unix mode written with the entry, when there is one - a
	// link is a ZIP entry whose mode says so and whose contents are its target.
	mode os.FileMode
	// raw is an entry written as it is, header and all, for the entries
	// archive/zip would not write: a size that lies, a checksum that is wrong.
	raw *zip.FileHeader
}

// zipOf is a ZIP holding these entries.
func zipOf(t *testing.T, parts ...zipPart) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, p := range parts {
		writeZipPart(t, w, p)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// writeZipPart writes one entry of zipOf.
func writeZipPart(t *testing.T, w *zip.Writer, p zipPart) {
	t.Helper()
	if p.raw != nil {
		out, err := w.CreateRaw(p.raw)
		if err == nil {
			_, err = io.WriteString(out, p.body)
		}
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	h := &zip.FileHeader{Name: p.name, Method: zip.Deflate}
	if p.mode != 0 {
		h.SetMode(p.mode)
	}
	out, err := w.CreateHeader(h)
	if err == nil {
		_, err = io.WriteString(out, p.body)
	}
	if err != nil {
		t.Fatal(err)
	}
}

// deflated is text through deflate, for an entry written raw.
func deflated(t *testing.T, text string) string {
	t.Helper()
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, flate.BestCompression)
	if err == nil {
		_, err = io.WriteString(w, text)
	}
	if err == nil {
		err = w.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

// tarPart is one entry of a TAR.
type tarPart struct {
	name string
	typ  byte
	link string
	body string
	mode int64
}

// tarOf is a TAR holding these entries, through gzip when asked.
func tarOf(t *testing.T, gzipped bool, parts ...tarPart) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, p := range parts {
		h := &tar.Header{Name: p.name, Typeflag: p.typ, Linkname: p.link, Size: int64(len(p.body)), Mode: 0o644}
		if p.mode != 0 {
			h.Mode = p.mode
		}
		if p.typ == 0 {
			h.Typeflag = tar.TypeReg
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(tw, p.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if !gzipped {
		return buf.Bytes()
	}
	return gzipOf(t, buf.Bytes())
}

// gzipOf is bytes through gzip.
func gzipOf(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if err == nil {
		_, err = w.Write(data)
	}
	if err == nil {
		err = w.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// zerosMembers is gzip that unpacks to a little over a gigabyte of zeros from
// about a megabyte: one member of 64 MiB, repeated, which gzip reads as one
// stream. Built in a fraction of a second where compressing a gigabyte would
// take several.
func zerosMembers(t *testing.T, in []byte) []byte {
	t.Helper()
	member := gzipOf(t, make([]byte, 64<<20))
	out := append([]byte(nil), in...)
	for range 17 {
		out = append(out, member...)
	}
	return out
}

// tarGzBomb is a TAR.GZ whose one entry says it holds 2 GiB of zeros, and
// does, through members of zeros after its header.
func tarGzBomb(t *testing.T) []byte {
	t.Helper()
	var head bytes.Buffer
	if err := tar.NewWriter(&head).WriteHeader(&tar.Header{Name: "big", Typeflag: tar.TypeReg, Size: 2 << 30, Mode: 0o644}); err != nil {
		t.Fatal(err)
	}
	return zerosMembers(t, gzipOf(t, head.Bytes()))
}

// handZip is a ZIP of one stored entry written byte by byte, which lists the
// entry under listed and writes it under local at the entry itself - and,
// when twice, lists a second entry starting at the same header, so two
// entries share their bytes.
func handZip(listed, local string, twice bool) []byte {
	data := []byte("shared bytes")
	sum := crc32.ChecksumIEEE(data)
	var out bytes.Buffer
	le := func(vs ...any) {
		for _, v := range vs {
			_ = binary.Write(&out, binary.LittleEndian, v)
		}
	}
	le(uint32(0x04034b50), uint16(20), uint16(0), uint16(0), uint16(0), uint16(0),
		sum, uint32(len(data)), uint32(len(data)), uint16(len(local)), uint16(0))
	out.WriteString(local)
	out.Write(data)
	listAt := out.Len()
	names := []string{listed}
	if twice {
		names = append(names, "second")
	}
	for _, name := range names {
		le(uint32(0x02014b50), uint16(20), uint16(20), uint16(0), uint16(0), uint16(0), uint16(0),
			sum, uint32(len(data)), uint32(len(data)), uint16(len(name)), uint16(0), uint16(0),
			uint16(0), uint16(0), uint32(0), uint32(0))
		out.WriteString(name)
	}
	listSize := out.Len() - listAt
	le(uint32(0x06054b50), uint16(0), uint16(0), uint16(len(names)), uint16(len(names)),
		uint32(listSize), uint32(listAt), uint16(0))
	return out.Bytes()
}
