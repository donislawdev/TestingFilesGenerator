package guard

import (
	"archive/tar"
	"bytes"
	"compress/flate"
	"context"
	"encoding/binary"
	"io"
	"testing"

	"github.com/donislawdev/TestingFilesGenerator/internal/format"
	_ "github.com/donislawdev/TestingFilesGenerator/internal/format/all"
	"github.com/donislawdev/TestingFilesGenerator/internal/format/archive"
)

// What the archive says about a file is what a reader of it finds.
//
// The settings are worth having because the interesting cases are the ones
// nobody creates by accident. 000 is a file nothing can read after unpacking,
// 777 is one a scanner should have something to say about, and an archive
// claiming root owns everything is what a careless extractor turns into a
// privilege problem. Producing one of those by hand needs a machine with the
// right permissions - producing one here is a flag.
//
// Read back with the same tar library that wrote it, so this says the values
// reached the header and not that another implementation agrees about them. GNU
// tar was run by hand on the same archives and shows "-rwxr-xr-x root/root"
// where this expects it, but that is a note rather than a gate. The honest
// limit of this guard is that it is one implementation talking to itself.
func TestWhatAnArchiveSaysAboutAFileIsWhatAReaderFinds(t *testing.T) {
	cases := []struct {
		mode      string
		owner     string
		wantMode  int64
		wantUID   int
		wantUname string
	}{
		{"644", archive.OwnerUnset, 0o644, 0, ""},
		{"000", archive.OwnerUnset, 0, 0, ""},
		{"777", archive.OwnerRoot, 0o777, 0, "root"},
		{"755", archive.OwnerUser, 0o755, 1000, "user"},
	}

	for _, c := range cases {
		t.Run(c.mode+"-"+c.owner, func(t *testing.T) {
			h := firstTarHeader(t, buildTargz(t, 12*1024, map[string]string{
				archive.EntryMode: c.mode, archive.EntryOwner: c.owner,
			}))
			if h.Mode != c.wantMode {
				t.Errorf("the archive records mode %#o and %s was asked for", h.Mode, c.mode)
			}
			if h.Uid != c.wantUID {
				t.Errorf("the archive records uid %d and owner %s means %d", h.Uid, c.owner, c.wantUID)
			}
			if h.Uname != c.wantUname {
				t.Errorf("the archive records the owner as %q and %s means %q", h.Uname, c.owner, c.wantUname)
			}
		})
	}
}

// Saying who owns a file costs nothing, and that is why these settings exist at
// all.
//
// Every field of a USTAR header is fixed width, so the mode and the owner are
// written into space that is already paid for whatever they say. If that were
// not true these would collide with the exact size the format promises, the way
// compression does, and they would be a different kind of setting entirely.
func TestRecordingAnOwnerCostsNoBytes(t *testing.T) {
	const size = 12 * 1024
	plain := buildTargz(t, size, nil)

	for _, props := range []map[string]string{
		{archive.EntryMode: "000"},
		{archive.EntryMode: "777"},
		{archive.EntryOwner: archive.OwnerRoot},
		{archive.EntryOwner: archive.OwnerUser},
		{archive.EntryMode: "700", archive.EntryOwner: archive.OwnerUser},
	} {
		got := buildTargz(t, size, props)
		if len(got) != len(plain) {
			t.Errorf("%v changed the archive from %d B to %d B", props, len(plain), len(got))
		}
		if int64(len(got)) != int64(size) {
			t.Errorf("%v: the archive is %d B and %d B was asked for", props, len(got), size)
		}
	}
}

// The default is what this tool has always written, to the byte.
//
// Not a preference. A default that recorded an owner would move the bytes of
// every archive already produced, which is untouchable rule 3 - and it would do
// it silently, because an archive with an owner in it is just as valid as one
// without.
func TestTheDefaultOwnershipIsWhatWasAlwaysWritten(t *testing.T) {
	never := buildTargz(t, 12*1024, nil)
	stated := buildTargz(t, 12*1024, map[string]string{
		archive.EntryMode: "644", archive.EntryOwner: archive.OwnerUnset,
	})
	if !bytes.Equal(never, stated) {
		t.Error("stating the defaults gives a different archive from leaving them alone, " +
			"so one of the two is not the default")
	}

	h := firstTarHeader(t, never)
	if h.Mode != 0o644 {
		t.Errorf("an archive with nothing said records mode %#o, and this format has always written 644", h.Mode)
	}
	if h.Uname != "" || h.Uid != 0 {
		t.Errorf("an archive with nothing said names an owner: uid %d, %q", h.Uid, h.Uname)
	}
}

// buildTargz makes one archive in memory.
func buildTargz(t *testing.T, size int64, props map[string]string) []byte {
	t.Helper()
	d := descriptorFor(t, "targz")
	plan, err := d.Generator.Plan(format.Request{Bytes: size, Seed: 7741, Label: true, Properties: props})
	if err != nil {
		t.Fatalf("planning %d B with %v: %v", size, props, err)
	}
	var buf bytes.Buffer
	if err := d.Generator.Write(context.Background(), &buf, plan); err != nil {
		t.Fatalf("writing %d B with %v: %v", size, props, err)
	}
	return buf.Bytes()
}

// firstTarHeader is the header of the first entry inside a gzipped tar.
//
// The gzip header is stepped over by hand rather than handed to
// compress/gzip, and the reason is a finding rather than a preference.
// Measured on 2026-09-01: Go's own gzip reader accepts a header comment of 511
// bytes and refuses one of 512, because it reads the field into a fixed buffer.
// This format pads through that comment, up to four kilobytes of it, so 318 of
// 866 archive sizes tried came back unreadable to compress/gzip - every one of
// them fine to 7-Zip, GNU tar and bsdtar. Written up in OBSERVATIONS.md.
//
// So this guard reads past the header itself. Leaning on the standard library
// here would have made it fail on sizes that are not the subject, and hidden
// the finding behind a test about permissions.
func firstTarHeader(t *testing.T, archiveBytes []byte) *tar.Header {
	t.Helper()
	h, err := tar.NewReader(flate.NewReader(bytes.NewReader(pastGzipHeader(t, archiveBytes)))).Next()
	if err == io.EOF {
		t.Fatal("the archive holds no entries, so there is no header to read")
	}
	if err != nil {
		t.Fatalf("reading the first entry: %v", err)
	}
	return h
}

// pastGzipHeader is everything after the gzip header, so the deflate stream can
// be handed to a reader that has no opinion about how long a comment may be.
func pastGzipHeader(t *testing.T, b []byte) []byte {
	t.Helper()
	const fixed = 10
	if len(b) < fixed || b[0] != 0x1f || b[1] != 0x8b {
		t.Fatalf("this is not a gzip stream: % x", b[:minInt(4, len(b))])
	}
	flags := b[3]
	at := fixed

	if flags&0x04 != 0 { // FEXTRA, a two byte length and then that many bytes
		if at+2 > len(b) {
			t.Fatal("the extra field runs off the end of the header")
		}
		at += 2 + int(binary.LittleEndian.Uint16(b[at:]))
	}
	// FNAME and FCOMMENT are each a run of bytes ending in a zero.
	for _, flag := range []byte{0x08, 0x10} {
		if flags&flag == 0 {
			continue
		}
		end := bytes.IndexByte(b[at:], 0)
		if end < 0 {
			t.Fatal("a header string never ends")
		}
		at += end + 1
	}
	if flags&0x02 != 0 { // FHCRC
		at += 2
	}
	if at > len(b) {
		t.Fatal("the header runs off the end of the archive")
	}
	return b[at:]
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
